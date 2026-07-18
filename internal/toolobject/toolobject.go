// Package toolobject implements the CLI side of the K-057 ToolObject contract
// (GH#873): the single render object that surfaces node-less capabilities —
// engine builtins today — as distinct, READ-ONLY objects wherever steps appear.
//
// This is a VIEW object only (no K-034 gate/risk/policy). The struct mirrors the
// LOCKED K-057 schema field-for-field (camelCase JSON tags), asserted by
// TestToolObjectJSONTags. One helper, two derivations — the registry form
// (FromDescriptor) and the instance form (FromStep, carrying stepId/params) —
// so no surface re-derives its own shape (the K-049 drift guard).
//
// The CLI's advantage over the Hub web: it imports the engine builtin registry
// (execution.Builtins()) directly, so display metadata never needs a manifest.
package toolobject

import (
	"strconv"

	exec "github.com/skrptiq/engine/execution"
)

// KindBuiltin is the only ToolObject kind today; the union grows by kind
// (browser/mcp) without churning existing variants.
const KindBuiltin = "builtin"

// stepTypeBuiltin is the planner step type for a node-less builtin step.
const stepTypeBuiltin = "local.builtin"

// ToolObject is the K-057 render object (builtin variant). Field names + JSON
// tags are the LOCKED contract; do not rename without an orchestrator ruling.
type ToolObject struct {
	Kind        string            `json:"kind"`
	Operation   string            `json:"operation"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Category    string            `json:"category"`
	ReadOnly    bool              `json:"readOnly"`
	Offline     bool              `json:"offline"`
	TokenCost   int               `json:"tokenCost"`
	StepID      string            `json:"stepId,omitempty"`
	Params      map[string]string `json:"params,omitempty"`
}

// Label is the surface-agnostic display text — a distinct "built-in" treatment
// callers style themselves (dim/glyph). Never blank: an unknown-op fallback
// still reads as a built-in.
//
//	"Word count gate · built-in · offline · 0 tokens"
func (o ToolObject) Label() string {
	title := o.Title
	if title == "" {
		if o.Operation != "" {
			title = o.Operation
		} else {
			title = "Built-in operation"
		}
	}
	state := " · built-in"
	if o.Offline {
		state += " · offline"
	}
	return title + state + " · " + strconv.Itoa(o.TokenCost) + " tokens"
}

// Registry maps operation → engine descriptor, built once from
// execution.Builtins() (the single source of truth for the op set).
type Registry struct {
	byOp map[string]exec.BuiltinDescriptor
}

// NewRegistry snapshots the engine builtin registry.
func NewRegistry() *Registry {
	m := make(map[string]exec.BuiltinDescriptor)
	for _, d := range exec.Builtins() {
		m[d.Name] = d
	}
	return &Registry{byOp: m}
}

// FromDescriptor builds the registry-form object (no instance data).
func FromDescriptor(d exec.BuiltinDescriptor) ToolObject {
	return ToolObject{
		Kind:        KindBuiltin,
		Operation:   d.Name,
		Title:       d.Title,
		Description: d.Description,
		Category:    d.Category,
		ReadOnly:    true,
		Offline:     true,
		TokenCost:   d.TokenCost,
	}
}

// All returns the registry-form objects for every known builtin.
func (r *Registry) All() []ToolObject {
	out := make([]ToolObject, 0, len(r.byOp))
	for _, d := range r.byOp {
		out = append(out, FromDescriptor(d))
	}
	return out
}

// FromStep builds the instance-form object for a builtin workflow step. When the
// operation is unknown to this build's registry it degrades to a generic builtin
// object (never blank) — the fail-safe posture for a run whose workflow drifted.
func (r *Registry) FromStep(operation, stepID string, context map[string]string) ToolObject {
	var o ToolObject
	if d, ok := r.byOp[operation]; ok {
		o = FromDescriptor(d)
	} else {
		o = ToolObject{Kind: KindBuiltin, Operation: operation, ReadOnly: true, Offline: true, TokenCost: 0}
	}
	o.StepID = stepID
	o.Params = stepParams(context)
	return o
}

// stepParams copies the step context minus the "operation" key (which is the
// top-level Operation field, not an instance param).
func stepParams(context map[string]string) map[string]string {
	if len(context) == 0 {
		return nil
	}
	params := make(map[string]string, len(context))
	for k, v := range context {
		if k == "operation" {
			continue
		}
		params[k] = v
	}
	if len(params) == 0 {
		return nil
	}
	return params
}

// ForRunStep resolves whether a run step is a builtin and, if so, its ToolObject.
// Detection is belt-and-braces: the plan's by-position map (authoritative for the
// current workflow structure) OR the persisted run-axis signal
// (provider=local/model=builtin), which still identifies a builtin when the
// workflow has drifted since the run — in that case it degrades to a generic
// builtin object rather than mislabeling or blanking.
func (r *Registry) ForRunStep(byPos map[int]ToolObject, position int, provider, model string) (ToolObject, bool) {
	if obj, ok := byPos[position]; ok {
		return obj, true
	}
	if provider == "local" && model == "builtin" {
		return ToolObject{Kind: KindBuiltin, ReadOnly: true, Offline: true, TokenCost: 0}, true
	}
	return ToolObject{}, false
}

// ByPosition maps a plan's builtin steps to their run-axis position → instance
// ToolObject, so run surfaces can enrich a step (detected as a builtin) with its
// operation by position. Returns an empty map for a nil plan.
func (r *Registry) ByPosition(plan *exec.ExecutionPlan) map[int]ToolObject {
	out := map[int]ToolObject{}
	if plan == nil {
		return out
	}
	for _, g := range plan.PositionGroups {
		for _, s := range g.Steps {
			if s.StepType == stepTypeBuiltin {
				out[s.Position] = r.FromStep(s.Context["operation"], s.NodeID, s.Context)
			}
		}
	}
	return out
}
