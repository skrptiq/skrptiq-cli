package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	exec "github.com/skrptiq/engine/execution"
	"github.com/skrptiq/engine/storage"
)

// GH#975 — `skrptiq run` refused an input the author had declared OPTIONAL.
//
// The declaration was never missing. `inputs: X: required: false` in node
// frontmatter is parsed, hoisted to node metadata, and carried into the plan by
// collectInputMeta as a three-state `*bool` on plan.InputMeta. The published
// weekly-report package declares BOTH its inputs that way, and 198 of the 198
// node files in the published corpus that carry an `inputs:` block use an
// explicit `required:`.
//
// The CLI simply never looked: the preflight iterated the flat
// plan.InputVariables and consulted no declaration at all, so every input was
// mandatory. These tests assert the gate now reads the author's declaration —
// and, as importantly, that it still gates everything the author did NOT
// declare optional.

// writeInputsDB builds a one-step local.template workflow whose skill node
// declares `inputs:` exactly as a published package does.
func writeInputsDB(t *testing.T, inputsJSON string, content string) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "inputs.db")
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	wfMeta := `{"execution":[{"skill":"only-step","step_type":"local.template"}]}`
	if err := db.CreateNode("wf", "workflow", "Inputs WF", nil, nil, gateStrp(wfMeta), gateStrp("inputs-wf"), nil); err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	var skillMeta *string
	if inputsJSON != "" {
		skillMeta = gateStrp(`{"inputs":` + inputsJSON + `}`)
	}
	if err := db.CreateNode("s1", "skill", "Only Step", nil, gateStrp(content), skillMeta, gateStrp("only-step"), nil); err != nil {
		t.Fatalf("create skill: %v", err)
	}
	if err := db.CreateEdge("es1", "wf", "s1", "uses", gateIntp(1)); err != nil {
		t.Fatalf("create edge: %v", err)
	}
	return dbPath
}

const optionalInput = `{"week_start":{"label":"Week Start Date","required":false,"type":"text"}}`
const requiredInput = `{"week_start":{"label":"Week Start Date","required":true,"type":"text"}}`
const noMarkerInput = `{"week_start":{"label":"Week Start Date","type":"text"}}`

// The headline regression: an input the author declared optional must not be
// demanded. Asserted through the real `Run` entry point, so the preflight, the
// plan build and the execution all take part.
func TestGH975_DeclaredOptionalInput_RunsWithoutIt(t *testing.T) {
	db := writeInputsDB(t, optionalInput, "Range: {{input.week_start}} END")

	code := Run([]string{"Inputs WF"}, db)
	if code != ExitOK {
		t.Fatalf("Run exit = %d, want ExitOK (%d) — a declared-optional input was demanded", code, ExitOK)
	}
}

// The other half of the three-state: an input the author declared REQUIRED is
// still gated at preflight, before a token is spent.
func TestGH975_DeclaredRequiredInput_StillGates(t *testing.T) {
	db := writeInputsDB(t, requiredInput, "Range: {{input.week_start}} END")

	code := Run([]string{"Inputs WF"}, db)
	if code != ExitBadArgs {
		t.Fatalf("Run exit = %d, want ExitBadArgs (%d) — a declared-required input must gate", code, ExitBadArgs)
	}
}

// An input with NO declaration is gated, not treated as optional. Inferring
// optionality from the absence of a marker is the inference Anti-Pattern 5
// forbids, and the published packages with no `inputs:` block at all are
// single-input utilities where the undeclared input is the whole payload.
func TestGH975_UndeclaredInput_IsGatedNotAssumedOptional(t *testing.T) {
	for name, inputs := range map[string]string{
		"no inputs block at all":  "",
		"declared with no marker": noMarkerInput,
	} {
		t.Run(name, func(t *testing.T) {
			db := writeInputsDB(t, inputs, "Range: {{input.week_start}} END")
			code := Run([]string{"Inputs WF"}, db)
			if code != ExitBadArgs {
				t.Errorf("Run exit = %d, want ExitBadArgs (%d) — silence is not a declaration of optionality", code, ExitBadArgs)
			}
		})
	}
}

