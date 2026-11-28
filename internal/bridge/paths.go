package bridge

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
)

// Rendezvous-socket path convention (mirrors skrptiq-extension companion/src/paths.ts
// and the app's browser-bridge.ts). Each engine-host role owns its OWN socket at a
// per-user path so the app and the CLI never collide and each engine's workflow
// routes to the extension unambiguously (K-055 port-per-host):
//
//	app → <runtimeDir>/app.sock
//	cli → <runtimeDir>/cli.sock   ← this binary
//
// The perms ARE part of the trust boundary: the runtime dir is 0700 and the socket
// 0600 (user-only). No port, no token. SKRPTIQ_BRIDGE_SOCKET overrides outright.
//
// ── THAT BOUNDARY IS NOW ENFORCED, NOT ASSERTED (GH#1414) ───────────────────
//
// The paragraph above was true as a statement of intent and verified nowhere. This
// file SELECTS a path; `secure_dir.go` decides whether the path may be used, and
// both `start()` and the dial side go through it. Selection stays here and policy
// stays there deliberately — a second validator in the caller is how two
// authorities come to disagree.
//
// TWO ROUTES REACH AN UNVALIDATED DIRECTORY, and they are named in refusals via
// socketPathSource so a person can tell which one produced the path:
//
//   - SKRPTIQ_BRIDGE_SOCKET skips selection entirely — it is not checked here for
//     absoluteness, for its parent, or for type.
//   - A nonempty XDG_RUNTIME_DIR is taken verbatim, with no check of ownership,
//     privacy or even absolute form.
//
// ── WHERE THE PRE-CREATABLE-LEAF HAZARD ACTUALLY LIVES ──────────────────────
//
// PRECONDITION, NOT PREVALENCE. Measured on the development machine: on darwin
// `runtimeDir()` returns the Application Support path and the temp fallback is
// NEVER REACHED, and `os.TempDir()` there is `/var/folders/…/T/` — not `/tmp`.
//
// So the hazard is the LINUX-WITHOUT-XDG branch, where `os.TempDir()` is `/tmp`
// and `/tmp/skrptiq-<uid>` can be pre-created by another UID: the sticky bit stops
// another user DELETING an entry it does not own, not CREATING a name nobody has
// taken. `MkdirAll`'s mode never applied to an existing directory, so that
// pre-created directory used to be adopted in silence.
//
// cli SHIPS on Linux — `v0.0.24` publishes `skrptiq-linux-amd64` and
// `skrptiq-linux-arm64` with checksums — so this branch is not defensive code for
// a platform nobody runs. CI is macOS-only, which is tracked as GH#1416 and is NOT
// closed by this change.

// HostRole identifies which engine-host owns a rendezvous socket.
type HostRole string

const (
	// RoleCLI is this binary's role; the app's is "app".
	RoleCLI HostRole = "cli"
	roleApp HostRole = "app"
)

// isHostRole reports whether v is a recognised role.
func isHostRole(v string) bool {
	return v == string(RoleCLI) || v == string(roleApp)
}

// runtimeDir is the per-user directory holding the rendezvous sockets. Kept
// identical to the app/companion so both engine-hosts agree on the location.
func runtimeDir() string {
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "skrptiq", "run")
	}
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		return filepath.Join(xdg, "skrptiq")
	}
	return filepath.Join(os.TempDir(), "skrptiq-"+strconv.Itoa(os.Getuid()))
}

// socketPathSource names the route that produced the current socket path, for
// refusal messages.
//
// A REFUSAL THAT DOES NOT SAY WHERE THE PATH CAME FROM sends the reader to the
// wrong place: "this directory is unsafe" about a path an environment variable
// chose reads as a problem with the install. Two of the four routes are env vars,
// and those are the two with no validation of their own.
func socketPathSource(role HostRole) string {
	if os.Getenv("SKRPTIQ_BRIDGE_SOCKET") != "" {
		return "SKRPTIQ_BRIDGE_SOCKET"
	}
	if runtime.GOOS == "windows" {
		return "named pipe (Windows)"
	}
	if runtime.GOOS == "darwin" {
		return "the per-user Application Support directory"
	}
	if os.Getenv("XDG_RUNTIME_DIR") != "" {
		return "XDG_RUNTIME_DIR"
	}
	return "the temporary-directory fallback (no XDG_RUNTIME_DIR set)"
}

// socketPathFor returns the rendezvous endpoint for a role. On Windows this is a
// named pipe (deferred, Chrome-first); on POSIX a UDS filesystem path.
// SKRPTIQ_BRIDGE_SOCKET wins if set (tests + explicit packaging).
func socketPathFor(role HostRole) string {
	if override := os.Getenv("SKRPTIQ_BRIDGE_SOCKET"); override != "" {
		return override
	}
	if runtime.GOOS == "windows" {
		return `\\.\pipe\skrptiq-` + string(role)
	}
	return filepath.Join(runtimeDir(), string(role)+".sock")
}
