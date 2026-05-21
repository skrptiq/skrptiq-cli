package cli

import (
	"archive/zip"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/skrptiq/engine/bundle"
)

func TestVerify_MissingArgs(t *testing.T) {
	code := Verify([]string{})
	if code != ExitBadArgs {
		t.Errorf("expected exit %d, got %d", ExitBadArgs, code)
	}
}

func TestVerify_NonexistentFile(t *testing.T) {
	code := verifyWithKeys("/nonexistent/bundle.zip", bundle.TrustedKeys{}, false)
	if code == ExitOK {
		t.Error("expected non-zero exit for nonexistent file")
	}
}

func TestVerify_RoundTrip(t *testing.T) {
	zipPath, keys := signAndZipFixture(t)

	code := verifyWithKeys(zipPath, keys, false)
	if code != ExitOK {
		t.Errorf("expected exit 0 for valid signed bundle, got %d", code)
	}
}

func TestVerify_RoundTrip_JSON(t *testing.T) {
	zipPath, keys := signAndZipFixture(t)

	// Capture stdout.
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	code := verifyWithKeys(zipPath, keys, true)

	w.Close()
	os.Stdout = old

	if code != ExitOK {
		t.Fatalf("expected exit 0, got %d", code)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])
	if len(output) == 0 {
		t.Fatal("expected JSON output")
	}
	// Verify JSON contains expected fields.
	for _, field := range []string{`"ok": true`, `"signer"`, `"signedAt"`, `"checksum"`, `"files"`} {
		if !contains(output, field) {
			t.Errorf("expected %s in JSON output, got:\n%s", field, output)
		}
	}
}

func TestVerify_TamperDetection(t *testing.T) {
	dir, seed := makeSignableFixture(t)

	// Sign the fixture.
	privKey := ed25519.NewKeyFromSeed(seed)
	pubKey := privKey.Public().(ed25519.PublicKey)
	signer := bundle.Signer{
		PrivateKey: privKey,
		Signer:     "test-signer",
		KeyID:      bundle.KeyFingerprint(pubKey),
	}
	if err := bundle.SignBundle(dir, signer); err != nil {
		t.Fatalf("sign: %v", err)
	}

	// Tamper with a file after signing.
	skillPath := filepath.Join(dir, "skills", "example.md")
	data, _ := os.ReadFile(skillPath)
	data[0] ^= 0xFF // flip a byte
	os.WriteFile(skillPath, data, 0644)

	zipPath := zipDir(t, dir)
	keys := bundle.TrustedKeys{signer.KeyID: pubKey}

	code := verifyWithKeys(zipPath, keys, false)
	if code != ExitFailed {
		t.Errorf("expected exit %d for tampered bundle, got %d", ExitFailed, code)
	}
}

func TestVerify_UntrustedKey(t *testing.T) {
	dir, seed := makeSignableFixture(t)

	// Sign with key A.
	privKey := ed25519.NewKeyFromSeed(seed)
	pubKey := privKey.Public().(ed25519.PublicKey)
	signer := bundle.Signer{
		PrivateKey: privKey,
		Signer:     "test-signer",
		KeyID:      bundle.KeyFingerprint(pubKey),
	}
	if err := bundle.SignBundle(dir, signer); err != nil {
		t.Fatalf("sign: %v", err)
	}

	zipPath := zipDir(t, dir)

	// Verify with a different key (key B) in trusted keys.
	otherPub, _, _ := ed25519.GenerateKey(rand.Reader)
	otherKeys := bundle.TrustedKeys{bundle.KeyFingerprint(otherPub): otherPub}

	code := verifyWithKeys(zipPath, otherKeys, false)
	if code != ExitMissingDep {
		t.Errorf("expected exit %d (untrusted_key), got %d", ExitMissingDep, code)
	}
}

func TestVerify_UnknownAlgorithm(t *testing.T) {
	dir, seed := makeSignableFixture(t)

	// Sign the fixture normally.
	privKey := ed25519.NewKeyFromSeed(seed)
	pubKey := privKey.Public().(ed25519.PublicKey)
	signer := bundle.Signer{
		PrivateKey: privKey,
		Signer:     "test-signer",
		KeyID:      bundle.KeyFingerprint(pubKey),
	}
	if err := bundle.SignBundle(dir, signer); err != nil {
		t.Fatalf("sign: %v", err)
	}

	// Tamper with algorithm field in the manifest.
	manifestPath := filepath.Join(dir, "skrptiq.yaml")
	data, _ := os.ReadFile(manifestPath)
	tampered := replaceInString(string(data), "skrptiq-bundle-v1", "skrptiq-bundle-v999")
	os.WriteFile(manifestPath, []byte(tampered), 0644)

	zipPath := zipDir(t, dir)
	keys := bundle.TrustedKeys{signer.KeyID: pubKey}

	code := verifyWithKeys(zipPath, keys, false)
	if code != ExitFailed {
		t.Errorf("expected exit %d for unknown algorithm, got %d", ExitFailed, code)
	}
}

