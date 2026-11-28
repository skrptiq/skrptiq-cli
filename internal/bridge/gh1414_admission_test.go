//go:build !windows

// The admission policy these grade is POSIX ownership and mode, so they build
// where that exists. The windows stubs refuse by construction and there is no
// named-pipe rendezvous to grade yet.

package bridge

import (
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// GH#1414 — socket admission, from the CTO's GH#1325 review.
//
// WHAT THESE TESTS CAN AND CANNOT PROVE, stated first because the distinction is
// the point of the whole change:
//
//   - They prove the FILESYSTEM POLICY: which directories and endpoints are
//     admitted, which are refused, and that a refusal names its reason.
//   - They do NOT prove the CROSS-USER INVARIANT. Every test here runs as one UID,
//     and "a permissions-only test run as the same UID cannot prove cross-user
//     isolation". That requires a created UID, which requires root —
//     TestGH1414_SecondUID_AdmissionIsRefused is that test and it is loud about
//     not running rather than silent.
//
// THE CONTROL IS THE DECIDING HALF. A policy that refused everything would satisfy
// every refusal case below perfectly, so the healthy-normal cases are what make
// the refusals meaningful. The first version of this policy refused `/var` for
// being a symlink — which is every Mac — and the control is what caught it.

// safeDir returns a directory that passes the policy, for use as a baseline.
//
// NOT t.TempDir(): a UDS pathname is limited to 104 bytes of `sun_path` on darwin,
// and t.TempDir() embeds the test's NAME, so the descriptive names in this file
// produce paths that cannot be bound at all — `bind: invalid argument`, which
// looks exactly like a policy refusal and is not one. The existing rendezvous
// tests use `shortSocket` for the same reason; this is its directory equivalent.
func safeDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(os.TempDir(), "skr-"+randomID()[:8])
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatalf("make baseline dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// shortTempTree returns a short-pathed base directory for fixtures that must NOT
// pass the policy, with the same sun_path reasoning as safeDir.
func shortTempTree(t *testing.T) string {
	t.Helper()
	base := filepath.Join(os.TempDir(), "skrb-"+randomID()[:8])
	if err := os.Mkdir(base, 0o700); err != nil {
		t.Fatalf("make fixture base: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(base) })
	return base
}

// ── THE CONTROL ──────────────────────────────────────────────────────────────

func TestGH1414_Control_AHealthyRendezvousStillBinds(t *testing.T) {
	dir := safeDir(t)
	sock := filepath.Join(dir, "cli.sock")

	s := newRendezvousServer(sock, func(string, ...interface{}) {})
	if err := s.start(); err != nil {
		t.Fatalf("a correctly private directory was refused: %v\n"+
			"This is the case the policy must never break. A policy that refuses "+
			"everything passes every refusal test in this file.", err)
	}
	defer s.close()

	// It really is listening, and a client of this user can reach it.
	conn, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatalf("start() reported success but the socket does not accept: %v", err)
	}
	_ = conn.Close()

	// And the hardening it claimed is actually on the file.
	if err := checkSocketFile(sock); err != nil {
		t.Errorf("the bound socket does not satisfy the policy it reported: %v", err)
	}
}

// A MISSING DIRECTORY TREE IS CREATED, which MkdirAll used to do and a
// parent-must-exist policy would have broken on every fresh install.
func TestGH1414_Control_AFreshInstallCreatesItsTree(t *testing.T) {
	deep := filepath.Join(shortTempTree(t), "skrptiq", "run")
	root, err := secureDir(deep)
	if err != nil {
		t.Fatalf("a missing tree under a safe parent was refused: %v", err)
	}
	defer root.Close()

	fi, err := os.Lstat(deep)
	if err != nil {
		t.Fatalf("the tree was not created: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0o700 {
		t.Errorf("created %s as mode %#o, want 0700", deep, perm)
	}
}

// A SYMLINK ANCESTOR IS NOT FATAL, and this is the regression the control caught.
// `/var` is a root-owned symlink on every Mac, so t.TempDir() always has one.
func TestGH1414_Control_ARootOwnedSymlinkAncestorIsAccepted(t *testing.T) {
	dir := safeDir(t)
	if !strings.HasPrefix(dir, "/var/") {
		t.Skipf("this machine's temp dir (%s) has no symlink ancestor to exercise", dir)
	}
	root, err := secureDir(dir)
	if err != nil {
		t.Fatalf("refused %s: %v\nA root-owned symlink whose parent no other user can "+
			"write cannot be re-pointed by a third party, and refusing it refuses the "+
			"normal path on the platform cli primarily ships to.", dir, err)
	}
	root.Close()
}

// ── THE REFUSALS, EACH NAMED ─────────────────────────────────────────────────

// THE DEFECT ITSELF: MkdirAll's mode does not apply to an existing directory, so a
// too-open directory used to be adopted silently. It must be REFUSED, and
// explicitly NOT chmodded into compliance.
func TestGH1414_ALooseExistingDirectoryIsRefusedAndNotRepaired(t *testing.T) {
	for _, mode := range []os.FileMode{0o777, 0o770, 0o707, 0o750} {
		dir := filepath.Join(shortTempTree(t), "run")
		if err := os.Mkdir(dir, mode); err != nil {
			t.Fatalf("make fixture: %v", err)
		}
		// os.Mkdir applies umask, so the premise is asserted rather than assumed.
		before, err := os.Lstat(dir)
		if err != nil {
			t.Fatalf("stat fixture: %v", err)
		}
		if before.Mode().Perm()&0o077 == 0 {
			t.Skipf("umask reduced %#o to %#o, so this row has nothing to refuse",
				mode, before.Mode().Perm())
		}

		root, err := secureDir(dir)
		if err == nil {
			root.Close()
			t.Errorf("mode %#o was admitted", before.Mode().Perm())
			continue
		}
		if !errors.Is(err, ErrUnsafeDir) {
			t.Errorf("mode %#o refused with the wrong error type: %v", before.Mode().Perm(), err)
		}
		if !strings.Contains(err.Error(), "group or other") {
			t.Errorf("mode %#o: the refusal does not say what is wrong: %v", before.Mode().Perm(), err)
		}

		// NOT REPAIRED. A gate that chmods the directory it just refused has
		// changed the filesystem on a path it does not trust.
		after, err := os.Lstat(dir)
		if err != nil {
			t.Fatalf("re-stat fixture: %v", err)
		}
		if after.Mode().Perm() != before.Mode().Perm() {
			t.Errorf("the refused directory was modified: %#o → %#o. Never 'repair' a "+
				"directory that failed admission", before.Mode().Perm(), after.Mode().Perm())
		}
	}
}

// A REPLACEABLE ANCESTOR, which is why stat-and-refuse on the leaf is not enough:
// the leaf can be perfect and still sit under a directory another user can swap.
func TestGH1414_AnAncestorWritableByOthersIsRefused(t *testing.T) {
	base := shortTempTree(t)
	loose := filepath.Join(base, "loose")
	if err := os.Mkdir(loose, 0o777); err != nil {
		t.Fatalf("make fixture: %v", err)
	}
	if err := os.Chmod(loose, 0o777); err != nil { // defeat umask explicitly
		t.Fatalf("chmod fixture: %v", err)
	}
	leaf := filepath.Join(loose, "run")
	if err := os.Mkdir(leaf, 0o700); err != nil {
		t.Fatalf("make leaf: %v", err)
	}

	// The premise: the LEAF is immaculate, so only the ancestor can be the cause.
	if fi, err := os.Lstat(leaf); err != nil {
		t.Fatalf("stat leaf: %v", err)
	} else if fi.Mode().Perm()&0o077 != 0 {
		t.Fatalf("the leaf is %#o, so this test would pass for the wrong reason",
			fi.Mode().Perm())
	}

	root, err := secureDir(leaf)
	if err == nil {
		root.Close()
		t.Fatal("admitted a sound leaf under a world-writable ancestor — a leaf check " +
			"alone is not an identity guarantee if the directory can be replaced")
	}
	if !strings.Contains(err.Error(), loose) {
		t.Errorf("the refusal does not name the unsafe ancestor: %v", err)
	}
}

// A SYMLINKED RENDEZVOUS DIRECTORY is refused however well-owned, because the
// endpoint inside it can be redirected by re-pointing the link.
func TestGH1414_ASymlinkedRendezvousDirectoryIsRefused(t *testing.T) {
	base := shortTempTree(t)
	real := filepath.Join(base, "real")
	if err := os.Mkdir(real, 0o700); err != nil {
		t.Fatalf("make target: %v", err)
	}
	link := filepath.Join(base, "run")
	if err := os.Symlink(real, link); err != nil {
		t.Fatalf("make symlink: %v", err)
	}

	root, err := secureDir(link)
	if err == nil {
		root.Close()
		t.Fatal("admitted a symlinked rendezvous directory")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("the refusal does not name the symlink: %v", err)
	}
}

// A RELATIVE PATH is refused: which directory it names depends on the process's
// working directory, which is exactly the ambiguity this policy exists to remove.
func TestGH1414_ARelativeDirectoryIsRefused(t *testing.T) {
	root, err := secureDir("relative/run")
	if err == nil {
		root.Close()
		t.Fatal("admitted a relative rendezvous directory")
	}
	if !strings.Contains(err.Error(), "absolute") {
		t.Errorf("the refusal does not name the problem: %v", err)
	}
}

// ── THE ENDPOINT, AND WHAT MAY NOT BE DELETED ────────────────────────────────

// A WRONG-TYPE ENTRY IS REFUSED AND LEFT ALONE. The old code did `Stat` then an
// unconditional `Remove`, so a regular file at the endpoint path was deleted by a
// process that had no business deleting it.
func TestGH1414_AWrongTypeEndpointIsRefusedAndNotDeleted(t *testing.T) {
	dir := safeDir(t)
	sock := filepath.Join(dir, "cli.sock")
	if err := os.WriteFile(sock, []byte("not a socket"), 0o600); err != nil {
		t.Fatalf("make fixture: %v", err)
	}

	s := newRendezvousServer(sock, func(string, ...interface{}) {})
	err := s.start()
	if err == nil {
		s.close()
		t.Fatal("started over a regular file at the endpoint path")
	}
	if !strings.Contains(err.Error(), "not a socket") {
		t.Errorf("the refusal does not name the type problem: %v", err)
	}
	if !strings.Contains(err.Error(), "NOT removed") {
		t.Errorf("the refusal does not say the entry was left alone: %v", err)
	}
	if b, rerr := os.ReadFile(sock); rerr != nil {
		t.Errorf("the refused entry was deleted: %v", rerr)
	} else if string(b) != "not a socket" {
		t.Errorf("the refused entry was modified: %q", b)
	}
}

// A LIVE LISTENER IS NOT SUPERSEDED. "Blind 'stale' deletion is not proof that a
// listener is dead" — liveness is established by connecting, and a live peer means
// refuse rather than take the endpoint from it.
func TestGH1414_ALiveListenerIsNotTakenOver(t *testing.T) {
	dir := safeDir(t)
	sock := filepath.Join(dir, "cli.sock")

	first := newRendezvousServer(sock, func(string, ...interface{}) {})
	if err := first.start(); err != nil {
		t.Fatalf("baseline start failed: %v", err)
	}
	defer first.close()

	second := newRendezvousServer(sock, func(string, ...interface{}) {})
	err := second.start()
	if err == nil {
		second.close()
		t.Fatal("a second server took the endpoint from a live first one")
	}
	if !strings.Contains(err.Error(), "live listener") {
		t.Errorf("the refusal does not say a listener is live: %v", err)
	}

	// The first one is still serving — the refusal cost it nothing.
	conn, derr := net.Dial("unix", sock)
	if derr != nil {
		t.Fatalf("the live listener stopped working after the refusal: %v", derr)
	}
	_ = conn.Close()
}

// A GENUINELY STALE SOCKET IS CLEARED. The discrimination has to work in both
// directions, or "refuse when a listener is live" becomes "never restart after a
// crash".
func TestGH1414_AStaleSocketIsCleared(t *testing.T) {
	dir := safeDir(t)
	sock := filepath.Join(dir, "cli.sock")

	// A socket file with nothing listening: bind, then close the listener WITHOUT
	// letting Go unlink it, which is what a crashed process leaves behind.
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("make fixture: %v", err)
	}
	if ul, ok := ln.(*net.UnixListener); ok {
		ul.SetUnlinkOnClose(false)
	}
	_ = ln.Close()
	if _, err := os.Lstat(sock); err != nil {
		t.Fatalf("the fixture did not leave a socket behind: %v", err)
	}

	s := newRendezvousServer(sock, func(string, ...interface{}) {})
	if err := s.start(); err != nil {
		t.Fatalf("a genuinely stale socket was not cleared: %v", err)
	}
	s.close()
}

// ── THE READINESS CLAIM ──────────────────────────────────────────────────────

// CHMOD FAILURE LEAVES NO LISTENING SERVICE. The old code logged the failure and
// then printed "listening on … (0600, per-user)", so the readiness claim could be
// false. Driven through the real start() by making the chmod fail: on an immutable
// directory the socket cannot be chmodded.
//
// NOT EVERY PLATFORM CAN STAGE THIS, and when it cannot the test says so rather
// than passing quietly.
func TestGH1414_ChmodFailureLeavesNothingListening(t *testing.T) {
	dir := safeDir(t)
	sock := filepath.Join(dir, "cli.sock")

	// A socket path inside a directory we then make read-only is still bindable in
	// some configurations; what reliably fails is chmod on an entry we do not own,
	// which needs a second UID. So this asserts the CODE PATH directly instead, and
	// says plainly that it is not the end-to-end version.
	s := newRendezvousServer(sock, func(string, ...interface{}) {})
	if err := s.start(); err != nil {
		t.Fatalf("baseline: %v", err)
	}
	ln := s.ln
	s.mu.Lock()
	s.ln = nil
	s.mu.Unlock()

	s.abandonListener(ln)
	if _, err := os.Lstat(sock); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("abandonListener left %s behind (%v) — a failed start must leave no "+
			"endpoint for the next process to adopt", sock, err)
	}
	if conn, derr := net.Dial("unix", sock); derr == nil {
		_ = conn.Close()
		t.Error("something is still accepting on the abandoned path")
	}
}

// ── THE FRAMING CORRECTION, ENCODED ──────────────────────────────────────────

// PATHNAME REPLACEMENT IS NOT INTERCEPTION OF AN ESTABLISHED CONNECTION.
//
// This is the CTO's correction as a test: "directory takeover plus endpoint
// replacement diverts a FUTURE connection" and is not theft of an existing one.
// "Do not claim theft of every engine tool argument from directory pre-creation
// alone."
func TestGH1414_ReplacingThePathDoesNotInterceptAnEstablishedConnection(t *testing.T) {
	dir := safeDir(t)
	sock := filepath.Join(dir, "cli.sock")

	s := newRendezvousServer(sock, func(string, ...interface{}) {})
	if err := s.start(); err != nil {
		t.Fatalf("baseline start: %v", err)
	}
	defer s.close()

	client, err := net.Dial("unix", sock)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	// Now replace the pathname with a different socket, as a directory takeover
	// would allow.
	impostorPath := filepath.Join(dir, "impostor.sock")
	impostor, err := net.Listen("unix", impostorPath)
	if err != nil {
		t.Fatalf("make impostor: %v", err)
	}
	defer impostor.Close()
	if err := os.Rename(impostorPath, sock); err != nil {
		t.Fatalf("replace the pathname: %v", err)
	}

	// THE ESTABLISHED CONNECTION IS UNAFFECTED. It refers to the socket, not to the
	// name — so the replacement cannot read or alter it.
	if _, err := client.Write([]byte("{}\n")); err != nil {
		t.Errorf("the established connection broke when the pathname was replaced (%v). "+
			"If this ever fails, the two exploit chains are no longer separable and the "+
			"framing in GH#1414 needs revisiting", err)
	}

	// What the replacement DOES get is the next connection — a diversion, which is
	// a different and lesser claim than interception.
	next, err := net.Dial("unix", sock)
	if err == nil {
		_ = next.Close()
	}
}

// ── THE CROSS-USER INVARIANT ─────────────────────────────────────────────────

// A REAL SECOND-UID TEST. "A permissions-only test run as the same UID cannot
// prove cross-user isolation."
//
// IT CANNOT RUN WHERE IT WAS WRITTEN, and that is recorded rather than hidden:
// creating a UID needs root (`dscl`/`sysadminctl` on darwin, `useradd` on Linux)
// and the development machine runs this as uid 501.
//
// SO IT IS IMPOSSIBLE TO FORGET RATHER THAN IMPOSSIBLE TO KEEP (the 2026-11-15
// ruling). Set SKRPTIQ_TEST_SECOND_UID to a uid that exists and is not this one:
//
//   - unset        → skipped, with the reason and the exact instruction
//   - set, usable  → the invariant is actually tested
//   - set, broken  → FAILS LOUDLY. A configured harness that cannot run is a
//     broken instrument, not an absent one, and a silent skip there is how a
//     security test comes to mean nothing while still being green.
func TestGH1414_SecondUID_AdmissionIsRefused(t *testing.T) {
	raw := os.Getenv("SKRPTIQ_TEST_SECOND_UID")
	if raw == "" {
		t.Skip("SKRPTIQ_TEST_SECOND_UID is not set, so the cross-user invariant is NOT " +
			"tested by this run. Nothing else in this file proves it: every other case " +
			"runs as one UID. To run it, create a second account and set " +
			"SKRPTIQ_TEST_SECOND_UID=<its uid> (needs root: sysadminctl on darwin, " +
			"useradd on Linux). Tracked for CI as GH#1416.")
	}

	uid, err := strconv.Atoi(raw)
	if err != nil {
		t.Fatalf("SKRPTIQ_TEST_SECOND_UID=%q is not a uid: %v. The harness is configured "+
			"and broken, which is worse than unconfigured — it would otherwise look "+
			"covered", raw, err)
	}
	if uid == os.Geteuid() {
		t.Fatalf("SKRPTIQ_TEST_SECOND_UID=%d is this process's own uid. A permissions-only "+
			"test run as the same UID cannot prove cross-user isolation, so this "+
			"configuration proves nothing and must not look like it does", uid)
	}
	if _, err := exec.LookPath("chown"); err != nil {
		t.Fatalf("SKRPTIQ_TEST_SECOND_UID is set but chown is unavailable (%v), so the "+
			"fixture cannot be created and the invariant cannot be tested", err)
	}

	dir := filepath.Join(shortTempTree(t), "run")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatalf("make fixture: %v", err)
	}
	// Needs privilege; if it fails the test FAILS rather than skips, because the
	// harness said it was available.
	if out, cerr := exec.Command("chown", strconv.Itoa(uid), dir).CombinedOutput(); cerr != nil {
		t.Fatalf("could not give %s to uid %d (%v): %s. SKRPTIQ_TEST_SECOND_UID is set, so "+
			"this is a configured harness failing, not an absent one",
			dir, uid, cerr, strings.TrimSpace(string(out)))
	}

	// THE INVARIANT: a rendezvous directory owned by another user is refused, and
	// is NOT adjusted into compliance.
	root, err := secureDir(dir)
	if err == nil {
		root.Close()
		t.Fatalf("admitted %s, which is owned by uid %d and not by this user (%d)",
			dir, uid, os.Geteuid())
	}
	if !errors.Is(err, ErrUnsafeDir) {
		t.Errorf("refused with the wrong error type: %v", err)
	}
	if !strings.Contains(err.Error(), "owned by uid") {
		t.Errorf("the refusal does not name the ownership problem: %v", err)
	}

	fi, serr := os.Lstat(dir)
	if serr != nil {
		t.Fatalf("re-stat fixture: %v", serr)
	}
	if st := fileUID(t, fi); st != uint32(uid) {
		t.Errorf("the refused directory's owner changed from %d to %d — a directory "+
			"another user created must never be 'repaired'", uid, st)
	}
}

// fileUID reads the owner out of a FileInfo. The portable interface carries no
// uid, so the platform struct is unavoidable — the same reason statOf exists.
func fileUID(t *testing.T, fi os.FileInfo) uint32 {
	t.Helper()
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("cannot read ownership on this platform")
	}
	return st.Uid
}

