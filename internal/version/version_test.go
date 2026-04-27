package version

import (
	"strings"
	"testing"
)

func TestFullDev(t *testing.T) {
	// Default values — not built with ldflags.
	result := Full()
	if !strings.Contains(result, "dev") {
		t.Errorf("expected 'dev' in default version, got %q", result)
	}
}

func TestFullWithVersion(t *testing.T) {
	// Simulate ldflags injection.
	orig := Version
	Version = "v0.2.0"
	defer func() { Version = orig }()

	result := Full()
	if !strings.HasPrefix(result, "v0.2.0") {
		t.Errorf("expected version prefix, got %q", result)
	}
	if !strings.Contains(result, "unknown") {
		t.Errorf("expected 'unknown' commit in default, got %q", result)
	}
}
