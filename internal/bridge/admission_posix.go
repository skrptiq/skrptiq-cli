//go:build !windows

// POSIX half of socket admission (GH#1414). The policy is filesystem ownership
// and mode on a unix-domain socket, so it is built only where those exist; the
// windows file refuses instead of pretending to check. Rendezvous on Windows is
// a named pipe and is already unimplemented (`start()` returns early), so no
// behaviour is removed by that refusal — but `syscall.Stat_t` does not exist
// there, and without this split the published `skrptiq-windows-amd64.exe` would
// not compile at all.

package bridge

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// statOf returns the syscall-level stat for a path WITHOUT following a final
// symlink. The portable os.FileInfo carries no uid, so the platform struct is
// unavoidable; it is read through os.Lstat rather than syscall.Lstat so the error
// values stay comparable with the rest of the package.
func statOf(path string) (os.FileInfo, *syscall.Stat_t, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, nil, err
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		// Not reachable on the POSIX platforms this file builds for. Refused rather
		// than skipped: a validator that silently passes when it cannot see is the
		// failure this whole change is about.
		return nil, nil, fmt.Errorf("%w: %s: cannot read ownership on this platform",
			ErrUnsafeDir, path)
	}
	return fi, st, nil
}

// ancestorsOf returns path's ancestors from the root downwards, excluding path.
// The leaf is validated separately and under different rules — it may legitimately
// not exist yet.
func ancestorsOf(path string) []string {
	var out []string
	for dir := filepath.Dir(path); ; dir = filepath.Dir(dir) {
		out = append([]string{dir}, out...)
		if dir == "/" || dir == filepath.Dir(dir) {
			break
		}
	}
	return out
}

// checkAncestor validates one component of the chain above the leaf.
//
// THE QUESTION IS "COULD ANOTHER UID REPLACE THIS COMPONENT", not "is this
// component tidy". A component passes when it is a real directory, is owned by
// this euid or by root, and grants write to nobody else — with ONE exception that
// is load-bearing rather than a loophole.
func checkAncestor(path string, euid uint32) error {
	fi, st, err := statOf(path)
	if err != nil {
		return fmt.Errorf("%w: %s: %v", ErrUnsafeDir, path, err)
	}
	if !fi.IsDir() {
		return fmt.Errorf("%w: %s is not a directory", ErrUnsafeDir, path)
	}
	if st.Uid != euid && st.Uid != 0 {
		return fmt.Errorf("%w: %s is owned by uid %d, which is neither this user (%d) nor root",
			ErrUnsafeDir, path, st.Uid, euid)
	}

	perm := fi.Mode().Perm()
	if perm&0o022 == 0 {
		return nil // writable by nobody else: sound regardless of ownership
	}
	// STICKY IS THE EXCEPTION, AND IT IS WHY /tmp IS NOT AUTOMATICALLY FATAL.
	// A sticky world-writable directory (`/tmp`, mode 1777) stops another UID
	// REMOVING OR RENAMING an entry it does not own, which is the replacement this
	// function exists to prevent. It does NOT stop another UID CREATING a name
	// nobody has taken yet — so a sticky ancestor is accepted here and the LEAF
	// check is what refuses a foreign pre-created child. Treating sticky /tmp as a
	// fatal ancestor would refuse the Linux fallback outright, including for the
	// user whose own leaf is perfectly sound.
	if fi.Mode()&os.ModeSticky != 0 && st.Uid == 0 {
		return nil
	}
	return fmt.Errorf("%w: %s is mode %#o — writable by another user and not root-owned "+
		"and sticky, so it or its children can be replaced", ErrUnsafeDir, path, perm)
}

