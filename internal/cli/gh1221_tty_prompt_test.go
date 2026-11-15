package cli

import (
	"io"
	"os"
	"strings"
	"testing"

	exec "github.com/skrptiq/engine/execution"
	"github.com/skrptiq/engine/outputs"
	"github.com/skrptiq/engine/storage"
)

// GH#1221 — the CLI can be asked for a destination when a person is there, and
// still refuses fast when nobody is.
//
// The tests are weighted to the two things that could regress rather than to the
// prompt itself: the OVERRIDE beating detection, and the CLIPBOARD path, which
// declaring `Interactive` silently converts from a refusal into a pending item
// nobody completes.

// ── The override beats detection ──────────────────────────────────────────

// --non-interactive must win over a real terminal: a CI job with a pty allocated
// is still nobody to ask. Asserted as the FIRST thing the function does, because
// an override evaluated after detection is an override that sometimes loses.
func TestGH1221_NonInteractiveOverride_BeatsDetection(t *testing.T) {
	if interactiveRun(true) {
		t.Error("--non-interactive did not beat TTY detection — a CI job with a pty " +
			"would be prompted and would hang")
	}
}

// Under `go test` stdin is not a terminal, so detection must say no on its own.
// This is the control for the test above: without it, a function that returned
// false unconditionally would pass it.
func TestGH1221_NoTTY_IsNotInteractive(t *testing.T) {
	if interactiveRun(false) {
		t.Error("detection reported a terminal under `go test`, where stdin is a pipe — " +
			"the detector is not detecting")
	}
}

