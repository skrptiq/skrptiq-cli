package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCatalogue_ValidDirectory(t *testing.T) {
	// Use the scan testdata which has valid-package, invalid-package, etc.
	// Create a temp catalogue with two valid skrpts.
	tmp := t.TempDir()

	// Create skrpt-a
	mkSkrpt(t, tmp, "skrpt-a", "name: skrpt-a\nversion: \"1.0.0\"\ndisplay_name: \"Skrpt A\"\nid: a1b2c3d4-e5f6-7890-abcd-ef1234567890\n")
	// Create skrpt-b
	mkSkrpt(t, tmp, "skrpt-b", "name: skrpt-b\nversion: \"2.0.0\"\n")

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := Catalogue([]string{tmp})

	w.Close()
	os.Stdout = old

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d", code)
	}

	var entries []catalogueEntry
	if err := json.NewDecoder(r).Decode(&entries); err != nil {
		t.Fatalf("JSON decode failed: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Sorted alphabetically.
	if entries[0].Slug != "skrpt-a" {
		t.Errorf("entries[0].slug = %q, want %q", entries[0].Slug, "skrpt-a")
	}
	if entries[0].DisplayName != "Skrpt A" {
		t.Errorf("entries[0].displayName = %q, want %q", entries[0].DisplayName, "Skrpt A")
	}
	if entries[0].ID != "a1b2c3d4-e5f6-7890-abcd-ef1234567890" {
		t.Errorf("entries[0].id = %q, want UUID", entries[0].ID)
	}
	if entries[1].Slug != "skrpt-b" {
		t.Errorf("entries[1].slug = %q, want %q", entries[1].Slug, "skrpt-b")
	}
	if entries[1].Version != "2.0.0" {
		t.Errorf("entries[1].version = %q, want %q", entries[1].Version, "2.0.0")
	}
}

func TestCatalogue_SkipsMalformed(t *testing.T) {
	tmp := t.TempDir()

	mkSkrpt(t, tmp, "good-skrpt", "name: good-skrpt\nversion: \"1.0.0\"\n")
	// Create a malformed skrpt (invalid YAML).
	badDir := filepath.Join(tmp, "bad-skrpt")
	os.MkdirAll(badDir, 0755)
	os.WriteFile(filepath.Join(badDir, "skrptiq.yaml"), []byte("{{invalid yaml"), 0644)

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := Catalogue([]string{tmp})

	w.Close()
	os.Stdout = old

	// Should exit non-zero due to malformed entry.
	if code != ExitFailed {
		t.Errorf("expected exit %d for malformed entry, got %d", ExitFailed, code)
	}

	var entries []catalogueEntry
	if err := json.NewDecoder(r).Decode(&entries); err != nil {
		t.Fatalf("JSON decode failed: %v", err)
	}

	// Should still include the good skrpt.
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (good skrpt), got %d", len(entries))
	}
	if entries[0].Slug != "good-skrpt" {
		t.Errorf("entries[0].slug = %q, want %q", entries[0].Slug, "good-skrpt")
	}
}

func TestCatalogue_SkipsNonSkrptDirs(t *testing.T) {
	tmp := t.TempDir()

	mkSkrpt(t, tmp, "real-skrpt", "name: real-skrpt\nversion: \"1.0.0\"\n")
	// Create a dir without skrptiq.yaml — should be skipped silently.
	os.MkdirAll(filepath.Join(tmp, "not-a-skrpt"), 0755)
	// Hidden dir — should be skipped.
	os.MkdirAll(filepath.Join(tmp, ".hidden"), 0755)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := Catalogue([]string{tmp})

	w.Close()
	os.Stdout = old

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d", code)
	}

	var entries []catalogueEntry
	json.NewDecoder(r).Decode(&entries)

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
}

func TestCatalogue_MissingArgs(t *testing.T) {
	code := Catalogue([]string{})
	if code != ExitBadArgs {
		t.Errorf("expected exit %d, got %d", ExitBadArgs, code)
	}
}

func TestCatalogue_NotADirectory(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "file.txt")
	os.WriteFile(f, []byte("not a dir"), 0644)

	code := Catalogue([]string{f})
	if code != ExitBadArgs {
		t.Errorf("expected exit %d, got %d", ExitBadArgs, code)
	}
}

// mkSkrpt creates a minimal skrpt directory with the given manifest YAML.
func mkSkrpt(t *testing.T, parent, name, manifestYAML string) {
	t.Helper()
	dir := filepath.Join(parent, name)
	os.MkdirAll(filepath.Join(dir, "skills"), 0755)
	os.WriteFile(filepath.Join(dir, "skrptiq.yaml"), []byte(manifestYAML), 0644)
}
