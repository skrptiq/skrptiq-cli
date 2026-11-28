package scan

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	exec "github.com/skrptiq/engine/execution"
	"github.com/skrptiq/engine/parse"
	"github.com/skrptiq/engine/storage"
)

// GH#975 — the scan is the canonical gate for skrpt content (K-060): Hub
// asserts shape and never vocabulary, and the engine resolves at runtime, so
// only the scan can refuse an input contract that is wrong before it ships.
//
// The rule being enforced is SKRPT-STRUCTURE-PRINCIPLES § Anti-Patterns 5:
//
//	"Genuinely-optional values may default, but must be declared as optional,
//	 not advertised as inputs that go nowhere."
//
// Optionality is a POSITIVE DECLARATION. It is never inferred — not from prose,
// not from a markdown table, not from emptiness at runtime (K-069).
//
// What these rules are NOT. They would not have caught the package in GH#975's
// title: weekly-report declares both of its inputs `required: false` in prompt
// frontmatter, which is correct, and its failure was entirely consumer-side.
//
// "Prevention for content that does not exist yet" was the framing when these
// shipped. It is TRUE of Rules 2 and 3 and FALSE of Rule 1 (GH#1055). Executing
// the shipped scan over all 97 published packages on 2026-10-14 — dep
// resolution on — found four live Rule 1 ERRORS, each a single-input utility
// whose whole contract lives in a table and in no `inputs:` block:
//
//	big-log-extractor-summariser  log    (workflow execution step 1 wires it)
//	csv-to-narrative-report       csv
//	ioc-defanger                  text
//	word-count-gated-writer       draft  (prompts/revise-draft.md)
//
// That list was in GH#975's approved plan, and the claim that no live instance
// existed was never checked by running the rule. It is Hub content, routed by
// the orchestrator into the GH#1040 family; the figure is recorded here so the
// next reader does not re-derive it. Rule 1 therefore gates live content today.
//
// ── CONTENT ABOUT THE SPEC READS AS CONTENT USING THE SPEC ──────────────────
//
// A prompt that INSTRUCTS an author to declare inputs contains the same
// `{{input.X}}` token as a prompt that CONSUMES one. Nothing in the file
// distinguishes them, so Rule 1 reports the metavariable, and the three live
// instances are both of the catalogue's meta-packages:
//
//	skrpt-builder/prompts/analyse-requirements.md:69     {{input.VAR}}
//	skrptiq-on-skrptiq/prompts/validate-build.md:46      {{input.field_name}}
//	skrptiq-on-skrptiq/prompts/check-publication.md:62   {{input.field_name}}
//
// In each, the token sits inside a sentence about declaring inputs — "what
// `{{input.VAR}}` variables will the generated skrpt collect?", "every
// `{{input.field_name}}` in a prompt body has a matching entry". `VAR` and
// `field_name` are placeholders for a name the READER will choose; they are not
// inputs these prompts consume, and the warnings are false positives.
//
// DO NOT SATISFY THEM BY ADDING `inputs:` ENTRIES. A `VAR` or `field_name` entry
// in either package's frontmatter would declare an input the prompt never reads
// — a Rule 3 defect and a false contract traded for a clean report. The warnings
// are the correct output on this content; the content is also correct. Both
// being true at once is the state here, and it has no fix in the package.
//
// AND THE RULE IS NOT APOLOGISING FOR THIS. Its own message says the scanner
// cannot tell — "if it is a real input, declare it; if it is an example, it needs
// no declaration" — and it is a WARNING rather than an error for exactly that
// reason. The alternative is a heuristic for "a metavariable inside a sentence
// about declaring inputs", whose blind spot would be an undeclared input shipping
// silently. Three known warnings on two packages is the cheaper side of that
// trade, and a scanner that stops reporting a real undeclared input is worse than
// one that over-reports two meta-packages.
//
// The three sites above were verified against a scanner run rather than copied
// forward: a comment naming sites that have moved is worse than no comment.
// ────────────────────────────────────────────────────────────────────────────
const (
	// CodeInputUndeclared fires on a {{input.X}} the runtime will actually
	// resolve, where no reachable node declares X.
	CodeInputUndeclared = "workflow.input_undeclared"
	// CodeNodeInputUndeclared is the same defect reported WITHOUT a workflow
	// (GH#1357). It is a separate code because it is a different claim: the
	// `workflow.` one says the workflow's reach cannot satisfy a reference, and
	// this one says a node's own declaration does not cover a reference in its
	// own body. A consumer keyed on codes must be able to tell them apart, and
	// nothing that reports today changes code or severity because of this one.
	CodeNodeInputUndeclared = "node.input_undeclared"
	// CodeInputOptionalityUndeclared fires on an input declared without a
	// `required:` marker whose only statement of optionality is prose.
	//
	// POPULATION: no live instance, corpus-wide — verified 2026-10-14 by
	// RUNNING this rule over all 97 published packages in skrptiq-hub/examples,
	// not by counting lines. Optionality is declared universally there: 196 of
	// 196 node files carrying an `inputs:` block, and 505 of 505 individual
	// input entries, use an explicit `required:` (447 true / 58 false). The
	// 196 files are 184 in the 97 packages plus 12 in examples/_shared.
	//
	// DERIVED WITH `parse.ReadPackage`, attributing every frontmatter
	// `required:` line to its top-level key. This supersedes a grep-derived
	// "198 of 198 (463 true / 246 false)" cited here until GH#1055, which
	// `grep -rl '^inputs:'` and `grep -c 'required:'` produced by counting two
	// FENCED `inputs:` examples in authoring documentation as declarations
	// (skrpt-authoring-reference, skrpt-builder/assets/node-templates-reference)
	// and 200 `context_params:` keys — a different contract — as input
	// declarations. The conclusion the figure carries is unchanged and now
	// holds per-entry as well as per-file: authors already declare, so silence
	// means undeclared, not optional.
	//
	// The rule's fixture is therefore SYNTHETIC by necessity. It is here
	// because an empty population is what lets the first instance through
	// unnoticed, not because a defect was observed.
	CodeInputOptionalityUndeclared = "workflow.input_optionality_undeclared"
	// CodeInputDeclaredUnused fires on a declaration nothing consumes. WARNING,
	// not error: a shared element legitimately declares input slots a given
	// consuming workflow does not bind (the GH#782 reuse model), so an unused
	// declaration is suspicious rather than wrong.
	CodeInputDeclaredUnused = "workflow.input_declared_unused"

	inputContractRef = "SKRPT-STRUCTURE-PRINCIPLES § Anti-Patterns 5"
)

