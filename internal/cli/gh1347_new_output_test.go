package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// GH#1347 F4 — `skrptiq new /tmp/x/my-skrpt` created `/tmp/x/my-skrpt` and printed
// `Created my-skrpt/`, because the line reported `name` (the validated basename)
// rather than `dir` (the written path).
//
// The smallest of the three findings and the most user-visible: a tool that names
// a location nothing was written to sends the reader to look at an empty place and
// conclude the tool is broken.

// captureStdout runs fn with os.Stdout redirected and returns what it printed.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	saved := os.Stdout
	os.Stdout = w

	done := make(chan string, 1)
	go func() {
		var sb strings.Builder
		buf := make([]byte, 4096)
		for {
			n, rerr := r.Read(buf)
			sb.Write(buf[:n])
			if rerr != nil {
				break
			}
		}
		done <- sb.String()
	}()

	fn()
	os.Stdout = saved
	w.Close()
	out := <-done
	r.Close()
	return out
}

func TestGH1347_New_PrintsTheDirectoryItActuallyCreated(t *testing.T) {
	// A path whose basename differs from the path, which is the whole point: the
	// old line printed the basename and the files went to the path.
	parent := t.TempDir()
	dir := filepath.Join(parent, "nested", "my-skrpt")

	var code int
	out := captureStdout(t, func() { code = New([]string{dir}) })
	if code != ExitOK {
		t.Fatalf("New(%q) = %d, want ExitOK; output: %s", dir, code, out)
	}

	// The files are where `dir` says.
	if _, err := os.Stat(filepath.Join(dir, "skrptiq.yaml")); err != nil {
		t.Fatalf("expected a manifest at %s: %v", filepath.Join(dir, "skrptiq.yaml"), err)
	}

	if !strings.Contains(out, dir) {
		t.Errorf("output %q does not name the directory that was created (%s) — a reader "+
			"following it would look in the wrong place", strings.TrimSpace(out), dir)
	}
	// And it must not report the bare basename as though that were the location.
	if strings.Contains(out, "Created my-skrpt/") {
		t.Errorf("output reports the basename as the location: %q", strings.TrimSpace(out))
	}
}

// The ordinary case — a bare name — still reads naturally, so the fix does not
// make the common invocation worse by printing something longer than it was given.
func TestGH1347_New_BareNameStillReportsThatName(t *testing.T) {
	parent := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(parent); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	var code int
	out := captureStdout(t, func() { code = New([]string{"plain-skrpt"}) })
	if code != ExitOK {
		t.Fatalf("New = %d, want ExitOK; output: %s", code, out)
	}
	if !strings.Contains(out, "Created plain-skrpt/") {
		t.Errorf("output %q should still read `Created plain-skrpt/` for a bare name",
			strings.TrimSpace(out))
	}
}
