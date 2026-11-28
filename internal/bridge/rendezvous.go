package bridge

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// rendezvousServer is the per-user socket the Chrome-spawned host-mode process
// dials into — the native-messaging replacement for the v1 loopback WebSocket.
// The MCP-mode bridge owns it; the host relays native-messaging frames ⇄ this
// socket. The MCP layer calls invoke() and always gets a normalised outcome (it
// never returns an error for an expected failure state).
//
// Trust: reachability of a 0600 socket by this user + the OS-enforced
// allowed_origins on the manifest are the entire boundary — no token, no auth
// handshake (mirrors skrptiq-extension companion/src/rendezvous.ts).

var serverInfo = peerInfo{Name: "skrptiq-cli-bridge", Version: "0.0.1"}

// invokeOutcome is what invoke() returns; the MCP layer maps it to a ToolResult.
type invokeOutcome struct {
	ok      bool
	content interface{}
	err     *BridgeError
}

type pendingInvoke struct {
	ch chan invokeOutcome
}

type rendezvousServer struct {
	socketPath    string
	invokeTimeout time.Duration
	logf          func(string, ...interface{})

	mu        sync.Mutex
	ln        net.Listener
	active    net.Conn
	announced map[string]struct{}
	pending   map[string]*pendingInvoke
	closed    bool
}

// staleProbeTimeout bounds the liveness probe in clearStaleSocket. It is short
// because the peer is on the same machine and either accepts immediately or is
// not there; a long timeout would turn a crashed-host restart into a stall.
const staleProbeTimeout = 250 * time.Millisecond

func newRendezvousServer(socketPath string, logf func(string, ...interface{})) *rendezvousServer {
	if logf == nil {
		logf = func(string, ...interface{}) {}
	}
	return &rendezvousServer{
		socketPath:    socketPath,
		invokeTimeout: time.Duration(DefaultInvokeTimeoutMS) * time.Millisecond,
		logf:          logf,
		pending:       map[string]*pendingInvoke{},
	}
}

// start establishes the protected directory, admits or refuses the existing
// endpoint, binds, and hardens — in that order, which is the order GH#1414 is
// about (see secure_dir.go for the namespace policy and what it cannot promise).
//
// EVERY FAILURE PATH LEAVES NO LISTENER. The old version reported
// "listening on … (0600, per-user)" after a chmod whose failure it only logged,
// so a readiness claim could be false. Readiness is now claimed only once the
// bound socket has been verified to be the one we created, owned by us, at 0600.
func (s *rendezvousServer) start() error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("named-pipe rendezvous not yet supported (Chrome-first, §9.6)")
	}

	// 1. THE NAMESPACE, BEFORE ANY CLEANUP. A foreign or replaceable directory is
	// refused here, which is before anything inside it is removed — "foreign or
	// replaceable directories are refused BEFORE cleanup".
	root, err := secureDir(filepath.Dir(s.socketPath))
	if err != nil {
		return err
	}
	// Closed at the end of start: the descriptor is what verifyBoundSocket compares
	// the bound path against, so it must outlive the bind.
	defer root.Close()

	// 2. THE EXISTING ENTRY IS CLASSIFIED, NOT DELETED. The old code did
	// `Stat` then an unconditional `Remove`, which "is not proof that a listener is
	// dead" — it was equally happy to delete a live peer's socket or a file it had
	// never seen before.
	if err := s.clearStaleSocket(); err != nil {
		return err
	}

	// 3. BIND. This is the step that cannot be anchored to the validated directory
	// fd: net.Listen takes a path and there is no bindat. See secure_dir.go for why
	// the fchdir workaround is refused in this process.
	ln, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.socketPath, err)
	}

	// 4. HARDEN, THEN VERIFY, AND FAIL CLOSED ON EITHER. chmod failure is fatal:
	// an unhardened socket in a sound directory is still a socket this user's other
	// processes can reach, and reporting it as ready would be the readiness lie.
	if err := os.Chmod(s.socketPath, 0o600); err != nil {
		s.abandonListener(ln)
		return fmt.Errorf("%w: could not set %s to 0600: %v. Nothing is listening — a socket "+
			"whose permissions could not be set is not reported as ready",
			ErrUnsafeSocket, s.socketPath, err)
	}
	if err := verifyBoundSocket(root, s.socketPath); err != nil {
		s.abandonListener(ln)
		return err
	}

	s.mu.Lock()
	s.ln = ln
	s.mu.Unlock()
	s.logf("[rendezvous] listening on %s (verified: socket, uid %d, 0600)",
		s.socketPath, os.Geteuid())
	go s.acceptLoop(ln)
	return nil
}