var (
	// Only FENCED blocks are stripped before detection. Inline backticks are
	// NOT: the corpus writes genuine, load-bearing references inside them —
	// weekly-report's prompt reads "if `{{input.week_start}}` is provided" —
	// so stripping inline code discards the real contract along with the
	// examples. The first draft of this rule did strip them, and Rule 3 then
	// reported weekly-report's two correctly-declared inputs as unused. That
	// false positive is what the corpus assertion exists to catch, and it
	// caught it before this shipped.
	//
	// The teaching examples in skrpt-builder and skrptiq-on-skrptiq
	// (`{{input.VAR}}`, `{{input.*}}`, `{{input.field_name}}`) are separated
	// instead by the table discriminator below, which asks whether the author
	// documented the name as an input of THIS skrpt.
	fencedBlock = regexp.MustCompile("(?s)```.*?```")

	// A legal input identifier. `{{input.*}}` is not one.
	legalInputName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_-]*$`)

	// An `## Inputs` table row: | `{{input.X}}` | Yes/No | ... |. This is how
	// the corpus documents inputs. It is NOT a declaration — but a `Yes`/`No`
	// in the Required column is machine-parseable enough to auto-fix from, and
	// its presence is what makes Rule 2's "the default lives in prose" concrete.
	inputTableRow = regexp.MustCompile(`(?mi)^\s*\|\s*` + "`" + `?\{\{\s*input\.([a-zA-Z_][a-zA-Z0-9_-]*)\s*\}\}` + "`" + `?\s*\|\s*(yes|no)\s*\|`)

	// A prose default statement inside an input's own description.
	proseDefault = regexp.MustCompile(`(?i)\bdefaults?\s*(to|:)`)

	// An EXPLICIT input reference, for the node-local rule only (GH#1357).
	//
	// `exec.DetectInputVariables` is the engine's authority on what the runtime
	// substitutes, and it deliberately returns BARE identifiers too — at run time
	// `{{ROLE}}` is filled from inputs just as `{{input.ROLE}}` is. That is right
	// for the runtime and too broad for this gate: `testdata/template-package` is
	// a fill-in-the-blank prompt whose body carries `{{ROLE}}`, `{{TASK}}` and
	// `{{PERSONA_NAME}}` as authoring markers, and the node-local rule reported all
	// five of them before this filter existed. The reach-based rule never saw that
	// package because it has no workflow — which is the very hole being closed, so
	// closing it inherits the exposure.
	//
	// A node-local finding therefore requires the reference to NAME the namespace.
	// `{{input.X}}` is an unambiguous claim on the input contract; a bare `{{X}}`
	// is not, and with no workflow there is no reach to disambiguate it.
	explicitInputRef = regexp.MustCompile(`\{\{\s*input\.([a-zA-Z_][a-zA-Z0-9_-]*)`)
)

