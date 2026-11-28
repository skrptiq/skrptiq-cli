package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/skrptiq/engine/outputs"
)

// GH#1348 — cli prompted "Enter a name relative to %s" and assigned the answer to
// `ChosenPath`, the engine's UNCONFINED field. Its only check is `filepath.IsAbs`
// and containment is never checked, by documented design. So the prompt promised a
// property the field it fed enforces the opposite of.
//
// EVERY ROW BELOW WAS PRODUCED BY RUNNING THE REAL `outputs.Write`, not derived
// from reading it. That mattered: `../../escaped.md` IS refused on the unconfined
// route, which looks exactly like traversal handling until the refusal is read —
// it says "not absolute". The relativity check is firing and the traversal is
// incidental to it. Only an ABSOLUTE path containing `..` separates the two, and
// that row is the one that showed containment is never checked at all.

// writeReq is the request shape cli builds, with the content planted so a write
// can be identified on disk rather than inferred from a returned path.
func writeReq(t *testing.T, outputDir string) outputs.Request {
	t.Helper()
	return outputs.Request{
		Setting:     outputDir,
		Home:        outputDir,
		Workspace:   outputDir,
		Interactive: true,
		Content:     "PLANTED BY THE GH#1348 TEST\n",
	}
}

// insideDir reports whether written is within dir, with both sides resolved —
// macOS serves /var as a symlink to /private/var, and comparing the raw strings
// reports a correct write as an escape.
func insideDir(t *testing.T, dir, written string) bool {
	t.Helper()
	realDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("resolve %q: %v", dir, err)
	}
	realWritten, err := filepath.EvalSymlinks(filepath.Dir(written))
	if err != nil {
		t.Fatalf("resolve %q: %v", filepath.Dir(written), err)
	}
	rel, err := filepath.Rel(realDir, filepath.Join(realWritten, filepath.Base(written)))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// ── THE CONFINED ROUTE: five rows, two accepts and three refusals ────────────
//
// Both GH#1215 directions in one table. A validator that refused all three
// hazards and also the two legitimate names would score perfectly on the
// refusals alone, so the accepts are the half that decides whether the fix is
// usable.

func TestGH1348_DeclaredPath_AcceptsTheNamesThePromptAsksFor(t *testing.T) {
	for _, name := range []string{"notes.md", filepath.Join("sub", "dir", "notes.md")} {
		dir := t.TempDir()
		req := writeReq(t, dir)
		req.DeclaredPath = name

		written, err := outputs.Write(req)
		if err != nil {
			t.Fatalf("%q was refused (%v) — this is exactly what the prompt asks for, and "+
				"before GH#1348 the answer went to ChosenPath, which refused it for not "+
				"being absolute", name, err)
		}
		if !insideDir(t, dir, written) {
			t.Errorf("%q wrote to %q, which is not inside the output directory %q the "+
				"prompt named", name, written, dir)
		}
		if b, rerr := os.ReadFile(written); rerr != nil {
			t.Errorf("%q reported %q but no file is there: %v", name, written, rerr)
		} else if !strings.Contains(string(b), "PLANTED") {
			t.Errorf("%q wrote %q but the content is not the planted content", name, written)
		}
	}
}

func TestGH1348_DeclaredPath_RefusesEveryEscape(t *testing.T) {
	outside := t.TempDir()

	for _, c := range []struct{ name, declared string }{
		{"an absolute path outside the output directory", filepath.Join(outside, "escaped.md")},
		{"a traversal that normalises outside", filepath.Join("..", "..", "escaped.md")},
		// THE DECIDING ROW. On the unconfined route this one WROTE OUTSIDE: it is
		// absolute, so `IsAbs` passed, and `filepath.Abs` normalised the `..` away.
		// It is the only input that distinguishes a containment check from a
		// relativity check, and it is why "../.. is refused" proved nothing.
		{"an absolute path rooted at the output directory, then climbing out", ""},
	} {
		dir := t.TempDir()
		declared := c.declared
		if declared == "" {
			declared = filepath.Join(dir, "..", "escaped-climb.md")
		}

		req := writeReq(t, dir)
		req.DeclaredPath = declared

		written, err := outputs.Write(req)
		if err == nil {
			t.Errorf("%s: %q was accepted and wrote %q — the prompt promised a name "+
				"relative to %q", c.name, declared, written, dir)
			continue
		}
		// The refusal must be about WHERE it lands, not about some other property
		// that happens to refuse it too. That distinction is the whole measurement.
		if !strings.Contains(err.Error(), "outside the output directory") &&
			!strings.Contains(err.Error(), "not inside") {
			t.Errorf("%s: refused, but for the wrong reason — %v. A refusal that is right "+
				"about a different property is indistinguishable from the one being tested for",
				c.name, err)
		}
	}
}

