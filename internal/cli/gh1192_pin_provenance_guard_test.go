package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// GH#1192 — the pin's SUBJECT, guarded structurally.
//
// `engine-chain-check.sh` asks two questions that want two different pins:
// checks 1 and 2 ask "is the chain DELIVERING" (subject: the fleet — what main
// pins, what app released), and check 3 asks "does THIS REF compile" (subject:
// this ref). One script, two subjects, one variable.
//
// In CI they coincide, because the workflow runs on main and the checkout IS
// origin/main — so the value was correct BY ACCIDENT OF ENVIRONMENT, and the only
// configuration where it is wrong is a local run, which is the one with no CI run
// to go red. Five months of correct behaviour was not evidence.
//
// The fix is three-valued: ok / failed / cannot-claim. These assertions exist
// because every one of them can be removed without anything failing:
//
//   - the delivery gate exists at all
//   - check 3 is OUTSIDE it (it is the offline pre-commit question)
//   - the SUMMARY respects the gate — otherwise the last line asserts the fleet
//     health the checks explicitly declined to measure
//   - no network is introduced (origin/main's pin is read from the local
//     remote-tracking ref, not fetched)

func TestGH1192_DeliveryClaimsAreGatedOnPinProvenance(t *testing.T) {
	root := moduleRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "engine-chain-check.sh"))
	if err != nil {
		t.Fatalf("reading engine-chain-check.sh: %v", err)
	}
	sh := string(raw)

	for _, want := range []struct{ frag, why string }{
		{"CAN_CLAIM_DELIVERY", "the delivery gate must exist — without it checks 1 and 2 answer a fleet question from a working tree"},
		{"CANNOT CLAIM", "the refusal must be worded as a refusal; reporting it as ok would make a non-answer look like a pass"},
		{"origin/main:.engine-pin", "the comparison pin must come from the remote-tracking ref"},
		{"read from ", "the pin's provenance must be stated in the output, not left for the reader to assume"},
	} {
		if !strings.Contains(sh, want.frag) {
			t.Errorf("engine-chain-check.sh no longer contains %q — %s", want.frag, want.why)
		}
	}

	// No network. `git show origin/main:...` is a local read; a `git fetch` here
	// would turn a correctness check into one that needs connectivity, which is
	// a different defect with better optics.
	for i, line := range strings.Split(sh, "\n") {
		code := strings.TrimSpace(line)
		if code == "" || strings.HasPrefix(code, "#") {
			continue
		}
		if strings.Contains(code, "git fetch") || strings.Contains(code, "git pull") {
			t.Errorf("engine-chain-check.sh line %d fetches (%q) — the provenance comparison must "+
				"stay offline; a check that needs connectivity is one people learn to skip", i+1, code)
		}
	}
}

// Check 3 must stay OUTSIDE the delivery gate: its subject is this ref, so a
// diverged pin changes nothing about its validity, and it is the pre-commit
// question — "would what I am about to push release?" — which must work offline.
//
// Asserted by position: the gate closes before check 3's header.
func TestGH1192_PinBuildCheckIsOutsideTheDeliveryGate(t *testing.T) {
	root := moduleRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "engine-chain-check.sh"))
	if err != nil {
		t.Fatalf("reading engine-chain-check.sh: %v", err)
	}
	sh := string(raw)

	closeIdx := strings.Index(sh, "fi  # end CAN_CLAIM_DELIVERY")
	if closeIdx < 0 {
		t.Fatal("the delivery gate's closing marker is gone — cannot establish whether check 3 " +
			"is inside or outside it, and inside would make the pre-commit check depend on the fleet")
	}
	check3Idx := strings.Index(sh, "# --- check 3:")
	if check3Idx < 0 {
		t.Fatal("check 3's header is gone")
	}
	if check3Idx < closeIdx {
		t.Error("check 3 is INSIDE the delivery gate — a diverged pin would skip the compile check " +
			"entirely, which is the opposite of what it is for")
	}
}

// The summary must not claim what the checks refused to claim. This is the bug
// the change itself nearly shipped: with a diverged pin that still builds,
// nothing failed, so the closing line said "delivering" — the script asserting
// fleet health it had explicitly declined to measure, i.e. the defect
// reintroduced in the last line. Found by testing the COMBINATION; both obvious
// cases hid it, because one fails on check 3 and the other has no divergence.
func TestGH1192_SummaryRespectsTheRefusal(t *testing.T) {
	root := moduleRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "scripts", "engine-chain-check.sh"))
	if err != nil {
		t.Fatalf("reading engine-chain-check.sh: %v", err)
	}
	sh := string(raw)

	deliverIdx := strings.LastIndex(sh, `note "Engine release chain: delivering."`)
	if deliverIdx < 0 {
		t.Fatal(`the "delivering" summary line is gone`)
	}
	// Searched AFTER the checks-1-and-2 gate closes, not from the top of the file.
	//
	// The first version of this assertion used LastIndex over everything before
	// the summary — which still found the gate around checks 1 and 2 and passed
	// happily when the summary's own guard was removed. A vacuous assertion of
	// exactly the kind this file exists to prevent, and it was caught by the
	// intervention rather than by reading it: replacing the summary guard with
	// `if false` produced no failure.
	closeIdx2 := strings.Index(sh, "fi  # end CAN_CLAIM_DELIVERY")
	if closeIdx2 < 0 || closeIdx2 >= deliverIdx {
		t.Fatal("cannot locate the region between the checks gate and the summary")
	}
	region := sh[closeIdx2:deliverIdx]
	if !strings.Contains(region, `if [ "$CAN_CLAIM_DELIVERY" -eq 0 ]`) {
		t.Error(`the "delivering" summary is not guarded by its OWN CAN_CLAIM_DELIVERY check — ` +
			`with a diverged pin the script would assert chain health it refused to measure, ` +
			`which is this issue's own defect reintroduced in the last line`)
	}
	if !strings.Contains(sh, "NO VERDICT") {
		t.Error("the no-verdict summary is gone — a refusal with no summary reads as a pass")
	}
}