// inputDecl is one `inputs:` entry as the author wrote it.
type inputDecl struct {
	node        string // slug of the node whose frontmatter declares it
	hasRequired bool   // an explicit `required:` key is present
	description string
}

// inputRef is one {{input.X}} the runtime will resolve.
type inputRef struct {
	name string
	node string // slug of the node whose body or execution entry carries it
	file string
}

// stripCode removes fenced blocks only. See fencedBlock on why inline spans stay.
func stripCode(s string) string {
	return fencedBlock.ReplaceAllString(s, "")
}

// checkWorkflowInputs applies the three GH#975 rules to one workflow and
// returns the issues it earned. localSlugs is every slug the package defines
// itself; a package whose execution pairs a local node with a dependency-
// provided one cannot be judged, because a dep node's declaration is not
// visible — DepNodeSummary carries Content but no Metadata.
func checkWorkflowInputs(wf parse.NodeFile, pkg *parse.Package, absPath string) ([]ScanIssue, []string) {
	bySlug := map[string]parse.NodeFile{}
	for _, n := range pkg.Nodes {
		if n.ID != "" {
			bySlug[n.ID] = n
		}
		if n.Title != "" {
			bySlug[strings.ToLower(strings.ReplaceAll(n.Title, " ", "-"))] = n
		}
	}

	// Walk the execution block: collect the nodes this workflow runs, the
	// strings it binds, and whether any step pairs across a dependency edge.
	var reach []parse.NodeFile
	var execStrings []string
	// GH#1266 — PER-NODE, not package-wide. This was a single `splitAcrossDep`
	// bool, and one straddling entry switched Rule 1 off for the WHOLE package,
	// both rungs. It was measured at 42 of 88 workflow-bearing catalogue
	// packages — 48% — and it was found by running a positive control that was
	// checking something else: deleting a real declaration from `weekly-report`
	// produced exit 0 and total silence, not a missed promotion.
	//
	// The flag itself is CORRECT and must not be deleted. A dependency's node
	// content is served with its frontmatter stripped — measured on the wire,
	// from the resolver's own cache of what hub serves: 203 dep nodes, 87 with
	// content, and ZERO carrying an `inputs:` block. So a declaration on the
	// dep half is genuinely unreadable, resolved or not, and reporting its
	// reference would be a false refusal of the GH#1131 / GH#1253 kind.
	//
	// What was wrong was the blast radius. A reference is excused only when the
	// node making it sits in an entry that actually straddles the edge.
	splitNodes := map[string]bool{}
	seen := map[string]bool{}
	if execList, ok := wf.Metadata["execution"].([]any); ok {
		for _, entry := range execList {
			m, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			execStrings = append(execStrings, flattenStrings(m)...)
			var localIDs []string
			var foreign int
			for _, key := range []string{"skill", "prompt"} {
				s, ok := m[key].(string)
				if !ok || s == "" {
					continue
				}
				if n, found := bySlug[s]; found {
					localIDs = append(localIDs, n.ID)
					if !seen[n.ID] {
						seen[n.ID] = true
						reach = append(reach, n)
					}
				} else {
					foreign++
				}
			}
			// A skill here and its prompt over there: the declaration may sit
			// on the half this scan cannot read — so excuse THIS entry's local
			// nodes, and only those. A node reached by an entry that is wholly
			// local is not excused by someone else's dep edge.
			if len(localIDs) > 0 && foreign > 0 {
				for _, id := range localIDs {
					splitNodes[id] = true
				}
			}
		}
	}

	// GH#1131 — `loops` is a SIBLING hoisted key, and until this it was never
	// read at all. Both halves of the walk above keyed on `execution` alone, so
	// everything a loop declares was invisible:
	//
	//   - `inputExpression` consumes a workflow input and is the engine's
	//     MANDATED form for a for_each (`storage/validate.go` requires `{{` in
	//     it), so `{{input.X}}` there is real consumption. Rule 3 reported such
	//     an input as unconsumed, which under hub's publish policy BLOCKS the
	//     package — `batch-competitor-analysis` and `profile-social-pipeline`
	//     could not be published for content that is correct.
	//   - a loop's `steps` and `verifier` name NODES whose bodies consume inputs
	//     like any other step's. Unless the same node also appeared in
	//     `execution`, its `{{input.X}}` was unreachable too.
	//
	// FLATTENED RATHER THAN FIELD-PICKED, and that is not laziness: the engine
	// accepts BOTH `inputExpression` and `input_expression`
	// (`storage/resolve.go:395-397`), plus `max_iterations`/`maxIterations` and
	// `fresh_context_per_iteration`/`freshContextPerIteration`. Keying on the one
	// spelling this issue happens to name would miss the other and re-earn the
	// bug. Flattening every string in the loop declaration covers both spellings
	// and anything a later loop field adds.
	if loopList, ok := wf.Metadata["loops"].([]any); ok {
		for _, entry := range loopList {
			m, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			execStrings = append(execStrings, flattenStrings(m)...)

			// Loop members are node slugs, exactly like a `skill:`/`prompt:`
			// entry, so they belong in `reach` for the same reason: their bodies
			// are sent to the model and the inputs they name are consumed.
			for _, slug := range loopMemberSlugs(m) {
				n, found := bySlug[slug]
				if !found {
					continue // dep-provided or absent; the step-ref clause owns that
				}
				if !seen[n.ID] {
					seen[n.ID] = true
					reach = append(reach, n)
				}
			}
		}
	}

	// Declarations: every reachable node's `inputs:` block, plus the workflow's.
	declared := map[string]inputDecl{}
	for _, n := range append(append([]parse.NodeFile{}, reach...), wf) {
		for name, d := range declsOf(n) {
			if _, exists := declared[name]; !exists {
				declared[name] = d
			} else if d.hasRequired {
				declared[name] = d // an explicit marker wins over a bare entry
			}
		}
	}

	// Load-bearing references: a prompt/skill body is sent to the model, and an
	// execution-block string is resolved by the runner. The workflow node's own
	// body is documentation for a human — a reference that exists only in its
	// `## Inputs` table is a documentation fact and cannot by itself fail a
	// package (per K-069, a table is not a declaration in either direction).
	var refs []inputRef
	for _, n := range reach {
		if n.Type == "workflow" {
			continue
		}
		for _, name := range exec.DetectInputVariables([]string{stripCode(n.Content)}) {
			refs = append(refs, inputRef{name: name, node: n.ID, file: relPath(absPath, n.FilePath)})
		}
	}
	for _, name := range exec.DetectInputVariables(execStrings) {
		refs = append(refs, inputRef{name: name, node: wf.ID, file: relPath(absPath, wf.FilePath)})
	}
	// `from_input: <name>` consumes a workflow input by BARE NAME, never as a
	// {{input.X}} expression — the explicit binding form the data-flow mandate
	// requires (SKRPT-STRUCTURE-PRINCIPLES § Data-Flow, GH#750/#755). Missing it
	// reported an input consumed only through a binding as unused; the existing
	// from_input suite caught that, which is what it was written for ("the K-034
	// trap: an input consumed ONLY via a binding never appears as an input
	// expression").
	for _, name := range collectFromInput(wf.Metadata["execution"]) {
		refs = append(refs, inputRef{name: name, node: wf.ID, file: relPath(absPath, wf.FilePath)})
	}

	wfFile := relPath(absPath, wf.FilePath)
	// GH#1287 — the Inputs table is read from the WORKFLOW AND EVERY REACHABLE EXECUTING NODE,
	// not from the workflow body alone.
	//
	// It used to be `parseInputTable(wf.Content)`, which left a latent hole: a reference documented
	// in a skill or prompt stayed a `warning`, and since GH#1256 a warning passes the publish gate.
	// The whole catalogue population was measured before widening — 8 table-bearing files outside
	// `workflows/`, of which 7 are `skill` nodes with real input names and exactly ONE is the
	// template row in `skrpt-builder/assets/node-templates-reference.md`.
	//
	// WHY THE EXCLUSION IS BY TYPE AND NOT BY CONTENT. That one template row reads
	// `| {{input.VAR}} | Yes | DESCRIPTION | EXAMPLE |`, so a content heuristic on the literal cells
	// was the obvious move and it is the wrong one: it asks the scanner to judge prose, it breaks
	// the day a real description reads "DESCRIPTION", and it misses any template that picks
	// different placeholders. The structural fact is better and it does not rot —
	//
	//	AN ASSET IS NOT AN EXECUTING NODE. It is never named as `skill:` or `prompt:` in an
	//	execution entry, declares no contract and runs no step, so a table in it CANNOT be a
	//	statement about this skrpt's input contract.
	//
	// The same holds for `document`, `source` and `service`. That is a property of the authoring
	// model rather than of today's catalogue. The corroboration is that the template file's own
	// `inputs:` frontmatter is ALSO a blank form (`INPUT_NAME` / `label: "LABEL"`): the whole file
	// teaches the authoring constructs, so type is the honest signal and the cell contents are a
	// symptom.
	//
	// Promoting on that row would refuse the package that documents the table format — the fourth
	// time that shape has come up (GH#1194, GH#1213, GH#1249, GH#1287).
	//
	// Measured delta before committing: 99 packages, 3 Rule 1 warnings, 0 flip under this scope and
	// 1 flips under a naive widening over every node type (that row). A pure REACH change.
	//
	// Scoped to `reach` rather than every node in the package, matching how `declared` is built: a
	// table in a node this workflow never runs is not evidence about this workflow's contract.
	tableRequired := map[string]bool{}
	for _, n := range reach {
		if !nodeCanStateInputContract(n.Type) {
			continue
		}
		for name, required := range parseInputTable(n.Content) {
			if _, exists := tableRequired[name]; !exists {
				tableRequired[name] = required
			}
		}
	}
	// The workflow goes LAST and overwrites, so its row wins on conflict — mirroring the engine's
	// `mergeMetadata` rule that the explicit block beats a hoisted one. Reusing an existing
	// precedence rule is why two places will not later disagree about which row is authoritative.
	//
	// BUT BE PRECISE ABOUT WHAT THAT BUYS TODAY: both readers of this map test EXISTENCE only —
	// `if _, documented := tableRequired[name]` in Rule 1 and `_, inTable :=` in Rule 2 — so the
	// `required` VALUE is currently never consumed and the precedence is unobservable. It is
	// written this way so that the ordering is already right if the value becomes load-bearing,
	// not because anything reads it now. Claiming an observable precedence here would be a comment
	// that explains a behaviour the code does not have.
	for name, required := range parseInputTable(wf.Content) {
		tableRequired[name] = required
	}

	// GH#1253 — RULE 1 AND RULE 3 ASK OPPOSITE QUESTIONS OF THE SAME TEXT, and
	// until this they shared one answer.
	//
	// `refs` above is built from `stripCode(n.Content)`, which removes FENCED
	// blocks. That is right for Rule 1, which asks "is this REFERENCE declared?":
	// a `{{input.X}}` inside a fence may be a teaching example, and counting it
	// would refuse a package for an example. The `fencedBlock` comment records
	// that an earlier draft stripped inline spans too and falsely refused
	// `weekly-report`'s two correctly-declared inputs.
	//
	// It is wrong for Rule 3, which asks "is this DECLARATION consumed?": a
	// `{{input.X}}` inside a fence IS consumption — the runtime substitutes it
	// wherever it appears. So stripping made Rule 3 report a consumed input as
	// unused, and under hub's publish policy that BLOCKS the package.
	//
	// It is the natural case, which is what made it bad: a prompt that feeds CODE
	// to a model puts the placeholder in a fence. `documentation-pipeline` and
	// `creator-profile-builder` were the only two packages in the catalogue with
	// ANY unused-input warning, and all three of their warnings were this.
	//
	// So the sets are separate. `consumed` is Rule 3's and includes fenced
	// references; `refs` stays exactly as it was and Rule 1 is untouched. A single
	// widened set would have leaked the leniency into Rule 1, where it would have
	// stopped refusing a reference that exists only as an example.
	consumed := map[string]bool{}
	for _, n := range reach {
		if n.Type == "workflow" {
			continue
		}
		// UNSTRIPPED, deliberately — this is the one place fenced content counts.
		for _, name := range exec.DetectInputVariables([]string{n.Content}) {
			consumed[name] = true
		}
	}

	var issues []ScanIssue
	referenced := map[string]bool{}
	reported := map[string]bool{}

	// Rule 1 — a load-bearing reference with no declaration.
	for _, r := range refs {
		referenced[r.name] = true
		// Everything Rule 1 can see is consumption too: execution-block strings,
		// `from_input:` bindings and the stripped bodies. `consumed` is the
		// superset, and the fenced half was added above.
		consumed[r.name] = true
		if !legalInputName.MatchString(r.name) {
			continue // `{{input.*}}` and friends are documentation, not a contract
		}
		if _, ok := declared[r.name]; ok {
			continue
		}
		if splitNodes[r.node] {
			continue // this node's entry straddles a dep edge: the declaration
			// may live in the half whose frontmatter is not served
		}
		if reported[r.name] {
			continue
		}
		reported[r.name] = true
		// The discriminator between "this skrpt's input contract is wrong" and
		// "this is a literal example in prose about authoring": did the author
		// document the name as an input of THIS skrpt? A documented-but-
		// undeclared input is Anti-Pattern 5 exactly — the contract exists, in
		// the one place that does not execute — and it auto-fixes from the
		// table row. A name documented nowhere is a warning: it may be a
		// teaching example (`{{input.VAR}}` in skrpt-builder's checklists), and
		// a gate must not refuse the content that documents the gate.
		//
		// The table is NEVER read as a declaration (K-069). It is read as
		// evidence of the author's intent, to choose a severity.
		severity := storage.SeverityWarning
		detail := "; it is documented nowhere either — if it is a real input, declare it; if it is an example, it needs no declaration"
		if _, documented := tableRequired[r.name]; documented {
			severity = storage.SeverityError
			detail = "; the Inputs table documents it, but a table is not a declaration — add it to that node's inputs: block with required: true|false"
		}
		issues = append(issues, ScanIssue{
			File:     r.file,
			NodeSlug: r.node,
			ValidationIssue: storage.ValidationIssue{
				Code:     CodeInputUndeclared,
				Severity: severity,
				Message: fmt.Sprintf(
					"input %q is referenced by node %q but never declared%s",
					r.name, r.node, detail),
				ContractRef:    inputContractRef,
				Field:          "inputs." + r.name,
				ReferencedSlug: r.name,
				ReferenceKind:  "input",
			},
		})
	}

	// Rule 2 — declared, no `required:` marker, optionality stated only in prose.
	for _, name := range sortedKeys(declared) {
		d := declared[name]
		if d.hasRequired {
			continue
		}
		_, inTable := tableRequired[name]
		if !inTable && !proseDefault.MatchString(d.description) {
			continue
		}
		issues = append(issues, ScanIssue{
			File:     wfFile,
			NodeSlug: d.node,
			ValidationIssue: storage.ValidationIssue{
				Code:     CodeInputOptionalityUndeclared,
				Severity: storage.SeverityError,
				Message: fmt.Sprintf(
					"input %q states its optionality in prose but declares no required: marker on node %q — optionality must be declared, not inferred",
					name, d.node),
				ContractRef:    inputContractRef,
				Field:          "inputs." + name + ".required",
				ReferencedSlug: name,
				ReferenceKind:  "input",
			},
		})
	}

	// Rule 3 — a declaration nothing consumes.
	//
	// THIS IS A GATE, AND THE SEVERITY NOW SAYS SO (GH#1249). This comment once
	// read "Warning, not a gate"; when that was shown false it was corrected to
	// say that `warning` described the label and not the consequence. Both
	// boundaries gate: `scan.go` returns exit 1 whenever WarnCount is non-zero,
	// and hub's `caller-release.yml` greps for `(ERROR|WARNING)` and fails the
	// release under the stated policy "any engine finding blocks — errors and
	// warnings alike". So the label is `error`, label and effect agree, and no
	// reader has to reconcile the two. There is nothing left here to re-discover.
	//
	// THE PROMOTION IS RULE 3'S ALONE — a deliberate scope, not an oversight.
	// The catalogue produced ZERO Rule 3 findings when it was made, so it changed
	// no author's message; that empty population is what made a pure label change
	// safe. The other warning emitters point the OTHER way and must stay warnings:
	//   - Rule 1's two-level ladder (above, where a name documented in the
	//     `## Inputs` table promotes to error) uses `warning` to mean "probably
	//     an example, not a contract" — real semantic work, and the levels are
	//     chosen from the author's evidence of intent.
	//   - `scan.cross_package_edge` says "will be resolved at runtime" —
	//     explicitly benign.
	//   - `parseToStorageSeverity` maps what a MANIFEST declared; promoting it
	//     would override the author.
	// That those three gate a publish anyway is hub's POLICY, not their labels —
	// tracked as GH#1256, and not a reason to relabel them here. Promoting them
	// would destroy the distinction Rule 1's ladder exists to draw.
	for _, name := range sortedKeys(declared) {
		if consumed[name] {
			continue
		}
		d := declared[name]
		issues = append(issues, ScanIssue{
			File:     wfFile,
			NodeSlug: d.node,
			ValidationIssue: storage.ValidationIssue{
				Code:     CodeInputDeclaredUnused,
				Severity: storage.SeverityError,
				Message: fmt.Sprintf(
					"input %q is declared on node %q but no step consumes it — either wire it or remove the declaration",
					name, d.node),
				ContractRef:    inputContractRef,
				Field:          "inputs." + name,
				ReferencedSlug: name,
				ReferenceKind:  "input",
			},
		})
	}

	return issues, sortedKeys(splitNodes)
}