// ── THE UNCONFINED ROUTE: four rows, recorded as DOCUMENTED BEHAVIOUR ────────
//
// `ChosenPath` is not defective. `write.go:32`: "it bypasses confinement, and that
// is not a hole … here the user is choosing." This records what it does so the
// next reader does not mistake it for a gap, and so a change to it is a visible
// test failure rather than a silent shift under cli's `dir == ""` branch.

func TestGH1348_ChosenPath_IsTheUnconfinedRouteAndSaysSo(t *testing.T) {
	outside := t.TempDir()

	t.Run("a relative name is refused for not being absolute", func(t *testing.T) {
		dir := t.TempDir()
		req := writeReq(t, dir)
		req.ChosenPath = "notes.md"
		if _, err := outputs.Write(req); err == nil {
			t.Fatal("accepted — GH#1244 made this field refuse a relative path, and cli's " +
				"confined branch depends on that refusal NOT being in its way any more")
		} else if !strings.Contains(err.Error(), "not absolute") {
			t.Errorf("refused, but not for relativity: %v", err)
		}
	})

	t.Run("a traversal is refused for relativity, NOT for traversing", func(t *testing.T) {
		dir := t.TempDir()
		req := writeReq(t, dir)
		req.ChosenPath = filepath.Join("..", "..", "escaped.md")
		_, err := outputs.Write(req)
		if err == nil {
			t.Fatal("accepted — the premise of the whole measurement is wrong")
		}
		// THIS ASSERTION IS THE FINDING. The refusal looks like traversal handling
		// and is not. If the message ever changes to name containment, this field
		// has gained a check it does not document and the next test will prove it.
		if !strings.Contains(err.Error(), "not absolute") {
			t.Errorf("refused with %v — this row exists to record that the refusal is "+
				"about RELATIVITY. If it now names containment, re-measure the row below", err)
		}
	})

	t.Run("an absolute path outside the directory is written, by design", func(t *testing.T) {
		dir := t.TempDir()
		req := writeReq(t, dir)
		req.ChosenPath = filepath.Join(outside, "escaped.md")
		written, err := outputs.Write(req)
		if err != nil {
			t.Fatalf("refused (%v) — this field is documented as bypassing confinement, "+
				"and cli's dir==\"\" branch relies on it accepting a full path", err)
		}
		if insideDir(t, dir, written) {
			t.Errorf("wrote %q INSIDE %q — the unconfined field would then be confined, "+
				"and the dir==\"\" branch could no longer honour a full path", written, dir)
		}
	})

	t.Run("an absolute path containing .. escapes, which is why containment is unchecked", func(t *testing.T) {
		dir := t.TempDir()
		req := writeReq(t, dir)
		req.ChosenPath = filepath.Join(dir, "..", "escaped-climb.md")
		written, err := outputs.Write(req)
		if err != nil {
			t.Fatalf("refused (%v) — if this field has gained a containment check, the "+
				"measurement behind GH#1348 is stale and the comment in destinations.go "+
				"overstates what it does", err)
		}
		if insideDir(t, dir, written) {
			t.Errorf("wrote %q inside %q — this row is the proof that containment is never "+
				"checked on this route, and it no longer proves it", written, dir)
		}
	})
}

// ── THE BRANCH ITSELF ────────────────────────────────────────────────────────
//
// A regression routing BOTH branches through `DeclaredPath` would pass every
// confined row above and break the working half. The prompt wording is the
// observable half of the branch, so it is asserted directly.

func TestGH1348_TheTwoAsksAskForDifferentShapes(t *testing.T) {
	withDir := promptFor(t, "/some/output/dir")
	if !strings.Contains(withDir, "relative to") {
		t.Errorf("the configured-directory ask does not promise a relative name: %q", withDir)
	}
	if strings.Contains(withDir, "full path") {
		t.Errorf("the configured-directory ask asks for a full path: %q", withDir)
	}

	noDir := promptFor(t, "")
	if !strings.Contains(noDir, "full path") {
		t.Errorf("the no-directory ask does not ask for a full path: %q — that branch must "+
			"keep asking for one, because it feeds ChosenPath and there is nothing to be "+
			"relative to", noDir)
	}
	if strings.Contains(noDir, "relative to") {
		t.Errorf("the no-directory ask promises relativity with no directory to be relative "+
			"to: %q", noDir)
	}
}

