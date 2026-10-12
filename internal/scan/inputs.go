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
// These rules are prevention for content that does not exist yet.
const (
	// CodeInputUndeclared fires on a {{input.X}} the runtime will actually
	// resolve, where no reachable node declares X.
	CodeInputUndeclared = "workflow.input_undeclared"
	// CodeInputOptionalityUndeclared fires on an input declared without a
	// `required:` marker whose only statement of optionality is prose.
	//
	// POPULATION: no live instance as of 2026-10-12, corpus-wide — measured
	// over the 97 published packages in skrptiq-hub/examples, where 198 of the
	// 198 node files carrying an `inputs:` block use an explicit `required:`.
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
func checkWorkflowInputs(wf parse.NodeFile, pkg *parse.Package, absPath string) []ScanIssue {
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
	splitAcrossDep := false
	seen := map[string]bool{}
	if execList, ok := wf.Metadata["execution"].([]any); ok {
		for _, entry := range execList {
			m, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			execStrings = append(execStrings, flattenStrings(m)...)
			var local, foreign int
			for _, key := range []string{"skill", "prompt"} {
				s, ok := m[key].(string)
				if !ok || s == "" {
					continue
				}
				if n, found := bySlug[s]; found {
					local++
					if !seen[n.ID] {
						seen[n.ID] = true
						reach = append(reach, n)
					}
				} else {
					foreign++
				}
			}
			// A skill here and its prompt over there: the declaration may sit
			// on the half this scan cannot read.
			if local > 0 && foreign > 0 {
				splitAcrossDep = true
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
	tableRequired := parseInputTable(wf.Content)

	var issues []ScanIssue
	referenced := map[string]bool{}
	reported := map[string]bool{}

	// Rule 1 — a load-bearing reference with no declaration.
	for _, r := range refs {
		referenced[r.name] = true
		if !legalInputName.MatchString(r.name) {
			continue // `{{input.*}}` and friends are documentation, not a contract
		}
		if _, ok := declared[r.name]; ok {
			continue
		}
		if splitAcrossDep {
			continue // the declaration may live in an unreadable dep node
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

	// Rule 3 — a declaration nothing consumes. Warning, not a gate.
	for _, name := range sortedKeys(declared) {
		if referenced[name] {
			continue
		}
		d := declared[name]
		issues = append(issues, ScanIssue{
			File:     wfFile,
			NodeSlug: d.node,
			ValidationIssue: storage.ValidationIssue{
				Code:     CodeInputDeclaredUnused,
				Severity: storage.SeverityWarning,
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

	return issues
}

// declsOf reads one node's `inputs:` block as the author wrote it.
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
