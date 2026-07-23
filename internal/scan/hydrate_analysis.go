package scan

// GH#941 — offline catalogue hydration for the order-diff harness.
//
// The order-diff report (the hard gate before ordering-derivation flips on)
// needs each published skrpt loaded into a storage.DB so the engine can build
// its plan twice — derivation OFF then ON. This exposes the scan pipeline's
// parse→bridge→HydratePackage flow as a reusable, offline (no-Hub) helper so
// the CLI's `order-diff` command can hydrate a bundle into an ephemeral temp DB
// without duplicating the bridge logic.

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/skrptiq/engine/parse"
	"github.com/skrptiq/engine/storage"
)

// WorkflowRef is a workflow node discovered in a hydrated bundle. ID is the
// node ID BuildExecutionPlan / ComputeOrderDiff key on; Title is for display.
type WorkflowRef struct {
	ID    string
	Title string
}

// HydratedBundle is a catalogue bundle parsed and hydrated into an ephemeral
// temp DB for offline analysis (GH#941). The DB stays live until Close, which
// also removes the temp directory. Not safe for concurrent use — an offline
// scan handle, by design.
type HydratedBundle struct {
	Path      string        // absolute path of the bundle directory
	DB        *storage.DB   // open handle to the temp DB the bundle was hydrated into
	Workflows []WorkflowRef // workflow nodes in the bundle, in package order
	tmpDir    string
}

// Close releases the DB and removes the temp directory. Idempotent.
func (h *HydratedBundle) Close() {
	if h.DB != nil {
		h.DB.Close()
		h.DB = nil
	}
	if h.tmpDir != "" {
		os.RemoveAll(h.tmpDir)
		h.tmpDir = ""
	}
}

// HydrateBundle parses the skrpt bundle at path and hydrates it into a fresh
// temp DB, returning the open DB plus its workflow node refs. It mirrors the
// scan pipeline (parse.ReadPackage → bridgeToHydration → HydratePackage) in
// legacy no-dependency mode — exact for the published catalogue, whose bundles
// are self-contained (zero `dependencies:` blocks). The caller owns Close().
func HydrateBundle(path string) (*HydratedBundle, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	pkg, _, err := parse.ReadPackage(absPath)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "skrptiq-orderdiff-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}

	// storage.Open replays migrations with fmt.Printf to stdout on every temp DB
	// (engine-side, cross-repo). Silence stdout around open+hydrate so a
	// whole-catalogue run emits a clean report rather than thousands of
	// migration lines. Restored before returning.
	restore := silenceStdout()
	db, err := storage.Open(filepath.Join(tmpDir, "order-diff.db"))
	if err != nil {
		restore()
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("open temp db: %w", err)
	}

	br := bridgeToHydration(pkg, absPath, depContext{})
	_, hydrateErr := db.HydratePackage(br.Input)
	restore()
	if hydrateErr != nil {
		db.Close()
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("hydrate: %w", hydrateErr)
	}

	var wfs []WorkflowRef
	for _, n := range pkg.Nodes {
		if n.Type == "workflow" {
			wfs = append(wfs, WorkflowRef{ID: n.ID, Title: n.Title})
		}
	}

	return &HydratedBundle{Path: absPath, DB: db, Workflows: wfs, tmpDir: tmpDir}, nil
}

// silenceStdout redirects os.Stdout to os.DevNull and returns a restore func.
// If /dev/null can't be opened it is a no-op (never fails the caller).
func silenceStdout() func() {
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return func() {}
	}
	saved := os.Stdout
	os.Stdout = devnull
	return func() {
		os.Stdout = saved
		devnull.Close()
	}
}
