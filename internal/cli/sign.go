package cli

import (
	"crypto/ed25519"
	"encoding/base64"
	"flag"
	"fmt"
	"os"

	"github.com/skrptiq/engine/bundle"
)

// Sign handles `skrptiq sign --key-env <VAR> <package-dir>`.
// Reads an ed25519 signing key from the named env var and calls
// bundle.SignBundle to rewrite skrptiq.yaml in place with integrity
// + trust blocks per BUNDLE-INTEGRITY-CONTRACT §4.3.
func Sign(args []string) int {
	fs := flag.NewFlagSet("sign", flag.ContinueOnError)
	keyEnv := fs.String("key-env", "", "Environment variable holding the base64-encoded ed25519 signing key")
	if err := fs.Parse(args); err != nil {
		return ExitBadArgs
	}

	if *keyEnv == "" {
		fmt.Fprintln(os.Stderr, "Usage: skrptiq sign --key-env <VAR> <package-dir>")
		return ExitBadArgs
	}

	dir := fs.Arg(0)
	if dir == "" {
		fmt.Fprintln(os.Stderr, "Usage: skrptiq sign --key-env <VAR> <package-dir>")
		return ExitBadArgs
	}

	key, err := parseSigningKey(*keyEnv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return ExitBadArgs
	}

	pubKey := key.Public().(ed25519.PublicKey)
	signer := bundle.Signer{
		PrivateKey: key,
		Signer:     "skrptiq-hub",
		KeyID:      bundle.KeyFingerprint(pubKey),
	}

	if err := bundle.SignBundle(dir, signer); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return ExitFailed
	}

	fmt.Fprintf(os.Stderr, "Signed %s/skrptiq.yaml\n", dir)
	return ExitOK
}

// parseSigningKey reads the named env var and decodes a base64-encoded
// ed25519 key. Accepts 32-byte seed or 64-byte full private key.
func parseSigningKey(envVar string) (ed25519.PrivateKey, error) {
	raw := os.Getenv(envVar)
	if raw == "" {
		return nil, fmt.Errorf("environment variable %s is not set", envVar)
	}

	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid key in %s: base64 decode failed: %v", envVar, err)
	}

	switch len(decoded) {
	case ed25519.SeedSize: // 32 bytes
		return ed25519.NewKeyFromSeed(decoded), nil
	case ed25519.PrivateKeySize: // 64 bytes
		return ed25519.PrivateKey(decoded), nil
	default:
		return nil, fmt.Errorf("invalid key in %s: expected 32-byte seed or 64-byte private key, got %d bytes", envVar, len(decoded))
	}
}