// loopMemberSlugs returns the node slugs one loop declaration names, from the
// keys the engine itself reads (`storage/resolve.go`): `steps` for the body and
// `verifier` for an until_pass check. A member may be a node-less execution step
// keyed by id (GH#831) — those resolve to no node here and are skipped, because
// the step-ref clause owns whether they exist.
func loopMemberSlugs(loop map[string]any) []string {
	var out []string
	if steps, ok := loop["steps"].([]any); ok {
		for _, s := range steps {
			if slug, ok := s.(string); ok && slug != "" {
				out = append(out, slug)
			}
		}
	}
	if v, ok := loop["verifier"].(string); ok && v != "" {
		out = append(out, v)
	}
	return out
}

// declsOf reads one node's `inputs:` block as the author wrote it.
// checkNodeLocalInputs is Rule 1 with no workflow in it (GH#1357).
//
// WHY IT EXISTS. `scan.go` gated the three input rules on the PACKAGE containing a
// workflow node, so a package shipping the same prompt with no local runner was
// graded on nothing: byte-identical content gave exit 1 with a workflow and exit 0
// without. Measured on the live catalogue, 145 of 234 packages have no workflow
// node — 134 of the 135 in `_shared/` — so that gate was switched off for most of
// the corpus, and hub's "zero of 135 shared objects fail the scan" was largely
// those 134 never reaching a rule.
//
// THE SUBJECT IS THE NODE, NOT THE PACKAGE, and it already existed:
// `nodeCanStateInputContract` (GH#1287) answers "does this node execute?", which is
// the right question. The package-level gate asked "does this package contain a
// workflow?", which is not.
//
// WHY IT IS NOT THE REACH-BASED RULE UNGATED. `checkWorkflowInputs` is DEFINED in
// terms of reach — it walks the execution block for the nodes a workflow runs, the
// strings it binds and the per-entry dep-straddle exemption. A prompt node has no
// execution block, so calling that function per-node would find every reference
// unsatisfiable and report every `{{input.X}}` in every library bundle. This rule
// asks a strictly local question instead: does ANY node in this package declare the
// input a node's body references? No reach, so no reach-based false refusals — and
// package-wide rather than per-node, for the reason recorded on the declared set
// below, which the existing corpus established before this shipped.
//
// TWO SUPPRESSIONS, BOTH LOAD-BEARING:
//
//   - `reported` — the input NAMES the reach-based pass already emitted, so a
//     workflow-bearing package gains no duplicate finding. This rule is additive,
//     and the granularity matches that pass rather than being finer than it.
//   - `depStraddled` — the nodes that pass EXCUSED because their execution entry
//     straddles a dependency edge, where the declaration may live in the half whose
//     frontmatter is not served (GH#1266). Re-reporting those here would recreate
//     the GH#1131 / GH#1253 false positives from a different direction.
//
// The severity discriminator is Rule 1's, unchanged and for its reason: a name the
// Inputs table documents is Anti-Pattern 5 and an error; a name documented nowhere
// may be a teaching example, and a gate must not refuse the content that documents
// the gate.
func checkNodeLocalInputs(pkg *parse.Package, absPath string, reported map[string]bool, depStraddled map[string]bool) []ScanIssue {
	// A DECLARED TEMPLATE PACKAGE IS EXEMPT, because it has already said what its
	// markers are. `manifest.template: true` (GH#374) is the author declaring the
	// package is fill-in-the-blank, and GH#524 already suppresses the engine's
	// unknown-namespace warnings on that basis. Measured on the live catalogue, the
	// six `templates-*` packages carry that flag and two of them — `templates-prompts`
	// and `templates-skills` — accounted for 14 of the 15 warnings this rule produced
	// before this clause: `{{input.specification}}`, `{{input.tone}}` and the rest are
	// the blanks an author fills in, not a contract the package makes.
	//
	// This is the same principle the invocation half of GH#1357 still lacks: key on a
	// declaration the author made, not on a shape the scanner infers. Here the
	// declaration already exists, so this half gets it for free.
	if pkg.Manifest.Template {
		return nil
	}

	// THE DECLARED SET IS PACKAGE-WIDE, NOT PER-NODE, and the existing corpus is
	// what established that. The first draft asked whether the node carrying a
	// reference declared it itself, and `valid-package` failed immediately:
	// `spaced-ref-prompt` reads `{{input.topic}}` while `example-prompt` — a
	// sibling in the same package — declares `topic`. Inputs are resolved for the
	// run, not for one node, and GH#1287 already reads the Inputs table from every
	// executing node rather than one. So a declaration anywhere in the package
	// satisfies a reference anywhere in it; asking per-node would have reported a
	// correct package as broken, which is the false-refusal direction this rule
	// exists to avoid rather than to create.
	declared := map[string]inputDecl{}
	tableRequired := map[string]bool{}
	for _, n := range pkg.Nodes {
		if !nodeCanStateInputContract(n.Type) {
			continue
		}
		for name, d := range declsOf(n) {
			declared[name] = d
		}
		for name, req := range parseInputTable(n.Content) {
			tableRequired[name] = req
		}
	}

	var issues []ScanIssue
	seen := map[string]bool{}

	for _, n := range pkg.Nodes {
		if !nodeCanStateInputContract(n.Type) || n.Type == "workflow" {
			// A workflow's own body is documentation for a human — the
			// reach-based rule skips it for the same reason (see the comment on
			// `refs` in checkWorkflowInputs). Reference material cannot state a
			// contract at all.
			continue
		}
		if depStraddled[n.ID] {
			continue
		}

		for _, m := range explicitInputRef.FindAllStringSubmatch(stripCode(n.Content), -1) {
			name := m[1]
			if !legalInputName.MatchString(name) {
				continue // `{{input.*}}` and friends are documentation, not a contract
			}
			if _, ok := declared[name]; ok {
				continue
			}
			// ONE FINDING PER INPUT NAME, PER PACKAGE — the granularity the
			// reach-based rule already uses (`reported[r.name]`, not per node).
			// `skrpt-builder` is what established it: `{{input.VAR}}` is a
			// teaching metavariable appearing in two nodes, the reach-based pass
			// reports it once for `analyse-requirements`, and keying this rule on
			// (node, name) emitted a second finding for `validate-structure` —
			// the same authoring fact counted twice. One mistake, one finding.
			if seen[name] || reported[name] {
				continue
			}
			seen[name] = true

			severity := storage.SeverityWarning
			detail := "; it is documented nowhere either — if it is a real input, declare it; if it is an example, it needs no declaration"
			if _, documented := tableRequired[name]; documented {
				severity = storage.SeverityError
				detail = "; the Inputs table documents it, but a table is not a declaration — add it to this node's inputs: block with required: true|false"
			}

			issues = append(issues, ScanIssue{
				File:     relPath(absPath, n.FilePath),
				NodeSlug: n.ID,
				ValidationIssue: storage.ValidationIssue{
					Code:     CodeNodeInputUndeclared,
					Severity: severity,
					Message: fmt.Sprintf(
						"input %q is referenced by node %q but nothing in this package declares it%s",
						name, n.ID, detail),
					ContractRef:    inputContractRef,
					Field:          "inputs." + name,
					ReferencedSlug: name,
					ReferenceKind:  "input",
				},
			})
		}
	}

	return issues
}

