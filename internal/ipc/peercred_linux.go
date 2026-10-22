//go:build linux

package ipc

import (
	"fmt"
	"net"
	"os"
	"syscall"
)

// checkPeer rejects connections from other users. The socket directory is
// already 0700, so this is defence in depth (e.g. against a misconfigured
// WATCHD_RUNTIME_DIR).
func checkPeer(conn net.Conn) error {
	uc, ok := conn.(*net.UnixConn)
	if !ok {
		return fmt.Errorf("not a unix connection")
	}
	raw, err := uc.SyscallConn()
	if err != nil {
		return err
	}
	var cred *syscall.Ucred
	var credErr error
	if err := raw.Control(func(fd uintptr) {
		cred, credErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	}); err != nil {
		return err
	}
	if credErr != nil {
		return credErr
	}
	if int(cred.Uid) != os.Getuid() {
		return fmt.Errorf("peer uid %d (pid %d) is not the daemon owner", cred.Uid, cred.Pid)
	}
	return nil
}