// checkSymlinkAncestor judges a symlink that appears ABOVE the rendezvous
// directory. It is judged rather than refused, and the first version of this file
// refused it — which refused the legitimate path on the platform cli primarily
// ships to.
//
// MEASURED: on macOS `/var` is `lrwxr-xr-x root wheel -> private/var`, so
// `os.TempDir()` (`/var/folders/…`) has a symlink ancestor on every Mac. Refusing
// all symlink ancestors made `start()` fail for every existing rendezvous test
// with "/var is a symlink" — a policy that refuses the healthy normal case, which
// is the failure mode the acceptance control exists to catch. It was caught by
// that control on the first run.
//
// WHAT ACTUALLY MATTERS is whether another UID can RE-POINT the link, and that is
// a question about the link's owner and its parent directory — not about the link
// existing. The parent is the preceding component and has already been judged by
// checkAncestor, so what remains is ownership: a link owned by this user or by
// root cannot be replaced by a third party who cannot write its parent.
//
// The link's TARGET is validated too: secureDir resolves the chain and judges every
// component of the resolved path, so a root-owned link pointing into a
// world-writable directory is still refused — by the component that is actually
// unsafe, and named as such.
func checkSymlinkAncestor(path string, euid uint32) error {
	_, st, err := statOf(path)
	if err != nil {
		return fmt.Errorf("%w: %s: %v", ErrUnsafeDir, path, err)
	}
	if st.Uid != euid && st.Uid != 0 {
		return fmt.Errorf("%w: %s is a symlink owned by uid %d, which is neither this user "+
			"(%d) nor root, so where it points can be changed by someone else",
			ErrUnsafeDir, path, st.Uid, euid)
	}
	return nil
}

// deepestExisting returns the longest prefix of dir that exists, and the
// components below it that do not.
//
// IT EXISTS BECAUSE THE OLD CODE USED MkdirAll AND A FRESH INSTALL HAS NO
// ANCESTORS. `~/Library/Application Support/skrptiq/run` may be missing both
// `skrptiq` and `run`, so a policy that required the parent to exist would refuse
// every first run. The missing tail is created one component at a time inside a
// validated os.Root instead, which is what MkdirAll could not do.
func deepestExisting(dir string) (string, []string, error) {
	var missing []string
	for cur := dir; ; {
		if _, err := os.Lstat(cur); err == nil {
			return cur, missing, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", nil, fmt.Errorf("%w: %s: %v", ErrUnsafeDir, cur, err)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", nil, fmt.Errorf("%w: %s has no existing ancestor", ErrUnsafeDir, dir)
		}
		missing = append([]string{filepath.Base(cur)}, missing...)
		cur = parent
	}
}

// secureDir establishes the rendezvous directory as a protected namespace and
// returns an os.Root anchored on it.
//
// ORDER IS THE POINT: every ancestor is validated BEFORE the leaf is created or
// trusted, and the leaf is judged before any cleanup happens inside it. The
// review's wording is "establish the protected directory BEFORE unlink/bind/chmod"
// and "foreign/replaceable directories are refused BEFORE cleanup".
//
// The caller closes the returned Root.
func secureDir(dir string) (*os.Root, error) {
	if !filepath.IsAbs(dir) {
		return nil, fmt.Errorf("%w: %q is not an absolute path, so which directory it names "+
			"depends on this process's working directory", ErrUnsafeDir, dir)
	}
	dir = filepath.Clean(dir)
	euid := uint32(os.Geteuid())

	// 1. Any symlink in the ORIGINAL chain must be one nobody else can re-point.
	// Judged before resolution, because resolution discards the links themselves.
	for _, a := range append(ancestorsOf(dir), dir) {
		fi, err := os.Lstat(a)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue // a component yet to be created; step 3 owns it
			}
			return nil, fmt.Errorf("%w: %s: %v", ErrUnsafeDir, a, err)
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			continue
		}
		// THE LEAF ITSELF MAY NOT BE A SYMLINK, however well-owned. The endpoint
		// inside it would then be reachable by redirecting the link, and nothing
		// legitimate needs the rendezvous directory to be one.
		if a == dir {
			return nil, fmt.Errorf("%w: %s is a symlink; the rendezvous directory must be a "+
				"real directory so the endpoint inside it cannot be redirected",
				ErrUnsafeDir, dir)
		}
		if err := checkSymlinkAncestor(a, euid); err != nil {
			return nil, err
		}
	}

	// 2. Validate the RESOLVED chain of the deepest existing prefix. Resolving
	// first means every component judged below is a real directory, and a
	// well-owned link into an unsafe directory is refused by the unsafe component
	// rather than passing because the link looked fine.
	base, missing, err := deepestExisting(dir)
	if err != nil {
		return nil, err
	}
	realBase, err := filepath.EvalSymlinks(base)
	if err != nil {
		return nil, fmt.Errorf("%w: %s could not be resolved: %v", ErrUnsafeDir, base, err)
	}
	for _, a := range append(ancestorsOf(realBase), realBase) {
		if err := checkAncestor(a, euid); err != nil {
			return nil, err
		}
	}

	root, err := os.OpenRoot(realBase)
	if err != nil {
		return nil, fmt.Errorf("%w: could not anchor %s: %v", ErrUnsafeDir, realBase, err)
	}

	// 3. Create the missing tail INSIDE the anchored root, one component at a time,
	// and validate each as it appears. os.Root is what makes this safe: a symlink
	// planted in the tail cannot be followed out of the root.
	rel := ""
	for _, comp := range missing {
		rel = filepath.Join(rel, comp)
		if err := root.Mkdir(rel, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
			root.Close()
			return nil, fmt.Errorf("%w: could not create %s inside %s: %v",
				ErrUnsafeDir, rel, realBase, err)
		}
		if err := validateLeafIn(root, rel, euid); err != nil {
			root.Close()
			return nil, err
		}
	}

	// 4. THE LEAF, AND THE EXACT DEFECT GH#1414 NAMES. `MkdirAll(dir, 0o700)`'s
	// mode argument does NOT apply to a directory that already exists, so the old
	// code adopted whatever was already there — including a directory another UID
	// had pre-created. Validated, never repaired.
	if len(missing) == 0 {
		if err := validateLeafIn(root, ".", euid); err != nil {
			root.Close()
			return nil, err
		}
	}

	// The returned root is anchored on the rendezvous directory itself, so callers
	// address entries by bare name.
	if rel != "" {
		leaf, err := root.OpenRoot(rel)
		root.Close()
		if err != nil {
			return nil, fmt.Errorf("%w: could not anchor %s: %v", ErrUnsafeDir, dir, err)
		}
		return leaf, nil
	}
	return root, nil
}

