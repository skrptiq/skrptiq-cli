package scan

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/skrptiq/engine/parse"
	"github.com/skrptiq/engine/storage"
)

// GH#1057 (second wave, from hub's GH#1045 classification) — two more
// structural facts about an execution block that only the scan can check.
//
// Both are mechanically decidable from the package alone, and both were found
// by classifying content rather than by running anything:
//
//	data-analysis-pipeline      findings-synthesiser reads
//	                            {{steps.Qualitative Coding.output}} — and the
//	                            entry running it IS qualitative-coding
//	bug-report-triage-pipeline  bug-report-triage reads
//	                            {{steps.Structured Data Extraction.output}}
//	                            while itself occupying that slot
//	revision-strategy-flow      post-exam-reflector reads
//	                            {{steps.Practice Question Generator.output}} —
//	                            and practice-question-generator does not ship
const (
	// CodeStepReadsOwnOutput fires on a step whose prompt reads the output of
	// the step itself. There is no reading under which that is correct: at best
	// the slot resolves to nothing, so the prompt asks the model to work from
	// material it is simultaneously producing.
	CodeStepReadsOwnOutput = "workflow.step_reads_own_output"

	// CodeStepRefNeverResolves fires on a {{steps.X.output}} that NO step in any
	// of the package's workflows is named by — so there is no execution under
	// which it resolves.
	//
	// MEASUREMENT CORRECTED THE PREMISE HERE. The directive specified "a
	// reference to a node that does not ship", and its one named instance,
	// `revision-strategy-flow`, turns out to declare
	// `practice-question-generator` as a dependency that RESOLVES — a published
	// prompt, title "Practice Question Generator". What is missing is not the
	// node; it is the execution entry that would make it a step. Written as
	// "does not ship", the clause would have had an empty population and would
	// have missed the only live case, so the condition is EXECUTION-BLOCK
	// RESOLVABILITY, and the message names whichever fix the author needs.
	//
	// Not a duplicate of the engine's `workflow.execution_step_ref_unresolved`
	// (§4.4, WARNING): that one reads only prompts attached to a skill entry
	// whose skill resolves locally, and it is deliberately tolerant of a stale
	// reference on an unexercised branch. This clause reads every prompt and
	// skill the package ships and treats the fact as provable, which is what it
	// is once the whole package is in view.
	CodeStepRefNeverResolves = "workflow.step_ref_never_resolves"
)

// stepRefPattern mirrors the engine's resolver (storage/validate.go:2328): X may
// be a skill slug, a skill title with spaces, a builtin step `id:`, or the
// literal `previous`.
var stepRefPattern = regexp.MustCompile(`\{\{\s*steps\.([^.{}]+)\.output\s*\}\}`)

// inlineCode matches a backtick span. Step refs inside markdown code are
// skipped, mirroring the engine's GH#522 behaviour for the same detector —
// `skrpt-builder` and `skrptiq-on-skrptiq` teach the syntax and write
// `{{steps.X.output}}` as a literal example. This DIVERGES from the input
// detector in inputs.go, which deliberately keeps inline spans because the
// corpus writes load-bearing input refs inside them ("if `{{input.week_start}}`
// is provided"). The two detectors answer different questions and the corpus
// writes the two namespaces differently; parity with the engine's own resolver
// decides it here. Both live instances of the self-reference clause are
// unfenced prose, so the strip costs nothing on the population it must catch.
var inlineCode = regexp.MustCompile("`[^`\n]*`")

// stepRefsIn returns every {{steps.X.output}} target in a body, outside
// markdown code, in first-seen order. `previous` is excluded: positional
// lookback is always resolvable.
func stepRefsIn(body string) []string {
	clean := inlineCode.ReplaceAllString(fencedBlock.ReplaceAllString(body, ""), "")
	seen := map[string]bool{}
	var out []string
	for _, m := range stepRefPattern.FindAllStringSubmatch(clean, -1) {
		ref := strings.TrimSpace(m[1])
		if ref == "" || ref == "previous" || seen[ref] {
			continue
		}
		seen[ref] = true
		out = append(out, ref)
	}
	return out
}

// refKey normalises a reference the way the runtime resolver does: lowercased,
// every run of non-alphanumerics collapsed to a single dash. Mirrors
// storage.slugifyForRef, which is unexported.
func refKey(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	prevDash := true
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			b.WriteByte(c)
			prevDash = false
			continue
		}
		if !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// stepRefDeps is what the two clauses need to know about dependency resolution.
// A reference to a dep-provided node is legitimate, and under
// --no-resolve-deps nothing was fetched, so no non-local reference can be
// judged at all — the same suppression Rule 1 carries on GH#858 and GH#975, for
// the same reason: the declaration may live on the half this scan cannot read.
type stepRefDeps struct {
	Names      map[string]bool   // refKey of every dep-provided slug and title
	TitleOf    map[string]string // refKey(dep slug) → that node's title
	Permissive bool              // --no-resolve-deps on a package with a deps block
}

