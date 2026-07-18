package bridge

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
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

// start binds the rendezvous socket (0700 dir, 0600 socket) and begins accepting.
func (s *rendezvousServer) start() error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("named-pipe rendezvous not yet supported (Chrome-first, §9.6)")
	}
	// Runtime dir user-only; clear a stale socket left by a crash so bind doesn't
	// hit EADDRINUSE.
	if err := os.MkdirAll(filepath.Dir(s.socketPath), 0o700); err != nil {
		return fmt.Errorf("create runtime dir: %w", err)
	}
	if _, err := os.Stat(s.socketPath); err == nil {
		_ = os.Remove(s.socketPath)
	}
	ln, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.socketPath, err)
	}
	if err := os.Chmod(s.socketPath, 0o600); err != nil {
		s.logf("[rendezvous] could not chmod socket 0600: %v", err)
	}
	s.mu.Lock()
	s.ln = ln
	s.mu.Unlock()
	s.logf("[rendezvous] listening on %s (0600, per-user)", s.socketPath)
	go s.acceptLoop(ln)
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
	// Newest-wins: a fresh host connection supersedes the old.
	old := s.active
	s.active = conn
	s.announced = tools
	s.mu.Unlock()
	if old != nil && old != conn {
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

func (s *rendezvousServer) isConnected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active != nil
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
		_ = os.Remove(s.socketPath)
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
