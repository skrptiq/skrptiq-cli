package bridge

import (
	"encoding/json"
	"os"
	"sort"
	"testing"
)

// The frozen protocol-v2 contract lives in skrptiq-extension shared/protocol.ts.
// scripts/regen-protocol.mjs mechanically derives protocol.golden.json from it;
// this test asserts the Go mirror equals the golden, so any drift from the SoT
// fails CI here (self-contained — no sibling repo needed at test time).

func loadProtocolGolden(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile("protocol.golden.json")
	if err != nil {
		t.Fatalf("read protocol.golden.json (run scripts/regen-protocol.mjs): %v", err)
	}
	var g map[string]any
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatalf("parse golden: %v", err)
	}
	return g
}

func TestProtocolConstantsMatchGolden(t *testing.T) {
	g := loadProtocolGolden(t)
	if int(g["protocolVersion"].(float64)) != ProtocolVersion {
		t.Errorf("ProtocolVersion = %d, golden = %v", ProtocolVersion, g["protocolVersion"])
	}
	if int(g["defaultInvokeTimeoutMs"].(float64)) != DefaultInvokeTimeoutMS {
		t.Errorf("DefaultInvokeTimeoutMS = %d, golden = %v", DefaultInvokeTimeoutMS, g["defaultInvokeTimeoutMs"])
	}
	if int64(g["nmHostToExtMaxBytes"].(float64)) != int64(NMHostToExtMaxBytes) {
		t.Errorf("NMHostToExtMaxBytes = %d, golden = %v", NMHostToExtMaxBytes, g["nmHostToExtMaxBytes"])
	}
	if int64(g["nmExtToHostMaxBytes"].(float64)) != int64(NMExtToHostMaxBytes) {
		t.Errorf("NMExtToHostMaxBytes = %d, golden = %v", NMExtToHostMaxBytes, g["nmExtToHostMaxBytes"])
	}
}

func TestErrorCodesMatchGolden(t *testing.T) {
	g := loadProtocolGolden(t)
	rawCodes := g["errorCodes"].([]any)
	want := make([]string, 0, len(rawCodes))
	for _, c := range rawCodes {
		want = append(want, c.(string))
	}
	got := make([]string, 0, len(allErrorCodes))
	for _, c := range allErrorCodes {
		got = append(got, string(c))
	}
	sort.Strings(want)
	sort.Strings(got)
	if len(want) != len(got) {
		t.Fatalf("error-code count: Go has %d, golden has %d\n go=%v\n golden=%v", len(got), len(want), got, want)
	}
	for i := range want {
		if want[i] != got[i] {
			t.Errorf("error-code drift at %d: Go %q vs golden %q", i, got[i], want[i])
		}
	}
}
