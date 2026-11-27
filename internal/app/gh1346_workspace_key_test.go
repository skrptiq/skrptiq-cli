package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	eng "github.com/skrptiq/skrptiq-cli/internal/engine"
)

// GH#1346 — `/workspace set` wrote `workspacePath`; every engine reader uses
// `WORKSPACE_PATH`. So the command moved this process's directory, reported
// success, and left shell steps, the LLM working directory and file-destination
// resolution pointing at whatever the desktop app last set.
//
// THESE TESTS EXERCISE THE WRITER, WHICH IS THE WHOLE POINT. The suite already
// had `gh1221_tty_prompt_test.go:225`, which did `db.SetSetting("WORKSPACE_PATH", …)`
// by hand and then proved the reader could read it back. That test is the
// defect's mirror image: it establishes the property it was given instead of the
// property the code produces, so the missing writer was invisible to `go test`.
// Nothing below sets the canonical key directly — every assertion goes through
// `/workspace set` and then reads what the engine would read.

// setWorkspace runs `/workspace set <dir>` against a real engine on a temp DB
// and returns the model so callers can inspect what was printed. The process
// directory is restored on cleanup because the handler calls os.Chdir.
func setWorkspace(t *testing.T, dir string) (*Model, *eng.App) {
	t.Helper()

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	app, err := eng.Open(filepath.Join(t.TempDir(), "ws.db"))
	if err != nil {
		t.Fatalf("open engine: %v", err)
	}
	t.Cleanup(func() { app.DB.Close() })

	m := &Model{engine: app}
	m.handleWorkspaceCmd("set", dir)
	return m, app
}

func TestGH1346_WorkspaceSet_WritesTheKeyTheEngineReads(t *testing.T) {
	dir := t.TempDir()
	m, app := setWorkspace(t, dir)

	got := app.DB.GetSetting(eng.SettingWorkspacePath)
	if got != dir {
		t.Errorf("%s = %q, want %q\n"+
			"`/workspace set` must persist the workspace on the key the engine reads — "+
			"execution/shell.go reads it for a shell step's working directory and "+
			"execution/destinations.go reads it as the root file destinations resolve against. "+
			"A value anywhere else is a workspace the engine never sees.",
			eng.SettingWorkspacePath, got, dir)
	}

	if out := strings.Join(m.pendingOutput, "\n"); !strings.Contains(out, dir) {
		t.Errorf("output %q does not name the workspace it set", out)
	}
}

// The dead key must not be written in parallel. A dual-write would need to
// declare itself, be graded, and have a named deletion commit; none of that is
// worth paying for a key nothing in the estate reads.
func TestGH1346_WorkspaceSet_LeavesNothingAtTheDeadKey(t *testing.T) {
	dir := t.TempDir()
	_, app := setWorkspace(t, dir)

	// Spelt in two halves on purpose: the drift guard below greps the tree for
	// this literal, and a test file that contained it would defeat its own check.
	dead := "workspace" + "Path"
	if got := app.DB.GetSetting(dead); got != "" {
		t.Errorf("%s = %q, want empty — the pre-GH#1346 key must not be written any more", dead, got)
	}
}

// GH#1215, the other direction: a refused `/workspace set` must persist nothing.
// The bug being fixed was reporting a move that did not happen, and a write on a
// rejected path would be the same bug inverted.
func TestGH1346_RefusedWorkspaceSet_PersistsNothing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	m, app := setWorkspace(t, missing)

	if got := app.DB.GetSetting(eng.SettingWorkspacePath); got != "" {
		t.Errorf("%s = %q, want empty — a path that is not a directory must not be persisted",
			eng.SettingWorkspacePath, got)
	}
	if out := strings.Join(m.pendingOutput, "\n"); !strings.Contains(out, "Not a directory") {
		t.Errorf("output %q does not say the path was refused", out)
	}
}