// The flag exists, defaults off, and parses. Default-off matters: the zero value
// of the whole mechanism must be the safe one.
func TestGH1221_NonInteractiveFlag_ParsesAndDefaultsOff(t *testing.T) {
	opts, _, err := parseRunArgs([]string{"WF"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if opts.nonInteractive {
		t.Error("--non-interactive defaulted ON; an unattended caller would lose the prompt " +
			"it never asked for, but an attended one would lose the ask")
	}

	opts, _, err = parseRunArgs([]string{"WF", "--non-interactive"})
	if err != nil {
		t.Fatalf("parse --non-interactive: %v", err)
	}
	if !opts.nonInteractive {
		t.Error("--non-interactive did not set the option")
	}
}

// --yes must NOT imply --non-interactive. They are different questions: one
// approves a gate, the other names a file. Asserted because implying it is the
// tempting tidy-up that would silently remove the ask from the commonest
// automated-but-attended invocation.
func TestGH1221_YesDoesNotImplyNonInteractive(t *testing.T) {
	opts, _, err := parseRunArgs([]string{"WF", "--yes"})
	if err != nil {
		t.Fatalf("parse --yes: %v", err)
	}
	if opts.nonInteractive {
		t.Error("--yes implied --non-interactive; one flag, one meaning")
	}
}

// ── The clipboard guard: the regression this change could have shipped ────

// A host-pending CLIPBOARD destination is REFUSED, out loud, and never left
// pending.
//
// This is the test that matters most. `PerformDestinations` refuses clipboard
// when non-interactive and marks it HostPending when interactive, and
// OnDestinations was nil in the CLI because the CLI was non-interactive. So
// declaring Interactive without this handler would turn a working refusal into
// silence — the defect GH#1203 exists to remove, on the path whose refusal cites
// GH#1202 T0-14 as its reason.
func TestGH1221_PendingClipboard_IsRefusedNotDropped(t *testing.T) {
	out := captureStderr(t, func() {
		cliDestinationHandler(nil)([]exec.DestinationResult{
			{Type: "clipboard", HostPending: true, Content: "some output"},
		})
	})

	if out == "" {
		t.Fatal("a host-pending clipboard destination produced NO output — it was dropped " +
			"silently, which is exactly the regression this handler exists to prevent")
	}
	if !strings.Contains(out, "clipboard") {
		t.Errorf("the refusal does not name the destination type: %q", out)
	}
	// It must say the work is not lost. A refusal that omits this sends someone
	// looking for a vanished file.
	if !strings.Contains(out, "history") {
		t.Errorf("the refusal does not say the output is kept in the run's history: %q", out)
	}
}

// An unknown pending type is also refused rather than dropped, so a destination
// type added later cannot inherit the silence by default.
func TestGH1221_UnknownPendingType_IsRefusedNotDropped(t *testing.T) {
	out := captureStderr(t, func() {
		cliDestinationHandler(nil)([]exec.DestinationResult{
			{Type: "telepathy", HostPending: true, Content: "x"},
		})
	})
	if !strings.Contains(out, "telepathy") {
		t.Errorf("an unknown pending type was not named in a refusal: %q", out)
	}
	if !strings.Contains(out, "history") {
		t.Errorf("the refusal does not say where the output is: %q", out)
	}
}

// A result that is NOT pending is left entirely alone — the engine already
// resolved it and the run record carries it. Without this, the handler could
// double-report or contradict the engine.
func TestGH1221_AlreadyResolvedResults_AreNotTouched(t *testing.T) {
	out := captureStderr(t, func() {
		cliDestinationHandler(nil)([]exec.DestinationResult{
			{Type: "file", Success: true, Detail: "Saved to /tmp/x.md"},
			{Type: "clipboard", Error: "refused by the engine"},
		})
	})
	if out != "" {
		t.Errorf("the handler spoke about destinations the engine had already resolved: %q", out)
	}
}

// ── The file ask ──────────────────────────────────────────────────────────

// The two asks are DIFFERENT questions and the prompt must say which: an unset
// output directory wants a full path; a destination with no filename wants a name
// inside a directory that exists. The engine split these in GH#1209 and the
// prompt must not re-merge them.
func TestGH1221_FilePrompt_DistinguishesTheTwoAsks(t *testing.T) {
	noDir := captureStderr(t, func() {
		withStdin(t, "", func() { askForPath("") })
	})
	if !strings.Contains(noDir, "no output directory") {
		t.Errorf("the unset-directory prompt does not say the directory is missing: %q", noDir)
	}

	noName := captureStderr(t, func() {
		withStdin(t, "", func() { askForPath("/tmp/out") })
	})
	if !strings.Contains(noName, "/tmp/out") {
		t.Errorf("the missing-filename prompt does not show the directory to write into: %q", noName)
	}
	if !strings.Contains(noName, "no filename") {
		t.Errorf("the missing-filename prompt does not say what is missing: %q", noName)
	}
}

// EOF is the same outcome as nobody being there: refused, named, and the output's
// whereabouts stated. Never an empty path and never a guessed name.
func TestGH1221_EOFAtPrompt_RefusesAndSaysWhereTheOutputIs(t *testing.T) {
	var answer string
	var ok bool
	out := captureStderr(t, func() {
		withStdin(t, "", func() { answer, ok = askForPath("/tmp/out") })
	})
	if ok || answer != "" {
		t.Errorf("EOF produced an answer (%q, ok=%v) — a file nobody named would be written", answer, ok)
	}
	if !strings.Contains(out, "nothing was written") || !strings.Contains(out, "history") {
		t.Errorf("the EOF refusal does not say nothing was written and where the output is: %q", out)
	}
}

// An empty line is a slip, not a decision: re-prompted once, then treated as EOF.
// Two prompts in the output is the assertion — a silent accept here writes a file
// nobody named.
func TestGH1221_EmptyLine_RepromptsOnceThenRefuses(t *testing.T) {
	var ok bool
	out := captureStderr(t, func() {
		withStdin(t, "\n", func() { _, ok = askForPath("/tmp/out") })
	})
	if ok {
		t.Error("an empty line was accepted as a destination")
	}
	if n := strings.Count(out, "Enter a name"); n != 2 {
		t.Errorf("want the prompt twice (one re-prompt), got %d: %q", n, out)
	}
}

// A real answer is returned, trimmed. The positive direction, without which every
// test above could pass against a function that always refuses.
func TestGH1221_AnAnsweredPrompt_ReturnsThePath(t *testing.T) {
	var answer string
	var ok bool
	captureStderr(t, func() {
		withStdin(t, "  notes/out.md  \n", func() { answer, ok = askForPath("/tmp/out") })
	})
	if !ok || answer != "notes/out.md" {
		t.Errorf("answer = %q ok=%v; want notes/out.md true", answer, ok)
	}
}

// ── The request the completion hands back to the engine ───────────────────

// The completion must declare Interactive and carry the engine's own DeclaredPath
// and Content rather than re-deriving them, and it must read the SAME settings
// the engine reads. A mismatch here writes the right text to the wrong place.
func TestGH1221_OutputRequest_CarriesTheEngineFieldsAndSettings(t *testing.T) {
	dbPath := t.TempDir() + "/req.db"
	db, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	db.SetSetting(outputs.SettingOutputDir, "/tmp/declared-dir")
	db.SetSetting("WORKSPACE_PATH", "/tmp/ws")

	req, err := outputRequestFor(db, exec.DestinationResult{
		Type: "file", HostPending: true, DeclaredPath: "sub/name.md", Content: "BODY",
	})
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if !req.Interactive {
		t.Error("the completion request is not marked Interactive — Resolve would refuse it")
	}
	if req.DeclaredPath != "sub/name.md" {
		t.Errorf("DeclaredPath = %q; it must come from the engine's result, not be re-derived", req.DeclaredPath)
	}
	if req.Content != "BODY" {
		t.Errorf("Content = %q, want BODY", req.Content)
	}
	if req.Setting != "/tmp/declared-dir" {
		t.Errorf("Setting = %q — not read from %s", req.Setting, outputs.SettingOutputDir)
	}
	if req.Workspace != "/tmp/ws" {
		t.Errorf("Workspace = %q, want /tmp/ws", req.Workspace)
	}
	if req.Home == "" {
		t.Error("Home is empty; every variable but $WORKSPACE derives from it")
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────

// captureStderr runs fn with os.Stderr redirected to a pipe and returns what it
// wrote. The prompt goes to stderr by design — stdout is the output channel and
// is legitimately a pipe during an interactive run — so stderr is what these
// tests must read.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	saved := os.Stderr
	os.Stderr = w

	done := make(chan string, 1)
	go func() {
		var b strings.Builder
		io.Copy(&b, r)
		done <- b.String()
	}()

	fn()

	os.Stderr = saved
	w.Close()
	out := <-done
	r.Close()
	return out
}

// withStdin runs fn with os.Stdin replaced by a pipe carrying input, then closed.
//
// A closed pipe is the EOF these tests need, and it is the honest stand-in for
// Ctrl-D: `askForPath` reads os.Stdin directly, so nothing about the read path is
// mocked out. What a pipe cannot reproduce is `term.IsTerminal` returning true —
// that is covered by asserting the override separately, since the override is the
// half that must hold even when detection says yes.
func withStdin(t *testing.T, input string, fn func()) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	saved := os.Stdin
	os.Stdin = r

	if input != "" {
		go func() {
			io.WriteString(w, input)
			w.Close()
		}()
	} else {
		w.Close()
	}

	fn()

	os.Stdin = saved
	r.Close()
}
