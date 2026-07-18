package bridge

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
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

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		// ENOENT/ECONNREFUSED ⇒ the engine-host is not running. Fail loud to the
		// extension, then exit non-zero.
		logf("[host] rendezvous dial failed (%v) — engine-host not running", err)
		writeMsg(ErrorMessage{
			Type:    "error",
			Code:    ErrAppNotRunning,
			Message: "The Skrptiq engine-host is not running. Start the app (or CLI) and retry.",
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
