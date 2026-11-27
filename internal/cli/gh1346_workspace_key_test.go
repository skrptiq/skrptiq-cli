package cli

import (
	"path/filepath"
	"testing"

	exec "github.com/skrptiq/engine/execution"
	"github.com/skrptiq/engine/outputs"
	"github.com/skrptiq/engine/storage"

	eng "github.com/skrptiq/skrptiq-cli/internal/engine"
)

// GH#1346, the file-destination direction. `outputRequestFor` builds the request
// the engine resolves a `file` destination against, and its Workspace field is
// the root a declared path resolves under.
//
// Before the fix this reader spelled the key as a bare literal while
// `/workspace set` wrote cli's own private spelling, so cli's OWN destination
// resolution could not see cli's own workspace. (The dead spelling is not
// repeated here: the drift guard greps for it, and naming it in a comment would
// trip the guard — which is itself the proof that the guard works.) Both sides
// now name one constant, and
// `TestGH1346_DeadWorkspaceKeyAppearsOnlyAtItsOwnConstant` in internal/app keeps
// them from drifting apart again.
func TestGH1346_DestinationRequest_ReadsTheCanonicalWorkspaceKey(t *testing.T) {
	db, err := storage.Open(filepath.Join(t.TempDir(), "dest.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	ws := "/tmp/gh1346-workspace"
	if err := db.SetSetting(eng.SettingWorkspacePath, ws); err != nil {
		t.Fatalf("set workspace: %v", err)
	}
	if err := db.SetSetting(outputs.SettingOutputDir, "/tmp/gh1346-out"); err != nil {
		t.Fatalf("set output dir: %v", err)
	}

	req, err := outputRequestFor(db, exec.DestinationResult{
		Type: "file", HostPending: true, DeclaredPath: "notes/out.md", Content: "BODY",
	})
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	if req.Workspace != ws {
		t.Errorf("req.Workspace = %q, want %q — a destination declared relative to the "+
			"workspace resolves against this field, so a workspace cli persisted but did "+
			"not read back here is a path the run writes to the wrong place",
			req.Workspace, ws)
	}
}
