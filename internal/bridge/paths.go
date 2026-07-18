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
