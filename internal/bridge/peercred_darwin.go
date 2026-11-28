//go:build darwin

package bridge

import (
	"net"

	"golang.org/x/sys/unix"
)

// peerUID reports the UID behind a unix-socket connection.
//
// AN ADDITION, NOT THE ANSWER (GH#1414, from the CTO's GH#1325 review): peer
// credentials are "stronger against pathname substitution where available — and
// do not authenticate same-UID applications". So they catch a substituted socket
// served by a DIFFERENT user even if the filesystem checks were bypassed, and they
// say nothing about which of this user's own programs is on the other end. The
// filesystem policy in secure_dir.go remains the admission control.
//
// ok is false when the credential cannot be read, and the caller treats that as
// "no additional evidence" rather than as a failure — there is no platform where
// an unreadable Xucred means the peer is hostile, and failing closed on it would
// refuse healthy connections on any platform or socket state that does not carry
// one.
func peerUID(conn net.Conn) (int, bool) {
	uc, ok := conn.(*net.UnixConn)
	if !ok {
		return 0, false
	}
	raw, err := uc.SyscallConn()
	if err != nil {
		return 0, false
	}
	var uid int
	var got bool
	_ = raw.Control(func(fd uintptr) {
		// LOCAL_PEERCRED at SOL_LOCAL is darwin's getpeereid equivalent, and the
		// struct is xucred rather than Linux's ucred — which is why this file is
		// build-tagged rather than written once.
		xu, serr := unix.GetsockoptXucred(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
		if serr == nil && xu != nil {
			uid, got = int(xu.Uid), true
		}
	})
	return uid, got
}
