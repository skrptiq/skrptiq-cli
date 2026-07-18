package toolobject

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	exec "github.com/skrptiq/engine/execution"
)

// K-057 is the LOCKED schema (orchestrator-owned). This pins the Go struct's
// JSON tags to the exact field list — a rename fails here rather than silently
// diverging from the app/web render object.
func TestToolObjectJSONTags(t *testing.T) {
	full := ToolObject{
		Kind: "builtin", Operation: "op", Title: "T", Description: "D",
		Category: "text", ReadOnly: true, Offline: true, TokenCost: 0,
		StepID: "s1", Params: map[string]string{"k": "v"},
	}
	b, _ := json.Marshal(full)
	var m map[string]json.RawMessage
	_ = json.Unmarshal(b, &m)

	got := make([]string, 0, len(m))
	for k := range m {
		got = append(got, k)
	}
	want := []string{"kind", "operation", "title", "description", "category", "readOnly", "offline", "tokenCost", "stepId", "params"}
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("K-057 JSON tags drifted:\n got=%v\nwant=%v", got, want)
	}
}

func TestFromDescriptor(t *testing.T) {
	d := exec.BuiltinDescriptor{Name: "json-filter", Title: "JSON filter", Description: "…", Category: "data", Offline: true, TokenCost: 0}
	o := FromDescriptor(d)
	if o.Kind != KindBuiltin || o.Operation != "json-filter" || o.Title != "JSON filter" || o.Category != "data" {
		t.Errorf("FromDescriptor fields wrong: %+v", o)
	}
	if !o.ReadOnly || !o.Offline || o.TokenCost != 0 {
		t.Errorf("builtin must be readOnly + offline + 0 tokens: %+v", o)
	}
	if o.StepID != "" || o.Params != nil {
		t.Errorf("registry form must carry no instance data: %+v", o)
	}
}

func TestFromStep_KnownOp(t *testing.T) {
	r := NewRegistry()
	// pick a real op from the registry
	all := exec.Builtins()
	op := all[0].Name
	o := r.FromStep(op, "step-1", map[string]string{"operation": op, "min": "150"})
	if o.Operation != op || o.StepID != "step-1" || !o.ReadOnly || !o.Offline {
		t.Errorf("instance form wrong: %+v", o)
	}
	if o.Title == "" {
		t.Errorf("known op should carry a title from the registry")
	}
	if _, leaked := o.Params["operation"]; leaked {
		t.Error("params must not include the 'operation' key")
	}
	if o.Params["min"] != "150" {
		t.Errorf("params should carry instance context: %+v", o.Params)
	}
}

func TestFromStep_UnknownOp_GenericFallback(t *testing.T) {
	r := NewRegistry()
	o := r.FromStep("no-such-op", "s", nil)
	if o.Kind != KindBuiltin || !o.ReadOnly || !o.Offline || o.TokenCost != 0 {
		t.Errorf("fallback must still be a read-only offline builtin: %+v", o)
	}
	if o.Label() == "" {
		t.Error("fallback label must never be blank")
	}
}

// Every op the engine exposes must derive cleanly with a title (the app-parity
// guarantee: no builtin renders untitled).
func TestRegistryCoverage(t *testing.T) {
	r := NewRegistry()
	for _, d := range exec.Builtins() {
		o := r.FromStep(d.Name, "", nil)
		if o.Title == "" {
			t.Errorf("op %q derived with no title", d.Name)
		}
		if !o.ReadOnly || !o.Offline || o.TokenCost != 0 {
			t.Errorf("op %q must be readOnly/offline/0-tokens: %+v", d.Name, o)
		}
	}
}

func TestByPosition(t *testing.T) {
	op := exec.Builtins()[0].Name
	plan := &exec.ExecutionPlan{
		PositionGroups: []exec.PositionGroup{
			{Position: 0, Steps: []exec.PlannedStep{{Position: 0, StepType: "generation", NodeTitle: "Write"}}},
			{Position: 1, Steps: []exec.PlannedStep{{Position: 1, StepType: "local.builtin", NodeID: "n1", Context: map[string]string{"operation": op}}}},
		},
	}
	byPos := NewRegistry().ByPosition(plan)
	if _, ok := byPos[0]; ok {
		t.Error("non-builtin step should not appear in the map")
	}
	if o, ok := byPos[1]; !ok || o.Operation != op {
		t.Errorf("builtin step at position 1 missing/wrong: %+v", byPos)
	}
}

func TestForRunStep(t *testing.T) {
	r := NewRegistry()
	byPos := map[int]ToolObject{2: {Kind: KindBuiltin, Operation: "count", ReadOnly: true, Offline: true}}

	// plan hit
	if o, ok := r.ForRunStep(byPos, 2, "", ""); !ok || o.Operation != "count" {
		t.Errorf("plan-hit should resolve: %+v ok=%v", o, ok)
	}
	// drift: persisted signal says builtin but plan has no entry → generic builtin
	if o, ok := r.ForRunStep(byPos, 9, "local", "builtin"); !ok || o.Kind != KindBuiltin || !o.ReadOnly {
		t.Errorf("provider/model fallback should yield a generic builtin: %+v ok=%v", o, ok)
	}
	// non-builtin
	if _, ok := r.ForRunStep(byPos, 9, "anthropic", "claude-x"); ok {
		t.Error("an authorable-node step must not be treated as a builtin")
	}
}

func TestLabel(t *testing.T) {
	o := ToolObject{Kind: KindBuiltin, Operation: "count", Title: "Count", Offline: true, TokenCost: 0}
	l := o.Label()
	for _, want := range []string{"Count", "built-in", "offline", "0 tokens"} {
		if !strings.Contains(l, want) {
			t.Errorf("label %q missing %q", l, want)
		}
	}
	// no title → never blank
	g := ToolObject{Kind: KindBuiltin, Offline: true}
	if g.Label() == "" || !strings.Contains(g.Label(), "built-in") {
		t.Errorf("generic label wrong: %q", g.Label())
	}
}
