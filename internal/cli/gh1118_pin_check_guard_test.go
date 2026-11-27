package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// GH#1118 — a structural guard, because the fix IS a structure.
//
// `ci.yml` builds against app `main`; `release.yml` builds against `.engine-pin`.
// Those are different engines, and for twelve days nothing said so: the
// distinction lived in a code comment, so "CI is green" meant two things at once
// and the one that ships was the one nobody measured. `engine/outputs` and three
// more symbols entered cli HEAD with CI green throughout, and the only instrument
// that noticed was a SCHEDULED watchdog — which had never once succeeded.
//
// The fix has three load-bearing structural properties, and all three are the
// kind that a later edit removes without anyone noticing, because removing them
// makes nothing fail:
//
//  1. `scripts/engine-pin-build.sh` exists and is executable.
//  2. `engine-chain-check.sh` DELEGATES to it rather than implementing the
//     compile itself — one authority per rule (GH#1058). Two copies of a compile
//     check drift, and the drift is invisible until they disagree.
//  3. `ci.yml` has both named jobs, and the pin job calls that same script.
//
// This test asserts the structure holds. It is deliberately about file contents
// rather than behaviour: the behaviour is verified on real history (cli 48ed3af
// against pin 18c63f01 fails; its parent passes), and that verification lives in
// the script's header where the next reader of the script will find it.

func repoRootForGuard(t *testing.T) string {
	t.Helper()
	root := moduleRoot(t)
	return root
}

// 1 + 2 — the script exists, and the watchdog calls it rather than reimplementing it.
func TestGH1118_PinBuildIsOneAuthorityWithTwoCallers(t *testing.T) {
	root := repoRootForGuard(t)

	script := filepath.Join(root, "scripts", "engine-pin-build.sh")
	info, err := os.Stat(script)
	if err != nil {
		t.Fatalf("scripts/engine-pin-build.sh is missing: %v — ci.yml and the watchdog both "+
			"invoke it, so its absence breaks both callers", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Errorf("scripts/engine-pin-build.sh is not executable (mode %v); both callers run it directly", info.Mode())
	}

	chain, err := os.ReadFile(filepath.Join(root, "scripts", "engine-chain-check.sh"))
	if err != nil {
		t.Fatalf("reading engine-chain-check.sh: %v", err)
	}
	if !strings.Contains(string(chain), "engine-pin-build.sh") {
		t.Error("engine-chain-check.sh no longer calls engine-pin-build.sh — check 3 has been " +
			"re-implemented locally, which makes two authorities for one rule (GH#1058). The " +
			"copies will drift and the drift is invisible until they disagree.")
	}
	// The delegation is only real if the watchdog does NOT also do the build itself.
	// `go build -C` inside the chain script is the signature of the old inlined copy.
	if strings.Contains(string(chain), "go build -C") {
		t.Error("engine-chain-check.sh contains its own `go build -C` — the delegation to " +
			"engine-pin-build.sh has been undone or shadowed, so there are two implementations again")
	}
}

// 3 — two named checks, with the subject in the NAME. A comment is what allowed
// two meanings of "CI is green" to coexist, so the names are the deliverable.
//
// Separate JOBS rather than steps, deliberately: GitHub's required status checks
// are per-job, so two subjects need two jobs in order to be two named checks.
// One job with two steps is one check whose meaning depends on which step failed.
func TestGH1118_CIHasTwoNamedEngineSubjects(t *testing.T) {
	root := repoRootForGuard(t)
	raw, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("reading ci.yml: %v", err)
	}
	ci := string(raw)

	for _, want := range []struct{ frag, why string }{
		{"engine-app-main:", "the app-main job must exist as its own job"},
		{"engine-release-pin:", "the release-pin job must exist as its own job — per-job is what makes it a separately requirable check"},
		{`name: "engine: app main"`, "the app-main subject must be in the job NAME, not only in a comment"},
		{`name: "engine: release pin"`, "the release-pin subject must be in the job NAME, not only in a comment"},
		{"scripts/engine-pin-build.sh", "the pin job must call the shared script, not reimplement the build"},
	} {
		if !strings.Contains(ci, want.frag) {
			t.Errorf("ci.yml no longer contains %q — %s", want.frag, want.why)
		}
	}

	// The pin job must NOT be made to depend on the whole chain script: checks 1
	// and 2 of that script query the releases API and ask whether app's chain is
	// DELIVERING. Running them per-PR fails cli's PRs whenever app simply has not
	// released recently — the coupling this split exists to confine to one check.
	//
	// Matched on NON-COMMENT lines only. The first version of this assertion
	// searched the whole file and failed on the comment that explains why CI must
	// not call that script — a guard that read prose as behaviour. Caught by
	// running it, which is the only way that kind of mistake surfaces.
	for i, line := range strings.Split(ci, "\n") {
		code := strings.TrimSpace(line)
		if code == "" || strings.HasPrefix(code, "#") {
			continue
		}
		if strings.Contains(code, "engine-chain-check.sh") {
			t.Errorf("ci.yml line %d invokes engine-chain-check.sh (%q) — its checks 1 and 2 "+
				"query the releases API, so per-PR runs would fail whenever app had not released "+
				"recently. CI must call engine-pin-build.sh only.", i+1, code)
		}
	}
}
