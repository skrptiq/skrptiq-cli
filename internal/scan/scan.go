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

	// Rule1SuppressedNodes names the nodes whose Rule 1 (undeclared-input)
	// findings were excused because their execution entry straddles a
	// dependency edge, whose node content is served without frontmatter and so
	// may carry the declaration unreadably (GH#1266).
	//
	// It is emitted because the alternative was a figure derived OUTSIDE the
	// scanner: the 48%-of-the-catalogue measurement that found this bug was
	// computed by replicating the suppression logic against workflow
	// frontmatter, which is a maintained instrument that can drift from the
	// rule it models. A count the scanner itself produces makes the next
	// measurement a diff rather than a re-derivation.
	//
	// Empty for the overwhelming majority of packages; omitted when empty.
	Rule1SuppressedNodes []string `json:"rule1SuppressedNodes,omitempty"`
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
// CodeUnrecognisedSeverity fires when a finding carries a severity this scanner
// does not know (GH#1358). It is the gate reporting a disagreement about what
// counts as a failure, not a finding about the package's content.
const CodeUnrecognisedSeverity = "scan.unrecognised_severity"

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
	depSummaries := map[string]storage.DepNodeSummary{}
	var depNodes []storage.DepNodeSummary
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
		depNodes = result.DepNodes
		for _, n := range depNodes {
			if n.Slug != "" {
				depSummaries[n.Slug] = n
			}
		}
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
		Summaries:    depSummaries,
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

	// 6. Validate workflows with dep-awareness (GH#650). The engine's
	// ValidateWorkflowWithDeps merges depNodes into slugToNode + Title
	// indexes so workflow.execution_*_missing /
	// workflow.loop_step_missing don't false-fire on dep-provided
	// slugs, and bindings.from_step / loop_for_each_no_loop_item_usage
	// resolve through dep target Title / Content.
	//
	// The three-tier re-coding loop only re-codes the residual cases
	// the dep-aware validator can't resolve:
	//   - opts.NoResolveDeps + hasDepsBlock → drop (permissive: caller
	//     opted out of dep resolution; treat any non-local ref as
	//     potentially dep-provided)
	//   - hasDepsBlock + still _missing → re-code as
	//     dependency.unresolved_slug (slug isn't in any declared dep
	//     either)
	//   - else (legacy bundle, no dependencies: block) → keep the
	//     historical _missing code verbatim
	//
	// The "dep-provided → drop" tier from GH#630 plan v1 is gone: the
	// engine no longer emits _missing for dep-provided slugs once
	// depNodes is supplied.
	for _, nf := range pkg.Nodes {
		if nf.Type != "workflow" {
			continue
		}
		rel := relPath(absPath, nf.FilePath)
		issues := db.ValidateWorkflowWithDeps(nf.ID, depNodes)
		for _, issue := range issues {
			if isResolvableMissingCode(issue.Code) {
				slug := issue.ReferencedSlug
				if slug != "" {
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

	// 6b. GH#975 — the input contract. Anti-Pattern 5 is an authoring rule, so
	// it is enforced here rather than in the engine's validator: the scan is
	// the canonical gate for skrpt content (K-060), and only the scan sees the
	// package as the author wrote it.
	var rule1Suppressed []string
	reportedInputs := map[string]bool{}
	reportedOptionality := map[string]bool{}
	depStraddled := map[string]bool{}
	for _, nf := range pkg.Nodes {
		if nf.Type != "workflow" {
			continue
		}
		inputIssues, suppressed := checkWorkflowInputs(nf, &pkg, absPath)
		allIssues = append(allIssues, inputIssues...)
		rule1Suppressed = append(rule1Suppressed, suppressed...)
		for _, iss := range inputIssues {
			if iss.Code == CodeInputUndeclared {
				reportedInputs[iss.ReferencedSlug] = true
			}
			if iss.Code == CodeInputOptionalityUndeclared {
				reportedOptionality[iss.ReferencedSlug] = true
			}
		}
		for _, id := range suppressed {
			depStraddled[id] = true
		}
	}

	// 6b-bis. GH#1357 — Rule 1 again, with the node as its subject rather than the
	// package. The loop above only runs where a workflow exists, which meant a
	// package shipping the same prompt without a local runner was graded on
	// nothing: 145 of the live catalogue's 234 packages have no workflow node.
	// This pass asks the strictly local question — does the node's own declaration
	// cover a reference in its own body — and it is ADDITIVE: every pair the loop
	// above reported, and every node it excused for straddling a dependency edge,
	// is suppressed here, so nothing that reports today changes. Suppression is by
	// input NAME, which is the granularity that pass itself uses.
	allIssues = append(allIssues, checkNodeLocalInputs(&pkg, absPath, reportedInputs, depStraddled)...)

	// 6b-ter. GH#1287 — Rule 2 with the node as its subject, for the same reason
	// 6b-bis exists and with one difference worth stating at the call site: Rule 2
	// never needed reach at all. `required:`, the declaration's own prose and a
	// node-local Inputs table are the whole of its subject, so the loop above
	// gated it on a fact — does this package contain a workflow — that has nothing
	// to do with the question. Byte-identical node content earned an ERROR with a
	// local runner and nothing without one. Suppressed by input name against what
	// that loop already emitted, so no workflow-bearing package gains a duplicate,
	// and measured at 0 of 234 live packages when it landed.
	allIssues = append(allIssues, checkNodeLocalOptionality(&pkg, absPath, reportedOptionality)...)

	// 6c. GH#858 A1 — a prompt or skill the package ships and no workflow runs.
	// Same reason as 6b: an authoring defect the structural scanner used to pass
	// clean, which only the scan can refuse before publication.
	allIssues = append(allIssues, checkNodeInvocation(&pkg, absPath)...)

	// 6c-bis. GH#1357 Phase 2 — the library declaration graded against the package
	// it is attached to. It exempts 6c above, and an exemption that can be set
	// without being graded is an allowlist, so disagreement between the declaration
	// and the node set is a refusal. Runs regardless of 6c's own exemptions: a
	// false declaration is a defect whether or not the rule it exempts would have
	// fired.
	allIssues = append(allIssues, checkLibraryDeclaration(&pkg, absPath)...)

	// 6c-ter. GH#1364 — the template declaration graded against the package it is
	// attached to. It exempts three behaviours (the engine's unknown-namespace
	// suppression, the node-local input rule and 6c above), and an exemption that
	// can be set without being graded is an allowlist. Same reasoning as the
	// library declaration beside it.
	allIssues = append(allIssues, checkTemplateDeclaration(&pkg, absPath)...)

	// 6c-quater. GH#1387 — the package itself as a subject. Every rule above
	// compares declared things against each other, so a manifest declaring almost
	// nothing passes them all; this refuses a package with neither content nor a
	// reference to content.
	allIssues = append(allIssues, checkPackageHasContent(&pkg, absPath)...)

	// 6d. GH#1057 — two execution entries naming one skill. The engine resolves
	// a skill to a single step, so the earlier entry's prompt is discarded; 6c
	// counts it invoked because it is NAMED. Both clauses are needed.
	allIssues = append(allIssues, checkDuplicateSkillEntries(&pkg, absPath)...)

	// 6d-bis. GH#1194 — a raw shell step in a package that declares publish
	// intent. K-046 ruled it non-publishable and nothing enforced it. Silent for
	// a local-only package (R7 permits author scripts there), keyed on the
	// manifest identity that already distinguishes the two.
	allIssues = append(allIssues, checkShellNotPublishable(&pkg, absPath)...)

	// 6d-ter. GH#1213 — `outputs:` decides where files land and nothing graded
	// it. The scan validates and never plans, so GH#1210's planner refusals fire
	// on a CONSUMER's machine; this brings them to publish time, adds the
	// unknown-key clause the engine can never catch, and grades a filePath that
	// is wrong on every machine.
	allIssues = append(allIssues, checkOutputDestinations(&pkg, absPath)...)

	// 6e. GH#1057 — step references that cannot resolve under any execution: a
	// step reading its own output, and a reference to a node the package never
	// ships. Dep-awareness is handed in rather than inferred: a reference to a
	// dep-provided node is legitimate, and under --no-resolve-deps nothing was
	// fetched, so no non-local reference can be judged at all.
	stepDeps := stepRefDeps{
		Names:      map[string]bool{},
		TitleOf:    map[string]string{},
		Permissive: opts.NoResolveDeps && hasDepsBlock,
	}
	for slug, summary := range depSummaries {
		stepDeps.Names[refKey(slug)] = true
		stepDeps.TitleOf[refKey(slug)] = summary.Title
		if summary.Title != "" {
			stepDeps.Names[refKey(summary.Title)] = true
		}
	}
	allIssues = append(allIssues, checkStepReferences(&pkg, absPath, stepDeps)...)

	// 6f. GH#1058 — a workflow whose derived plan status is cycle or error
	// cannot run at all. Calls the engine's ComputeOrderDiff, the same single
	// authority `order-diff` uses; `reordered` is deliberately not refused (it
	// is the GH#1059 cutover question, not a present defect).
	allIssues = append(allIssues, checkPlanStatus(&pkg, absPath)...)

	// 7. Build result and output.
	for i := range pkg.Nodes {
		pkg.Nodes[i].FilePath = ""
	}
	result := ScanResult{
		Path:                 absPath,
		NodeCount:            hydration.NodesInserted,
		EdgeCount:            hydration.EdgesInserted,
		Issues:               allIssues,
		Package:              &pkg,
		Rule1SuppressedNodes: rule1Suppressed,
	}
	unrecognised, errs, warns, infos := tallySeverities(allIssues)
	if len(unrecognised) > 0 {
		result.Issues = append(result.Issues, unrecognised...)
	}
	result.ErrorCount, result.WarnCount, result.InfoCount = errs, warns, infos

	if jsonOutput {
		OutputJSON(result, w)
	} else {
		OutputTable(result, w)
	}

	return exitCodeFor(result)
}

// tallySeverities counts issues by severity, and reports any severity the gate
// does not recognise as a finding of its own (GH#1358).
//
// THE MISSING DEFAULT WAS A FAIL-OPEN IN THE CANONICAL GATE. This was a bare
// `switch` over the three severity constants with no `default`, while
// `OutputTable` prints every issue unconditionally and `severityLabel` has a
// `default` that renders an unknown severity's raw string. So an issue carrying a
// fourth value was PRINTED, counted toward nothing, and the scan exited 0 — a
// finding on stdout with a success status, which is indistinguishable from a clean
// scan to anything that checks the status. hub's publish path keys on the exit
// code, so the content would have published.
//
// The exposure was never cli's own rules, which all set a constant, nor the parse
// path, which `bridge.go`'s parseToStorageSeverity defaults. It was the three
// engine-fed paths — `hydration.IssuesByNodeID`, `IssuesByEdgeID` and
// `ValidateWorkflowWithDeps` — which pass `storage.ValidationIssue` through
// untouched. So cli's exit code was correct only for as long as the engine's
// vocabulary stayed exactly three values, and nothing asserted that.
//
// IT NAMES THE SEVERITY RATHER THAN GUESSING AT IT. Silently promoting an unknown
// value to error would be a second guess the next reader cannot see: they would
// find an error with no rule behind it and no way to learn why. One synthetic
// finding per distinct unrecognised value, naming the value, how many issues
// carried it, and one example code — and it is an error, so the verdict is
// non-zero by the ordinary path rather than by a special case.
func tallySeverities(issues []ScanIssue) (unrecognised []ScanIssue, errors, warnings, infos int) {
	// Keyed on the raw string: `sortedKeys` takes a string-keyed map, and the
	// value here is an unrecognised label rather than a known constant anyway.
	counts := map[string]int{}
	example := map[string]string{}

	for _, issue := range issues {
		switch issue.Severity {
		case storage.SeverityError:
			errors++
		case storage.SeverityWarning:
			warnings++
		case storage.SeverityInfo:
			infos++
		default:
			raw := string(issue.Severity)
			counts[raw]++
			if _, seen := example[raw]; !seen {
				example[raw] = issue.Code
			}
		}
	}

	for _, sev := range sortedKeys(counts) {
		unrecognised = append(unrecognised, ScanIssue{
			ValidationIssue: storage.ValidationIssue{
				Code:     CodeUnrecognisedSeverity,
				Severity: storage.SeverityError,
				Message: fmt.Sprintf(
					"%d finding(s) carry the severity %q, which this scanner does not recognise "+
						"(it knows %q, %q and %q) — for example %q. The findings themselves are in this "+
						"report, but they counted toward no verdict, so the scan would have exited 0 with "+
						"findings printed. Refusing instead: an unrecognised severity means the scanner and "+
						"whatever produced the finding disagree about what counts as a failure",
					counts[sev], sev,
					string(storage.SeverityError), string(storage.SeverityWarning), string(storage.SeverityInfo),
					example[sev]),
				Field:          "severity",
				ReferencedSlug: sev,
				ReferenceKind:  "severity",
			},
		})
		errors++
	}

	return unrecognised, errors, warnings, infos
}

// exitCodeFor is the single place the verdict is derived. Both output paths read
// the same ScanResult, so the table and the JSON cannot disagree with it.
func exitCodeFor(result ScanResult) int {
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

// isResolvableMissingCode identifies the engine codes that signal a
// slug reference (workflow.execution[].skill/prompt, loops[].steps[])
// pointing at a slug not present in the local package. Three-tier
// resolution intercepts these and either drops them (dep-provided) or
// re-codes them (dep-declared but unresolved). Each code in this set
// must populate ValidationIssue.ReferencedSlug structurally — adding
// a new code here without engine support would crash the re-coding
// silently (slug = "" → noop).
func isResolvableMissingCode(code string) bool {
	return code == "workflow.execution_skill_missing" ||
		code == "workflow.execution_prompt_missing" ||
		code == "workflow.loop_step_missing"
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
