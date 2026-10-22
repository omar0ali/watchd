package supervisor

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/omar0ali/watchd/internal/api"
	"github.com/omar0ali/watchd/internal/config"
)

func newProject(t *testing.T, body string) (*Project, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, config.DefaultFile)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	spec, err := config.Load(path, config.Options{})
	if err != nil {
		t.Fatal(err)
	}
	p := New(spec, os.Environ(), log.New(io.Discard, "", 0))
	t.Cleanup(p.Down)
	return p, dir
}

func waitFor(t *testing.T, what string, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func status(p *Project, app string) api.AppStatus { return p.apps[app].Status() }

func logText(p *Project, app string) string {
	lines, _, _ := p.apps[app].logs.TailAndSubscribe(0, false)
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(l.Text + "\n")
	}
	return b.String()
}

func TestOutputAndCleanExit(t *testing.T) {
	p, _ := newProject(t, `return { apps = { { name = "a", cmd = "echo hello $WATCHD_APP; echo oops >&2", restart = "never" } } }`)
	if _, err := p.Up(nil); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "exit", 3*time.Second, func() bool { return status(p, "a").State == api.StateExited })

	s := status(p, "a")
	if s.ExitCode == nil || *s.ExitCode != 0 {
		t.Errorf("exit code = %v", s.ExitCode)
	}
	out := logText(p, "a")
	if !strings.Contains(out, "hello a") || !strings.Contains(out, "oops") {
		t.Errorf("logs = %q", out)
	}
}

func TestOnFailureGivesUpAfterMaxRestarts(t *testing.T) {
	p, _ := newProject(t, `return { apps = { { name = "a", cmd = "exit 3", restart = "on-failure", max_restarts = 1 } } }`)
	p.Up(nil)
	waitFor(t, "failed", 5*time.Second, func() bool { return status(p, "a").State == api.StateFailed })

	s := status(p, "a")
	if s.Restarts != 1 || s.ExitCode == nil || *s.ExitCode != 3 {
		t.Errorf("restarts = %d, exit = %v", s.Restarts, s.ExitCode)
	}
	if !strings.Contains(logText(p, "a"), "giving up") {
		t.Errorf("logs = %q", logText(p, "a"))
	}

	// A manual restart revives a failed app.
	p.Restart([]string{"a"})
	waitFor(t, "restart", 3*time.Second, func() bool { return status(p, "a").Restarts >= 2 })
}

func TestStopKillsWholeProcessGroup(t *testing.T) {
	p, dir := newProject(t, `return { apps = { { name = "a", cmd = "sleep 300 & echo $! > child.pid; wait" } } }`)
	p.Up(nil)

	pidFile := filepath.Join(dir, "child.pid")
	var child int
	waitFor(t, "child pid", 3*time.Second, func() bool {
		b, err := os.ReadFile(pidFile)
		if err != nil {
			return false
		}
		child, err = strconv.Atoi(strings.TrimSpace(string(b)))
		return err == nil
	})

	p.Stop(nil)
	if s := status(p, "a"); s.State != api.StateStopped || s.PID != 0 {
		t.Errorf("after stop: %+v", s)
	}
	waitFor(t, "child to die", 2*time.Second, func() bool {
		return syscall.Kill(child, 0) == syscall.ESRCH
	})
}

func TestStopEscalatesToSIGKILL(t *testing.T) {
	p, _ := newProject(t, `return { apps = { { name = "a", cmd = "trap '' TERM; while true; do sleep 0.1; done", stop_timeout = "300ms" } } }`)
	p.Up(nil)
	waitFor(t, "running", 3*time.Second, func() bool { return status(p, "a").State == api.StateRunning })
	time.Sleep(100 * time.Millisecond) // let the trap install

	start := time.Now()
	p.Stop(nil)
	if d := time.Since(start); d > 3*time.Second {
		t.Errorf("stop took %s", d)
	}
	if !strings.Contains(logText(p, "a"), "killed") {
		t.Errorf("logs = %q", logText(p, "a"))
	}
}

func TestDependsOnWaitsForHealthcheck(t *testing.T) {
	p, dir := newProject(t, `return { apps = {
		{ name = "db", cmd = "sleep 300", healthcheck = { cmd = "test -f ready", interval = "100ms" } },
		{ name = "api", cmd = "sleep 300", depends_on = { "db" } },
	} }`)
	// Starting api alone pulls in db.
	started, err := p.Up([]string{"api"})
	if err != nil || strings.Join(started, ",") != "db,api" {
		t.Fatalf("started = %v, err = %v", started, err)
	}

	waitFor(t, "db running", 3*time.Second, func() bool { return status(p, "db").State == api.StateRunning })
	time.Sleep(300 * time.Millisecond)
	if s := status(p, "api").State; s != api.StateWaiting {
		t.Fatalf("api state = %s, want waiting", s)
	}
	// After `retries` failed probes the app is reported unhealthy.
	if h := status(p, "db").Health; h == api.HealthHealthy {
		t.Fatalf("db health = %s before ready file exists", h)
	}

	if err := os.WriteFile(filepath.Join(dir, "ready"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "api running", 3*time.Second, func() bool { return status(p, "api").State == api.StateRunning })
	if h := status(p, "db").Health; h != api.HealthHealthy {
		t.Errorf("db health = %s", h)
	}
}

func TestFileChangeRestarts(t *testing.T) {
	p, dir := newProject(t, `return { apps = { {
		name = "a", cmd = "sleep 300", watch = { "*.txt" }, debounce = "50ms",
		on_change = function(file) return file ~= "skip.txt" end,
	} } }`)
	p.StartWatchers()
	p.Up(nil)
	waitFor(t, "running", 3*time.Second, func() bool { return status(p, "a").State == api.StateRunning })
	firstPID := status(p, "a").PID

	os.WriteFile(filepath.Join(dir, "skip.txt"), []byte("x"), 0o644)
	time.Sleep(400 * time.Millisecond)
	if s := status(p, "a"); s.Restarts != 0 {
		t.Fatalf("skip.txt caused a restart")
	}

	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644)
	waitFor(t, "restart", 3*time.Second, func() bool {
		s := status(p, "a")
		return s.Restarts == 1 && s.State == api.StateRunning && s.PID != firstPID
	})
	if !strings.Contains(logText(p, "a"), "changed notes.txt") {
		t.Errorf("logs = %q", logText(p, "a"))
	}
}

func TestLogsFollowEndsOnDown(t *testing.T) {
	p, _ := newProject(t, `return { apps = { { name = "a", cmd = "while true; do echo tick; sleep 0.05; done" } } }`)
	p.Up(nil)

	got := make(chan string, 100)
	done := make(chan error, 1)
	go func() {
		done <- p.Logs(context.Background(), nil, 0, true, func(l api.LogLine) error {
			got <- l.Text
			return nil
		})
	}()
	waitFor(t, "tick", 3*time.Second, func() bool {
		for {
			select {
			case s := <-got:
				if s == "tick" {
					return true
				}
			default:
				return false
			}
		}
	})
	go func() {
		for range got {
		}
	}()
	p.Down()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("logs: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("logs -f did not end after Down")
	}
}

func TestLineWriter(t *testing.T) {
	var lines []string
	w := &lineWriter{emit: func(s string) { lines = append(lines, s) }}
	w.Write([]byte("a\r\nb"))
	w.Write([]byte("c\nd"))
	w.Flush()
	if strings.Join(lines, "|") != "a|bc|d" {
		t.Errorf("lines = %q", lines)
	}
}
