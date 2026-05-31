package scan

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/skrptiq/engine/parse"
	"github.com/skrptiq/engine/storage"
	"github.com/skrptiq/skrptiq-cli/internal/scan/depresolver"
)

// ScanIssue wraps a ValidationIssue with file context.
type ScanIssue struct {
	File     string `json:"file"`
	NodeSlug string `json:"nodeSlug,omitempty"`
	storage.ValidationIssue
}

// ScanResult is the complete scan output.
type ScanResult struct {
	Path       string         `json:"path"`
	NodeCount  int            `json:"nodeCount"`
	EdgeCount  int            `json:"edgeCount"`
	Issues     []ScanIssue    `json:"issues"`
	ErrorCount int            `json:"errorCount"`
	WarnCount  int            `json:"warnCount"`
	InfoCount  int            `json:"infoCount"`
	Package    *parse.Package `json:"package,omitempty"`
}

// Options controls scanner behaviour beyond path + output format.
type Options struct {
	// NoResolveDeps disables Hub-fetching of declared dependency node
	// lists (GH#630 plan §4). Default = false (strict mode, the publish
	// gate). With this flag set, workflow-execution refs not resolved
	// locally are accepted without further validation — for offline /
	// local-dev authoring only.
	NoResolveDeps bool
	// HubBaseURL overrides the Hub origin used by the dep resolver.
	// Empty → depresolver.DefaultHubBaseURL.
	HubBaseURL string
	// DepCacheDir overrides ~/.skrptiq/cache for the dep-nodes cache.
	// Empty → resolver default. Tests inject a tempdir here.
	DepCacheDir string
}

// Run executes the scan in strict mode (the publish gate), writing
// output to stdout, and returns the exit code (0 pass, 1 warnings, 2
// errors).
func Run(scanPath string, jsonOutput bool) int {
	return RunTo(scanPath, jsonOutput, os.Stdout)
}

// RunTo is the back-compat entry point with stdout redirection.
func RunTo(scanPath string, jsonOutput bool, w io.Writer) int {
	return RunWithOptions(scanPath, jsonOutput, Options{}, w)
}

