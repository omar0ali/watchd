// Package config loads and validates watchd.lua project files.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

// DefaultFile is the config file name looked up by the CLI.
const DefaultFile = "watchd.lua"

type RestartPolicy string

const (
	RestartAlways    RestartPolicy = "always"
	RestartOnFailure RestartPolicy = "on-failure"
	RestartNever     RestartPolicy = "never"
)

// Defaults applied when a field is omitted.
const (
	DefaultRestart      = RestartOnFailure
	DefaultDebounce     = 300 * time.Millisecond
	DefaultStopTimeout  = 10 * time.Second
	DefaultHCInterval   = 2 * time.Second
	DefaultHCTimeout    = 1 * time.Second
	DefaultHCRetries    = 3
	DefaultHCHost       = "127.0.0.1"
	DefaultLoadTimeout  = 5 * time.Second
	DefaultHookTimeout  = 2 * time.Second
	DefaultMaxRestarts  = 0 // unlimited
	DefaultStopSignalNm = "SIGTERM"
)

type Project struct {
	Name string
	Path string // absolute path of the config file
	Dir  string // directory of the config file; relative paths resolve here
	Apps []*App

	// Order lists app names so that every app comes after its dependencies.
	Order []string

	rt *runtime
}

type App struct {
	Name        string
	Cmd         string
	Cwd         string            // absolute
	Env         map[string]string // project env + app env, env files applied
	DependsOn   []string
	Watch       []string
	Ignore      []string
	Debounce    time.Duration
	Restart     RestartPolicy
	MaxRestarts int
	StopSignal  syscall.Signal
	StopTimeout time.Duration
	Health      *HealthCheck
	Autostart   bool

	hooks hooks
}

// HealthCheck decides when an app is ready. Exactly one of Port, HTTP or
// Cmd is set.
type HealthCheck struct {
	Port     int
	Host     string
	HTTP     string
	Cmd      string
	Interval time.Duration
	Timeout  time.Duration
	Retries  int
}

func (h *HealthCheck) String() string {
	switch {
	case h.Port != 0:
		return fmt.Sprintf("tcp %s:%d", h.Host, h.Port)
	case h.HTTP != "":
		return "http " + h.HTTP
	default:
		return "cmd " + h.Cmd
	}
}

// App returns the app with the given name, or nil.
func (p *Project) App(name string) *App {
	for _, a := range p.Apps {
		if a.Name == name {
			return a
		}
	}
	return nil
}

// Find looks for DefaultFile in dir and its parents, like git does for .git.
func Find(dir string) (string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		p := filepath.Join(dir, DefaultFile)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no %s found in this directory or any parent (create one with: watchd init)", DefaultFile)
		}
		dir = parent
	}
}

var nameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

func validName(s string) bool { return nameRe.MatchString(s) }

// errList collects validation errors so users see all problems at once.
type errList []string

func (e *errList) add(format string, args ...any) { *e = append(*e, fmt.Sprintf(format, args...)) }

func (e errList) err(path string) error {
	if len(e) == 0 {
		return nil
	}
	return fmt.Errorf("%s:\n  - %s", path, strings.Join(e, "\n  - "))
}

// resolvePath makes p absolute relative to base, expanding a leading ~.
func resolvePath(base, p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, p[1:])
		}
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(base, p)
	}
	return filepath.Clean(p)
}

var signals = map[string]syscall.Signal{
	"SIGTERM": syscall.SIGTERM,
	"SIGINT":  syscall.SIGINT,
	"SIGQUIT": syscall.SIGQUIT,
	"SIGHUP":  syscall.SIGHUP,
	"SIGKILL": syscall.SIGKILL,
	"SIGUSR1": syscall.SIGUSR1,
	"SIGUSR2": syscall.SIGUSR2,
}

func parseSignal(s string) (syscall.Signal, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if !strings.HasPrefix(s, "SIG") {
		s = "SIG" + s
	}
	if sig, ok := signals[s]; ok {
		return sig, nil
	}
	return 0, fmt.Errorf("unknown signal %q", s)
}

// sortByDeps returns app names ordered so dependencies come first, keeping
// definition order otherwise. It reports unknown dependencies and cycles.
func sortByDeps(apps []*App) ([]string, error) {
	byName := map[string]*App{}
	for _, a := range apps {
		byName[a.Name] = a
	}
	for _, a := range apps {
		for _, d := range a.DependsOn {
			if _, ok := byName[d]; !ok {
				return nil, fmt.Errorf("app %q depends on unknown app %q", a.Name, d)
			}
			if d == a.Name {
				return nil, fmt.Errorf("app %q depends on itself", a.Name)
			}
		}
	}

	const (
		unvisited = iota
		visiting
		done
	)
	state := map[string]int{}
	var order, stack []string
	var visit func(n string) error
	visit = func(n string) error {
		switch state[n] {
		case done:
			return nil
		case visiting:
			i := 0
			for stack[i] != n {
				i++
			}
			cycle := append(append([]string{}, stack[i:]...), n)
			return errors.New("dependency cycle: " + strings.Join(cycle, " -> "))
		}
		state[n] = visiting
		stack = append(stack, n)
		for _, d := range byName[n].DependsOn {
			if err := visit(d); err != nil {
				return err
			}
		}
		stack = stack[:len(stack)-1]
		state[n] = done
		order = append(order, n)
		return nil
	}
	for _, a := range apps {
		if err := visit(a.Name); err != nil {
			return nil, err
		}
	}
	return order, nil
}
