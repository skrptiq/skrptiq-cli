//go:build windows

package bridge

import (
	"fmt"
	"os"
)

// Socket admission on Windows (GH#1414).
//
// THESE REFUSE RATHER THAN PASS. The rendezvous there is a named pipe and is not
// implemented — `start()` returns "named-pipe rendezvous not yet supported"
// before reaching any of this — so nothing is taken away. What matters is the
// failure DIRECTION: a stub that returned nil would report every pipe as
// validated, which is the silent-pass this whole change exists to remove.
//
// A named pipe's security descriptor is a different mechanism from POSIX mode
// bits, so implementing it means an ACL check, not a port of this file. Named as
// the prerequisite rather than left as a gap nobody wrote down.

func secureDir(dir string) (*os.Root, error) {
	return nil, fmt.Errorf("%w: %s: the rendezvous directory policy is POSIX ownership and "+
		"mode, which does not apply on Windows. A named pipe needs a security-descriptor "+
		"check instead, and that is not implemented", ErrUnsafeDir, dir)
}

func checkSocketOwnership(path string) error {
	return fmt.Errorf("%w: %s: endpoint ownership cannot be established on Windows by this "+
		"policy", ErrUnsafeSocket, path)
}

func checkSocketFile(path string) error { return checkSocketOwnership(path) }

// verifyBoundSocket refuses on Windows for the same reason as the rest of this
// file. It is referenced by `start()`, which returns before reaching it, so the
// stub exists to keep the published windows binary compiling — and it refuses
// rather than returning nil, so a future named-pipe implementation cannot
// accidentally inherit a check that always passed.
func verifyBoundSocket(_ *os.Root, path string) error {
	return fmt.Errorf("%w: %s: a bound endpoint cannot be verified against a validated "+
		"directory on Windows by this policy", ErrUnsafeSocket, path)
}