// abandonListener closes a listener that failed hardening and removes the socket
// it created. Used on every failure path after bind, so no half-hardened endpoint
// is left behind for the next process to find and "adopt".
func (s *rendezvousServer) abandonListener(ln net.Listener) {
	_ = ln.Close()
	// ln.Close() already unlinks for a Go-created UnixListener; this is the
	// belt-and-braces removal and its failure is logged rather than returned,
	// because the caller is already returning the reason that matters.
	if err := os.Remove(s.socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		s.logf("[rendezvous] could not remove %s after a failed start: %v", s.socketPath, err)
	}
}

// clearStaleSocket decides whether the existing endpoint may be removed, and
// REFUSES rather than guessing.
//
// THE THREE ANSWERS ARE GENUINELY DIFFERENT and the old single `Remove` collapsed
// them:
//
//   - ABSENT — nothing to do, and ENOENT is the only error that means this.
//   - PRESENT AND LIVE — another listener holds it. Refused: superseding a running
//     peer by deleting its socket is not cleanup, and the operator needs to know
//     which one is running.
//   - PRESENT AND DEAD — a crash leftover. Removed, and a removal FAILURE is fatal
//     rather than discarded, because binding after a failed unlink either fails
//     confusingly or succeeds against something unexpected.
//
// A wrong-type entry is refused by name: a regular file or a directory at the
// endpoint path is not something this code may delete.
func (s *rendezvousServer) clearStaleSocket() error {
	// OWNERSHIP, NOT MODE, LICENSES REMOVAL. `net.Listen` creates a socket at
	// 0777&~umask (0755 on a default macOS), so a process that died between bind
	// and chmod leaves a perfectly ordinary crash leftover with a loose mode.
	// Checking the mode here would refuse to clear it and the CLI could never
	// rebind without someone deleting the file by hand.
	err := checkSocketOwnership(s.socketPath)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil
	case err != nil:
		// Wrong type or wrong owner. Named, and NOT removed — this process does not
		// delete entries it cannot account for.
		return fmt.Errorf("%w. It was NOT removed: an entry this role did not create is "+
			"refused rather than cleared", err)
	}

	// It is a socket we own. Liveness is ESTABLISHED by connecting, not inferred
	// from the file existing.
	conn, derr := net.DialTimeout("unix", s.socketPath, staleProbeTimeout)
	if derr == nil {
		_ = conn.Close()
		return fmt.Errorf("%w: %s already has a live listener. Another engine-host for this "+
			"role is running; this one will not take the endpoint from it",
			ErrUnsafeSocket, s.socketPath)
	}

	if rerr := os.Remove(s.socketPath); rerr != nil && !errors.Is(rerr, os.ErrNotExist) {
		return fmt.Errorf("%w: %s looks stale (%v) but could not be removed: %v",
			ErrUnsafeSocket, s.socketPath, derr, rerr)
	}
	s.logf("[rendezvous] removed stale socket %s (probe: %v)", s.socketPath, derr)
	return nil
}

func (s *rendezvousServer) acceptLoop(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			s.mu.Lock()
			done := s.closed
			s.mu.Unlock()
			if !done {
				s.logf("[rendezvous] accept error: %v", err)
			}
			return
		}
		go s.handleConn(conn)
	}
}

func (s *rendezvousServer) handleConn(conn net.Conn) {
	announced := false
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		switch messageType(line) {
		case "announce":
			if announced {
				continue
			}
			var msg AnnounceMessage
			if json.Unmarshal(line, &msg) != nil {
				continue
			}
			announced = s.handleAnnounce(conn, msg)
			if !announced {
				return // version mismatch already closed the conn
			}
		case "result":
			if !announced {
				continue
			}
			var msg ResultMessage
			if json.Unmarshal(line, &msg) != nil {
				continue
			}
			s.handleResult(conn, msg)
		}
	}
	// Connection ended — drop it if it's still the active one.
	s.mu.Lock()
	if s.active == conn {
		s.active = nil
		s.announced = nil
	}
	s.mu.Unlock()
	_ = conn.Close()
}

func (s *rendezvousServer) handleAnnounce(conn net.Conn, msg AnnounceMessage) bool {
	if msg.V != ProtocolVersion {
		s.sendError(conn, ErrUnsupportedProtocolVersion,
			fmt.Sprintf("Bridge speaks protocol v%d, extension sent v%d.", ProtocolVersion, msg.V))
		_ = conn.Close()
		return false
	}
	tools := make(map[string]struct{}, len(msg.Tools))
	for _, t := range msg.Tools {
		tools[t.Name] = struct{}{}
	}
	s.mu.Lock()
	// NEWEST-WINS, AND IT IS NOT AN AUTHENTICATION DECISION (GH#1414).
	//
	// A fresh host connection supersedes the old one. With admission confined to
	// this UID by the directory and endpoint policy in secure_dir.go, every peer
	// that reaches this point is one of this user's own processes, so the contest is
	// between a stale Chrome host and a fresh one — which is what newest-wins is
	// for, and the CTO ruled a token or handshake explicitly unnecessary for a
	// boundary that trusts the user's UID.
	//
	// WHAT CHANGED IS THAT IT IS NO LONGER SILENT. Supersession used to close the
	// previous peer with no record, so an unexpected displacement left no trace to
	// diagnose it from. Nothing else here is a security control and this comment
	// exists so the next reader does not mistake it for one.
	old := s.active
	s.active = conn
	s.announced = tools
	s.mu.Unlock()
	if old != nil && old != conn {
		s.logf("[rendezvous] %s@%s superseded the attached host %s — the previous connection "+
			"is being closed", msg.Client.Name, msg.Client.Version, old.RemoteAddr())
		_ = old.Close()
	}
	s.send(conn, ReadyMessage{Type: "ready", V: ProtocolVersion, Server: serverInfo})
	s.logf("[rendezvous] extension attached: %s@%s; tools=%d", msg.Client.Name, msg.Client.Version, len(tools))
	return true
}

