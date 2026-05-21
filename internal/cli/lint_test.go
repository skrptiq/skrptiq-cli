package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLint_CleanPackage(t *testing.T) {
	code := Lint([]string{filepath.Join("..", "scan", "testdata", "valid-package")})
	if code != ExitOK {
		t.Errorf("expected exit 0 for valid package, got %d", code)
	}
}

func TestLint_PackageWithErrors(t *testing.T) {
	// Create a package with an invalid manifest name (error-severity).
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "skills"), 0755)
	os.WriteFile(filepath.Join(tmp, "skrptiq.yaml"), []byte(
		"name: INVALID_NAME\nversion: \"1.0.0\"\n",
	), 0644)

	code := Lint([]string{tmp})
	if code == ExitOK {
		t.Error("expected non-zero exit for package with invalid manifest name")
	}
}

func TestLint_MissingArgs(t *testing.T) {
	code := Lint([]string{})
	if code != ExitBadArgs {
		t.Errorf("expected exit %d for missing args, got %d", ExitBadArgs, code)
	}
}

func TestLint_AutoFix_UppercaseUUID(t *testing.T) {
	tmp := t.TempDir()
	// Create a package with an uppercase UUID.
	os.MkdirAll(filepath.Join(tmp, "skills"), 0755)
	os.WriteFile(filepath.Join(tmp, "skrptiq.yaml"), []byte(
		"id: 05C1E486-3C4D-4FE8-A617-82431BB6FC2C\nname: test-fix\nversion: \"1.0.0\"\n",
	), 0644)
	os.WriteFile(filepath.Join(tmp, "skills", "example.md"), []byte(
		"---\ntype: skill\nid: example\ntitle: Example\n---\nContent.\n",
	), 0644)

	code := Lint([]string{"--auto-fix", tmp})
	if code != ExitOK {
		t.Errorf("expected exit 0 after auto-fix, got %d", code)
	}

	// Verify the UUID was lowercased.
	data, err := os.ReadFile(filepath.Join(tmp, "skrptiq.yaml"))
	if err != nil {
		t.Fatalf("read skrptiq.yaml: %v", err)
	}
	if !strings.Contains(string(data), "05c1e486-3c4d-4fe8-a617-82431bb6fc2c") {
		t.Errorf("expected lowercased UUID, got:\n%s", data)
	}
}
