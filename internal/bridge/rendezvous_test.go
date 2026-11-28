package bridge

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// shortSocket returns a short UDS path (macOS caps sun_path at ~104 bytes, so a
// long t.TempDir() path would fail to bind).
//
// IT CREATES A PRIVATE SUBDIRECTORY, AND IT HAS TO (GH#1414). It used to return a
// path directly inside `os.TempDir()`, which made the rendezvous DIRECTORY the
// temp directory itself — and the admission policy correctly refuses a
// world-writable one.
//
// THAT WAS INVISIBLE ON macOS AND FAILED ON LINUX. `os.TempDir()` is the per-user
// `/var/folders/…/T` on darwin (0700) and `/tmp` on Linux (1777), so these tests
// passed here and broke there — found by running the suite in a Linux container
// for GH#1416, not by CI, which is macOS-only.
//
// PRODUCTION NEVER DID THIS: `runtimeDir()` returns `<temp>/skrptiq-<uid>`, a
// private subdirectory, which the policy accepts (verified on Linux: `/tmp` is
// accepted as a sticky root-owned ancestor, `/tmp/skrptiq-0` is created 0700 and
// admitted, and a world-writable pre-created leaf is refused). So the helper was
// testing a shape the product does not use, and the fix is to make it realistic
// rather than to loosen the policy.
func shortSocket(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(os.TempDir(), "skr-"+randomID()[:8])
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatalf("make rendezvous dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "cli.sock")
}

func startTestServer(t *testing.T) *rendezvousServer {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("named-pipe rendezvous deferred")
	}
	srv := newRendezvousServer(shortSocket(t), nil)
	if err := srv.start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(srv.close)
	return srv
}

func dial(t *testing.T, srv *rendezvousServer) (net.Conn, *bufio.Reader) {
	t.Helper()
	conn, err := net.Dial("unix", srv.socketPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn, bufio.NewReader(conn)
}

func writeJSON(t *testing.T, conn net.Conn, v any) {
	t.Helper()
	b, _ := json.Marshal(v)
	if _, err := conn.Write(append(b, '\n')); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func readMsg(t *testing.T, r *bufio.Reader) map[string]any {
	t.Helper()
	line, err := r.ReadBytes('\n')
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(line, &m); err != nil {
		t.Fatalf("bad line %q: %v", line, err)
	}
	return m
}

func announce(t *testing.T, conn net.Conn, v int, tools ...string) {
	t.Helper()
	at := make([]AnnouncedTool, len(tools))
	for i, n := range tools {
		at[i] = AnnouncedTool{Name: n}
	}
	writeJSON(t, conn, AnnounceMessage{Type: "announce", V: v, Client: peerInfo{Name: "t", Version: "0"}, Tools: at})
}

func TestRendezvous_HappyInvokeResult(t *testing.T) {
	srv := startTestServer(t)
	conn, r := dial(t, srv)
	announce(t, conn, ProtocolVersion, "read_page")

	if m := readMsg(t, r); m["type"] != "ready" {
		t.Fatalf("expected ready, got %v", m)
	}

	outc := make(chan invokeOutcome, 1)
	go func() { outc <- srv.invoke("read_page", map[string]any{"tabId": 1}) }()

	// The bridge should send us an invoke; reply with a result.
	inv := readMsg(t, r)
	if inv["type"] != "invoke" || inv["tool"] != "read_page" {
		t.Fatalf("expected invoke read_page, got %v", inv)
	}
	writeJSON(t, conn, ResultMessage{Type: "result", ID: inv["id"].(string), OK: true, Content: "hello"})

	select {
	case out := <-outc:
		if !out.ok || out.content != "hello" {
			t.Errorf("outcome = %+v, want ok/hello", out)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("invoke did not resolve")
	}
}

func TestRendezvous_VersionMismatchRejected(t *testing.T) {
	srv := startTestServer(t)
	conn, r := dial(t, srv)
	announce(t, conn, 1) // wrong version

	m := readMsg(t, r)
	if m["type"] != "error" || m["code"] != string(ErrUnsupportedProtocolVersion) {
		t.Fatalf("expected unsupported_protocol_version error, got %v", m)
	}
}

func TestRendezvous_CapabilityNotGranted(t *testing.T) {
	srv := startTestServer(t)
	conn, r := dial(t, srv)
	announce(t, conn, ProtocolVersion) // announces NO tools
	if m := readMsg(t, r); m["type"] != "ready" {
		t.Fatalf("expected ready, got %v", m)
	}
	// give the server a moment to register the announce
	time.Sleep(50 * time.Millisecond)

	out := srv.invoke("read_page", nil)
	if out.ok || out.err.Code != ErrCapabilityNotGranted {
		t.Errorf("expected capability_not_granted, got %+v", out)
	}
}

func TestRendezvous_ExtensionDisconnected(t *testing.T) {
	srv := startTestServer(t)
	out := srv.invoke("read_page", nil) // no extension connected
	if out.ok || out.err.Code != ErrExtensionDisconnected {
		t.Errorf("expected extension_disconnected, got %+v", out)
	}
}
