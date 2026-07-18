package bridge

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"
)

func TestEncodeFrame_RoundTrip(t *testing.T) {
	msg := ReadyMessage{Type: "ready", V: ProtocolVersion, Server: peerInfo{Name: "x", Version: "1"}}
	frame, err := encodeFrame(msg)
	if err != nil {
		t.Fatalf("encodeFrame: %v", err)
	}
	// 4-byte LE length prefix + JSON payload.
	if len(frame) < 4 {
		t.Fatalf("frame too short: %d", len(frame))
	}
	n := binary.LittleEndian.Uint32(frame[:4])
	if int(n) != len(frame)-4 {
		t.Fatalf("length prefix %d != payload %d", n, len(frame)-4)
	}
	var got ReadyMessage
	if err := json.Unmarshal(frame[4:], &got); err != nil {
		t.Fatalf("payload not valid JSON: %v", err)
	}
	if got.Type != "ready" || got.V != ProtocolVersion {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}

func TestEncodeFrame_OverCapFailsLoud(t *testing.T) {
	// A payload past the 1 MB host→ext cap must error, not silently drop.
	big := map[string]string{"blob": strings.Repeat("x", NMHostToExtMaxBytes)}
	if _, err := encodeFrame(big); err == nil {
		t.Fatal("expected an over-cap frame to fail loud, got nil error")
	}
}

func TestNativeFrameDecoder_StreamingAndPartial(t *testing.T) {
	// Two messages, fed in fragments across frame boundaries.
	f1, _ := encodeFrame(map[string]any{"type": "a", "n": 1})
	f2, _ := encodeFrame(map[string]any{"type": "b", "n": 2})
	full := append(append([]byte{}, f1...), f2...)

	dec := &nativeFrameDecoder{}
	var got [][]byte
	// Feed one byte at a time — the hardest partial case.
	for i := 0; i < len(full); i++ {
		got = append(got, dec.feed(full[i:i+1])...)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 decoded frames, got %d", len(got))
	}
	var m0, m1 map[string]any
	_ = json.Unmarshal(got[0], &m0)
	_ = json.Unmarshal(got[1], &m1)
	if m0["type"] != "a" || m1["type"] != "b" {
		t.Errorf("decoded out of order / wrong: %v %v", m0, m1)
	}
}

func TestReadFrames_DeliversThenEOF(t *testing.T) {
	f1, _ := encodeFrame(map[string]any{"type": "x"})
	r := bytes.NewReader(f1)
	var count int
	err := readFrames(r, func([]byte) { count++ })
	if count != 1 {
		t.Errorf("expected 1 frame, got %d", count)
	}
	if err == nil {
		t.Error("expected a terminating error (io.EOF) at stream end")
	}
}
