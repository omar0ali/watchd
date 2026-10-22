package supervisor

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/omar0ali/watchd/internal/api"
	"github.com/omar0ali/watchd/internal/config"
)

// healthLoop probes the app until ctx is cancelled (the process exited or
// is being stopped). The app is "healthy" after the first passing probe and
// "unhealthy" after Retries consecutive failures.
func (a *App) healthLoop(ctx context.Context, env []string) {
	hc := a.spec.Health
	// Probe soon after start so fast apps don't wait a full interval.
	timer := time.NewTimer(min(500*time.Millisecond, hc.Interval))
	defer timer.Stop()

	fails := 0
	healthy := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			timer.Reset(hc.Interval)
		}

		err := probe(ctx, hc, a.spec.Cwd, env)
		if ctx.Err() != nil {
			return
		}
		if err == nil {
			fails = 0
			if !healthy {
				healthy = true
				a.setHealth(api.HealthHealthy)
				a.sys("healthy (%s)", hc)
			}
			continue
		}
		fails++
		if fails == hc.Retries {
			a.setHealth(api.HealthUnhealthy)
			a.sys("unhealthy: %v", err)
			healthy = false
		}
	}
}

func probe(ctx context.Context, hc *config.HealthCheck, dir string, env []string) error {
	ctx, cancel := context.WithTimeout(ctx, hc.Timeout)
	defer cancel()

	switch {
	case hc.Port != 0:
		var d net.Dialer
		conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(hc.Host, strconv.Itoa(hc.Port)))
		if err != nil {
			return err
		}
		return conn.Close()

	case hc.HTTP != "":
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, hc.HTTP, nil)
		if err != nil {
			return err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		resp.Body.Close()
		if resp.StatusCode >= 400 {
			return fmt.Errorf("GET %s: %s", hc.HTTP, resp.Status)
		}
		return nil

	default:
		cmd := exec.CommandContext(ctx, "/bin/sh", "-c", hc.Cmd)
		cmd.Dir = dir
		cmd.Env = env
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
		cmd.WaitDelay = time.Second
		out, err := cmd.CombinedOutput()
		if err != nil {
			msg := strings.TrimSpace(string(out))
			if len(msg) > 200 {
				msg = msg[:200] + "..."
			}
			if msg != "" {
				return fmt.Errorf("%v: %s", err, msg)
			}
			return err
		}
		return nil
	}
}