// Supplying an optional input still works — the gate change must not swallow a
// value the caller did pass.
func TestGH975_OptionalInputSupplied_IsUsed(t *testing.T) {
	db := writeInputsDB(t, optionalInput, "Range: {{input.week_start}} END")

	code := Run([]string{"--input", "week_start=2026-10-05", "Inputs WF"}, db)
	if code != ExitOK {
		t.Fatalf("Run exit = %d, want ExitOK (%d)", code, ExitOK)
	}
}

// The declaration reaches the plan at all — the fact the whole fix rests on.
// If collectInputMeta ever stops carrying `required`, this fails here rather
// than as a mysterious change of gate behaviour.
func TestGH975_PlanCarriesTheDeclaration(t *testing.T) {
	db := writeInputsDB(t, optionalInput, "Range: {{input.week_start}} END")

	engine := OpenEngine(db)
	if engine == nil {
		t.Fatal("OpenEngine returned nil")
	}
	defer engine.Close()

	node, err := engine.FindNodeByTitle("Inputs WF")
	if err != nil || node == nil {
		t.Fatalf("find workflow: %v (node=%v)", err, node)
	}
	plan, err := engine.BuildPlan(node.ID)
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}

	if len(plan.InputVariables) != 1 || plan.InputVariables[0] != "week_start" {
		t.Errorf("plan.InputVariables = %v, want [week_start]", plan.InputVariables)
	}
	meta, ok := plan.InputMeta["week_start"]
	if !ok {
		t.Fatalf("plan.InputMeta has no entry for week_start; keys = %v", inputMetaKeys(plan))
	}
	if meta.Required == nil {
		t.Fatal("plan.InputMeta[week_start].Required is nil — the author's `required: false` did not reach the plan")
	}
	if *meta.Required {
		t.Errorf("plan.InputMeta[week_start].Required = true, want false")
	}
	if meta.Label != "Week Start Date" {
		t.Errorf("label = %q, want %q", meta.Label, "Week Start Date")
	}
}

// Unit coverage of the three-state itself, so the rule is pinned independently
// of any database.
func TestGH975_InputIsDeclaredOptional_ThreeState(t *testing.T) {
	yes, no := true, false
	tests := []struct {
		name string
		meta map[string]exec.InputMeta
		want bool
	}{
		{"required: false → optional", map[string]exec.InputMeta{"x": {Required: &no}}, true},
		{"required: true → gated", map[string]exec.InputMeta{"x": {Required: &yes}}, false},
		{"declared, no marker → gated", map[string]exec.InputMeta{"x": {Label: "X"}}, false},
		{"not in InputMeta → gated", map[string]exec.InputMeta{}, false},
		{"nil InputMeta → gated", nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			plan := &exec.ExecutionPlan{InputMeta: tc.meta}
			if got := inputIsDeclaredOptional(plan, "x"); got != tc.want {
				t.Errorf("inputIsDeclaredOptional = %v, want %v", got, tc.want)
			}
		})
	}
}

func inputMetaKeys(plan *exec.ExecutionPlan) []string {
	var out []string
	for k := range plan.InputMeta {
		out = append(out, k)
	}
	return out
}

// Guard: the preflight must keep reading the declaration. A future edit that
// drops back to iterating plan.InputVariables alone would pass every test above
// that supplies a value, so this asserts the call site directly.
func TestGH975_PreflightConsultsInputMeta(t *testing.T) {
	src, err := os.ReadFile("run.go")
	if err != nil {
		t.Fatalf("read run.go: %v", err)
	}
	if !strings.Contains(string(src), "inputIsDeclaredOptional(plan, varName)") {
		t.Error("run.go's input preflight no longer consults the author's declaration — GH#975 regressed")
	}
}
