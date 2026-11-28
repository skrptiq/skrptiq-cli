package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// GH#1347 F2 — `home, _ := os.UserHomeDir(); path = filepath.Join(home, path[1:])`
// had GH#1295's property: a transformation that INTRODUCES a path the input never
// named. Every row below was produced by running the old expression before it was
// changed, which is why they are the rows and not a guess at what might go wrong.

func TestGH1347_TildeExpansion_ExpandsOnlyTheCurrentUsersHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no resolvable home directory on this machine: %v", err)
	}

	for _, c := range []struct {
		in   string
		want string
	}{
		{"~", home},
		{"~/projects", filepath.Join(home, "projects")},
		{"~/a/b/c", filepath.Join(home, "a", "b", "c")},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"", ""},
		{"not~in~front", "not~in~front"},
	} {
		got, err := expandLeadingTilde(c.in)
		if err != nil {
			t.Errorf("expandLeadingTilde(%q) errored: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("expandLeadingTilde(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// THE PLANTED CASES. Each of these was a path the old code FABRICATED — verified
// by running it, with `HOME=/Users/me`:
//
//	~alice/docs  ->  /Users/me/alice/docs   a path inside the WRONG home
//	~root        ->  /Users/me/root         fabricated
//	~~           ->  /Users/me/~            fabricated
//
// A refusal is a correct answer here and a plausible wrong path is not: resolving
// `~alice` needs the user database, and a guess writes somewhere the operator did
// not name — then reports the invented path in the error, misdirecting twice.
func TestGH1347_TildeExpansion_RefusesAnotherUsersHomeInsteadOfFabricating(t *testing.T) {
	for _, in := range []string{"~alice/docs", "~root", "~~", "~bob"} {
		got, err := expandLeadingTilde(in)
		if err == nil {
			t.Errorf("expandLeadingTilde(%q) = %q with no error — it must refuse rather than "+
				"guess which user's home that is", in, got)
			continue
		}
		if got != "" {
			t.Errorf("expandLeadingTilde(%q) returned %q alongside its error; a refused "+
				"expansion must yield no path at all", in, got)
		}
		if !strings.Contains(err.Error(), in) {
			t.Errorf("the refusal for %q does not quote the input: %q", in, err.Error())
		}
	}
}

// The discarded `os.UserHomeDir()` error. With it discarded and the home as "",
// `~` resolved to the process's CURRENT DIRECTORY and `~/projects` to `/projects`
// — a path written as absolute silently became a different location.
func TestGH1347_TildeExpansion_SurfacesAnUnresolvableHome(t *testing.T) {
	// os.UserHomeDir reads $HOME on unix and errors when it is empty.
	t.Setenv("HOME", "")
	if _, err := os.UserHomeDir(); err == nil {
		t.Skip("os.UserHomeDir still resolves with HOME empty on this platform")
	}

	for _, in := range []string{"~", "~/projects"} {
		got, err := expandLeadingTilde(in)
		if err == nil {
			t.Errorf("expandLeadingTilde(%q) = %q with no error — an unresolvable home must "+
				"be reported, not substituted with the empty string", in, got)
		}
		if got != "" {
			t.Errorf("expandLeadingTilde(%q) returned %q despite an unresolvable home", in, got)
		}
	}

	// And a path with no tilde is unaffected: the home is only consulted when it
	// is actually needed.
	if got, err := expandLeadingTilde("/tmp/x"); err != nil || got != "/tmp/x" {
		t.Errorf("expandLeadingTilde(%q) = (%q, %v); a path needing no home must not "+
			"consult one", "/tmp/x", got, err)
	}
}