// An unset workspace stays unset. A CLI-only user on a machine where the desktop
// app has never run starts with no value at all, and that is a supported state —
// the engine treats an empty workspace as "not configured", deliberately.
func TestGH1346_NoWorkspace_StaysUnsetAndIsNotInvented(t *testing.T) {
	app, err := eng.Open(filepath.Join(t.TempDir(), "empty.db"))
	if err != nil {
		t.Fatalf("open engine: %v", err)
	}
	defer app.DB.Close()

	if got := app.AdoptLegacyWorkspacePath(); got != "" {
		t.Errorf("AdoptLegacyWorkspacePath() = %q on a fresh DB, want empty", got)
	}
	if got := app.DB.GetSetting(eng.SettingWorkspacePath); got != "" {
		t.Errorf("%s = %q on a fresh DB, want empty", eng.SettingWorkspacePath, got)
	}
}

// The one-shot migration: a workspace set by a pre-GH#1346 cli is adopted onto
// the canonical key and the old key is erased, so the second start has nothing
// to do.
func TestGH1346_LegacyWorkspacePath_IsAdoptedOnceThenErased(t *testing.T) {
	dead := "workspace" + "Path"
	legacy := t.TempDir()

	app, err := eng.Open(filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatalf("open engine: %v", err)
	}
	defer app.DB.Close()
	if err := app.DB.SetSetting(dead, legacy); err != nil {
		t.Fatalf("seed legacy key: %v", err)
	}

	if got := app.AdoptLegacyWorkspacePath(); got != legacy {
		t.Errorf("first adoption returned %q, want %q", got, legacy)
	}
	if got := app.DB.GetSetting(eng.SettingWorkspacePath); got != legacy {
		t.Errorf("%s = %q after adoption, want %q", eng.SettingWorkspacePath, got, legacy)
	}
	if got := app.DB.GetSetting(dead); got != "" {
		t.Errorf("%s = %q after adoption, want empty — the migration must erase it, "+
			"otherwise it is a fallback rather than a one-shot", dead, got)
	}
	if got := app.AdoptLegacyWorkspacePath(); got != legacy {
		t.Errorf("second adoption returned %q, want %q — it must be idempotent", got, legacy)
	}
}

// When both keys are present the machine has run the desktop app, so the
// canonical value is one the engine has actually been honouring while the legacy
// value never took effect. Preferring the dead key would move where shell steps
// run on the strength of a setting that never worked.
func TestGH1346_CanonicalWorkspaceWins_WhenBothKeysExist(t *testing.T) {
	dead := "workspace" + "Path"
	canonical := t.TempDir()
	stale := t.TempDir()

	app, err := eng.Open(filepath.Join(t.TempDir(), "both.db"))
	if err != nil {
		t.Fatalf("open engine: %v", err)
	}
	defer app.DB.Close()
	if err := app.DB.SetSetting(eng.SettingWorkspacePath, canonical); err != nil {
		t.Fatalf("seed canonical: %v", err)
	}
	if err := app.DB.SetSetting(dead, stale); err != nil {
		t.Fatalf("seed legacy: %v", err)
	}

	if got := app.AdoptLegacyWorkspacePath(); got != canonical {
		t.Errorf("adoption returned %q, want the canonical %q", got, canonical)
	}
	if got := app.DB.GetSetting(eng.SettingWorkspacePath); got != canonical {
		t.Errorf("%s = %q, want %q — adoption must not overwrite a live value",
			eng.SettingWorkspacePath, got, canonical)
	}
	if got := app.DB.GetSetting(dead); got != "" {
		t.Errorf("%s = %q, want empty — the dead key is removed either way", dead, got)
	}
}

// The structural half of the fix. Two spellings of one setting is what caused
// this, so the literal is allowed in exactly one place: the named constant it is
// scheduled to be deleted with.
func TestGH1346_DeadWorkspaceKeyAppearsOnlyAtItsOwnConstant(t *testing.T) {
	dead := "\"workspace" + "Path\""
	const allowed = "internal/engine/engine.go"

	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}

	var offenders []string
	err = filepath.WalkDir(filepath.Join(root, "internal"), func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") {
			return err
		}
		body, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		if !strings.Contains(string(body), dead) {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		if filepath.ToSlash(rel) != allowed {
			offenders = append(offenders, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	if len(offenders) > 0 {
		t.Errorf("the pre-GH#1346 settings key %s appears in %v.\n"+
			"Only %s may name it, and only so there is one symbol to delete. "+
			"Every reader and writer of the workspace root uses eng.SettingWorkspacePath.",
			dead, offenders, allowed)
	}
}
