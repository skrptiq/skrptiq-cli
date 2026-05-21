// Package scan implements the headless `skrptiq scan <path>` command.
// It reads a skrpt directory from disk and runs the engine's structural
// validation against it, outputting results as JSON or a human-readable table.
package scan

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Frontmatter is the YAML structure parsed from .md file headers.
type Frontmatter struct {
	Type         string           `yaml:"type"`
	ID           string           `yaml:"id"`
	Title        string           `yaml:"title"`
	Description  string           `yaml:"description,omitempty"`
	Tags         []string         `yaml:"tags,omitempty"`
	Connections  []Connection     `yaml:"connections,omitempty"`
	Metadata     map[string]any   `yaml:"metadata,omitempty"`
	Inputs       map[string]any   `yaml:"inputs,omitempty"`
	Execution    []map[string]any `yaml:"execution,omitempty"`
	OutputStep   *string          `yaml:"output_step,omitempty"`
	OutputFormat *string          `yaml:"output_format,omitempty"`
	Loops        any              `yaml:"loops,omitempty"`
}

// Connection represents an edge declared in frontmatter.
type Connection struct {
	Target   string `yaml:"target"`
	Type     string `yaml:"type"`
	Position *int   `yaml:"position,omitempty"`
}

// ParsedFile holds a parsed .md file ready for DB loading.
type ParsedFile struct {
	Path        string
	Dir         string // e.g. "skills", "prompts"
	Frontmatter Frontmatter
	Body        string // markdown content after frontmatter
}

// nodeDirs maps directory names to their expected node types.
var nodeDirs = map[string]string{
	"skills":    "skill",
	"prompts":   "prompt",
	"workflows": "workflow",
	"sources":   "source",
	"services":  "service",
	"documents": "document",
	"assets":    "asset",
}

// ParseDirectory reads a skrpt directory and parses all node files.
// Returns an error if skrptiq.yaml is missing or unreadable.
//
// The parsed manifest map is also returned so the scanner can populate
// the temp DB's `skrpt_manifests` table — without it the engine's
// `IsTemplatePackage` check (GH#524) can't distinguish template
// packages from regular ones and the `unknown_namespace` warnings
// fire for legitimate fill-in-the-blank markers.
func ParseDirectory(path string) ([]ParsedFile, map[string]any, []ScanIssue, error) {
	manifestPath := filepath.Join(path, "skrptiq.yaml")
	if _, err := os.Stat(manifestPath); err != nil {
		return nil, nil, nil, fmt.Errorf("skrptiq.yaml not found in %s", path)
	}
	// Validate manifest is parseable YAML.
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("cannot read skrptiq.yaml: %w", err)
	}
	var manifest map[string]any
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, nil, nil, fmt.Errorf("invalid skrptiq.yaml: %w", err)
	}

	var files []ParsedFile
	var issues []ScanIssue

	for dirName, expectedType := range nodeDirs {
		dirPath := filepath.Join(path, dirName)
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			continue // directory doesn't exist — fine
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			filePath := filepath.Join(dirPath, entry.Name())
			parsed, err := ParseFile(filePath, dirName)
			if err != nil {
				issues = append(issues, ScanIssue{
					File:            relPath(path, filePath),
					ValidationIssue: makeIssue("scan.parse_error", "error", err.Error(), "§2.2", ""),
				})
				continue
			}
			// Check directory/type mismatch.
			if parsed.Frontmatter.Type != "" && parsed.Frontmatter.Type != expectedType {
				issues = append(issues, ScanIssue{
					File:     relPath(path, filePath),
					NodeSlug: parsed.Frontmatter.ID,
					ValidationIssue: makeIssue("scan.type_mismatch", "warning",
						fmt.Sprintf("file in %s/ has type %q, expected %q", dirName, parsed.Frontmatter.Type, expectedType),
						"§2.1", "type"),
				})
			}
			files = append(files, *parsed)
		}
	}

	return files, manifest, issues, nil
}

// ParseFile reads a single .md file and extracts frontmatter and body.
func ParseFile(path, dir string) (*ParsedFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := string(data)

	fm, body, err := splitFrontmatter(content)
	if err != nil {
		return nil, err
	}

	var frontmatter Frontmatter
	if err := yaml.Unmarshal([]byte(fm), &frontmatter); err != nil {
		return nil, fmt.Errorf("invalid frontmatter YAML: %w", err)
	}

	// Derive id from filename if missing (per contract §2.3).
	if frontmatter.ID == "" {
		base := filepath.Base(path)
		frontmatter.ID = strings.TrimSuffix(base, ".md")
	}

	return &ParsedFile{
		Path:        path,
		Dir:         dir,
		Frontmatter: frontmatter,
		Body:        body,
	}, nil
}

// splitFrontmatter splits content on --- delimiters.
// Returns (frontmatter YAML, body markdown, error).
func splitFrontmatter(content string) (string, string, error) {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "---") {
		return "", "", fmt.Errorf("file does not start with --- frontmatter delimiter")
	}

	// Find the closing ---.
	rest := content[3:]
	// Skip the newline after opening ---.
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	} else if len(rest) > 1 && rest[0] == '\r' && rest[1] == '\n' {
		rest = rest[2:]
	}

	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return "", "", fmt.Errorf("no closing --- frontmatter delimiter found")
	}

	fm := rest[:idx]
	body := strings.TrimSpace(rest[idx+4:])
	return fm, body, nil
}

// BuildMetadataJSON merges frontmatter fields into the metadata JSON
// column per contract §6.2.
func BuildMetadataJSON(fm Frontmatter) string {
	meta := make(map[string]any)
	// Copy direct metadata fields.
	for k, v := range fm.Metadata {
		meta[k] = v
	}
	if fm.Inputs != nil {
		meta["inputs"] = fm.Inputs
	}
	if fm.Execution != nil {
		meta["execution"] = fm.Execution
	}
	if fm.Loops != nil {
		meta["loops"] = fm.Loops
	}
	if fm.OutputStep != nil {
		meta["output_step"] = *fm.OutputStep
	}
	if fm.OutputFormat != nil {
		meta["output_format"] = *fm.OutputFormat
	}
	if len(meta) == 0 {
		return ""
	}
	// Use encoding/json for the output since it's going into a JSON column.
	// Import is in scan.go; keep this package simple.
	return marshalJSON(meta)
}

// relPath moved to bridge.go
