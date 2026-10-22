package supervisor

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/omar0ali/watchd/internal/api"
	"github.com/omar0ali/watchd/internal/config"
)

const (
	// A run longer than this resets the crash counter used for backoff and
	// max_restarts.
	stableAfter = 10 * time.Second
	maxBackoff  = 30 * time.Second
)

// App supervises one configured app: it runs the command, restarts it per
// its policy or on request, and records its output.
type App struct {
	spec *config.App
	proj *Project
	logs *LogBuffer

	mu        sync.Mutex
	state     string
	health    string
	pid       int
	startedAt time.Time
	restarts  int
	exitCode  *int
	lastErr   string

	// Set while the supervision loop runs (between Start and Stop).
	cancel context.CancelFunc
	done   chan struct{}

	restartCh chan string // restart requests carrying a reason
}

func newApp(spec *config.App, proj *Project) *App {
	return &App{
		spec:      spec,
		proj:      proj,
		logs:      NewLogBuffer(),
		state:     api.StateStopped,
		restartCh: make(chan string, 1),
	}
}

func (a *App) Name() string { return a.spec.Name }

func (a *App) emit(stream, text string) {
	a.logs.Add(api.LogLine{
		Time:    time.Now(),
		Project: a.proj.Name(),
		App:     a.spec.Name,
		Stream:  stream,
		Text:    text,
	})
}

// sys records a lifecycle event in the app's log.
func (a *App) sys(format string, args ...any) {
	a.emit(api.StreamSystem, fmt.Sprintf(format, args...))
}

func (a *App) update(fn func()) {
	a.mu.Lock()
	fn()
	a.mu.Unlock()
}

func (a *App) setState(s string) { a.update(func() { a.state = s }) }

func (a *App) setHealth(h string) { a.update(func() { a.health = h }) }

// Active reports whether the app is being supervised (started and not
// stopped), whatever the state of its process.
func (a *App) Active() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cancel != nil
}

// Ready reports whether dependents may start: the process is running and,
// if the app has a healthcheck, it passed.
func (a *App) Ready() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.state == api.StateRunning && (a.spec.Health == nil || a.health == api.HealthHealthy)
}

// Start begins supervision. It returns false if already started.
func (a *App) Start() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancel != nil {
		return false
	}
	select { // drop a restart request left over from a previous run
	case <-a.restartCh:
	default:
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.done = make(chan struct{})
	a.lastErr = ""
	go a.loop(ctx, a.done)
	return true
}

// Stop ends supervision and waits for the process to exit. It returns false
// if the app was not started.
func (a *App) Stop() bool {
	a.mu.Lock()
	cancel, done := a.cancel, a.done
	a.cancel = nil
	a.mu.Unlock()
	if cancel == nil {
		return false
	}
	cancel()
	<-done
	return true
}

// Restart restarts the process now, or starts the app if it is stopped.
func (a *App) Restart(reason string) {
	if a.Start() {
		return
	}
	select {
	case a.restartCh <- reason:
	default: // a restart is already pending
	}
}

func (a *App) Status() api.AppStatus {
	a.mu.Lock()
	defer a.mu.Unlock()
	return api.AppStatus{
		Project:   a.proj.Name(),
		Name:      a.spec.Name,
		State:     a.state,
		Health:    a.health,
		PID:       a.pid,
		StartedAt: a.startedAt,
		Restarts:  a.restarts,
		ExitCode:  a.exitCode,
		LastError: a.lastErr,
	}
}

// environ is the CLI's environment, overlaid with the app's env and
// WATCHD_PROJECT / WATCHD_APP.
func (a *App) environ() []string {
	env := map[string]string{}
	for _, kv := range a.proj.baseEnv {
		if k, v, ok := strings.Cut(kv, "="); ok {
			env[k] = v
		}
	}
	maps.Copy(env, a.spec.Env)
	env["WATCHD_PROJECT"] = a.proj.Name()
	env["WATCHD_APP"] = a.spec.Name

	out := make([]string, 0, len(env))
	for _, k := range slices.Sorted(maps.Keys(env)) {
		out = append(out, k+"="+env[k])
	}
	return out
}