func (s *rendezvousServer) handleResult(conn net.Conn, msg ResultMessage) {
	s.mu.Lock()
	if s.active != conn { // ignore frames from a superseded socket
		s.mu.Unlock()
		return
	}
	p := s.pending[msg.ID]
	if p == nil { // already timed out or unknown id
		s.mu.Unlock()
		return
	}
	delete(s.pending, msg.ID)
	s.mu.Unlock()

	if msg.OK {
		p.ch <- invokeOutcome{ok: true, content: msg.Content}
		return
	}
	e := msg.Error
	if e == nil {
		e = &BridgeError{Code: ErrInternalError, Message: "Extension reported failure without an error."}
	}
	p.ch <- invokeOutcome{ok: false, err: e}
}

// invoke calls a tool on the connected extension, returning a normalised outcome.
// It never returns an error for an expected failure state (no extension, tool not
// announced, timeout) — those are encoded in the outcome so the MCP layer emits a
// loud isError result.
func (s *rendezvousServer) invoke(tool string, args map[string]interface{}) invokeOutcome {
	s.mu.Lock()
	conn := s.active
	_, announced := s.announced[tool]
	if conn == nil {
		s.mu.Unlock()
		return invokeOutcome{ok: false, err: &BridgeError{
			Code: ErrExtensionDisconnected, Message: "No browser extension is connected to the bridge."}}
	}
	if !announced {
		s.mu.Unlock()
		return invokeOutcome{ok: false, err: &BridgeError{
			Code:    ErrCapabilityNotGranted,
			Message: fmt.Sprintf("The connected extension did not announce the tool %q.", tool)}}
	}
	id := randomID()
	p := &pendingInvoke{ch: make(chan invokeOutcome, 1)}
	s.pending[id] = p
	s.mu.Unlock()

	if args == nil {
		args = map[string]interface{}{}
	}
	s.send(conn, InvokeMessage{Type: "invoke", ID: id, Tool: tool, Args: args})

	timer := time.NewTimer(s.invokeTimeout)
	defer timer.Stop()
	select {
	case out := <-p.ch:
		return out
	case <-timer.C:
		s.mu.Lock()
		delete(s.pending, id)
		s.mu.Unlock()
		return invokeOutcome{ok: false, err: &BridgeError{
			Code:    ErrToolTimeout,
			Message: fmt.Sprintf("Tool %q did not return within %dms.", tool, DefaultInvokeTimeoutMS)}}
	}
}

func (s *rendezvousServer) close() {
	s.mu.Lock()
	s.closed = true
	ln := s.ln
	active := s.active
	s.active = nil
	s.pending = map[string]*pendingInvoke{}
	s.mu.Unlock()
	if active != nil {
		_ = active.Close()
	}
	if ln != nil {
		_ = ln.Close()
	}
	if runtime.GOOS != "windows" {
		// SAME DISCRIMINATION AS clearStaleSocket, for the same reason, at lower
		// stakes: this is our own socket on the way out. A removal failure is logged
		// rather than swallowed, because the next start will find the leftover and
		// refuse it, and the operator should be able to see which process left it.
		if err := os.Remove(s.socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.logf("[rendezvous] could not remove %s on close: %v", s.socketPath, err)
		}
	}
}

func (s *rendezvousServer) sendError(conn net.Conn, code BridgeErrorCode, message string) {
	s.send(conn, ErrorMessage{Type: "error", Code: code, Message: message})
}

// send writes one message as an NDJSON line. Writes are serialised on the server
// mutex so concurrent invoke/ready/error frames never interleave on the wire.
func (s *rendezvousServer) send(conn net.Conn, msg interface{}) {
	line, err := json.Marshal(msg)
	if err != nil {
		s.logf("[rendezvous] marshal outbound: %v", err)
		return
	}
	line = append(line, '\n')
	s.mu.Lock()
	_, err = conn.Write(line)
	s.mu.Unlock()
	if err != nil {
		s.logf("[rendezvous] write: %v", err)
	}
}

// randomID returns a 128-bit hex correlation id for invoke↔result matching.
func randomID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
