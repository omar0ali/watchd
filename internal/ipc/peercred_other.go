//go:build !linux

package ipc

import "net"

// checkPeer is a no-op off Linux; access is restricted by the 0700 socket
// directory and the 0600 socket mode.
func checkPeer(net.Conn) error { return nil }
