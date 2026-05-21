package cli

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSign_MissingArgs(t *testing.T) {
	code := Sign([]string{})
	if code != ExitBadArgs {
		t.Errorf("expected exit %d, got %d", ExitBadArgs, code)
	}
}

func TestSign_MissingKeyEnv(t *testing.T) {
	code := Sign([]string{t.TempDir()})
	if code != ExitBadArgs {
		t.Errorf("expected exit %d, got %d", ExitBadArgs, code)
	}
}

func TestSign_EmptyEnvVar(t *testing.T) {
	t.Setenv("TEST_EMPTY_KEY", "")
	code := Sign([]string{"--key-env", "TEST_EMPTY_KEY", t.TempDir()})
	if code != ExitBadArgs {
		t.Errorf("expected exit %d, got %d", ExitBadArgs, code)
	}
}

func TestSign_InvalidKey(t *testing.T) {
	t.Setenv("TEST_BAD_KEY", "not-valid-base64!!!")
	code := Sign([]string{"--key-env", "TEST_BAD_KEY", t.TempDir()})
	if code != ExitBadArgs {
		t.Errorf("expected exit %d, got %d", ExitBadArgs, code)
	}
}

func TestSign_WrongKeyLength(t *testing.T) {
	// 16 bytes — neither 32 (seed) nor 64 (full key).
	t.Setenv("TEST_SHORT_KEY", base64.StdEncoding.EncodeToString(make([]byte, 16)))
	code := Sign([]string{"--key-env", "TEST_SHORT_KEY", t.TempDir()})
	if code != ExitBadArgs {
		t.Errorf("expected exit %d, got %d", ExitBadArgs, code)
	}
}

func TestSign_RoundTrip(t *testing.T) {
	dir, seed := makeSignableFixture(t)

	t.Setenv("TEST_SIGN_KEY", base64.StdEncoding.EncodeToString(seed))
	code := Sign([]string{"--key-env", "TEST_SIGN_KEY", dir})
	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d", code)
	}

	// Verify skrptiq.yaml has integrity + trust blocks.
	data, err := os.ReadFile(filepath.Join(dir, "skrptiq.yaml"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "integrity:") {
		t.Error("expected integrity block in signed manifest")
	}
	if !strings.Contains(content, "trust:") {
		t.Error("expected trust block in signed manifest")
	}
	if !strings.Contains(content, "signature:") {
		t.Error("expected signature field in trust block")
	}
}

// makeSignableFixture creates a minimal valid package dir and returns
// the path + the 32-byte ed25519 seed.
func makeSignableFixture(t *testing.T) (string, []byte) {
	t.Helper()
	dir := t.TempDir()

	os.MkdirAll(filepath.Join(dir, "skills"), 0755)
	os.WriteFile(filepath.Join(dir, "skrptiq.yaml"), []byte(
		"id: a1b2c3d4-e5f6-7890-abcd-ef1234567890\nname: test-sign\nversion: \"1.0.0\"\n",
	), 0644)
	os.WriteFile(filepath.Join(dir, "skills", "example.md"), []byte(
		"---\ntype: skill\nid: example\ntitle: Example\n---\nContent.\n",
	), 0644)

	_, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	return dir, privKey.Seed()
}
