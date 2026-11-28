//go:build linux

package bridge

import (
	"net"

	"golang.org/x/sys/unix"
)

// peerUID reports the UID behind a unix-socket connection. See the darwin file for
// why this is an addition to the filesystem policy and not a replacement for it.
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
		cred, serr := unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
		if serr == nil && cred != nil {
			uid, got = int(cred.Uid), true
		}
	})
	return uid, got
}
