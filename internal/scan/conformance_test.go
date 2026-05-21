package scan

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/skrptiq/engine/manifest"
	"github.com/skrptiq/engine/parse"
)

// readPackageResult mirrors the golden fixture shape — same struct the
// engine's own conformance test uses. The golden JSON wraps the Package
// in {"package": ..., "issues": ...} to match the cgo return payload.
type readPackageResult struct {
	Package parse.Package          `json:"package"`
	Issues  []manifest.ParseIssue `json:"issues"`
}

// TestConformance_RealisticSkrptPackage calls engine/parse.ReadPackage
// against the shared golden fixture and asserts byte-equality against
// the golden JSON file. This is the F10 enforcement rule for F12:
// drift between the CLI's import of engine/parse and the engine's own
// test surfaces immediately.
func TestConformance_RealisticSkrptPackage(t *testing.T) {
	// Resolve fixture path from test file location → repo root → engine fixtures.
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	fixtureDir := filepath.Join(repoRoot, "..", "skrptiq-app", "engine", "test", "fixtures", "parse")

	goldenFile := filepath.Join(fixtureDir, "realistic-skrpt-package.json")
	if _, err := os.Stat(goldenFile); os.IsNotExist(err) {
		t.Skip("golden fixture not found — sibling repo not available (CI without co-checkout)")
	}

	pkgDir := filepath.Join(fixtureDir, "realistic-skrpt")
	pkg, issues, err := parse.ReadPackage(pkgDir)
	if err != nil {
		t.Fatalf("ReadPackage failed: %v", err)
	}

	// Zero out host-specific fields for comparison.
	pkg.RootDir = ""
	for i := range pkg.Nodes {
		pkg.Nodes[i].FilePath = ""
	}

	result := readPackageResult{
		Package: pkg,
		Issues:  issues,
	}

	// Match the engine's golden-file encoding: json.Encoder with
	// SetEscapeHTML(false), two-space indent.
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(result); err != nil {
		t.Fatalf("json.Encode failed: %v", err)
	}
	got := buf.Bytes()

	want, err := os.ReadFile(goldenFile)
	if err != nil {
		t.Fatalf("read golden file: %v", err)
	}

	if !bytes.Equal(bytes.TrimRight(got, "\n"), bytes.TrimRight(want, "\n")) {
		t.Errorf("ReadPackage output does not match golden fixture.\n"+
			"Run with -update on the engine side to regenerate.\n"+
			"Got length: %d, want length: %d", len(got), len(want))
		// Show first divergence point for debugging.
		gotStr, wantStr := string(got), string(want)
		for i := 0; i < len(gotStr) && i < len(wantStr); i++ {
			if gotStr[i] != wantStr[i] {
				start := i - 40
				if start < 0 {
					start = 0
				}
				end := i + 60
				if end > len(gotStr) {
					end = len(gotStr)
				}
				endW := i + 60
				if endW > len(wantStr) {
					endW = len(wantStr)
				}
				t.Logf("first divergence at byte %d:", i)
				t.Logf("  got:  ...%s...", gotStr[start:end])
				t.Logf("  want: ...%s...", wantStr[start:endW])
				break
			}
		}
	}
}