// validateLeafIn judges a directory reached through an anchored root: it is a real
// directory, this user owns it, and it grants nothing to group or other.
//
// NEVER REPAIRED, and the message says why. "Never 'repair' an attacker-owned
// directory by chmodding it" — and a directory that HAS been open to other users
// may already hold their entries, so making the mode right afterwards does not
// make the contents trustworthy.
func validateLeafIn(root *os.Root, name string, euid uint32) error {
	fi, err := root.Lstat(name)
	if err != nil {
		return fmt.Errorf("%w: %s in %s: %v", ErrUnsafeDir, name, root.Name(), err)
	}
	full := filepath.Join(root.Name(), name)
	if fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: %s is a symlink; the rendezvous directory must be a real "+
			"directory so the endpoint inside it cannot be redirected", ErrUnsafeDir, full)
	}
	if !fi.IsDir() {
		return fmt.Errorf("%w: %s exists and is not a directory", ErrUnsafeDir, full)
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("%w: %s: cannot read ownership on this platform", ErrUnsafeDir, full)
	}
	if st.Uid != euid {
		return fmt.Errorf("%w: %s is owned by uid %d, not by this user (%d). It is NOT "+
			"adjusted into compliance — a directory another user created is not made safe "+
			"by changing its mode", ErrUnsafeDir, full, st.Uid, euid)
	}
	if perm := fi.Mode().Perm(); perm&0o077 != 0 {
		return fmt.Errorf("%w: %s is mode %#o and must grant nothing to group or other. It "+
			"is refused rather than chmodded, because a directory that has been open to "+
			"other users may already contain their entries", ErrUnsafeDir, full, perm)
	}
	return nil
}

// checkSocketOwnership establishes that an endpoint is a socket THIS USER owns.
//
// SEPARATE FROM THE MODE CHECK, AND THE SPLIT IS LOAD-BEARING. Two different
// questions get confused if one predicate answers both:
//
//   - "May this process DELETE it?" — licensed by TYPE and OWNERSHIP. A socket we
//     own is ours to clean up however loose its mode is.
//   - "May this process USE it?" — additionally requires the mode, because write
//     access to a socket is what lets a peer connect.
//
// The first version of this file used one predicate for both, and it REFUSED TO
// CLEAR A NORMAL CRASH LEFTOVER: `net.Listen` creates a socket at 0777&~umask —
// 0755 on a default macOS — so a process that died between bind and chmod leaves
// an endpoint that fails the mode check. The CLI could then never rebind without
// someone deleting the file by hand. Caught by the stale-socket test, which is the
// recovery path a user actually depends on.
func checkSocketOwnership(path string) error {
	fi, st, err := statOf(path)
	if err != nil {
		return err // ErrNotExist is a legitimate answer; the caller classifies it
	}
	if fi.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("%w: %s is %s, not a socket", ErrUnsafeSocket, path, fi.Mode().Type())
	}
	if euid := uint32(os.Geteuid()); st.Uid != euid {
		return fmt.Errorf("%w: %s is owned by uid %d, not by this user (%d)",
			ErrUnsafeSocket, path, st.Uid, euid)
	}
	return nil
}

