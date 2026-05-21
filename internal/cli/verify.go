package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/skrptiq/engine/bundle"
)

// Verify handles `skrptiq verify <bundle.zip> [--json]`.
// Calls bundle.VerifyBundle with the engine's embedded trusted keys.
// Structured error codes map to distinct exit codes per contract §5.1.
func Verify(args []string) int {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "Emit structured JSON output")
	if err := fs.Parse(args); err != nil {
		return ExitBadArgs
	}

	bundlePath := fs.Arg(0)
	if bundlePath == "" {
		fmt.Fprintln(os.Stderr, "Usage: skrptiq verify <bundle.zip> [--json]")
		return ExitBadArgs
	}

	keys, err := bundle.LoadEmbeddedTrustedKeys()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: load trusted keys: %v\n", err)
		return ExitFailed
	}

	return verifyWithKeys(bundlePath, keys, *jsonOut)
}

// verifyWithKeys is the internal verify implementation. Exported Verify
// passes production keys; tests inject ephemeral test keys.
func verifyWithKeys(bundlePath string, keys bundle.TrustedKeys, jsonOut bool) int {
	result, err := bundle.VerifyBundle(bundlePath, keys)
	if err != nil {
		var verr *bundle.VerifyError
		if errors.As(err, &verr) {
			return handleVerifyError(verr, jsonOut)
		}
		// Non-VerifyError (IO, etc.)
		if jsonOut {
			outputJSON(os.Stdout, map[string]any{"ok": false, "error": err.Error()})
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		return ExitFailed
	}

	if jsonOut {
		outputJSON(os.Stdout, map[string]any{
			"ok":        true,
			"signer":    result.Signer,
			"signedAt":  result.SignedAt,
			"checksum":  result.Checksum,
			"algorithm": string(result.Algorithm),
			"files":     result.Files,
		})
	} else {
		fmt.Printf("Verified: %s\n", result.Signer)
		fmt.Printf("  Signed:    %s\n", result.SignedAt)
		fmt.Printf("  Checksum:  %s\n", result.Checksum)
		fmt.Printf("  Algorithm: %s\n", result.Algorithm)
		fmt.Printf("  Files:     %d files\n", len(result.Files))
	}
	return ExitOK
}

// handleVerifyError maps VerifyError.Code to exit codes and prints output.
func handleVerifyError(verr *bundle.VerifyError, jsonOut bool) int {
	if jsonOut {
		outputJSON(os.Stdout, map[string]any{
			"ok":     false,
			"code":   string(verr.Code),
			"detail": verr.Detail,
		})
	} else {
		fmt.Fprintf(os.Stderr, "Verification failed: %s\n  %s\n", verr.Code, verr.Detail)
	}

	switch verr.Code {
	case bundle.CodeUntrustedKey:
		return ExitMissingDep
	default:
		return ExitFailed
	}
}