// RunWithOptions executes the scan with caller-supplied options. The
// dep-aware GH#630 path lives here.
func RunWithOptions(scanPath string, jsonOutput bool, opts Options, w io.Writer) int {
	absPath, err := filepath.Abs(scanPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 2
	}

	// 1. Read the package via the canonical engine reader.
	pkg, parseIssues, err := parse.ReadPackage(absPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 2
	}

	// 2. Open a temp DB.
	tmpDir, err := os.MkdirTemp("", "skrptiq-scan-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating temp directory: %v\n", err)
		return 2
	}
	defer os.RemoveAll(tmpDir)

	db, err := storage.Open(filepath.Join(tmpDir, "scan.db"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening temp database: %v\n", err)
		return 2
	}
	defer db.Close()

	var allIssues []ScanIssue
	allIssues = append(allIssues, parseIssuesToScanIssues(parseIssues)...)

	// 3. Dep resolution (GH#630). Runs BEFORE the bridge so that
	// connection edges to dep-provided slugs can be filtered out at
	// bridge time (plan v2 §1). Only active when the package declares
	// a `dependencies:` block (pkg.Dependencies != nil); legacy bundles
	// bypass this entirely and keep historical codes verbatim.
	depProvided := map[string]string{}
	hasDepsBlock := pkg.Dependencies != nil
	manifestRel := manifestRelPath(absPath, pkg)
	if hasDepsBlock && !opts.NoResolveDeps {
		resolver, rErr := depresolver.New(depresolver.Config{
			HubBaseURL: opts.HubBaseURL,
			CacheDir:   opts.DepCacheDir,
		})
		if rErr != nil {
			fmt.Fprintf(os.Stderr, "Error initialising dep resolver: %v\n", rErr)
			return 2
		}
		result := resolver.Resolve(pkg.Dependencies)
		depProvided = result.ProvidedSlugs
		for _, issue := range result.Issues {
			allIssues = append(allIssues, ScanIssue{
				File:            manifestRel,
				ValidationIssue: issue,
			})
		}
	}

	// 4. Convert parse.Package → storage.HydrationInput via bridge.
	// The bridge consults depProvided when validating connection edge
	// targets so dep-provided slugs neither error nor get added to the
	// temp DB (which has no dep nodes hydrated).
	br := bridgeToHydration(pkg, absPath, depContext{
		Provided:     depProvided,
		HasDepsBlock: hasDepsBlock,
		Permissive:   opts.NoResolveDeps && hasDepsBlock,
	})
	allIssues = append(allIssues, br.Issues...)

	// 5. Hydrate into the temp DB.
	hydration, err := db.HydratePackage(br.Input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error hydrating package: %v\n", err)
		return 2
	}

	for nodeID, issues := range hydration.IssuesByNodeID {
		file := br.NodeFiles[nodeID]
		for _, issue := range issues {
			allIssues = append(allIssues, ScanIssue{
				File:            file,
				NodeSlug:        nodeID,
				ValidationIssue: issue,
			})
		}
	}
	for edgeID, issues := range hydration.IssuesByEdgeID {
		ctx := br.EdgeFiles[edgeID]
		for _, issue := range issues {
			allIssues = append(allIssues, ScanIssue{
				File:            ctx.file,
				NodeSlug:        ctx.sourceSlug,
				ValidationIssue: issue,
			})
		}
	}

	// 6. Validate workflows, then re-code workflow.execution_*_missing
	// per the three-tier resolution (plan §2 + v2 §2 — same algorithm
	// the bridge applied to connection edges in step 4):
	//   - slug in dep-provided set → drop the issue (dep resolves it)
	//   - else, if hasDepsBlock → re-code as dependency.unresolved_slug
	//   - else (legacy bundle) → keep the historical _missing code
	for _, nf := range pkg.Nodes {
		if nf.Type != "workflow" {
			continue
		}
		rel := relPath(absPath, nf.FilePath)
		issues := db.ValidateWorkflow(nf.ID)
		for _, issue := range issues {
			if isExecutionMissingCode(issue.Code) {
				slug := issue.ReferencedSlug
				if slug != "" {
					if _, hit := depProvided[slug]; hit {
						continue // dep resolves it; drop
					}
					if opts.NoResolveDeps && hasDepsBlock {
						continue // permissive mode; treat as dep-provided
					}
					if hasDepsBlock {
						issue = recodeAsUnresolvedDepSlug(issue, slug)
					}
				}
			}
			allIssues = append(allIssues, ScanIssue{
				File:            rel,
				NodeSlug:        nf.ID,
				ValidationIssue: issue,
			})
		}
	}

	// 7. Build result and output.
	for i := range pkg.Nodes {
		pkg.Nodes[i].FilePath = ""
	}
	result := ScanResult{
		Path:      absPath,
		NodeCount: hydration.NodesInserted,
		EdgeCount: hydration.EdgesInserted,
		Issues:    allIssues,
		Package:   &pkg,
	}
	for _, issue := range allIssues {
		switch issue.Severity {
		case storage.SeverityError:
			result.ErrorCount++
		case storage.SeverityWarning:
			result.WarnCount++
		case storage.SeverityInfo:
			result.InfoCount++
		}
	}

	if jsonOutput {
		OutputJSON(result, w)
	} else {
		OutputTable(result, w)
	}

	if result.ErrorCount > 0 {
		return 2
	}
	if result.WarnCount > 0 {
		return 1
	}
	return 0
}

// edgeFileCtx tracks the originating file + source slug for a built
// EdgeInput so we can re-attach file context to any issue HydratePackage
// surfaces against the edge.
type edgeFileCtx struct {
	file       string
	sourceSlug string
}

// makeIssue creates a ValidationIssue.
func makeIssue(code string, severity string, message, contractRef, field string) storage.ValidationIssue {
	return storage.ValidationIssue{
		Code:        code,
		Severity:    storage.ValidationSeverity(severity),
		Message:     message,
		ContractRef: contractRef,
		Field:       field,
	}
}

// isExecutionMissingCode identifies the engine codes that signal a
// workflow.execution[].skill/prompt ref pointing at a slug not present
// in the local package. Three-tier resolution (plan §2) intercepts
// these and either drops them (dep-provided) or re-codes them
// (dep-declared but unresolved).
func isExecutionMissingCode(code string) bool {
	return code == "workflow.execution_skill_missing" ||
		code == "workflow.execution_prompt_missing"
}

func recodeAsUnresolvedDepSlug(issue storage.ValidationIssue, slug string) storage.ValidationIssue {
	issue.Code = "dependency.unresolved_slug"
	issue.Message = fmt.Sprintf("%s — slug %q is not provided by any declared dependency", issue.Message, slug)
	return issue
}

// manifestRelPath returns a stable relative path for skrptiq.yaml so dep
// resolution issues attribute to the manifest rather than to a random
// node file. Falls back to "skrptiq.yaml" if the package's manifest path
// isn't available (the engine's parse.Package doesn't expose it
// directly in v1.2).
func manifestRelPath(absPath string, _ parse.Package) string {
	_ = absPath
	return "skrptiq.yaml"
}
