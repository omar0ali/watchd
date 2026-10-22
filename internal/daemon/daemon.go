// Package daemon is the long-running watchd process. It owns every loaded
// project and serves requests from the CLI over the control socket.
package daemon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/omar0ali/watchd/internal/api"
	"github.com/omar0ali/watchd/internal/config"
	"github.com/omar0ali/watchd/internal/ipc"
	"github.com/omar0ali/watchd/internal/paths"
	"github.com/omar0ali/watchd/internal/supervisor"
	"github.com/omar0ali/watchd/internal/version"
)

// ErrAlreadyRunning is returned by Run when another daemon holds the lock.
var ErrAlreadyRunning = errors.New("the watchd daemon is already running")

type Daemon struct {
	log *log.Logger

	mu       sync.Mutex
	projects map[string]*supervisor.Project

	quit     chan struct{}
	quitOnce sync.Once
}

// Run starts the daemon and blocks until it is shut down. In foreground
// mode the log also goes to stderr (for running under systemd or by hand).
func Run(foreground bool) error {
	lockPath, err := paths.Lock()
	if err != nil {
		return err
	}
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return ErrAlreadyRunning
		}
		return fmt.Errorf("lock %s: %w", lockPath, err)
	}

	logPath, err := paths.DaemonLog()
	if err != nil {
		return err
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer logFile.Close()
	var out io.Writer = logFile
	if foreground {
		out = io.MultiWriter(logFile, os.Stderr)
	}
	logger := log.New(out, "", log.LstdFlags)

	if os.Geteuid() == 0 {
		logger.Println("warning: running as root; every app will run as root too")
	}
	if !foreground {
		_ = os.Chdir("/") // don't keep the launching directory busy
	}

	sock, err := paths.Socket()
	if err != nil {
		return err
	}
	srv, err := ipc.Listen(sock, logger)
	if err != nil {
		return err
	}
	defer os.Remove(sock)

	d := &Daemon{
		log:      logger,
		projects: map[string]*supervisor.Project{},
		quit:     make(chan struct{}),
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		s := <-sigs
		logger.Printf("received %s, shutting down", s)
		d.stop()
	}()

	logger.Printf("watchd %s started (pid %d), listening on %s", version.Version, os.Getpid(), sock)
	go func() {
		if err := srv.Serve(d.handle); err != nil {
			logger.Println("serve:", err)
		}
	}()

	<-d.quit
	srv.Close()
	d.downAll()
	logger.Println("daemon stopped")
	return nil
}

func (d *Daemon) stop() { d.quitOnce.Do(func() { close(d.quit) }) }

// downAll stops every project in parallel and returns their names.
func (d *Daemon) downAll() []string {
	d.mu.Lock()
	projects := d.projects
	d.projects = map[string]*supervisor.Project{}
	d.mu.Unlock()

	var wg sync.WaitGroup
	var names []string
	for name, p := range projects {
		names = append(names, name)
		wg.Go(p.Down)
	}
	wg.Wait()
	slices.Sort(names)
	return names
}

func (d *Daemon) handle(ctx context.Context, req *api.Request, w *ipc.Writer) error {
	text := func(format string, args ...any) error {
		return w.Send(api.Message{Text: fmt.Sprintf(format, args...)})
	}

	switch req.Cmd {
	case api.CmdPing:
		d.mu.Lock()
		n := len(d.projects)
		d.mu.Unlock()
		return text("watchd %s (pid %d), %d project(s) loaded", version.Version, os.Getpid(), n)

	case api.CmdUp:
		return d.up(req, text)

	case api.CmdReload:
		return d.reload(req, text)

	case api.CmdDown:
		return d.down(req, text)

	case api.CmdStart, api.CmdStop, api.CmdRestart:
		return d.appOp(req, text)

	case api.CmdPS:
		apps, err := d.ps(req)
		if err != nil {
			return err
		}
		return w.Send(api.Message{Apps: apps})

	case api.CmdList:
		d.mu.Lock()
		var list []api.ProjectStatus
		for _, p := range d.projects {
			list = append(list, p.Summary())
		}
		d.mu.Unlock()
		slices.SortFunc(list, func(a, b api.ProjectStatus) int { return strings.Compare(a.Name, b.Name) })
		return w.Send(api.Message{Projects: list})

	case api.CmdLogs:
		targets, err := d.targets(req)
		if err != nil {
			return err
		}
		if len(targets) != 1 {
			return errors.New("logs can only show one project at a time")
		}
		t := targets[0]
		return t.proj.Logs(ctx, t.apps, req.Lines, req.Follow, func(l api.LogLine) error {
			return w.Send(api.Message{Log: &l})
		})

	case api.CmdShutdown:
		names := d.downAll()
		if err := text("stopped %d project(s)%s; daemon exiting", len(names), listSuffix(names)); err != nil {
			return err
		}
		time.AfterFunc(100*time.Millisecond, d.stop) // let the reply go out first
		return nil

	default:
		return fmt.Errorf("unknown command %q", req.Cmd)
	}
}

