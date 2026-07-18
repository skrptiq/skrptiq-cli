package bridge

import (
	"fmt"
	"os"
)

// The two hidden bridge modes hang off the `skrptiq __bridge --mode <mcp|host>`
// subcommand — undiscoverable (not in --help), invoked only by the engine
// (mcp) or Chrome via the native-messaging wrapper (host). Neither mode inits
// the engine/DB: MCP-mode is a stdio MCP server that owns the rendezvous socket;
// host-mode is a pure relay. Dispatch runs before any engine setup in main.

// Dispatch handles the hidden `__bridge` subcommand. Returns handled=false for
// any other invocation so normal CLI startup proceeds. When handled, the caller
// should os.Exit(code).
func Dispatch(args []string) (handled bool, code int) {
	mode, isBridge := parseBridgeMode(args)
	if !isBridge {
		return false, 0
	}
	switch mode {
	case "mcp":
		return true, runMCPMode()
	case "host":
		return true, runHostStdio(socketPathFor(RoleCLI))
	default:
		fmt.Fprintln(os.Stderr, "usage: skrptiq __bridge --mode <mcp|host>")
		return true, 2
	}
}

// parseBridgeMode reports whether args invoke the hidden `__bridge` subcommand
// and, if so, the `--mode` value (empty when absent/invalid). Chrome appends the
// extension origin after our args, which is ignored.
func parseBridgeMode(args []string) (mode string, isBridge bool) {
	if len(args) == 0 || args[0] != "__bridge" {
		return "", false
	}
	for i := 1; i < len(args); i++ {
		if args[i] == "--mode" && i+1 < len(args) {
			return args[i+1], true
		}
	}
	return "", true
}

// runMCPMode is the engine-spawned stdio MCP server: it owns the CLI rendezvous
// socket and serves the frozen catalogue to the engine. stdout is the JSON-RPC
// transport; all logging goes to stderr.
func runMCPMode() int {
	logf := func(format string, args ...interface{}) {
		_, _ = fmt.Fprintf(os.Stderr, "[bridge-mcp] "+format+"\n", args...)
	}
	socket := socketPathFor(RoleCLI)
	srv := newRendezvousServer(socket, logf)
	if err := srv.start(); err != nil {
		logf("could not bind rendezvous socket: %v", err)
		return 1
	}
	defer srv.close()

	server, err := newMCPServer(srv, os.Stdout, logf)
	if err != nil {
		logf("could not start MCP server: %v", err)
		return 1
	}
	logf("MCP stdio server ready; rendezvous at %s", socket)
	if err := server.serve(os.Stdin); err != nil {
		logf("MCP serve ended with error: %v", err)
		return 1
	}
	return 0
}
