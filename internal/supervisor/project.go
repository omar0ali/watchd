// Package supervisor runs and monitors the apps of loaded projects.
package supervisor

import (
	"context"
	"fmt"
	"log"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/omar0ali/watchd/internal/api"
	"github.com/omar0ali/watchd/internal/config"
	"github.com/omar0ali/watchd/internal/watcher"
)

// Project is a loaded config and its supervised apps.
type Project struct {
	spec     *config.Project
	baseEnv  []string
	loadedAt time.Time
	log      *log.Logger

	apps  map[string]*App
	order []*App // dependencies first

	mu       sync.Mutex
	watchers []*watcher.Watcher
}

// New creates a project from a loaded config. baseEnv is the environment
// apps inherit (the CLI's environment at `watchd up`).
func New(spec *config.Project, baseEnv []string, logger *log.Logger) *Project {
	p := &Project{
		spec:     spec,
		baseEnv:  baseEnv,
		loadedAt: time.Now(),
		log:      logger,
		apps:     map[string]*App{},
	}
	for _, s := range spec.Apps {
		p.apps[s.Name] = newApp(s, p)
	}
	for _, name := range spec.Order {
		p.order = append(p.order, p.apps[name])
	}
	spec.SetPrint(func(app, msg string) {
		if a, ok := p.apps[app]; ok {
			a.emit(api.StreamSystem, msg)
		} else {
			p.log.Printf("[%s] %s", spec.Name, msg)
		}
	})
	return p
}

func (p *Project) Name() string       { return p.spec.Name }
func (p *Project) ConfigPath() string { return p.spec.Path }

// StartWatchers begins watching files for apps that define `watch`.
// Failures are logged to the app rather than failing the project.
func (p *Project) StartWatchers() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, a := range p.order {
		if len(a.spec.Watch) == 0 {
			continue
		}
		w, err := watcher.New(a.spec.Cwd, a.spec.Watch, a.spec.Ignore, a.spec.Debounce,
			a.onFilesChanged,
			func(err error) { a.sys("watch error: %v", err) })
		if err != nil {
			a.sys("file watching disabled: %v", err)
			continue
		}
		p.watchers = append(p.watchers, w)
	}
}

// resolve maps names to apps, in dependency order. With withDeps, the
// dependencies of each named app are included too.
func (p *Project) resolve(names []string, withDeps bool) ([]*App, error) {
	want := map[string]bool{}
	var add func(n string)
	add = func(n string) {
		if want[n] {
			return
		}
		want[n] = true
		if withDeps {
			for _, d := range p.apps[n].spec.DependsOn {
				add(d)
			}
		}
	}
	for _, n := range names {
		if _, ok := p.apps[n]; !ok {
			return nil, fmt.Errorf("project %q has no app %q (apps: %s)", p.Name(), n, strings.Join(p.appNames(), ", "))
		}
		add(n)
	}
	var out []*App
	for _, a := range p.order {
		if want[a.Name()] {
			out = append(out, a)
		}
	}
	return out, nil
}

func (p *Project) appNames() []string {
	names := make([]string, 0, len(p.spec.Apps))
	for _, a := range p.spec.Apps {
		names = append(names, a.Name)
	}
	return names
}

// Up starts the named apps and their dependencies. With no names it starts
// every app with autostart = true. It returns the apps that were started.
func (p *Project) Up(names []string) ([]string, error) {
	if len(names) == 0 {
		for _, a := range p.spec.Apps {
			if a.Autostart {
				names = append(names, a.Name)
			}
		}
	}
	apps, err := p.resolve(names, true)
	if err != nil {
		return nil, err
	}
	var started []string
	for _, a := range apps {
		if a.Start() {
			started = append(started, a.Name())
		}
	}
	return started, nil
}

// Stop stops the named apps (all if none), dependents before their
// dependencies. It returns the apps that were stopped.
func (p *Project) Stop(names []string) ([]string, error) {
	if len(names) == 0 {
		names = p.appNames()
	}
	apps, err := p.resolve(names, false)
	if err != nil {
		return nil, err
	}
	slices.Reverse(apps)
	var stopped []string
	for _, a := range apps {
		if a.Stop() {
			stopped = append(stopped, a.Name())
		}
	}
	return stopped, nil
}

// Restart restarts the named apps (all running apps if none). Stopped apps
// that are named explicitly are started.
func (p *Project) Restart(names []string) ([]string, error) {
	explicit := len(names) > 0
	if !explicit {
		names = p.appNames()
	}
	apps, err := p.resolve(names, false)
	if err != nil {
		return nil, err
	}
	var restarted []string
	for _, a := range apps {
		if !explicit && !a.Active() {
			continue
		}
		a.Restart("requested by user")
		restarted = append(restarted, a.Name())
	}
	return restarted, nil
}

// Down stops every app, stops watching and releases the Lua state. The
// project cannot be used afterwards.
func (p *Project) Down() {
	p.mu.Lock()
	for _, w := range p.watchers {
		w.Close()
	}
	p.watchers = nil
	p.mu.Unlock()

	_, _ = p.Stop(nil)
	for _, a := range p.order {
		a.logs.Close()
	}
	p.spec.Close()
}

// Status returns app statuses in config order.
func (p *Project) Status() []api.AppStatus {
	out := make([]api.AppStatus, 0, len(p.spec.Apps))
	for _, s := range p.spec.Apps {
		out = append(out, p.apps[s.Name].Status())
	}
	return out
}

func (p *Project) Summary() api.ProjectStatus {
	ps := api.ProjectStatus{
		Name:       p.Name(),
		ConfigPath: p.ConfigPath(),
		LoadedAt:   p.loadedAt,
		Apps:       len(p.apps),
	}
	for _, a := range p.apps {
		if a.Status().State == api.StateRunning {
			ps.Running++
		}
	}
	return ps
}

// Logs sends the last n lines of the named apps (all if none), merged by
// time. With follow it keeps sending new lines until ctx is cancelled or the
// project goes down.
func (p *Project) Logs(ctx context.Context, names []string, n int, follow bool, send func(api.LogLine) error) error {
	if len(names) == 0 {
		names = p.appNames()
	}
	apps, err := p.resolve(names, false)
	if err != nil {
		return err
	}

	var tail []api.LogLine
	var subs []<-chan api.LogLine
	for _, a := range apps {
		lines, ch, cancel := a.logs.TailAndSubscribe(n, follow)
		defer cancel()
		tail = append(tail, lines...)
		if ch != nil {
			subs = append(subs, ch)
		}
	}
	slices.SortStableFunc(tail, func(x, y api.LogLine) int { return x.Time.Compare(y.Time) })
	if n > 0 && len(tail) > n {
		tail = tail[len(tail)-n:]
	}
	for _, l := range tail {
		if err := send(l); err != nil {
			return err
		}
	}
	if !follow {
		return nil
	}

	merged := make(chan api.LogLine)
	var wg sync.WaitGroup
	for _, ch := range subs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for l := range ch {
				select {
				case merged <- l:
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	go func() {
		wg.Wait()
		close(merged)
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case l, ok := <-merged:
			if !ok {
				return nil // project went down
			}
			if err := send(l); err != nil {
				return err
			}
		}
	}
}