func declsOf(n parse.NodeFile) map[string]inputDecl {
	out := map[string]inputDecl{}
	raw, ok := n.Metadata["inputs"].(map[string]any)
	if !ok {
		return out
	}
	for name, v := range raw {
		d := inputDecl{node: n.ID}
		if m, ok := v.(map[string]any); ok {
			_, d.hasRequired = m["required"]
			if s, ok := m["description"].(string); ok {
				d.description = s
			}
		}
		out[name] = d
	}
	return out
}

// parseInputTable reads the `## Inputs` table's Required column. The table is
// documentation, never a declaration — but a `Yes`/`No` cell is unambiguous
// enough to auto-fix from, which is the one place prose may inform a fix.
// nodeCanStateInputContract reports whether a node of this type can make a statement about the
// skrpt's input contract (GH#1287).
//
// `workflow`, `skill` and `prompt` execute: they are named in execution entries, their bodies are
// sent to the model or resolved by the runner, and `{{input.X}}` in them is substituted at runtime.
// `asset`, `document`, `source` and `service` are reference material — nothing runs them, so an
// Inputs table in one is documentation ABOUT skrpts rather than a declaration BY one.
//
// Unknown types are excluded. A new node type should have to opt in by name rather than silently
// gain authority over a publish gate.
func nodeCanStateInputContract(nodeType string) bool {
	switch nodeType {
	case "workflow", "skill", "prompt":
		return true
	default:
		return false
	}
}

