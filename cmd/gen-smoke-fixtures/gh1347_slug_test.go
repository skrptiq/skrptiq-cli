package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/skrptiq/engine/storage"
)

// GH#1347 F3 — `dn.Slug` is decoded straight from the Hub response
// (`NodeInfo.Slug`, `json:"slug"`), nothing in this repo validates it, and
// `writeSyntheticNode` made it a filename. Verified by running the expression
// before the fix:
//
//	../../../../../../tmp/pwn  ->  /tmp/pwn.md   escaped the fixture root
//
// THE GATE IS ON WHAT THE SLUG BECOMES. `filepath.Join` makes an absolute slug
// relative and harmless while a `../` slug escapes, so a guard written against
// absolute paths would read as protection and provide none. That asymmetry is why
// the check resolves the destination instead of inspecting the string.

func TestGH1347_WithinRoot(t *testing.T) {
	root := filepath.Join("/tmp", "gh1347-root")
	for _, c := range []struct {
		candidate string
		want      bool
	}{
		{filepath.Join(root, "skills", "normal.md"), true},
		{filepath.Join(root, "a", "b", "c.md"), true},
		{root, true},
		{filepath.Join(root, "..", "sibling.md"), false},
		{filepath.Join(root, "..", "..", "far.md"), false},
		{"/etc/passwd", false},
		{filepath.Dir(root), false},
	} {
		if got := withinRoot(root, c.candidate); got != c.want {
			t.Errorf("withinRoot(%q, %q) = %v, want %v", root, c.candidate, got, c.want)
		}
	}
}

// THE PLANT. `gh1347-planted-escape` cannot occur naturally in this corpus — a
// traversal that might appear in real data would make the test a coincidence
// rather than a control, which is the same reasoning as GH#1358's `"critical"`.
func TestGH1347_HostileSlug_IsRefusedAndNamed(t *testing.T) {
	root := t.TempDir()
	const planted = "../../../../tmp/gh1347-planted-escape"

	err := writeSyntheticNode(root, storage.DepNodeSummary{
		ID: "id-1", Type: "skill", Slug: planted, Title: "T", Content: "body",
	})
	if err == nil {
		t.Fatal("a slug resolving outside the fixture root was accepted — it must be refused")
	}
	if !strings.Contains(err.Error(), planted) {
		t.Errorf("the refusal does not name the slug: %v", err)
	}
	if !strings.Contains(err.Error(), "outside the fixture root") {
		t.Errorf("the refusal does not say what was wrong: %v", err)
	}

	// And nothing was written anywhere the slug pointed.
	escaped := filepath.Join(filepath.Dir(root), "gh1347-planted-escape.md")
	if _, statErr := os.Stat(escaped); statErr == nil {
		t.Errorf("a file was written at %s despite the refusal", escaped)
	}
}

// THE KNOWN-GOOD CONTROL, and the half that decides whether the check survives: a
// guard refusing every slug passes the traversal case perfectly.
func TestGH1347_LegitimateSlug_StillWrites(t *testing.T) {
	root := t.TempDir()

	if err := writeSyntheticNode(root, storage.DepNodeSummary{
		ID: "id-1", Type: "skill", Slug: "ordinary-skill", Title: "T", Content: "body",
	}); err != nil {
		t.Fatalf("a legitimate slug was refused: %v", err)
	}

	written := filepath.Join(root, "skills", "ordinary-skill.md")
	if _, err := os.Stat(written); err != nil {
		t.Errorf("expected %s to exist: %v", written, err)
	}

	// A nested-but-confined slug passes the GUARD — the check is about escaping,
	// not about shape — and then fails on the write, because `writeSyntheticNode`
	// only creates the node-type directory and not a slug's own parents. That is
	// pre-existing and out of GH#1347's scope; asserted here so the two failures
	// stay distinguishable. If someone later creates the parents, this expectation
	// changes and the guard's verdict does not.
	err := writeSyntheticNode(root, storage.DepNodeSummary{
		ID: "id-2", Type: "skill", Slug: "group/nested-skill", Title: "T", Content: "body",
	})
	if err == nil {
		t.Skip("nested slug parents are now created; the guard still allows it, nothing to assert")
	}
	if strings.Contains(err.Error(), "outside the fixture root") {
		t.Errorf("a CONFINED nested slug was refused by the confinement guard: %v", err)
	}
}
