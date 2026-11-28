//go:build !darwin && !linux

package bridge

import "net"

// peerUID has no implementation on this platform, so it reports no evidence.
//
// NOT A STUB THAT LIES: it returns ok=false, which the caller reads as "no
// additional evidence" and NOT as "the peer is this user". The filesystem policy
// in secure_dir.go is what admits a connection on every platform; this only ever
// adds a refusal, never an approval.
func peerUID(net.Conn) (int, bool) { return 0, false }
