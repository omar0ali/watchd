// Package paths resolves where watchd keeps its socket, lock and logs, and
// makes sure those directories are private to the current user.
package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// RuntimeDir holds the control socket and the daemon lock. It is
// $WATCHD_RUNTIME_DIR, else $XDG_RUNTIME_DIR/watchd, else /tmp/watchd-<uid>.
func RuntimeDir() (string, error) {
	dir := os.Getenv("WATCHD_RUNTIME_DIR")
	if dir == "" {
		if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
			dir = filepath.Join(xdg, "watchd")
		} else {
			dir = filepath.Join(os.TempDir(), fmt.Sprintf("watchd-%d", os.Getuid()))
		}
	}
	return dir, EnsurePrivateDir(dir)
}

// StateDir holds the daemon log. It is $XDG_STATE_HOME/watchd, else
// ~/.local/state/watchd.
func StateDir() (string, error) {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "state")
	}
	dir := filepath.Join(base, "watchd")
	return dir, EnsurePrivateDir(dir)
}

// maxSocketPath is the portable limit for a Unix socket path (sun_path is
// 104 bytes on macOS/BSD, 108 on Linux, including the terminator).
const maxSocketPath = 103

func Socket() (string, error) {
	dir, err := RuntimeDir()
	if err != nil {
		return "", err
	}
	p := filepath.Join(dir, "watchd.sock")
	if len(p) > maxSocketPath {
		return "", fmt.Errorf("socket path %s is too long for a Unix socket (%d > %d bytes); set WATCHD_RUNTIME_DIR to a shorter directory", p, len(p), maxSocketPath)
	}
	return p, nil
}

func Lock() (string, error) {
	dir, err := RuntimeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "watchd.lock"), nil
}

func DaemonLog() (string, error) {
	dir, err := StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "daemon.log"), nil
}

// EnsurePrivateDir creates dir with mode 0700 and verifies that it is a real
// directory (not a symlink) owned by the current user. This matters most for
// the /tmp fallback, where another user could pre-create the path to
// intercept the socket.
func EnsurePrivateDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	fi, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return fmt.Errorf("%s: not a directory (refusing to use it)", dir)
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok && int(st.Uid) != os.Getuid() {
		return fmt.Errorf("%s: owned by uid %d, not you (refusing to use it)", dir, st.Uid)
	}
	if fi.Mode().Perm()&0o077 != 0 {
		if err := os.Chmod(dir, 0o700); err != nil {
			return fmt.Errorf("%s: cannot make private: %w", dir, err)
		}
	}
	return nil
}

// CheckNotWorldWritable rejects files that any user on the system could
// modify. watchd executes what config files say, so such a file would let
// other users run commands as you.
func CheckNotWorldWritable(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	if fi.Mode().Perm()&0o002 != 0 {
		return fmt.Errorf("%s is world-writable; refusing to load it (fix with: chmod o-w %q)", path, path)
	}
	return nil
}
