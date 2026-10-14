package scan

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
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

	// CodeDuplicateSkillEntry fires on two execution entries naming one skill.
	// GH#1057 — the shape the rule above counts as INVOKED and the engine
	// cannot run. See checkDuplicateSkillEntries.
	CodeDuplicateSkillEntry = "workflow.duplicate_skill_entry"

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

// GH#1057 — the blind spot in the rule above, and the reason it needs a second
// clause rather than a repair.
//
// `storage.ResolveStepPrompts` builds `skill -> prompt` as a MAP KEYED BY SKILL
// SLUG (`out[skill] = prompt`), and the planner resolves every `skill:` entry
// through `slugToStep[skill]` to the SAME PlannedStep. So two execution entries
// naming one skill produce ONE step, and the earlier entry's prompt never runs.
//
// `report-writing-flow` ships exactly that:
//
//   - skill: "report-authoring"
//     prompt: "outline-report"
//   - skill: "report-authoring"
//     prompt: "draft-report"
//
// `outline-report` cannot run — the report is drafted with no outline step —
// and `workflow.node_never_invoked` counts it INVOKED, because that rule asks
// whether the slug is NAMED in the execution block, not whether it is
// REACHABLE. Naming is the right question for the defect it was built for (a
// node the execution block never mentions at all) and the wrong one here, so
// this is a second clause and not a change to the first: both facts are worth
// refusing, and a reachability-only rule would stop catching GH#858's shape.
//
// WHY THE SKILL IS THE ADDRESSABLE UNIT, and why the fix is not "let the engine
// make two steps" (the ruling on GH#1057, verified at source):
// `storage/validate.go:683` — "the from_step is the dep skill's TITLE".
// `from_step` binds by skill title, so two steps sharing a skill share a title,
// and every `from_step` naming it resolves to one of them SILENTLY rather than
// failing. Growing a second step per skill therefore costs the binding model,
// not an engine change: titles would have to become per-step, a contract change
// across Hub, the engine and the app, to support a shape no author has ever
// deliberately written. A distinct stage needs a distinct skill node — which is
// what GH#858's own A1 repair did.
//
// Measured over the 97 published packages before shipping: ONE live instance,
// `report-writing-flow`, the one GH#1057 names. A duplicate is never correct —
// it is an entry the engine discards — so unlike anything on the input axis
// this needs no severity ladder, no exemption and no opt-out.
func checkDuplicateSkillEntries(pkg *parse.Package, absPath string) []ScanIssue {
	var issues []ScanIssue
	for _, wf := range pkg.Nodes {
		if wf.Type != "workflow" {
			continue
		}
		entries := skillEntriesIn(wf.Metadata["execution"])

		// Group by skill, keeping authored order of both the skills and the
		// prompts attached to each.
		order := []string{}
		bySkill := map[string][]skillEntry{}
		for _, e := range entries {
			if _, seen := bySkill[e.skill]; !seen {
				order = append(order, e.skill)
			}
			bySkill[e.skill] = append(bySkill[e.skill], e)
		}

		for _, skill := range order {
			group := bySkill[skill]
			if len(group) < 2 {
				continue
			}
			// Every entry but the last is discarded by the map write, so every
			// prompt but the last never runs.
			var shadowed []string
			for _, e := range group[:len(group)-1] {
				if e.prompt != "" {
					shadowed = append(shadowed, strconv.Quote(e.prompt))
				}
			}
			detail := "so only the last entry runs"
			if len(shadowed) > 0 {
				detail = fmt.Sprintf("so %s never runs", strings.Join(shadowed, ", "))
			}
			issues = append(issues, ScanIssue{
				File:     relPath(absPath, wf.FilePath),
				NodeSlug: wf.ID,
				ValidationIssue: storage.ValidationIssue{
					Code:     CodeDuplicateSkillEntry,
					Severity: storage.SeverityError,
					Message: fmt.Sprintf(
						"skill %q is named by %d execution entries; the engine resolves a skill to one step, %s — give the other stage its own skill node",
						skill, len(group), detail),
					ContractRef:    invocationContractRef,
					Field:          "execution",
					ReferencedSlug: skill,
					ReferenceKind:  "skill",
				},
			})
		}
	}
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].NodeSlug != issues[j].NodeSlug {
			return issues[i].NodeSlug < issues[j].NodeSlug
		}
		return issues[i].ReferencedSlug < issues[j].ReferencedSlug
	})
	return issues
}

// skillEntry is one execution entry that names a skill, with the prompt that
// entry pairs with it (empty if it names none).
type skillEntry struct {
	skill  string
	prompt string
}

// skillEntriesIn returns every skill-naming execution entry in authored order,
// at any depth. Depth matters for the same reason it does in invocationsIn: a
// `parallel:` group's members are entries, and a duplicate spanning a group
// boundary collapses exactly like a flat one. Map values are walked in sorted
// key order so the report is deterministic regardless of YAML key order.
func skillEntriesIn(v any) []skillEntry {
	switch t := v.(type) {
	case map[string]any:
		var out []skillEntry
		if s, ok := t["skill"].(string); ok && s != "" {
			e := skillEntry{skill: s}
			if p, ok := t["prompt"].(string); ok {
				e.prompt = p
			}
			out = append(out, e)
		}
		keys := make([]string, 0, len(t))
		for k := range t {
			if k == "skill" || k == "prompt" {
				continue
			}
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			out = append(out, skillEntriesIn(t[k])...)
		}
		return out
	case []any:
		var out []skillEntry
		for _, vv := range t {
			out = append(out, skillEntriesIn(vv)...)
		}
		return out
	}
	return nil
}
