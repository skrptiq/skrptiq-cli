package scan

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/skrptiq/engine/parse"
	"github.com/skrptiq/engine/storage"
)

// GH#858 A1 — `competitive-analysis-flow` hard-failed in a live run:
//
//	step "Strategic Recommendation" halted by LLM input validator:
//	SWOT analysis and positioning map narrative inputs are empty
//
// The package shipped `prompts/swot-generator.md` and
// `prompts/feature-comparison-matrix.md`, its own documentation said to invoke
// them, and the `execution:` list never did. It scanned clean.
//
// WHY THIS AXIS, and not the obvious one. The obvious rule — "every input a
// prompt requires must be produced by some step" — does NOT catch A1, measured
// against the version that actually failed (reconstructed from the parent of
// hub's `8acd9ac4`): the brief-writer referenced
// `{{steps.Competitor Profiling.output}}` twice and
// `{{steps.Market Positioning Analysis.output}}` twice, and BOTH titles exist in
// the pre-fix execution block. Every reference resolved, to a real step, earlier
// in the order. The defect was four semantically distinct roles wired to two
// producers, and nothing mechanical separates that from two slots legitimately
// fed by one upstream output — which is common and correct. A scanner that
// guessed would refuse working content.
//
// So this rule asks a question with no semantics in it at all: does the package
// ship a prompt or skill that no workflow invokes? Two local, unambiguous facts.
// It needs no input resolution, no caller model and no dependency metadata, so
// unlike anything on the input axis it also works under --no-resolve-deps.
//
// Measured before shipping: 4 findings on the pre-fix package — exactly the four
// nodes the issue names — and 0 on the repaired one. 19 findings across 13 of
// the 88 published packages, every one of which is documented in its own
// workflow body as a stage that runs ("Invoke the X prompt", "**Node:** `x`",
// "Run these two prompts in parallel"). None is described as an optional
// variant, so the corpus holds no deliberate case and this rule needs no
// opt-out. If one ever appears it wants a DECLARATION on the node, not a
// skip-list in here — a list rots, a declaration travels with the thing — and
// that is a schema change, so it is not invented here.
const (
	CodeNodeNeverInvoked = "workflow.node_never_invoked"

	invocationContractRef = "SKRPT-STRUCTURE-PRINCIPLES § Data-Flow & I/O Mapping"
)

// checkNodeInvocation reports prompt/skill nodes the package ships but never
// runs. Returns nothing for a package with no workflow of its own: a package
// with no execution block anywhere ships its nodes to be IMPORTED, not to be
// run in place, so "uninvoked" is its normal and correct state. That covers
// every one of the 135 `_shared/` packages in the published catalogue — all 135
// carry a manifest and zero workflows — without the gate having to know
// anything about the `_shared/` path convention.
func checkNodeInvocation(pkg *parse.Package, absPath string) []ScanIssue {
	var workflows []parse.NodeFile
	for _, n := range pkg.Nodes {
		if n.Type == "workflow" {
			workflows = append(workflows, n)
		}
	}
	if len(workflows) == 0 {
		return nil
	}

	// Invocations are unioned across EVERY workflow in the package: in a
	// multi-workflow package a node invoked by one workflow is invoked, even if
	// the others never name it.
	invoked := map[string]bool{}
	for _, wf := range workflows {
		for _, slug := range invocationsIn(wf.Metadata) {
			invoked[slug] = true
		}
	}

	var issues []ScanIssue
	var reported []ScanIssue
	for _, n := range pkg.Nodes {
		if n.Type != "prompt" && n.Type != "skill" {
			// Only prompt and skill nodes exist in order to be executed.
			// Sources, documents, assets and services are reference material
			// and services; they have no execution entry by design.
			continue
		}
		if invoked[n.ID] || invoked[fileSlugOf(n)] {
			continue
		}
		reported = append(reported, ScanIssue{
			File:     relPath(absPath, n.FilePath),
			NodeSlug: n.ID,
			ValidationIssue: storage.ValidationIssue{
				Code:     CodeNodeNeverInvoked,
				Severity: storage.SeverityError,
				Message: fmt.Sprintf(
					"%s %q ships in this package but no workflow invokes it; add an execution entry for it, or remove the node and the documentation that promises it",
					n.Type, n.ID),
				ContractRef:    invocationContractRef,
				Field:          "execution",
				ReferencedSlug: n.ID,
				ReferenceKind:  n.Type,
			},
		})
	}
	sort.Slice(reported, func(i, j int) bool { return reported[i].NodeSlug < reported[j].NodeSlug })
	issues = append(issues, reported...)
	return issues
}

// invocationsIn returns every node slug a workflow's metadata invokes: every
// `skill:`, `prompt:` and step-level `id:` value ANYWHERE in the execution
// block, plus the step slugs listed in each `loops:` body.
//
// "Anywhere" is load-bearing and was learned the same way hub learned its
// equivalent rule. An execution entry can be a GROUP rather than a step:
//
//	execution:
//	  - parallel:
//	    - skill: "email-fetch"
//	      prompt: "fetch-emails"
//	    - skill: "calendar-fetch"
//	      prompt: "fetch-calendar"
//
// A first version of this function read only the top-level `skill`/`prompt`/`id`
// of each entry, and therefore reported every step inside a `parallel:` group as
// never invoked — 34 false refusals across 10 working packages, including
// `morning-briefing`, which is one of the simplest shapes there is. The corpus
// measurement caught it before it shipped. Grouping constructs are not
// enumerable from outside the engine, so the whole structure is walked instead
// of named fields being picked out of it.
//
// Loop bodies are read even though no published package currently needs it —
// all 12 workflows with a `loops:` block also list those steps in their
// execution block. That is a property of today's content, not a guarantee, and
// a node invoked only from a loop body would otherwise be a false refusal.
func invocationsIn(meta map[string]any) []string {
	out := collectInvocationSlugs(meta["execution"])

	if loops, ok := meta["loops"].([]any); ok {
		for _, l := range loops {
			m, ok := l.(map[string]any)
			if !ok {
				continue
			}
			steps, ok := m["steps"].([]any)
			if !ok {
				continue
			}
			for _, s := range steps {
				if str, ok := s.(string); ok && str != "" {
					out = append(out, str)
				}
			}
		}
	}

	return out
}

// collectInvocationSlugs walks an execution structure at any depth, collecting
// every value of a key that names a node to run.
func collectInvocationSlugs(v any) []string {
	switch t := v.(type) {
	case map[string]any:
		var out []string
		for k, vv := range t {
			if k == "skill" || k == "prompt" || k == "id" {
				if s, ok := vv.(string); ok && s != "" {
					out = append(out, s)
					continue
				}
			}
			out = append(out, collectInvocationSlugs(vv)...)
		}
		return out
	case []any:
		var out []string
		for _, vv := range t {
			out = append(out, collectInvocationSlugs(vv)...)
		}
		return out
	}
	return nil
}

// fileSlugOf is the node's slug as its filename spells it, which is what an
// execution entry names when the frontmatter `id:` differs from the file.
func fileSlugOf(n parse.NodeFile) string {
	if n.FilePath == "" {
		return ""
	}
	return strings.TrimSuffix(filepath.Base(n.FilePath), ".md")
}
