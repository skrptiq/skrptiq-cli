package bridge

import (
	"strings"
	"testing"
)

func TestParseBridgeMode(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantMode string
		wantIsBr bool
	}{
		{"not bridge", []string{"scan", "."}, "", false},
		{"empty", nil, "", false},
		{"mcp", []string{"__bridge", "--mode", "mcp"}, "mcp", true},
		{"host", []string{"__bridge", "--mode", "host"}, "host", true},
		{"host with chrome origin appended", []string{"__bridge", "--mode", "host", "chrome-extension://abc/"}, "host", true},
		{"bridge no mode", []string{"__bridge"}, "", true},
		{"bridge dangling flag", []string{"__bridge", "--mode"}, "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mode, isBr := parseBridgeMode(c.args)
			if mode != c.wantMode || isBr != c.wantIsBr {
				t.Errorf("parseBridgeMode(%v) = (%q,%v), want (%q,%v)", c.args, mode, isBr, c.wantMode, c.wantIsBr)
			}
		})
	}
}

func TestDispatch_PassesThroughNonBridge(t *testing.T) {
	handled, code := Dispatch([]string{"list", "workflows"})
	if handled || code != 0 {
		t.Errorf("Dispatch(non-bridge) = (%v,%d), want (false,0)", handled, code)
	}
}

func TestSocketPathFor_Override(t *testing.T) {
	t.Setenv("SKRPTIQ_BRIDGE_SOCKET", "/tmp/explicit.sock")
	if got := socketPathFor(RoleCLI); got != "/tmp/explicit.sock" {
		t.Errorf("override ignored: %q", got)
	}
}

func TestSocketPathFor_RoleSuffix(t *testing.T) {
	t.Setenv("SKRPTIQ_BRIDGE_SOCKET", "")
	got := socketPathFor(RoleCLI)
	if !strings.HasSuffix(got, "cli.sock") {
		t.Errorf("CLI socket %q should end in cli.sock", got)
	}
}

func TestIsHostRole(t *testing.T) {
	if !isHostRole("cli") || !isHostRole("app") {
		t.Error("cli/app should be valid roles")
	}
	if isHostRole("nope") {
		t.Error("unknown role accepted")
	}
}
