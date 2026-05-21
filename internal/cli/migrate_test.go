package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateIdentity_AlreadyCanonical(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "skrptiq.yaml"), []byte(
		"id: 05c1e486-3c4d-4fe8-a617-82431bb6fc2c\nname: test\nversion: \"1.0.0\"\n",
	), 0644)

	code := MigrateIdentity([]string{tmp})
	if code != ExitOK {
		t.Errorf("expected exit 0 for canonical ID, got %d", code)
	}
}

func TestMigrateIdentity_UppercaseUUID(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "skrptiq.yaml"), []byte(
		"id: 05C1E486-3C4D-4FE8-A617-82431BB6FC2C\nname: test\nversion: \"1.0.0\"\n",
	), 0644)

	code := MigrateIdentity([]string{tmp})
	if code != ExitOK {
		t.Errorf("expected exit 0 after migration, got %d", code)
	}

	data, err := os.ReadFile(filepath.Join(tmp, "skrptiq.yaml"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), "05c1e486-3c4d-4fe8-a617-82431bb6fc2c") {
		t.Errorf("expected lowercased UUID, got:\n%s", data)
	}
}

func TestMigrateIdentity_MalformedUUID(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "skrptiq.yaml"), []byte(
		"id: not-a-uuid-at-all\nname: test\nversion: \"1.0.0\"\n",
	), 0644)

	code := MigrateIdentity([]string{tmp})
	if code != ExitFailed {
		t.Errorf("expected exit %d for malformed UUID, got %d", ExitFailed, code)
	}
}

func TestMigrateIdentity_NoID(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "skrptiq.yaml"), []byte(
		"name: test\nversion: \"1.0.0\"\n",
	), 0644)

	code := MigrateIdentity([]string{tmp})
	if code != ExitOK {
		t.Errorf("expected exit 0 for missing ID, got %d", code)
	}
}

func TestMigrateIdentity_MissingArgs(t *testing.T) {
	code := MigrateIdentity([]string{})
	if code != ExitBadArgs {
		t.Errorf("expected exit %d for missing args, got %d", ExitBadArgs, code)
	}
}