func listSuffix(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return ": " + strings.Join(names, ", ")
}

// loadedNames returns the sorted project names. d.mu must be held.
func (d *Daemon) loadedNames() []string {
	var names []string
	for n := range d.projects {
		names = append(names, n)
	}
	slices.Sort(names)
	return names
}

func (d *Daemon) byPath(path string) *supervisor.Project {
	for _, p := range d.projects {
		if p.ConfigPath() == path {
			return p
		}
	}
	return nil
}

// find picks the project a request refers to: by name, by config path, or
// the only loaded project.
func (d *Daemon) find(req *api.Request) (*supervisor.Project, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.findLocked(req)
}

func (d *Daemon) findLocked(req *api.Request) (*supervisor.Project, error) {
	if req.Project != "" {
		if p, ok := d.projects[req.Project]; ok {
			return p, nil
		}
		return nil, fmt.Errorf("no project named %q is loaded%s", req.Project, d.loadedHint())
	}
	if req.ConfigPath != "" {
		if p := d.byPath(req.ConfigPath); p != nil {
			return p, nil
		}
		return nil, fmt.Errorf("the project in %s is not loaded (start it with: watchd up)%s", req.ConfigPath, d.loadedHint())
	}
	switch len(d.projects) {
	case 0:
		return nil, errors.New("no projects are loaded (start one with: watchd up)")
	case 1:
		for _, p := range d.projects {
			return p, nil
		}
	}
	return nil, fmt.Errorf("several projects are loaded (%s); choose one with -p NAME, use PROJECT/APP, or run from a project directory",
		strings.Join(d.loadedNames(), ", "))
}

func (d *Daemon) loadedHint() string {
	if len(d.projects) == 0 {
		return ""
	}
	return " (loaded: " + strings.Join(d.loadedNames(), ", ") + ")"
}

type target struct {
	proj *supervisor.Project
	apps []string
}

// targets groups req.Apps by project. "app" refers to the request's
// project (see find); "project/app" names it explicitly.
func (d *Daemon) targets(req *api.Request) ([]target, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if len(req.Apps) == 0 {
		p, err := d.findLocked(req)
		if err != nil {
			return nil, err
		}
		return []target{{proj: p}}, nil
	}

	var out []target
	index := map[*supervisor.Project]int{}
	for _, entry := range req.Apps {
		var p *supervisor.Project
		app := entry
		if projName, a, ok := strings.Cut(entry, "/"); ok {
			var found bool
			if p, found = d.projects[projName]; !found {
				return nil, fmt.Errorf("%s: no project named %q is loaded%s", entry, projName, d.loadedHint())
			}
			app = a
		} else {
			var err error
			if p, err = d.findLocked(req); err != nil {
				return nil, err
			}
		}
		i, ok := index[p]
		if !ok {
			i = len(out)
			index[p] = i
			out = append(out, target{proj: p})
		}
		out[i].apps = append(out[i].apps, app)
	}
	return out, nil
}

func (d *Daemon) load(path string, env []string) (*config.Project, error) {
	return config.Load(path, config.Options{
		Env: env,
		Print: func(app, msg string) {
			d.log.Printf("[%s] %s", path, msg)
		},
	})
}

func (d *Daemon) up(req *api.Request, text func(string, ...any) error) error {
	if req.ConfigPath == "" {
		return errors.New("up needs a config file path")
	}

	d.mu.Lock()
	existing := d.byPath(req.ConfigPath)
	d.mu.Unlock()
	if existing != nil {
		return startedReply(existing, req.Apps, text)
	}

	spec, err := d.load(req.ConfigPath, req.Env)
	if err != nil {
		return err
	}
	proj := supervisor.New(spec, req.Env, d.log)

	d.mu.Lock()
	if p := d.byPath(req.ConfigPath); p != nil {
		// Loaded concurrently by another `up`; use that one.
		d.mu.Unlock()
		spec.Close()
		return startedReply(p, req.Apps, text)
	}
	if other, taken := d.projects[spec.Name]; taken {
		d.mu.Unlock()
		spec.Close()
		return fmt.Errorf("a project named %q is already loaded from %s; set a different `name` in one of the configs, or run: watchd down %s",
			spec.Name, other.ConfigPath(), spec.Name)
	}
	d.projects[spec.Name] = proj
	d.mu.Unlock()

	d.log.Printf("loaded project %s from %s", spec.Name, spec.Path)
	proj.StartWatchers()
	if err := startedReply(proj, req.Apps, text); err != nil {
		d.mu.Lock()
		delete(d.projects, spec.Name)
		d.mu.Unlock()
		proj.Down()
		return err
	}
	return nil
}