// checkStepReferences applies both clauses. They have deliberately DIFFERENT
// scopes, and the corpus decided each:
//
//   - Self-reference is a fact about one STEP, so it is checked per execution
//     entry: "itself" is only defined relative to the entry running the prompt.
//     Prompt-only entries are included — a prompt with no backing skill is
//     still a step, referenced by its own title, so it can read its own output
//     too.
//   - A reference to something the package never ships is a fact about the
//     PACKAGE, so it is checked over every prompt/skill the package ships.
//     Scoping it to attached prompts only would have missed its single live
//     instance: `revision-strategy-flow`'s dangling Stage-4 reference sits in
//     `post-exam-reflector`, which the execution block never attaches (it is a
//     GH#858 orphan as well). Both facts are separately true and separately
//     actionable: the stage is neither wired nor published.
//
// Packages with no workflow are skipped entirely, for the same reason
// checkNodeInvocation skips them: all 135 `_shared/` elements ship their nodes
// to be IMPORTED, and a shared prompt legitimately references steps its
// CONSUMER provides (the GH#782 reuse model). Judging those would refuse the
// reuse model itself.
func checkStepReferences(pkg *parse.Package, absPath string, deps stepRefDeps) []ScanIssue {
	var workflows []parse.NodeFile
	for _, n := range pkg.Nodes {
		if n.Type == "workflow" {
			workflows = append(workflows, n)
		}
	}
	if len(workflows) == 0 {
		return nil
	}

	// Everything the package itself ships, by normalised slug AND title.
	ships := map[string]bool{}
	byName := map[string]parse.NodeFile{}
	for _, n := range pkg.Nodes {
		for _, name := range nodeNames(n) {
			ships[name] = true
			if _, exists := byName[name]; !exists {
				byName[name] = n
			}
		}
	}

	// Every name any workflow's execution block makes referenceable: skill
	// slugs and their node titles, plus `id:` (local.builtin / local.template)
	// and `label:` (MCP action) step names. Missing the last two would
	// false-fire the second clause on every builtin pipeline —
	// `big-log-extractor-summariser` reads {{steps.extract-errors.output}},
	// which is an `id:`. Unioned across workflows, as invocations are.
	execNames := map[string]bool{}
	for _, wf := range workflows {
		for _, name := range collectStepNames(wf.Metadata["execution"]) {
			key := refKey(name)
			execNames[key] = true
			if n, ok := byName[key]; ok && n.Title != "" {
				execNames[refKey(n.Title)] = true
			}
			// A step can be a DEP-PROVIDED node, and the corpus references it
			// by title: `research-proposal-flow` wires `skill: gap-analysis`
			// and its prompt reads {{steps.Research Gap Identification.output}}
			// — the dep node's title, which slugifies to nothing like its slug.
			// Without this the clause reports a working package.
			if title, ok := deps.TitleOf[key]; ok && title != "" {
				execNames[refKey(title)] = true
			}
		}
	}

	var issues []ScanIssue

	// ── Clause 1, per execution entry ─────────────────────────────────────
	for _, wf := range workflows {
		stepPrompts := map[string]string{}
		if sp, ok := wf.Metadata["stepPrompts"].(map[string]any); ok {
			for k, v := range sp {
				if s, ok := v.(string); ok {
					stepPrompts[k] = s
				}
			}
		}

		for _, e := range promptBearingEntries(wf.Metadata["execution"]) {
			promptSlug := e.prompt
			if promptSlug == "" && e.skill != "" {
				promptSlug = stepPrompts[e.skill]
			}
			if promptSlug == "" {
				continue
			}
			prompt, ok := byName[refKey(promptSlug)]
			if !ok {
				continue // a dep-provided prompt: its body is not ours to read
			}

			// The names that mean "this very step". For a skill entry that is
			// the skill slug and the skill node's TITLE, which is what the
			// corpus writes; for a prompt-only entry it is the prompt itself.
			self := map[string]bool{}
			owner := e.skill
			if owner == "" {
				owner = promptSlug
			}
			self[refKey(owner)] = true
			if n, ok := byName[refKey(owner)]; ok && n.Title != "" {
				self[refKey(n.Title)] = true
			}

			for _, ref := range stepRefsIn(prompt.Content) {
				if !self[refKey(ref)] {
					continue
				}
				issues = append(issues, ScanIssue{
					File:     relPath(absPath, prompt.FilePath),
					NodeSlug: prompt.ID,
					ValidationIssue: storage.ValidationIssue{
						Code:     CodeStepReadsOwnOutput,
						Severity: storage.SeverityError,
						Message: fmt.Sprintf(
							"prompt %q reads {{steps.%s.output}}, which is the step it runs as (%q) — a step cannot consume its own output; reference the upstream step that produces this material",
							prompt.ID, ref, owner),
						ContractRef:    invocationContractRef,
						Field:          "execution",
						ReferencedSlug: owner,
						ReferenceKind:  "skill",
					},
				})
			}
		}
	}

	// ── Clause 2, over every prompt/skill the package ships ───────────────
	for _, n := range pkg.Nodes {
		if n.Type != "prompt" && n.Type != "skill" {
			continue
		}
		for _, ref := range stepRefsIn(n.Content) {
			key := refKey(ref)
			if execNames[key] {
				continue // a step by that name runs: resolvable
			}
			if deps.Permissive {
				continue // nothing was fetched; no non-local name can be judged
			}
			// WHICH fix the author needs depends on whether the thing exists.
			detail := "and no node of that name ships in this package or any declared dependency — the reference can never resolve; publish the missing node or remove the reference"
			if ships[key] || deps.Names[key] {
				detail = "and no execution entry runs a step of that name — the node exists but nothing invokes it, so the reference can never resolve; wire the stage or remove the reference"
			}
			issues = append(issues, ScanIssue{
				File:     relPath(absPath, n.FilePath),
				NodeSlug: n.ID,
				ValidationIssue: storage.ValidationIssue{
					Code:     CodeStepRefNeverResolves,
					Severity: storage.SeverityError,
					Message: fmt.Sprintf(
						"%s %q reads {{steps.%s.output}}, %s",
						n.Type, n.ID, ref, detail),
					ContractRef:    invocationContractRef,
					Field:          "execution",
					ReferencedSlug: key,
					ReferenceKind:  "step",
				},
			})
		}
	}

	sort.Slice(issues, func(i, j int) bool {
		if issues[i].NodeSlug != issues[j].NodeSlug {
			return issues[i].NodeSlug < issues[j].NodeSlug
		}
		if issues[i].Code != issues[j].Code {
			return issues[i].Code < issues[j].Code
		}
		return issues[i].ReferencedSlug < issues[j].ReferencedSlug
	})
	return issues
}

