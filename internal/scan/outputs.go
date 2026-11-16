package scan

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/skrptiq/engine/outputs"
	"github.com/skrptiq/engine/parse"
	"github.com/skrptiq/engine/storage"
)

// Codes for the `outputs:` clause. GH#1213.
const (
	// CodeOutputsNotAList fires when `outputs:` is not a list of destinations.
	CodeOutputsNotAList = "workflow.outputs_not_a_list"
	// CodeOutputsEntryNotAMapping fires on a list entry that is not a mapping.
	CodeOutputsEntryNotAMapping = "workflow.outputs_entry_not_a_mapping"
	// CodeOutputsUnknownType fires on a destination `type` the engine cannot produce.
	CodeOutputsUnknownType = "workflow.outputs_unknown_type"
	// CodeOutputsUnknownKey fires on a key inside a destination that nothing reads.
	CodeOutputsUnknownKey = "workflow.outputs_unknown_key"
	// CodeOutputsFilePathNotRelative fires on a filePath that is wrong on every machine.
	CodeOutputsFilePathNotRelative = "workflow.outputs_file_path_not_relative"
)

const outputsContractRef = "SKRPT-STRUCTURE-PRINCIPLES § Output destinations"

// GH#1213 — `outputs:` decides where files land and no validator graded it.
//
// WHY THE SCAN WAS SILENT, which is the finding rather than the fix. The scan
// calls `db.ValidateWorkflowWithDeps` (scan.go) — the VALIDATOR. Every refusal
// GH#1210 added for a malformed `outputs:` lives in `BuildExecutionPlan` — the
// PLANNER. The scan never plans, so the key was never dropped by a stage; it was
// in a different code path from the one the publish gate runs.
//
// The consequence is the issue: the engine's refusals are loud but LATE. They
// fire when a CONSUMER builds a plan, on the consumer's machine. A package
// declaring `outputs: "notes/out.md"` published clean and then failed for
// everyone who installed it. The gate that decides what is publishable could not
// see the key that decides where files land.
//
// Measured over the published catalogue before shipping: ZERO workflows declare
// an `outputs:` destination block, so this lands with no content wave.
//
// THE FALSE POSITIVE THIS ISSUE'S WORDING INVITES. `type: file` appears in 28
// published files and NOT ONE is a destination — every one is an `inputs:`
// declaration (`label`, `required`, `example`, `type: file`), a different key
// space entirely. A rule keyed on that string would refuse 28 working packages,
// so this clause reads the `outputs` key and never the text, and those 28 are
// corpus fixtures rather than synthetic ones. Same lesson as GH#858's 34 false
// refusals and GH#1194's two documentation packages.
func checkOutputDestinations(pkg *parse.Package, absPath string) []ScanIssue {
	var issues []ScanIssue
	for _, wf := range pkg.Nodes {
		if wf.Type != "workflow" {
			continue
		}
		raw, present := wf.Metadata["outputs"]
		if !present || raw == nil {
			continue
		}
		issues = append(issues, outputsIssuesFor(wf, absPath, raw)...)
	}
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].NodeSlug != issues[j].NodeSlug {
			return issues[i].NodeSlug < issues[j].NodeSlug
		}
		return issues[i].Message < issues[j].Message
	})
	return issues
}

// knownDestinationKeys is exactly what the planner reads from a destination
// entry. Anything else is read by NOTHING.
var knownDestinationKeys = map[string]bool{
	"type":          true,
	"step":          true,
	"nodeType":      true,
	"titleTemplate": true,
	"filePath":      true,
}

// destinationKeySuggestions maps the plausible misspelling to the real key. Only
// near misses an author would actually type — a general edit-distance search
// would start suggesting `step` for `stop`.
var destinationKeySuggestions = map[string]string{
	"file_path":      "filePath",
	"filepath":       "filePath",
	"path":           "filePath",
	"file":           "filePath",
	"node_type":      "nodeType",
	"nodetype":       "nodeType",
	"title_template": "titleTemplate",
	"titletemplate":  "titleTemplate",
	"template":       "titleTemplate",
}

