package bridge

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
)

// Host mode — the process Chrome spawns via the native-messaging manifest
// (runtime.connectNative). A thin, stateless relay (mirrors skrptiq-extension
// companion/src/host.ts):
//
//	extension ⇄ (native-messaging frames on THIS process's stdio) ⇄ host
//	host      ⇄ (NDJSON on the rendezvous socket)                 ⇄ MCP-mode bridge
//
// It owns NO tool logic and NO MCP. Fail-loud, never silent:
//   - rendezvous dial fails (engine-host not running) → app_not_running, exit 1.
//   - rendezvous drops mid-session → native_disconnected, exit 1.
//   - stdin EOF (Chrome closed the port) → exit 0.
//
// CRITICAL: stdout is the native-messaging channel to Chrome — every log line
// goes to stderr.

// validateDialTarget applies the rendezvous directory policy and then judges the
// endpoint itself, BEFORE anything is dialled or relayed.
//
// The directory is validated first for the reason the whole of GH#1414 turns on:
// statting the socket immediately before connect is not an identity guarantee if
// the directory holding it can be replaced through a writable ancestor.
func validateDialTarget(socketPath string) error {
	root, err := secureDir(filepath.Dir(socketPath))
	if err != nil {
		return err
	}
	defer root.Close()
	return checkSocketFile(socketPath)
}

// runHost dials the rendezvous socket for the CLI role and relays until exit,
// returning the process exit code. stdin is the Chrome native-messaging channel.
func runHost(socketPath string, stdin io.Reader, stdout io.Writer, logf func(string, ...interface{})) int {
	if logf == nil {
		logf = func(string, ...interface{}) {}
	}
	var frameMu sync.Mutex // serialise host→Chrome frame writes
	writeMsg := func(msg interface{}) {
		frame, err := encodeFrame(msg)
		if err != nil {
			logf("[host] failed to frame message to extension: %v", err)
			return
		}
		frameMu.Lock()
		_, _ = stdout.Write(frame)
		frameMu.Unlock()
	}

	// SAME DIRECTORY POLICY ON THE DIAL SIDE (GH#1414). The listener's checks do
	// nothing for this process: it is a separate short-lived child spawned by
	// Chrome, and before this it dialled whatever was at the path. A validated
	// directory plus a validated endpoint is what makes "this socket belongs to
	// this user's engine-host" a checked claim rather than a comment.
	if err := validateDialTarget(socketPath); err != nil {
		// ITS OWN DIAGNOSTIC, NOT app-not-running (GH#1325 D3). Every dial failure
		// used to report the engine-host as not running, so a refusal to admit was
		// indistinguishable from an absence of service — and the person was told to
		// start something that may well already be running.
		//
		// WHY host_unavailable AND NOT A NEW CODE. `protocol.golden.json` pins the
		// complete error-code list and the extension shares that contract, so adding
		// `bridge_endpoint_unsafe` would be a protocol change that reaches a peer
		// which does not know it — and a protocol change is not in GH#1414's scope.
		// `host_unavailable` is already distinct from `app_not_running`, which is the
		// distinction D3 asks for, and the reason travels in the message. If a
		// dedicated code is wanted it is a contract addition to route with extension,
		// not something to take unilaterally here.
		logf("[host] refusing the rendezvous endpoint: %v", err)
		writeMsg(ErrorMessage{
			Type: "error",
			Code: ErrHostUnavailable,
			Message: fmt.Sprintf("The Skrptiq bridge endpoint was refused: %v. "+
				"The path came from %s. Nothing was sent to it.", err, socketPathSource(RoleCLI)),
		})
		return 1
	}

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		// ENOENT/ECONNREFUSED ⇒ the engine-host is not running. Fail loud to the
		// extension, then exit non-zero. Reached only once the endpoint has been
		// validated, so this now means what it says.
		logf("[host] rendezvous dial failed (%v) — engine-host not running", err)
		writeMsg(ErrorMessage{
			Type:    "error",
			Code:    ErrAppNotRunning,
			Message: "The Skrptiq engine-host is not running. Start the app (or CLI) and retry.",
		})
		return 1
	}

	// PEER CREDENTIALS, WHERE THE PLATFORM HAS THEM — an addition, not the answer.
	// They establish the UID behind the socket independently of the filesystem, so
	// a pathname substitution that somehow passed the checks above is still caught.
	// They do NOT authenticate same-UID applications, so they replace nothing.
	if uid, ok := peerUID(conn); ok && uid != os.Geteuid() {
		logf("[host] rendezvous peer runs as uid %d, not %d — refusing", uid, os.Geteuid())
		_ = conn.Close()
		writeMsg(ErrorMessage{
			Type: "error",
			Code: ErrHostUnavailable,
			Message: fmt.Sprintf("The Skrptiq bridge endpoint is served by uid %d, not by "+
				"this user (%d). Nothing was sent to it.", uid, os.Geteuid()),
		})
		return 1
	}
	logf("[host] connected to rendezvous %s", socketPath)

	var once sync.Once
	exitCode := make(chan int, 1)
	die := func(code int) { once.Do(func() { exitCode <- code }) }

	// bridge → extension: NDJSON lines on the socket → native-messaging frames.
	go func() {
		scanner := bufio.NewScanner(conn)
		scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			writeFramePayload(stdout, &frameMu, line, logf)
		}
		// Socket ended mid-session.
		logf("[host] rendezvous closed")
		writeMsg(ErrorMessage{
			Type:    "error",
			Code:    ErrNativeDisconnected,
			Message: "The bridge connection to the Skrptiq engine-host dropped.",
		})
		die(1)
	}()

	// extension → bridge: native-messaging frames on stdin → NDJSON on the socket.
	go func() {
		err := readFrames(stdin, func(payload []byte) {
			_, _ = conn.Write(append(payload, '\n'))
		})
		if err == io.EOF {
			logf("[host] stdin closed by Chrome — extension disconnected")
			die(0)
		} else {
			logf("[host] stdin error: %v", err)
			die(1)
		}
		_ = conn.Close() // unblock the socket→stdout goroutine
	}()

	code := <-exitCode
	_ = conn.Close()
	return code
}

// writeFramePayload frames a raw JSON payload (bridge→extension direction, 1 MB
// cap) and writes it to Chrome. Forwarding the raw line avoids a re-marshal that
// could drift from the wire bytes.
func writeFramePayload(stdout io.Writer, mu *sync.Mutex, payload []byte, logf func(string, ...interface{})) {
	if len(payload) > NMHostToExtMaxBytes {
		logf("[host] dropping oversized bridge→extension frame (%d bytes > %d cap)", len(payload), NMHostToExtMaxBytes)
		return
	}
	header := make([]byte, 4)
	binary.LittleEndian.PutUint32(header, uint32(len(payload)))
	mu.Lock()
	_, _ = stdout.Write(header)
	_, _ = stdout.Write(payload)
	mu.Unlock()
}

// runHostStdio wires the OS stdio + a stderr logger and runs host mode.
func runHostStdio(socketPath string) int {
	logf := func(format string, args ...interface{}) {
		_, _ = fmt.Fprintf(os.Stderr, "[host] "+format+"\n", args...)
	}
	return runHost(socketPath, os.Stdin, os.Stdout, logf)
}
