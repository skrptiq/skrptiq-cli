package engine

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	exec "github.com/skrptiq/engine/execution"
)

// GH#1072 / GH#1084 — the CLI must never claim project attribution it has not
// resolved.
//
// `execution.RunWorkflow` takes `projectAuthority *storage.Authority` as a
// POSITIONAL parameter, deliberately: a field on the nilable RunOptions could
// be forgotten and the run would be unattributed while looking attributed at
// the call site. The engine's contract is that nil means NO project — a real
// absence, nothing defaulted and nothing inferred — and that every caller must
// state it.
//
// The CLI's answer is nil, and it is the correct one rather than a convenient
// one: the CLI has no project concept at all. It never calls
// storage.ResolveAuthority, so it holds no authority to present, and a
// zero-value Authority is invalid by construction — the engine rejects it with
// ErrAuthorityNotResolved rather than trusting a struct literal.
//
// This asserts the OBSERVABLE rather than the argument: a run started through
// the CLI leaves executions.project_id NULL. It fails if someone later forges
// an Authority to satisfy the compiler, which is the one way this call site can
// go wrong without anything else noticing.
func TestGH1072_CLIRun_IsNotAttributedToAnyProject(t *testing.T) {
	app, err := Open(filepath.Join(t.TempDir(), "attribution.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer app.Close()

	// A single local.template step: deterministic, zero tokens, no provider
	// key and no network, so the run reaches completion in CI.
	meta := `{"execution":[{"skill":"only-step","step_type":"local.template"}]}`
	strp := func(s string) *string { return &s }
	intp := func(i int) *int { return &i }

	if err := app.DB.CreateNode("wf", "workflow", "WF", nil, nil, strp(meta), strp("wf"), nil); err != nil {
		t.Fatalf("create workflow: %v", err)
	}
	if err := app.DB.CreateNode("s1", "skill", "Only Step", nil, strp("OUTPUT"), nil, strp("only-step"), nil); err != nil {
		t.Fatalf("create skill: %v", err)
	}
	if err := app.DB.CreateEdge("e1", "wf", "s1", "uses", intp(1)); err != nil {
		t.Fatalf("create edge: %v", err)
	}

	execID, err := app.RunWorkflow(context.Background(), "wf", map[string]string{}, func(exec.ProgressEvent) {})
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}

	var projectID sql.NullString
	if err := app.DB.QueryRow(`SELECT project_id FROM executions WHERE id = ?`, execID).Scan(&projectID); err != nil {
		t.Fatalf("read project_id for %s: %v", execID, err)
	}
	if projectID.Valid {
		t.Errorf("execution %s is attributed to project %q — the CLI resolves no authority and must claim none", execID, projectID.String)
	}
}