// promptBearingEntries returns every execution entry that carries a prompt, in
// authored order and at any depth: skill+prompt entries AND prompt-only ones.
// A prompt-only entry IS a step (the planner's step types are {skill, prompt}),
// which is why it cannot be filtered out here.
func promptBearingEntries(v any) []skillEntry {
	switch t := v.(type) {
	case map[string]any:
		var out []skillEntry
		skill, _ := t["skill"].(string)
		prompt, _ := t["prompt"].(string)
		if skill != "" || prompt != "" {
			out = append(out, skillEntry{skill: skill, prompt: prompt})
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
			out = append(out, promptBearingEntries(t[k])...)
		}
		return out
	case []any:
		var out []skillEntry
		for _, vv := range t {
			out = append(out, promptBearingEntries(vv)...)
		}
		return out
	}
	return nil
}

// nodeNames returns every normalised name a node can be referenced by: its id,
// its filename slug, and its title.
func nodeNames(n parse.NodeFile) []string {
	var out []string
	for _, raw := range []string{n.ID, fileSlugOf(n), n.Title} {
		if k := refKey(raw); k != "" {
			out = append(out, k)
		}
	}
	return out
}

// collectStepNames walks an execution structure at any depth and returns every
// value that names a step for reference purposes: `skill:`, `id:`, `label:`,
// and `prompt:` ONLY on an entry that names no skill.
//
// That last condition is the whole subtlety. A skill+prompt entry is referenced
// by its SKILL — naming the prompt as well would make a prompt reading its own
// output look resolvable, which is the defect clause 1 exists to catch. But a
// prompt-ONLY entry is still a step, and the only name it has is the prompt's,
// so excluding it made every legitimate reference to such a step unresolvable.
// The prompt-only self-reference test is what surfaced it: both clauses fired on
// one line, which is the signature of exactly this confusion.
func collectStepNames(v any) []string {
	switch t := v.(type) {
	case map[string]any:
		var out []string
		for _, k := range []string{"skill", "id", "label"} {
			if s, ok := t[k].(string); ok && s != "" {
				out = append(out, s)
			}
		}
		if skill, _ := t["skill"].(string); skill == "" {
			if p, ok := t["prompt"].(string); ok && p != "" {
				out = append(out, p)
			}
		}
		keys := make([]string, 0, len(t))
		for k := range t {
			if k == "skill" || k == "id" || k == "label" || k == "prompt" {
				continue
			}
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			out = append(out, collectStepNames(t[k])...)
		}
		return out
	case []any:
		var out []string
		for _, vv := range t {
			out = append(out, collectStepNames(vv)...)
		}
		return out
	}
	return nil
}
