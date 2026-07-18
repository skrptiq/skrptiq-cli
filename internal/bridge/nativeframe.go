package bridge

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

// Chrome native-messaging stdio framing (mirrors skrptiq-extension
// companion/src/nativeframe.ts): a 4-byte little-endian uint32 byte length, then
// that many bytes of UTF-8 JSON. Used ONLY on the host-mode process's stdio
// (Chrome frames the extension side via the runtime Port). The rendezvous hop
// uses newline-delimited JSON instead.
//
//	stdin  (Chrome → host): up to 4 GB per message.
//	stdout (host → Chrome): Chrome caps at 1 MB.

// encodeFrame serialises obj as a native-messaging frame (host → Chrome). It
// fails loud on an over-cap frame rather than letting Chrome silently drop it —
// commands are tiny, so this only trips on a pathological invoke.
func encodeFrame(obj interface{}) ([]byte, error) {
	payload, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	if len(payload) > NMHostToExtMaxBytes {
		return nil, fmt.Errorf("native-messaging frame is %d bytes; exceeds the %d-byte host→extension cap", len(payload), NMHostToExtMaxBytes)
	}
	frame := make([]byte, 4+len(payload))
	binary.LittleEndian.PutUint32(frame[:4], uint32(len(payload)))
	copy(frame[4:], payload)
	return frame, nil
}

// nativeFrameDecoder is a streaming decoder for the Chrome → host direction. Feed
// raw stdin chunks; get back the complete JSON payloads that have fully arrived.
// Partial frames are buffered until the rest lands.
type nativeFrameDecoder struct {
	buf []byte
}

// feed appends chunk and returns every complete frame payload now available.
func (d *nativeFrameDecoder) feed(chunk []byte) [][]byte {
	d.buf = append(d.buf, chunk...)
	var out [][]byte
	for {
		if len(d.buf) < 4 {
			break
		}
		n := binary.LittleEndian.Uint32(d.buf[:4])
		if uint64(len(d.buf)) < uint64(4)+uint64(n) {
			break
		}
		payload := make([]byte, n)
		copy(payload, d.buf[4:4+n])
		d.buf = d.buf[4+n:]
		out = append(out, payload)
	}
	return out
}

// readFrames streams frames from r (Chrome → host) into onFrame until EOF. The
// callback receives each complete JSON payload. Returns the terminating error
// (io.EOF on a clean Chrome port close).
func readFrames(r io.Reader, onFrame func([]byte)) error {
	dec := &nativeFrameDecoder{}
	chunk := make([]byte, 32*1024)
	for {
		n, err := r.Read(chunk)
		if n > 0 {
			for _, payload := range dec.feed(chunk[:n]) {
				onFrame(payload)
			}
		}
		if err != nil {
			return err
		}
	}
}
