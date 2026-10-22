package supervisor

import (
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"time"
)

// waitDelay bounds how long Wait keeps reading output after the process
// exits, in case a grandchild still holds the pipes open.
const waitDelay = 2 * time.Second

type exitResult struct {
	code   int
	signal syscall.Signal
	err    error // set when the result is not a normal exit
}

func (r exitResult) success() bool { return r.err == nil && r.signal == 0 && r.code == 0 }

func (r exitResult) String() string {
	switch {
	case r.err != nil:
		return r.err.Error()
	case r.signal != 0:
		return "killed by " + signalName(r.signal)
	default:
		return fmt.Sprintf("exit code %d", r.code)
	}
}

func signalName(s syscall.Signal) string {
	for name, sig := range map[string]syscall.Signal{
		"SIGTERM": syscall.SIGTERM, "SIGINT": syscall.SIGINT, "SIGKILL": syscall.SIGKILL,
		"SIGQUIT": syscall.SIGQUIT, "SIGHUP": syscall.SIGHUP, "SIGSEGV": syscall.SIGSEGV,
		"SIGABRT": syscall.SIGABRT, "SIGUSR1": syscall.SIGUSR1, "SIGUSR2": syscall.SIGUSR2,
	} {
		if sig == s {
			return name
		}
	}
	return s.String()
}

// proc is one running instance of an app. The command runs through sh in
// its own process group, so signals reach everything it spawned (go run,
// npm and friends start child processes that would otherwise be orphaned).
type proc struct {
	cmd    *exec.Cmd
	pgid   int
	exited chan exitResult // receives exactly once
}

func startProc(command, dir string, env []string, stdout, stderr *lineWriter) (*proc, error) {
	cmd := exec.Command("/bin/sh", "-c", command)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = waitDelay
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	p := &proc{cmd: cmd, pgid: cmd.Process.Pid, exited: make(chan exitResult, 1)}
	go func() {
		err := cmd.Wait()
		stdout.Flush()
		stderr.Flush()
		var res exitResult
		if ps := cmd.ProcessState; ps != nil {
			if ws, ok := ps.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
				res.signal = ws.Signal()
			} else {
				res.code = ps.ExitCode()
			}
		} else {
			res.err = err
		}
		p.exited <- res
	}()
	return p, nil
}

func (p *proc) pid() int { return p.cmd.Process.Pid }

// signalGroup sends sig to the whole process group.
func (p *proc) signalGroup(sig syscall.Signal) {
	if err := syscall.Kill(-p.pgid, sig); err != nil && !errors.Is(err, syscall.ESRCH) {
		_ = p.cmd.Process.Signal(sig)
	}
}

// terminate sends sig to the group, waits up to timeout, then SIGKILLs.
func (p *proc) terminate(sig syscall.Signal, timeout time.Duration) (exitResult, bool) {
	p.signalGroup(sig)
	select {
	case res := <-p.exited:
		p.signalGroup(syscall.SIGKILL) // leftovers that ignored the signal
		return res, false
	case <-time.After(timeout):
	}
	p.signalGroup(syscall.SIGKILL)
	return <-p.exited, true
}
