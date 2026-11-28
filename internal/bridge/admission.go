package bridge

import "errors"

// Socket admission (GH#1414, from the CTO's GH#1325 review).
//
// THE TRUST BOUNDARY WAS ASSERTED IN COMMENTS AND VERIFIED NOWHERE. `paths.go`
// said "the perms ARE part of the trust boundary: the runtime dir is 0700 and the
// socket 0600", and nothing established either. `MkdirAll(dir, 0o700)` does not
// apply its mode to an existing directory, so a foreign pre-created directory was
// adopted silently; the socket chmod logged its failure and carried on; and the
// dial side checked nothing at all.
//
// ── WHY A PROTECTED NAMESPACE AND NOT STAT-AND-REFUSE ───────────────────────
//
// From the review: "the proposed stat-and-refuse checks are necessary but
// insufficient if the directory can still be replaced through an attacker-writable
// ancestor. Checking a leaf once, or statting a socket immediately before
// `connect`, is not an atomic identity guarantee."
//
// So the ancestors are validated before the leaf is trusted, and the leaf is
// reached through an `os.Root` (Go 1.24+) so a symlink planted inside it cannot
// walk out. A foreign leaf FAILS CLOSED and is never chmodded into compliance:
// "never 'repair' an attacker-owned directory by chmodding it."
//
// ── WHAT THIS CANNOT DO, STATED SO NOBODY INHERITS A STRONGER GUARANTEE ─────
//
// `net.Listen("unix", path)` takes a PATH, not a directory fd, and there is no
// `bindat` in Go or in either kernel. The usual workaround is `fchdir` to a
// validated dirfd and bind a relative name.
//
// THAT WORKAROUND IS REFUSED HERE, for the same reason the review warns against
// an early umask in this process: `fchdir` changes PROCESS-GLOBAL working
// directory, and cli's process is concurrent. It is the umask hazard with a wider
// blast radius — every concurrent relative path in the process would move under
// it.
//
// So the bind is validated-then-bound, and the mitigation is DETECTION: after
// `net.Listen`, the listener's own fd is `Fstat`ed and its dev+ino compared
// against an `Lstat` of the path (`verifyBoundSocket`). A substitution between
// validation and bind makes those disagree and the start fails.
//
//	**A detectable window, not an eliminated one.**
//
// ── AND WHAT MODE BITS CANNOT ANSWER ────────────────────────────────────────
//
// The review asks to "account for platform ACLs where mode bits alone do not
// establish effective access". Ownership and mode are validated here; an ACL
// granting another UID write access to a user-owned 0700 directory is NOT
// detected.
//
// DECLARED RATHER THAN SILENT, with its prerequisite named: it needs
// `acl_get_file(3)` or `getattrlist` with `ATTR_CMN_EXTENDED_SECURITY`, and
// NEITHER Go's stdlib NOR `golang.org/x/sys/unix` exposes them — it would take
// cgo, or parsing `ls -lde` output, and a security gate that parses `ls` is a
// worse instrument than a declared gap. It also cannot be closed by probing,
// because "can another UID write here" is only answerable AS another UID.
//
// Measured on the development machine when this landed: the darwin chain carries
// `group:everyone deny delete` on the home directory — a DENY ACE, which
// restricts and grants nothing, so the live configuration is sound.

// ErrUnsafeDir says a directory on the rendezvous path cannot be trusted. Callers
// surface it as its own diagnostic rather than as "the engine-host is not running"
// (GH#1325 D3): a refusal to admit is not an absence of a service.
var ErrUnsafeDir = errors.New("rendezvous directory is not safe to use")

// ErrUnsafeSocket says the endpoint itself is not a socket this role may use.
var ErrUnsafeSocket = errors.New("rendezvous socket is not safe to use")
