package scan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/skrptiq/engine/manifest"
	"github.com/skrptiq/engine/parse"
)

// Q4 round-trip parity guard (GH#530). The accepted asymmetry is:
// engine reads + TS writes. The risk: TS workspace serialiser changes
// shape, engine parser doesn't follow, and the drift goes silent until
// a real workflow fails far downstream.
//
// This test exercises the contract by reading every available
// catalogue skrpt (TS-serialised output, since Hub generates the
// catalogue via its TS pipeline) and asserting the engine parser
// returns zero severity-error issues per skrpt. If TS adds a field or
// changes a delimiter convention the engine doesn't know about, the
// failure surfaces here as a parse error rather than as a runtime
// surprise.
//
// The Hub repo is opportunistically present at ../skrptiq-hub during
// local development. CI sees no Hub checkout — t.Skip there. The
// gen-smoke-fixtures tool (GH#538) is the production analogue of this
// check; this test pins the same contract for the no-fixtures path.
//
// To run locally:
//
//	go test ./internal/scan/ -run TestQ4
//
// If/when CI is widened to clone Hub, this test runs there too; until
// then it's a local + manual-PR-gate guard.
func TestQ4_RoundTrip_CatalogueParsesCleanlyViaEngine(t *testing.T) {
	hubExamples := "../../../skrptiq-hub/examples"
	if _, err := os.Stat(hubExamples); err != nil {
		t.Skipf("Hub examples not present at %s — Q4 catalogue round-trip skipped (local-only test)", hubExamples)
	}

	entries, err := os.ReadDir(hubExamples)
	if err != nil {
		t.Fatalf("read hub examples dir: %v", err)
	}

	var scanned, failed int
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "_shared" {
			continue
		}
		slug := e.Name()
		dir := filepath.Join(hubExamples, slug)
		t.Run(slug, func(t *testing.T) {
			_, issues, err := parse.ReadPackage(dir)
			if err != nil {
				t.Fatalf("parse.ReadPackage: %v", err)
			}
			var errs []manifest.ParseIssue
			for _, i := range issues {
				if i.Severity == manifest.SeverityError {
					errs = append(errs, i)
				}
			}
			if len(errs) > 0 {
				t.Fatalf("%d parse error(s); first: %s (%s:%d) %s",
					len(errs), errs[0].Code, errs[0].File, errs[0].Line, errs[0].Message)
			}
		})
		scanned++
		// Per-skrpt failure surfaces via the t.Run above; this counter
		// is just for the eventual summary log line.
		_ = failed
	}

	if scanned == 0 {
		t.Fatal("no skrpts under examples/ — Hub layout changed or fixture path wrong")
	}
	t.Logf("Q4 round-trip: parsed %d catalogue skrpts cleanly via engine", scanned)
}