func parseInputTable(body string) map[string]bool {
	out := map[string]bool{}
	for _, m := range inputTableRow.FindAllStringSubmatch(body, -1) {
		out[m[1]] = strings.EqualFold(m[2], "yes")
	}
	return out
}

// collectFromInput returns every `from_input:` value nested anywhere inside v.
func collectFromInput(v any) []string {
	switch t := v.(type) {
	case map[string]any:
		var out []string
		for k, vv := range t {
			if k == "from_input" {
				if s, ok := vv.(string); ok && s != "" {
					out = append(out, s)
					continue
				}
			}
			out = append(out, collectFromInput(vv)...)
		}
		return out
	case []any:
		var out []string
		for _, vv := range t {
			out = append(out, collectFromInput(vv)...)
		}
		return out
	}
	return nil
}

// flattenStrings returns every string value nested anywhere inside v, so an
// execution entry's bindings and context values are searched as well as its
// top-level fields.
func flattenStrings(v any) []string {
	switch t := v.(type) {
	case string:
		return []string{t}
	case map[string]any:
		var out []string
		for _, vv := range t {
			out = append(out, flattenStrings(vv)...)
		}
		return out
	case []any:
		var out []string
		for _, vv := range t {
			out = append(out, flattenStrings(vv)...)
		}
		return out
	}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