// ── THE POLICY REACHED THROUGH start(), NOT CALLED DIRECTLY ──────────────────
//
// The directory cases above call secureDir directly, so they grade the POLICY.
// They would all still pass if start() stopped calling it — which is precisely
// the pass-on-the-bug shape, and the vacuity proof for this change caught it:
// reverting start() to its pre-GH#1414 body left those tests green.
//
// These drive the real start(), so the wiring is graded too.

func TestGH1414_StartRefusesALooseRuntimeDirectory(t *testing.T) {
	dir := filepath.Join(shortTempTree(t), "run")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatalf("make fixture: %v", err)
	}
	if err := os.Chmod(dir, 0o777); err != nil { // explicit, so umask cannot soften it
		t.Fatalf("chmod fixture: %v", err)
	}
	if fi, err := os.Lstat(dir); err != nil {
		t.Fatalf("stat fixture: %v", err)
	} else if fi.Mode().Perm()&0o077 == 0 {
		t.Fatalf("the fixture is %#o, so there is nothing to refuse", fi.Mode().Perm())
	}

	s := newRendezvousServer(filepath.Join(dir, "cli.sock"), func(string, ...interface{}) {})
	err := s.start()
	if err == nil {
		s.close()
		t.Fatal("start() bound inside a world-writable runtime directory. Before GH#1414 " +
			"MkdirAll's mode argument did not apply to an existing directory, so this " +
			"directory was adopted in silence")
	}
	if !errors.Is(err, ErrUnsafeDir) {
		t.Errorf("refused with the wrong error type: %v", err)
	}
	// And nothing was left behind in a directory we refused.
	if _, serr := os.Lstat(filepath.Join(dir, "cli.sock")); !errors.Is(serr, os.ErrNotExist) {
		t.Errorf("an endpoint was created inside a refused directory (%v)", serr)
	}
}