// signAndZipFixture creates a signed bundle zip and returns the path + trusted keys.
func signAndZipFixture(t *testing.T) (string, bundle.TrustedKeys) {
	t.Helper()
	dir, seed := makeSignableFixture(t)

	privKey := ed25519.NewKeyFromSeed(seed)
	pubKey := privKey.Public().(ed25519.PublicKey)
	signer := bundle.Signer{
		PrivateKey: privKey,
		Signer:     "test-signer",
		KeyID:      bundle.KeyFingerprint(pubKey),
	}
	if err := bundle.SignBundle(dir, signer); err != nil {
		t.Fatalf("sign: %v", err)
	}

	zipPath := zipDir(t, dir)
	keys := bundle.TrustedKeys{signer.KeyID: pubKey}
	return zipPath, keys
}

// zipDir creates a zip archive from a directory and returns the zip path.
func zipDir(t *testing.T, srcDir string) string {
	t.Helper()
	zipPath := filepath.Join(t.TempDir(), "bundle.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(srcDir, path)
		fw, err := w.Create(rel)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = fw.Write(data)
		return err
	})

	return zipPath
}

func replaceInString(s, old, new string) string {
	i := 0
	for i < len(s) {
		idx := indexOf(s[i:], old)
		if idx < 0 {
			break
		}
		s = s[:i+idx] + new + s[i+idx+len(old):]
		i = i + idx + len(new)
	}
	return s
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func contains(s, sub string) bool {
	return indexOf(s, sub) >= 0
}

// --- Cross-side drift-gate tests ---
// These pin the same constants engine + App tests reference. A CLI-side
// checksum or trust-loading regression fails CLI CI in lockstep with
// engine + App — that's the design per BUNDLE-INTEGRITY-CONTRACT §9.4.

const pinnedFixtureChecksum = "sha256:e93447842a7493ebc2d0dc9dc1e1e5f5cf13a79a66c1fe87724ca78beca9ba8a"
const pinnedHubKeyFingerprint = "64c8dc7ad56dcd8fee9a3f460ade4a0f3b2aa5bf21b17cde3f755ab4b6758ebe"

func TestDriftGate_FixtureChecksum(t *testing.T) {
	fixtureDir := resolveEngineFixture(t, "bundle", "signable-skrpt")

	got, err := bundle.ComputeBundleChecksum(fixtureDir)
	if err != nil {
		t.Fatalf("ComputeBundleChecksum: %v", err)
	}
	if got != pinnedFixtureChecksum {
		t.Errorf("fixture checksum drift:\n  got:  %s\n  want: %s", got, pinnedFixtureChecksum)
	}
}

func TestDriftGate_HubKeyFingerprint(t *testing.T) {
	keys, err := bundle.LoadEmbeddedTrustedKeys()
	if err != nil {
		t.Fatalf("LoadEmbeddedTrustedKeys: %v", err)
	}

	if _, ok := keys[pinnedHubKeyFingerprint]; !ok {
		var found []string
		for k := range keys {
			found = append(found, k)
		}
		t.Errorf("Hub key fingerprint not in trusted registry.\n  want: %s\n  got:  %v", pinnedHubKeyFingerprint, found)
	}
}

// resolveEngineFixture finds the engine fixture directory. go test sets
// cwd to the package directory (internal/cli/), so we navigate up to
// the repo root (../..) then across to the sibling repo via the same
// relative path the go.mod replace directive uses.
func resolveEngineFixture(t *testing.T, category, name string) string {
	t.Helper()
	// internal/cli/ → ../.. = repo root → ../skrptiq-app/engine
	fixtureDir := filepath.Join("..", "..", "..", "skrptiq-app", "engine", "test", "fixtures", category, name)
	abs, _ := filepath.Abs(fixtureDir)

	if _, err := os.Stat(abs); os.IsNotExist(err) {
		t.Skip("engine fixture not found — sibling repo not available")
	}
	return abs
}

// Ensure base64 import is used (for test readability in error messages).
var _ = base64.StdEncoding