func outputsIssuesFor(wf parse.NodeFile, absPath string, raw any) []ScanIssue {
	mk := func(code, msg string) ScanIssue {
		return ScanIssue{
			File:     relPath(absPath, wf.FilePath),
			NodeSlug: wf.ID,
			ValidationIssue: storage.ValidationIssue{
				Code:        code,
				Severity:    storage.SeverityError,
				Message:     msg,
				ContractRef: outputsContractRef,
				Field:       "outputs",
			},
		}
	}

	// ── Clause A: structure ────────────────────────────────────────────────
	//
	// THIS MIRRORS THE PLANNER AND THE PLANNER IS THE AUTHORITY
	// (`execution/planner.go`, the three refusals GH#1210 added). It is here
	// because the planner's refusals fire on a consumer's machine and this gate
	// is what decides publishability — not because the predicate is ours.
	//
	// Its death is NAMED, not intended: GH#1243 exports `ValidateOutputs` from
	// the engine, and when it lands this clause collapses into a call to it and
	// `TestGH1213_DriftGuard_*` becomes redundant by construction. Until then
	// that drift guard builds a plan from every fixture refused here and asserts
	// the planner refuses it too, so a divergence fails CI rather than being
	// discovered by someone's install.
	list, ok := raw.([]any)
	if !ok {
		return []ScanIssue{mk(CodeOutputsNotAList, fmt.Sprintf(
			"`outputs:` is %T and must be a LIST of destinations — for example "+
				"`outputs:` then `  - type: file` and `    filePath: notes/out.md`. "+
				"A single value is not a destination", raw))}
	}

	var issues []ScanIssue
	for i, entry := range list {
		dm, ok := entry.(map[string]any)
		if !ok {
			issues = append(issues, mk(CodeOutputsEntryNotAMapping, fmt.Sprintf(
				"`outputs[%d]` is %T and every destination must be a mapping with a `type`", i, entry)))
			continue
		}

		dtype, _ := dm["type"].(string)
		if dtype != "new_node" && dtype != "clipboard" && dtype != "file" {
			shown := dtype
			if shown == "" {
				shown = "<missing>"
			}
			issues = append(issues, mk(CodeOutputsUnknownType, fmt.Sprintf(
				"`outputs[%d]` has type %q, which is not a destination this engine can "+
					"produce — it must be one of new_node, clipboard, file", i, shown)))
			// Keep checking this entry's keys: a wrong type and a typo'd key are
			// independent mistakes and reporting one should not hide the other.
		}

		// ── Clause B: unknown keys ─────────────────────────────────────────
		//
		// THE ONLY CLAUSE HERE THAT IS PERMANENTLY THE SCAN'S. The engine cannot
		// ever catch this: at plan time an unknown key and an absent one are
		// indistinguishable, and `{type: file}` with no `filePath` is LEGITIMATE
		// (GH#1193 treats an absent path as ask-shaped). So a misspelled
		// `file_path:` silently becomes an ask, and the author learns at run time
		// that the destination "needs a path" rather than that they typed the key
		// wrong. GH#1243 will not subsume this one.
		var unknown []string
		for k := range dm {
			if !knownDestinationKeys[k] {
				unknown = append(unknown, k)
			}
		}
		sort.Strings(unknown)
		for _, k := range unknown {
			msg := fmt.Sprintf(
				"`outputs[%d]` has the key %q, which nothing reads — a destination's keys are "+
					"exactly type, step, nodeType, titleTemplate, filePath", i, k)
			if did, found := destinationKeySuggestions[strings.ToLower(k)]; found {
				msg += fmt.Sprintf(". Did you mean %q? As written the destination silently "+
					"behaves as though no %s were given", did, did)
			}
			issues = append(issues, mk(CodeOutputsUnknownKey, msg))
		}

		// ── Clause C: filePath shape ───────────────────────────────────────
		if fp, isStr := dm["filePath"].(string); isStr && strings.TrimSpace(fp) != "" {
			if msg := filePathShapeIssue(fp); msg != "" {
				issues = append(issues, mk(CodeOutputsFilePathNotRelative,
					fmt.Sprintf("`outputs[%d]` %s", i, msg)))
			}
		}
	}
	return issues
}

// filePathShapeIssue grades a declared filePath, or returns "" when it is fine.
//
// TWO CASES, AND ONLY TWO. Both are wrong on EVERY machine, which is what makes
// them judgeable at publish time:
//
//   - ABSOLUTE — graded by calling `outputs.Resolve`, the engine's own predicate,
//     rather than reimplementing `filepath.IsAbs` here (the GH#1058 lesson, and
//     the ruling on GH#1208: the scan calls the engine's check). The check is
//     directory-independent, so a synthetic directory grades it correctly.
//   - a `..` SEGMENT — graded textually.
//
// WHAT THIS DELIBERATELY DOES NOT CLAIM. The engine's `confine` resolves symlinks
// on the RUNNING machine's filesystem, which at publish time is the author's and
// not the consumer's. So this refuses the textual cases honestly and does NOT
// claim to have verified confinement — that verdict belongs to the machine that
// writes, and nothing here should let a future reader think otherwise.
//
// AND `~` IS NOT GRADED HERE, ON PURPOSE. A leading `~` is neither absolute nor
// an escape, so it survives both engine checks and becomes a literal `~`
// directory inside the output directory — confined, but never what any author
// meant. GH#1244 adds that refusal to `outputs.Resolve`, and this clause will
// inherit it through the call below with no new scan-local rule. Until GH#1244
// lands, a `~` path publishes clean.
func filePathShapeIssue(declared string) string {
	for _, seg := range strings.Split(filepath.ToSlash(declared), "/") {
		if seg == ".." {
			return fmt.Sprintf(
				"declares the path %q, which escapes the output directory with a `..` segment. "+
					"A destination path must stay inside the directory the person running it configured",
				declared)
		}
	}

	// The engine's predicate, called. A synthetic directory is sound because the
	// absolute-path refusal does not depend on which directory is configured.
	_, err := outputs.Resolve(outputs.Request{
		Setting:      filepath.FromSlash("/skrptiq-scan-probe"),
		Home:         filepath.FromSlash("/skrptiq-scan-probe"),
		DeclaredPath: declared,
	})
	if errors.Is(err, outputs.ErrOutsideDirectory) {
		return fmt.Sprintf(
			"declares the path %q, which is absolute. A destination path must be relative to the "+
				"output directory the person running it configured — an absolute path is refused "+
				"rather than reinterpreted, because that would write somewhere the skrpt did not name",
			declared)
	}
	return ""
}