// loop is the supervision state machine. It runs until ctx is cancelled.
func (a *App) loop(ctx context.Context, done chan struct{}) {
	defer close(done)
	defer a.update(func() {
		a.state = api.StateStopped
		a.health = ""
		a.pid = 0
	})

	if !a.waitDeps(ctx) {
		return
	}

	crashes := 0 // consecutive short-lived runs, for backoff and max_restarts
	first := true
	for {
		if !first {
			a.update(func() { a.restarts++ })
		}
		first = false

		var res exitResult
		env := a.environ()
		stdout := &lineWriter{emit: func(s string) { a.emit(api.StreamStdout, s) }}
		stderr := &lineWriter{emit: func(s string) { a.emit(api.StreamStderr, s) }}
		p, err := startProc(a.spec.Cmd, a.spec.Cwd, env, stdout, stderr)

		if err != nil {
			a.sys("failed to start: %v", err)
			a.update(func() { a.lastErr = err.Error() })
			res = exitResult{err: err}
		} else {
			started := time.Now()
			pid := p.pid()
			a.update(func() {
				a.state = api.StateRunning
				a.pid = pid
				a.startedAt = started
				a.exitCode = nil
				a.health = ""
				if a.spec.Health != nil {
					a.health = api.HealthStarting
				}
			})
			a.sys("started (pid %d): %s", pid, a.spec.Cmd)
			go a.hook("on_start", func() error { return a.proj.spec.OnStart(a.spec, pid) })

			hctx, hcancel := context.WithCancel(ctx)
			if a.spec.Health != nil {
				go a.healthLoop(hctx, env)
			}

			select {
			case <-ctx.Done():
				hcancel()
				a.sys("stopping (%s)", signalName(a.spec.StopSignal))
				r, killed := p.terminate(a.spec.StopSignal, a.spec.StopTimeout)
				if killed {
					a.sys("did not stop within %s, killed", a.spec.StopTimeout)
				}
				a.sys("stopped (%s)", r)
				return

			case reason := <-a.restartCh:
				hcancel()
				a.sys("restarting: %s", reason)
				if _, killed := p.terminate(a.spec.StopSignal, a.spec.StopTimeout); killed {
					a.sys("did not stop within %s, killed", a.spec.StopTimeout)
				}
				crashes = 0
				continue

			case res = <-p.exited:
				hcancel()
				p.signalGroup(syscall.SIGKILL) // reap anything the command left behind
			}

			a.sys("exited (%s)", res)
			a.hook("on_exit", func() error { return a.proj.spec.OnExit(a.spec, pid, res.code) })
			if time.Since(started) >= stableAfter {
				crashes = 0
			}
		}

		code := res.code
		a.update(func() {
			a.pid = 0
			a.health = ""
			if res.err == nil {
				a.exitCode = &code
			}
		})

		restart := a.spec.Restart == config.RestartAlways ||
			(a.spec.Restart == config.RestartOnFailure && !res.success())
		gaveUp := false
		if restart {
			crashes++
			if a.spec.MaxRestarts > 0 && crashes > a.spec.MaxRestarts {
				restart, gaveUp = false, true
			}
		}

		if !restart {
			if gaveUp {
				a.sys("giving up after %d restarts in a row (max_restarts = %d)", crashes-1, a.spec.MaxRestarts)
			}
			a.update(func() {
				if res.success() {
					a.state = api.StateExited
				} else {
					a.state = api.StateFailed
					if a.lastErr == "" || res.err == nil {
						a.lastErr = res.String()
					}
				}
			})
			// Stay idle until stopped or asked to restart (e.g. a file
			// change after fixing the bug that made it crash).
			select {
			case <-ctx.Done():
				return
			case reason := <-a.restartCh:
				a.sys("restarting: %s", reason)
				crashes = 0
				continue
			}
		}

		delay := min(time.Second<<(crashes-1), maxBackoff)
		a.setState(api.StateBackoff)
		a.sys("restarting in %s (restart = %s)", delay, a.spec.Restart)
		select {
		case <-ctx.Done():
			return
		case reason := <-a.restartCh:
			a.sys("restarting: %s", reason)
		case <-time.After(delay):
		}
	}
}

// waitDeps blocks until every dependency is ready. It returns false if ctx
// is cancelled first.
func (a *App) waitDeps(ctx context.Context) bool {
	if len(a.spec.DependsOn) == 0 {
		return true
	}
	a.setState(api.StateWaiting)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	var lastWaiting string
	for {
		var waiting []string
		for _, d := range a.spec.DependsOn {
			if !a.proj.apps[d].Ready() {
				waiting = append(waiting, d)
			}
		}
		if len(waiting) == 0 {
			if lastWaiting != "" {
				a.sys("dependencies ready")
			}
			return true
		}
		if w := strings.Join(waiting, ", "); w != lastWaiting {
			a.sys("waiting for %s", w)
			lastWaiting = w
		}
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
		}
	}
}

func (a *App) hook(name string, fn func() error) {
	if err := fn(); err != nil {
		a.sys("%s hook failed: %v", name, err)
	}
}

// onFilesChanged is called by the app's watcher after a debounced batch.
func (a *App) onFilesChanged(files []string) {
	if !a.Active() {
		return
	}
	restart := true
	if a.spec.HasOnChange() {
		restart = false
		for _, f := range files {
			ok, err := a.proj.spec.OnChange(a.spec, f)
			if err != nil {
				a.sys("on_change hook failed: %v", err)
			}
			if ok {
				restart = true
			}
		}
	}
	desc := files[0]
	if len(files) > 1 {
		desc = fmt.Sprintf("%s and %d more", files[0], len(files)-1)
	}
	if !restart {
		a.sys("change in %s ignored by on_change", desc)
		return
	}
	a.Restart("changed " + desc)
}