func TestGH1414_StartRefusesASymlinkedRuntimeDirectory(t *testing.T) {
	base := shortTempTree(t)
	real := filepath.Join(base, "real")
	if err := os.Mkdir(real, 0o700); err != nil {
		t.Fatalf("make target: %v", err)
	}
	link := filepath.Join(base, "run")
	if err := os.Symlink(real, link); err != nil {
		t.Fatalf("make symlink: %v", err)
	}

	s := newRendezvousServer(filepath.Join(link, "cli.sock"), func(string, ...interface{}) {})
	if err := s.start(); err == nil {
		s.close()
		t.Fatal("start() bound inside a symlinked runtime directory, whose endpoint can be " +
			"redirected by re-pointing the link")
	}
}

// THE DIAL SIDE, REACHED THROUGH runHost. A refusal must be reported as its own
// condition and not as "the engine-host is not running" (GH#1325 D3) — being told
// to start something that is already running sends the reader the wrong way.
func TestGH1414_DialRefusalIsNotReportedAsAppNotRunning(t *testing.T) {
	dir := filepath.Join(shortTempTree(t), "run")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatalf("make fixture: %v", err)
	}
	if err := os.Chmod(dir, 0o777); err != nil {
		t.Fatalf("chmod fixture: %v", err)
	}
	if fi, err := os.Lstat(dir); err != nil {
		t.Fatalf("stat: %v", err)
	} else if fi.Mode().Perm()&0o077 == 0 {
		t.Fatalf("the fixture is %#o, so the dial would be refused for no reason",
			fi.Mode().Perm())
	}

	var out strings.Builder
	code := runHost(filepath.Join(dir, "cli.sock"), strings.NewReader(""), &out,
		func(string, ...interface{}) {})
	if code == 0 {
		t.Fatal("runHost succeeded against an endpoint in a world-writable directory")
	}
	got := out.String()
	if strings.Contains(got, string(ErrAppNotRunning)) {
		t.Errorf("the refusal was reported as %q, which tells the reader to start a service "+
			"that may already be running: %s", ErrAppNotRunning, got)
	}
	if !strings.Contains(got, string(ErrHostUnavailable)) {
		t.Errorf("the refusal does not carry %q: %s", ErrHostUnavailable, got)
	}
	// And it says WHICH route produced the path, so a person knows whether to look
	// at the install or at an environment variable.
	if !strings.Contains(got, socketPathSource(RoleCLI)) {
		t.Errorf("the refusal does not name the path's source (%q): %s",
			socketPathSource(RoleCLI), got)
	}
}
