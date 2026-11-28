package scan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/skrptiq/engine/parse"
)

// GH#1387 — a manifest carrying almost nothing passed the gate the publish path
// calls the strictest of three, because every other rule compares declared things
// against each other and a package declaring nothing has no relationships to be
// wrong about. This is the rule whose subject is the package.

// TEST 1 — THE KNOWN-BAD, and it is hub's reproducible `vacuous-probe` recipe
// rather than a fixture invented here: identity, name, version, and nothing else.
func TestGH1387_VacuousPackage_IsRefused(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "skrptiq.yaml"), []byte(
		"id: \"550e8400-e29b-41d4-a716-446655440000\"\nname: \"vacuous-probe\"\nversion: \"1.0.0\"\n"), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	pkg, _, err := parse.ReadPackage(dir)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	issues := checkPackageHasContent(&pkg, dir)
	if len(issues) != 1 {
		t.Fatalf("got %d issues, want 1 — a manifest with three keys and no nodes must be refused", len(issues))
	}
	if issues[0].Code != CodePackageNoContent {
		t.Errorf("code = %q, want %q", issues[0].Code, CodePackageNoContent)
	}
	if issues[0].Severity != "error" {
		t.Errorf("severity = %q, want error — the publish gate refuses on errorCount > 0, which is the point",
			issues[0].Severity)
	}
	// The message must name BOTH halves, because the fix is genuinely either one.
	for _, want := range []string{"ships no nodes", "dependencies", "Ship at least one node"} {
		if !strings.Contains(issues[0].Message, want) {
			t.Errorf("the message does not mention %q: %q", want, issues[0].Message)
		}
	}
}

// TEST 2 — THE CONTROL, and the half that decides it. `github-mcp-reference`'s
// exact shape: zero nodes, honest `total: 0`, and a `dependencies:` pin whose
// nodes are the package's whole substance.
//
// A rule refusing this has refused the catalogue's own worked example of how a
// skrpt declares a service.
func TestGH1387_ZeroNodePackageWithADependency_Passes(t *testing.T) {
	dir := t.TempDir()
	manifest := `id: "a371fae7-7c14-4f94-a1b9-fb47cf8086d7"
name: "thin-reference"
version: "1.0.1"
display_name: "Thin Reference"
description: "Dep-references a canonical service so an author can see how it is declared"
dependencies:
  - id: "f4c0bfa9-5588-4590-ab27-ca696bcd959f"
    name: "github-mcp"
    version: 1.0.0
    checksum: "sha256:705e6376b99589c5d34916f5559068517c25c9e5151461212fada3039ff7c5e4"
contents:
  total: 0
`
	if err := os.WriteFile(filepath.Join(dir, "skrptiq.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	pkg, _, err := parse.ReadPackage(dir)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(pkg.Dependencies) == 0 {
		t.Fatalf("the fixture's dependencies block did not parse — this test would pass vacuously")
	}
	if issues := checkPackageHasContent(&pkg, dir); len(issues) != 0 {
		t.Errorf("a zero-node package with a dependency pin was refused: %+v", issues)
	}
}

// TEST 4 — the near-misses, including the loophole closed before anyone finds it.
func TestGH1387_NearMisses(t *testing.T) {
	const base = "id: \"550e8400-e29b-41d4-a716-446655440000\"\nname: \"probe\"\nversion: \"1.0.0\"\n"

	// One node of ANY type is content. Reference material counts: the 28 `_shared/`
	// bundles ship exactly one source, service, document or asset each.
	for _, nodeType := range []string{"prompt", "skill", "workflow", "source", "service", "document", "asset"} {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "skrptiq.yaml"), []byte(base), 0o644)
		sub := map[string]string{
			"prompt": "prompts", "skill": "skills", "workflow": "workflows", "source": "sources",
			"service": "services", "document": "documents", "asset": "assets",
		}[nodeType]
		os.MkdirAll(filepath.Join(dir, sub), 0o755)
		os.WriteFile(filepath.Join(dir, sub, "n.md"), []byte(
			"---\ntype: "+nodeType+"\nid: n\ntitle: N\ndescription: d\n---\n\nbody\n"), 0o644)

		pkg, _, err := parse.ReadPackage(dir)
		if err != nil {
			t.Errorf("%s: parse: %v", nodeType, err)
			continue
		}
		if len(pkg.Nodes) == 0 {
			t.Errorf("%s: the fixture produced no nodes — this case would pass vacuously", nodeType)
			continue
		}
		if issues := checkPackageHasContent(&pkg, dir); len(issues) != 0 {
			t.Errorf("a package shipping one %s node was refused: %+v", nodeType, issues)
		}
	}

	// THE LOOPHOLE: a `dependencies:` key with nothing under it references nothing.
	// Treated as absent on purpose, or the escape is a one-liner.
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "skrptiq.yaml"), []byte(base+"dependencies: []\n"), 0o644)
	pkg, _, err := parse.ReadPackage(dir)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if issues := checkPackageHasContent(&pkg, dir); len(issues) != 1 {
		t.Errorf("an empty `dependencies:` block satisfied the rule — it references nothing, "+
			"so it must be treated as absent; got %d issues", len(issues))
	}
}

// TEST 3 — THE CORPUS ASSERTION. The count goes in a test rather than only in a
// plan comment, which is the difference between a measurement and a claim.
func TestGH1387_NoLivePackageIsRefused(t *testing.T) {
	base := "/Users/bencrocker/Developer/skrptiq-hub/examples"
	if _, err := os.Stat(base); err != nil {
		t.Skip("no hub checkout on this machine")
	}

	var dirs []string
	for _, root := range []string{base, filepath.Join(base, "_shared")} {
		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatalf("read %s: %v", root, err)
		}
		for _, e := range entries {
			if !e.IsDir() || e.Name() == "_shared" {
				continue
			}
			d := filepath.Join(root, e.Name())
			if _, err := os.Stat(filepath.Join(d, "skrptiq.yaml")); err == nil {
				dirs = append(dirs, d)
			}
		}
	}
	if len(dirs) < 200 {
		t.Fatalf("found only %d live packages — the corpus walk is wrong and this assertion "+
			"would pass vacuously", len(dirs))
	}

	var refused []string
	for _, d := range dirs {
		pkg, _, err := parse.ReadPackage(d)
		if err != nil {
			continue // parse failures are other rules' subject
		}
		if len(checkPackageHasContent(&pkg, d)) > 0 {
			refused = append(refused, filepath.Base(d))
		}
	}
	if len(refused) != 0 {
		t.Errorf("the rule refuses %d live packages %v — it must refuse none. If a package "+
			"genuinely became vacuous, fix the package; if the predicate widened, fix the rule",
			len(refused), refused)
	}
	t.Logf("corpus: %d live packages, %d refused", len(dirs), len(refused))
}
