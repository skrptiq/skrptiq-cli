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
	// GH#1057 — the shape the rule above counts as INVOKED and nothing
	// downstream can address. See checkDuplicateSkillEntries.
	CodeDuplicateSkillEntry = "workflow.duplicate_skill_entry"

	invocationContractRef = "SKRPT-STRUCTURE-PRINCIPLES § Data-Flow & I/O Mapping"
)

// checkNodeInvocation reports prompt/skill nodes the package ships but never runs.
//
// IT USED TO RETURN NOTHING FOR A PACKAGE WITH NO WORKFLOW OF ITS OWN, and removing
// that early return is what GH#1357 was about. The original reasoning was deliberate
// and documented — a package with no execution block ships its nodes to be IMPORTED
// rather than run in place — and it was still wrong for one reason:
//
//	it conflated "ships nodes for others to run", a property the author knows
//	and can state, with "happens to contain no workflow", a shape the scanner
//	infers. AN INFERENCE THAT CANNOT BE WRONG IS NOT A CHECK.
//
// The replacement is a declaration the author makes and this scan grades:
// `manifest.library: true`, honoured below and graded by `checkLibraryDeclaration`.
// The inference was load-bearing for 109 packages; the declaration now carries 107
// of them, and the two that were refused are reporting, which is the point.
//
// THE MIGRATION IS DONE AND THIS COMMENT IS WHAT REPLACED THE HOLD. Order 1 was
// annotate then enforce, because enforcing first would have put 108 packages at exit
// 2 — an outage of the publish path rather than a transition. hub annotated at
// `9a9d90e` (hub #233) and enforcement went on here. Expect exactly two live
// packages to report, 11 nodes between them: `quick-actions`, whose nodes the app's
// quick panel invokes directly, and `api-weather-pipeline`, which narrates a
// three-stage sequence and ships no workflow. BOTH REFUSALS ARE HUB'S JUDGEMENT AND
// NEITHER IS SOMETHING THIS GATE COULD HAVE MADE: it checks "runs its own nodes" and
// "ships no nodes", and cannot check "has no workflow for a reason other than being
// a library", which is where both sit. GH#1384 carries that gap; `_shared/` is not a
// signal this code may read, then or now.
//
// A SMALLER CORRECTION KEPT BECAUSE IT IS ABOUT COUNTING: the original comment said
// all 135 `_shared/` packages carry zero workflows. 134 of 135 do —
// `_shared/skrpt-authoring-reference` declares `type: workflow` in a file not under
// a `workflows/` directory, so a directory-only count misses it.
func checkNodeInvocation(pkg *parse.Package, absPath string) []ScanIssue {
	// A declared template package is exempt. `manifest.template: true` (GH#374)
	// says the package ships fill-in-the-blank material, and material nobody runs
	// is what a template IS — so reporting its nodes as uninvoked would refuse it
	// for being exactly what it declares. Two of the six live declarers,
	// `templates-prompts` and `templates-skills`, are packages this rule now reaches
	// and this exemption keeps passing: 20 nodes between them.
	//
	// This HANDLES the declaration; it does not GRADE it. Nothing checks that a
	// package declaring `template: true` has any template property — see GH#1364
	// and the skipped test naming it.
	if pkg.Manifest.Template {
		return nil
	}

	// A DECLARED LIBRARY IS EXEMPT, and unlike the template flag beside it this one
	// is GRADED — `checkLibraryDeclaration` refuses the declaration when the package
	// does not have the shape (GH#1357 Phase 2, option A). That is the whole
	// difference between a declaration and an allowlist: this exemption can be
	// wrong, and when it is wrong the scan says so.
	//
	// It is keyed on declared intent rather than on the absence of a workflow,
	// because "ships nodes for others to run" is a property the author knows and
	// can state while "happens to contain no workflow" is a shape the scanner
	// infers — and an inference that cannot be wrong is not a check.
	if pkg.Manifest.Library {
		return nil
	}

	var workflows []parse.NodeFile
	for _, n := range pkg.Nodes {
		if n.Type == "workflow" {
			workflows = append(workflows, n)
		}
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
// THE MECHANISM MOVED AND THE REFUSAL DID NOT, so both halves are recorded here
// rather than left to be re-derived from a red test:
//
// UNTIL app GH#1092, `storage.ResolveStepPrompts` built `skill -> prompt` as a
// MAP KEYED BY SKILL SLUG (`out[skill] = prompt`) and the planner resolved every
// `skill:` entry through `slugToStep[skill]` to the SAME PlannedStep. Two
// entries naming one skill produced ONE step and the earlier entry's prompt
// never ran. SINCE GH#1092 each entry plans its own step: both prompts run,
// both steps are visible, nothing is discarded.
//
// The refusal survives that fix because it never rested on the discard — see
// the ruling below. GH#1092 says so itself: "same-skill-twice is now
// EXPRESSIBLE, not supported ... Hub keeps refusing the shape at publish time,
// which is consistent: plan-time expressible, publish-time refused." This rule
// IS that publish-time refusal, and `gh1057_duplicate_skill_test.go` holds the
// probe that grades the engine's half of it.
//
// `report-writing-flow` ships exactly that:
//
//   - skill: "report-authoring"
//     prompt: "outline-report"
//   - skill: "report-authoring"
//     prompt: "draft-report"
//
// Two steps, both running, both titled `Report Authoring` — so nothing can say
// which one it means. `workflow.node_never_invoked` is silent on both prompts,
// because that rule asks whether the slug is NAMED in the execution block, not
// whether what it names can be ADDRESSED. Naming is the right question for the
// defect it was built for (a node the execution block never mentions at all)
// and the wrong one here, so this is a second clause and not a change to the
// first: both facts are worth refusing, and an addressability-only rule would
// stop catching GH#858's shape.
//
// WHY THE SKILL IS THE ADDRESSABLE UNIT, and why the fix was never "let the
// engine make two steps" — the engine now does, and it changed nothing here
// (the ruling on GH#1057, verified at source and upheld verbatim by GH#1092):
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
// it is a step nothing downstream can bind to — so unlike anything on the input
// axis this needs no severity ladder, no exemption and no opt-out.
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
			// Since GH#1092 every entry plans its own step, so no entry is
			// "the shadowed one" any more — the ambiguity is symmetric and
			// every prompt in the group is equally unaddressable. Name them
			// all, in authored order, so the author can see which stages
			// collide; say nothing about running or discarding, because the
			// engine now runs all of them.
			var colliding []string
			for _, e := range group {
				if e.prompt != "" {
					colliding = append(colliding, strconv.Quote(e.prompt))
				}
			}
			detail := ""
			if len(colliding) > 0 {
				detail = fmt.Sprintf(" (the colliding stages run %s)", strings.Join(colliding, ", "))
			}
			issues = append(issues, ScanIssue{
				File:     relPath(absPath, wf.FilePath),
				NodeSlug: wf.ID,
				ValidationIssue: storage.ValidationIssue{
					Code:     CodeDuplicateSkillEntry,
					Severity: storage.SeverityError,
					Message: fmt.Sprintf(
						"skill %q is named by %d execution entries; each entry plans its own step, but they all carry the skill's title, which is the only thing a from_step can name, so nothing downstream can address them separately%s — give the other stage its own skill node",
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

// Codes for the library declaration's own grading (GH#1357 Phase 2).
const (
	// CodeLibraryRunsItsOwnNodes fires when a package declares `library: true` and
	// a workflow it ships invokes one of its own nodes.
	CodeLibraryRunsItsOwnNodes = "library.runs_its_own_nodes"
	// CodeLibraryShipsNoNodes fires when a package declares `library: true` and
	// ships nothing for anyone to run.
	CodeLibraryShipsNoNodes = "library.ships_no_nodes"
)

// checkLibraryDeclaration grades `manifest.library: true` against the package it
// is attached to (GH#1357 Phase 2, option A).
//
// WHY THIS EXISTS AT ALL. The declaration exempts `checkNodeInvocation`, and
// **a declaration-based exemption becomes an allowlist the moment it can be set
// without being graded.** `manifest.template: true` is the cautionary case: it is
// parsed, consumed by three scan behaviours, and graded nowhere (GH#1364). So this
// one is refused when it is not true, in both directions.
//
// THE TRUTH IS DERIVED FROM THE ARTEFACT, which is GH#1313's shape in a different
// domain: the package's own node set says whether it runs its own nodes, so
// disagreement between the declaration and the nodes is a FAILURE rather than a
// preference. Two ways to disagree, and both refuse:
//
//   - a workflow in the package invokes one of the package's own nodes — it is not
//     a library, it is a package that runs things, and the exemption would hide
//     genuinely uninvoked siblings;
//   - the package ships no executable node at all — the declaration is vacuous,
//     exempting a rule for a package with nothing to exempt. That case is easy to
//     miss and is a real shape: a bundle of documents and assets declaring itself
//     a library of nodes it does not have.
//
// WHAT IT DELIBERATELY DOES NOT CHECK. Nothing verifies that anything actually
// CONSUMES a declared library's nodes, so a library nobody depends on passes. That
// needs the cross-package view Rules 2 and 3 are waiting on, and nothing here
// notices if that view ever arrives — the limit is real and this comment is where
// the next reader finds it rather than rediscovering it.
func checkLibraryDeclaration(pkg *parse.Package, absPath string) []ScanIssue {
	if !pkg.Manifest.Library {
		return nil
	}

	var executable []parse.NodeFile
	var workflows []parse.NodeFile
	for _, n := range pkg.Nodes {
		switch n.Type {
		case "prompt", "skill":
			executable = append(executable, n)
		case "workflow":
			workflows = append(workflows, n)
		}
	}

	if len(executable) == 0 {
		return []ScanIssue{{
			File: manifestRelPath(absPath, *pkg),
			ValidationIssue: storage.ValidationIssue{
				Code:     CodeLibraryShipsNoNodes,
				Severity: storage.SeverityError,
				Message: "manifest declares `library: true` but the package ships no prompt or " +
					"skill node, so there is nothing for another package to run. A library " +
					"declaration exempts the uninvoked-node rule, and exempting a rule for a " +
					"package with nothing to exempt is a declaration that cannot be true — " +
					"remove `library: true`, or ship the nodes it promises",
				Field: "library",
			},
		}}
	}

	// Which of this package's own nodes do its own workflows invoke?
	invoked := map[string]bool{}
	for _, wf := range workflows {
		for _, slug := range invocationsIn(wf.Metadata) {
			invoked[slug] = true
		}
	}

	var issues []ScanIssue
	for _, n := range executable {
		if !invoked[n.ID] && !invoked[fileSlugOf(n)] {
			continue
		}
		issues = append(issues, ScanIssue{
			File:     relPath(absPath, n.FilePath),
			NodeSlug: n.ID,
			ValidationIssue: storage.ValidationIssue{
				Code:     CodeLibraryRunsItsOwnNodes,
				Severity: storage.SeverityError,
				Message: fmt.Sprintf(
					"manifest declares `library: true`, but a workflow in this package invokes "+
						"%s %q. A library supplies nodes to OTHER packages and runs none of them "+
						"itself — a package that runs its own nodes is an ordinary package, and "+
						"accepting the declaration here would hide genuinely uninvoked siblings. "+
						"Remove `library: true`, or move the workflow out",
					n.Type, n.ID),
				Field:          "library",
				ReferencedSlug: n.ID,
				ReferenceKind:  n.Type,
			},
		})
	}

	return issues
}