// promptFor captures what askForPath writes for a given directory. stdin is closed
// so the ask terminates at EOF after printing.
func promptFor(t *testing.T, dir string) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	oldStderr, oldStdin := os.Stderr, os.Stdin
	os.Stderr = w

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	os.Stdin = devNull
	defer func() {
		os.Stderr, os.Stdin = oldStderr, oldStdin
		devNull.Close()
	}()

	done := make(chan string, 1)
	go func() {
		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		done <- string(buf[:n])
	}()

	if answer, ok := askForPath(dir); ok {
		t.Fatalf("askForPath returned %q with no input — it must treat EOF as a refusal", answer)
	}
	w.Close()
	return <-done
}

// ── THE FIELD CHOICE ITSELF ──────────────────────────────────────────────────
//
// The rows above drive `outputs.Write` directly, so they grade the ENGINE's two
// contracts. They would ALL still pass if cli chose the wrong field again — which
// is the shape of guard that has passed on the bug in this estate before. These
// two grade the choice, and the composition test after them grades the pair.

func TestGH1348_AssignAnswer_PutsTheAnswerInTheFieldThePromptDescribed(t *testing.T) {
	t.Run("a configured directory means the confined field", func(t *testing.T) {
		var req outputs.Request
		assignAnswer(&req, "/some/output/dir", "notes.md")
		if req.DeclaredPath != "notes.md" {
			t.Errorf("DeclaredPath = %q, want %q — the prompt promised a name relative to "+
				"the directory, and DeclaredPath is the field that keeps that promise",
				req.DeclaredPath, "notes.md")
		}
		if req.ChosenPath != "" {
			t.Errorf("ChosenPath = %q, want empty — the unconfined field must not be set "+
				"on this branch, or the answer bypasses containment", req.ChosenPath)
		}
	})

	t.Run("no directory means the unconfined field", func(t *testing.T) {
		var req outputs.Request
		assignAnswer(&req, "", "/tmp/full/path.md")
		if req.ChosenPath != "/tmp/full/path.md" {
			t.Errorf("ChosenPath = %q, want %q — with no directory there is nothing to be "+
				"relative to, and the prompt asked for a full path", req.ChosenPath, "/tmp/full/path.md")
		}
		if req.DeclaredPath != "" {
			t.Errorf("DeclaredPath = %q, want empty — routing this branch through the "+
				"confined field would refuse the absolute path just asked for", req.DeclaredPath)
		}
	})
}

// THE COMPOSITION, which is what the person actually experiences: the answer goes
// through cli's field choice and then through the engine's write. This is the test
// that fails if the field flips back, and the two legitimate-name rows are the half
// that proves the fix is usable rather than merely strict.
func TestGH1348_TheAnswerCliBuildsIsWrittenWhereThePromptPromised(t *testing.T) {
	t.Run("a relative name lands inside the directory the prompt named", func(t *testing.T) {
		dir := t.TempDir()
		req := writeReq(t, dir)
		assignAnswer(&req, dir, "notes.md")

		written, err := outputs.Write(req)
		if err != nil {
			t.Fatalf("the prompt asked for a name relative to %q, the person typed "+
				"%q, and the run refused it: %v", dir, "notes.md", err)
		}
		if !insideDir(t, dir, written) {
			t.Errorf("wrote %q, which is not inside %q", written, dir)
		}
	})

	t.Run("an answer that climbs out is refused, not written outside", func(t *testing.T) {
		dir := t.TempDir()
		climb := filepath.Join(dir, "..", "escaped-climb.md")
		req := writeReq(t, dir)
		assignAnswer(&req, dir, climb)

		written, err := outputs.Write(req)
		if err == nil {
			t.Fatalf("accepted %q and wrote %q — the prompt promised a name relative to "+
				"%q, and this is the input that distinguishes a containment check from a "+
				"relativity check", climb, written, dir)
		}
		if !strings.Contains(err.Error(), "outside the output directory") &&
			!strings.Contains(err.Error(), "not inside") {
			t.Errorf("refused for the wrong reason: %v", err)
		}
	})

	t.Run("the no-directory branch still honours a full path", func(t *testing.T) {
		target := filepath.Join(t.TempDir(), "chosen.md")
		req := writeReq(t, "")
		assignAnswer(&req, "", target)

		written, err := outputs.Write(req)
		if err != nil {
			t.Fatalf("the full path %q was refused: %v — this branch was working before "+
				"GH#1348 and a fix that breaks it is a regression the confined rows "+
				"cannot see", target, err)
		}
		if filepath.Base(written) != "chosen.md" {
			t.Errorf("wrote %q, want the person's chosen name", written)
		}
	})
}