// checkSocketFile is checkSocketOwnership plus the mode, for an endpoint this
// process is about to USE — bind-verify on the listen side, pre-dial on the host
// side.
//
// The write bit is the whole reason the mode matters, and it was measured rather
// than assumed — on a listening pathname UDS, `0400 → EACCES`, `0200 → CONNECT
// OK`, `0555 → EACCES`, `0600 → CONNECT OK`, because XNU's `unp_connect` calls
// `vnode_authorize(..., KAUTH_VNODE_WRITE_DATA, ...)`. Linux `unix(7)` states the
// same requirement. So a group- or other-writable socket is connectable by that
// group or by anyone, which is the invariant and not a mode cosmetic.
func checkSocketFile(path string) error {
	if err := checkSocketOwnership(path); err != nil {
		return err
	}
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if perm := fi.Mode().Perm(); perm&0o077 != 0 {
		return fmt.Errorf("%w: %s is mode %#o; another user with write access to a socket "+
			"can connect to it", ErrUnsafeSocket, path, perm)
	}
	return nil
}

// verifyBoundSocket establishes that the endpoint we bound is the one inside the
// directory we validated.
//
// ── THE DESIGN THIS REPLACES, AND WHY IT COULD NOT WORK ─────────────────────
//
// The plan for GH#1414 said: `Fstat` the listener's own descriptor and compare
// dev+ino against an `Lstat` of the path. THAT IS NOT AVAILABLE.
// `(*net.UnixListener).File()` yields a descriptor for the SOCKET, not for the
// pathname, so `fstat` reports the socket's identity in the kernel rather than the
// directory entry's. Measured on darwin when this was built: the listener side
// reported `dev/ino -1/2522339` while the path reported `16777230/307406183` —
// not a mismatch, an INCOMPARABLE pair, and `dev = -1` is the tell.
//
// It failed the healthy-normal control on the first run, which is the only reason
// it is not in the shipped code as a check that always fires.
//
// ── WHAT IS DONE INSTEAD, AND IT IS STRONGER ────────────────────────────────
//
// The comparison that IS available is between the path and the directory
// DESCRIPTOR we validated. `secureDir` returns an `os.Root` holding an open fd on
// the real rendezvous directory, so looking the endpoint up THROUGH that root
// asks a different and better question than re-walking the path: not "is the thing
// at this path acceptable now" but "is the thing in the directory I validated the
// same file as the thing at the path I bound".
//
// A directory swapped after validation makes those two disagree, because the root
// fd still refers to the original directory while the path now resolves to the
// replacement.
//
//	**A detectable window, not an eliminated one.** The bind itself still took a
//	path, because there is no `bindat` and the `fchdir` workaround is refused in
//	this concurrent process. Nobody reading this should inherit a stronger
//	guarantee than that sentence.
func verifyBoundSocket(root *os.Root, path string) error {
	// The path side: type, owner and mode, which is where the write bit matters.
	if err := checkSocketFile(path); err != nil {
		return err
	}
	_, byPath, err := statOf(path)
	if err != nil {
		return fmt.Errorf("%w: %s: %v", ErrUnsafeSocket, path, err)
	}

	// The directory-descriptor side. os.Root refuses to follow a symlink out of
	// itself, so this cannot be redirected by planting one.
	fi, err := root.Lstat(filepath.Base(path))
	if err != nil {
		return fmt.Errorf("%w: %s is not present in the directory this process validated "+
			"(%s): %v. The path was replaced between the bind and this check, so nothing "+
			"is reported as listening", ErrUnsafeSocket, filepath.Base(path), root.Name(), err)
	}
	byRoot, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("%w: cannot read the endpoint's identity on this platform", ErrUnsafeSocket)
	}

	if byRoot.Dev != byPath.Dev || byRoot.Ino != byPath.Ino {
		return fmt.Errorf("%w: %s is not the same file as %s inside the validated directory "+
			"(path dev/ino %d/%d, validated directory %d/%d). The path was replaced between "+
			"the bind and this check, so nothing is reported as listening",
			ErrUnsafeSocket, path, filepath.Base(path),
			byPath.Dev, byPath.Ino, byRoot.Dev, byRoot.Ino)
	}
	return nil
}
