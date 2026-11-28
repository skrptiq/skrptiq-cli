package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/skrptiq/engine/manifest"
	"github.com/skrptiq/engine/parse"
)

// Lint handles `skrptiq lint <dir> [--auto-fix]`.
// Surfaces fixable identity issues; --auto-fix rewrites in place.
func Lint(args []string) int {
	fs := flag.NewFlagSet("lint", flag.ContinueOnError)
	autoFix := fs.Bool("auto-fix", false, "Rewrite fixable issues in place")
	if err := fs.Parse(args); err != nil {
		return ExitBadArgs
	}

	dir := fs.Arg(0)
	if dir == "" {
		fmt.Fprintln(os.Stderr, "Usage: skrptiq lint <dir> [--auto-fix]")
		return ExitBadArgs
	}

	pkg, issues, err := parse.ReadPackage(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return ExitFailed
	}

	if *autoFix {
		return lintAutoFix(dir, pkg, issues)
	}

	return lintReport(issues)
}

// lintReport prints issues and returns the appropriate exit code.
func lintReport(issues []manifest.ParseIssue) int {
	if len(issues) == 0 {
		fmt.Println("No issues found.")
		return ExitOK
	}

	var errors, warnings int
	for _, issue := range issues {
		label := severityLabel(issue.Severity)
		fmt.Printf("  %s  %s  %s\n", label, issue.Code, issue.Message)
		if issue.Suggestion != "" {
			fmt.Printf("         suggestion: %s\n", issue.Suggestion)
		}
		switch issue.Severity {
		case manifest.SeverityError:
			errors++
		case manifest.SeverityWarn:
			warnings++
		}
	}

	if errors > 0 {
		return ExitFailed
	}
	if warnings > 0 {
		return 1
	}
	return ExitOK
}

// lintAutoFix applies MigrateIdentity to the manifest and rewrites.
func lintAutoFix(dir string, pkg parse.Package, issues []manifest.ParseIssue) int {
	if pkg.Manifest.ID == "" {
		return lintReport(issues)
	}

	canonical, migIssue, err := manifest.MigrateIdentity(pkg.Manifest.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot auto-fix id: %v\n", err)
		return ExitFailed
	}

	if canonical == pkg.Manifest.ID {
		fmt.Println("manifest.id is already canonical — no changes needed.")
		return lintReport(issues)
	}

	// Rewrite skrptiq.yaml: read, replace the old ID, write back.
	yamlPath := dir + "/skrptiq.yaml"
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return ExitFailed
	}

	// Simple string replacement — the ID appears as `id: <value>` in YAML.
	updated := replaceYAMLField(string(data), "id", canonical)
	if err := os.WriteFile(yamlPath, []byte(updated), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return ExitFailed
	}

	if migIssue != nil {
		fmt.Printf("  Fixed: %s\n", migIssue.Message)
		if migIssue.Suggestion != "" {
			fmt.Printf("  Rewritten to: %s\n", migIssue.Suggestion)
		}
	}
	fmt.Printf("  manifest.id updated to %s\n", canonical)
	return ExitOK
}

func severityLabel(s manifest.Severity) string {
	switch s {
	case manifest.SeverityError:
		return "ERROR  "
	case manifest.SeverityWarn:
		return "WARNING"
	case manifest.SeverityInfo:
		return "INFO   "
	default:
		return string(s)
	}
}

// replaceYAMLField rewrites the FIRST line whose space/tab-trimmed text begins
// `<field>:`, and returns the document unchanged if no line does.
//
// IT IS NOT RESTRICTED TO A TOP-LEVEL FIELD, to a mapping key, or to YAML
// structure at all, and this comment used to claim it was (GH#1359). It matches a
// nested mapping key, and a line of prose inside a block scalar, before it reaches
// a top-level key further down the file — so on such a manifest it rewrites the
// wrong line while the caller reports the right one. A list item (`- id:`) is NOT
// matched, because the trimmed line still begins `- `.
//
// GH#1331 dimension 2 has the cases and the end-to-end runs. The BEHAVIOUR is
// unfixed and out of scope here; what is fixed is a description that made the
// defect hard to see. Anchor the surgery to what the parser reported, not to a
// second search with weaker rules.
func replaceYAMLField(content, field, newValue string) string {
	lines := splitLines(content)
	prefix := field + ":"
	for i, line := range lines {
		trimmed := trimLeftSpace(line)
		if len(trimmed) >= len(prefix) && trimmed[:len(prefix)] == prefix {
			// Preserve leading whitespace.
			indent := line[:len(line)-len(trimmed)]
			lines[i] = indent + field + ": " + newValue
			break
		}
	}
	return joinLines(lines)
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// joinLines rejoins split lines and restores the trailing newline.
//
// `strings.Join` rather than `result += line` in a loop (GH#1359). The old shape
// reallocated and copied the whole accumulated string on every line, so it was
// quadratic in the file: 20,000 lines took 2.01s against 837µs for a single
// join, and 40,000 took 9.20s. Every manifest rewrite passes through here, so
// that was ~80% of `migrate-identity`'s wall-clock on a 1.3MB manifest.
//
// NO BENCHMARK GUARDS THIS, deliberately. A timing assertion in CI is a flake
// generator, and the property is structural rather than measured: a single join
// cannot be quadratic. The numbers above are the record.
func joinLines(lines []string) string {
	return strings.Join(lines, "\n") + "\n"
}

func trimLeftSpace(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' && s[i] != '\t' {
			return s[i:]
		}
	}
	return ""
}
