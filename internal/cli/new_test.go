package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNew_ValidName(t *testing.T) {
	tmp := t.TempDir()
	name := "test-skrpt"
	dir := filepath.Join(tmp, name)

	// Run from parent dir by changing name to the full path.
	code := New([]string{dir})
	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d", code)
	}

	// Check skrptiq.yaml exists.
	data, err := os.ReadFile(filepath.Join(dir, "skrptiq.yaml"))
	if err != nil {
		t.Fatalf("skrptiq.yaml not found: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "name: test-skrpt") {
		t.Errorf("expected name field, got:\n%s", content)
	}
	if !strings.Contains(content, "id: ") {
		t.Errorf("expected id field, got:\n%s", content)
	}
	if !strings.Contains(content, "version: \"0.1.0\"") {
		t.Errorf("expected version field, got:\n%s", content)
	}

	// Check subdirectories.
	for _, sub := range []string{"skills", "prompts", "workflows"} {
		if _, err := os.Stat(filepath.Join(dir, sub)); err != nil {
			t.Errorf("expected %s/ directory: %v", sub, err)
		}
	}
}

func TestNew_InvalidName(t *testing.T) {
	code := New([]string{"Invalid_Name"})
	if code != ExitBadArgs {
		t.Errorf("expected exit %d for invalid name, got %d", ExitBadArgs, code)
	}
}

func TestNew_MissingArgs(t *testing.T) {
	code := New([]string{})
	if code != ExitBadArgs {
		t.Errorf("expected exit %d for missing args, got %d", ExitBadArgs, code)
	}
}

func TestNew_DirectoryExists(t *testing.T) {
	tmp := t.TempDir()
	existing := filepath.Join(tmp, "already-here")
	os.MkdirAll(existing, 0755)

	code := New([]string{existing})
	if code != ExitFailed {
		t.Errorf("expected exit %d for existing dir, got %d", ExitFailed, code)
	}
}

func TestToDisplayName(t *testing.T) {
	tests := []struct {
		slug string
		want string
	}{
		{"my-cool-skrpt", "My Cool Skrpt"},
		{"simple", "Simple"},
		{"a-b-c", "A B C"},
	}
	for _, tt := range tests {
		got := toDisplayName(tt.slug)
		if got != tt.want {
			t.Errorf("toDisplayName(%q) = %q, want %q", tt.slug, got, tt.want)
		}
	}
}