func startedReply(p *supervisor.Project, apps []string, text func(string, ...any) error) error {
	started, err := p.Up(apps)
	if err != nil {
		return err
	}
	if len(started) == 0 {
		return text("%s: already running", p.Name())
	}
	return text("%s: started %s", p.Name(), strings.Join(started, ", "))
}

func (d *Daemon) reload(req *api.Request, text func(string, ...any) error) error {
	old, err := d.find(req)
	if err != nil {
		return err
	}

	// Parse first: a broken config must not take down the running project.
	spec, err := d.load(old.ConfigPath(), req.Env)
	if err != nil {
		return fmt.Errorf("config has errors, keeping the running version:\n%w", err)
	}

	d.mu.Lock()
	if other, taken := d.projects[spec.Name]; taken && other != old {
		d.mu.Unlock()
		spec.Close()
		return fmt.Errorf("the new name %q is already used by the project in %s", spec.Name, other.ConfigPath())
	}
	delete(d.projects, old.Name())
	proj := supervisor.New(spec, req.Env, d.log)
	d.projects[spec.Name] = proj
	d.mu.Unlock()

	// Bring back the apps that were running, plus new autostart apps.
	var restart []string
	wasDefined := map[string]bool{}
	for _, s := range old.Status() {
		wasDefined[s.Name] = true
		if s.State != api.StateStopped && spec.App(s.Name) != nil {
			restart = append(restart, s.Name)
		}
	}
	for _, a := range spec.Apps {
		if !wasDefined[a.Name] && a.Autostart {
			restart = append(restart, a.Name)
		}
	}

	old.Down()
	proj.StartWatchers()
	if len(restart) == 0 {
		return text("%s: reloaded (no apps to start)", proj.Name())
	}
	if _, err := proj.Up(restart); err != nil {
		return err
	}
	return text("%s: reloaded, started %s", proj.Name(), strings.Join(restart, ", "))
}

func (d *Daemon) down(req *api.Request, text func(string, ...any) error) error {
	var projects []*supervisor.Project
	d.mu.Lock()
	switch {
	case req.All:
		for _, n := range d.loadedNames() {
			projects = append(projects, d.projects[n])
		}
	case len(req.Apps) > 0: // project names
		for _, n := range req.Apps {
			p, ok := d.projects[n]
			if !ok {
				hint := d.loadedHint()
				d.mu.Unlock()
				return fmt.Errorf("no project named %q is loaded%s", n, hint)
			}
			projects = append(projects, p)
		}
	default:
		p, err := d.findLocked(req)
		if err != nil {
			d.mu.Unlock()
			return err
		}
		projects = append(projects, p)
	}
	for _, p := range projects {
		delete(d.projects, p.Name())
	}
	d.mu.Unlock()

	if len(projects) == 0 {
		return text("no projects are loaded")
	}
	var wg sync.WaitGroup
	for _, p := range projects {
		wg.Go(p.Down)
	}
	wg.Wait()
	for _, p := range projects {
		d.log.Printf("unloaded project %s", p.Name())
		if err := text("%s: stopped and unloaded", p.Name()); err != nil {
			return err
		}
	}
	return nil
}

func (d *Daemon) appOp(req *api.Request, text func(string, ...any) error) error {
	targets, err := d.targets(req)
	if err != nil {
		return err
	}
	for _, t := range targets {
		var done []string
		var verb string
		switch req.Cmd {
		case api.CmdStart:
			verb = "started"
			done, err = t.proj.Up(t.apps)
		case api.CmdStop:
			verb = "stopped"
			done, err = t.proj.Stop(t.apps)
		case api.CmdRestart:
			verb = "restarting"
			done, err = t.proj.Restart(t.apps)
		}
		if err != nil {
			return err
		}
		if len(done) == 0 {
			if err := text("%s: nothing to do", t.proj.Name()); err != nil {
				return err
			}
			continue
		}
		if err := text("%s: %s %s", t.proj.Name(), verb, strings.Join(done, ", ")); err != nil {
			return err
		}
	}
	return nil
}

// ps returns app statuses for the requested project, or for every project
// when the request names none (or the named directory isn't loaded).
func (d *Daemon) ps(req *api.Request) ([]api.AppStatus, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	var projects []*supervisor.Project
	switch {
	case req.Project != "":
		p, ok := d.projects[req.Project]
		if !ok {
			return nil, fmt.Errorf("no project named %q is loaded%s", req.Project, d.loadedHint())
		}
		projects = append(projects, p)
	case !req.All && req.ConfigPath != "" && d.byPath(req.ConfigPath) != nil:
		projects = append(projects, d.byPath(req.ConfigPath))
	default:
		for _, n := range d.loadedNames() {
			projects = append(projects, d.projects[n])
		}
	}
	var out []api.AppStatus
	for _, p := range projects {
		out = append(out, p.Status()...)
	}
	return out, nil
}
