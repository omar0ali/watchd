package config

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	goruntime "runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/omar0ali/watchd/internal/paths"
	lua "github.com/yuin/gopher-lua"
)

// Options control how a config is loaded.
type Options struct {
	// Env is the environment seen by watchd.env() and os.getenv();
	// defaults to os.Environ().
	Env []string
	// Print receives output of Lua print() calls. app is empty while the
	// config itself is being evaluated. Defaults to stderr.
	Print func(app, msg string)
	// LoadTimeout bounds evaluation of the config file.
	LoadTimeout time.Duration
	// HookTimeout bounds each hook call.
	HookTimeout time.Duration
}

// runtime owns the Lua state. Lua states are not goroutine-safe, and hooks
// are called from several goroutines, so every call holds mu.
type runtime struct {
	mu          sync.Mutex
	L           *lua.LState
	closed      bool
	currentApp  string
	print       func(app, msg string)
	hookTimeout time.Duration
}

type hooks struct {
	onChange, onStart, onExit *lua.LFunction
}

// Load evaluates the config at path and validates it. The returned project
// keeps the Lua state alive for hooks; call Close when done with it.
func Load(path string, opts Options) (*Project, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err := paths.CheckNotWorldWritable(path); err != nil {
		return nil, err
	}
	if opts.Env == nil {
		opts.Env = os.Environ()
	}
	if opts.Print == nil {
		opts.Print = func(app, msg string) {
			if app != "" {
				msg = app + ": " + msg
			}
			fmt.Fprintln(os.Stderr, msg)
		}
	}
	if opts.LoadTimeout == 0 {
		opts.LoadTimeout = DefaultLoadTimeout
	}
	if opts.HookTimeout == 0 {
		opts.HookTimeout = DefaultHookTimeout
	}

	dir := filepath.Dir(path)
	rt := &runtime{print: opts.Print, hookTimeout: opts.HookTimeout}
	rt.L = newSandbox(dir, opts.Env, rt)

	ctx, cancel := context.WithTimeout(context.Background(), opts.LoadTimeout)
	defer cancel()
	rt.L.SetContext(ctx)
	err = rt.L.DoFile(path)
	rt.L.RemoveContext()
	if err != nil {
		rt.L.Close()
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("%s: evaluation took longer than %s", path, opts.LoadTimeout)
		}
		return nil, err
	}

	top, ok := rt.L.Get(-1).(*lua.LTable)
	if !ok || rt.L.GetTop() == 0 {
		rt.L.Close()
		return nil, fmt.Errorf("%s: the file must end with `return { ... }`", path)
	}

	p, err := parseProject(top, path, dir)
	if err != nil {
		rt.L.Close()
		return nil, err
	}
	p.rt = rt
	return p, nil
}

// newSandbox creates a Lua state with only the libraries a config needs.
// io, debug and the dangerous parts of os are left out so a config or hook
// cannot kill, block or write files from inside the daemon.
func newSandbox(dir string, env []string, rt *runtime) *lua.LState {
	L := lua.NewState(lua.Options{SkipOpenLibs: true})
	for _, lib := range []struct {
		name string
		fn   lua.LGFunction
	}{
		{lua.LoadLibName, lua.OpenPackage},
		{lua.BaseLibName, lua.OpenBase},
		{lua.TabLibName, lua.OpenTable},
		{lua.StringLibName, lua.OpenString},
		{lua.MathLibName, lua.OpenMath},
		{lua.OsLibName, lua.OpenOs},
	} {
		L.Push(L.NewFunction(lib.fn))
		L.Push(lua.LString(lib.name))
		L.Call(1, 0)
	}

	if osT, ok := L.GetGlobal("os").(*lua.LTable); ok {
		for _, f := range []string{"exit", "execute", "remove", "rename", "tmpname", "setlocale", "setenv"} {
			osT.RawSetString(f, lua.LNil)
		}
	}
	// dofile/loadfile resolve against the daemon's cwd, which is surprising;
	// require (scoped to the config directory) covers splitting configs.
	L.SetGlobal("dofile", lua.LNil)
	L.SetGlobal("loadfile", lua.LNil)
	if pkg, ok := L.GetGlobal("package").(*lua.LTable); ok {
		pkg.RawSetString("path", lua.LString(filepath.Join(dir, "?.lua")+";"+filepath.Join(dir, "?", "init.lua")))
		pkg.RawSetString("cpath", lua.LString(""))
	}

	envMap := map[string]string{}
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok {
			envMap[k] = v
		}
	}
	// os.getenv sees the CLI's environment, not the daemon's.
	if osT, ok := L.GetGlobal("os").(*lua.LTable); ok {
		osT.RawSetString("getenv", L.NewFunction(func(L *lua.LState) int {
			if v, ok := envMap[L.CheckString(1)]; ok {
				L.Push(lua.LString(v))
			} else {
				L.Push(lua.LNil)
			}
			return 1
		}))
	}

	L.SetGlobal("print", L.NewFunction(func(L *lua.LState) int {
		parts := make([]string, L.GetTop())
		for i := range parts {
			parts[i] = L.ToStringMeta(L.Get(i + 1)).String()
		}
		rt.print(rt.currentApp, strings.Join(parts, "\t"))
		return 0
	}))

	w := L.NewTable()
	w.RawSetString("dir", lua.LString(dir))
	w.RawSetString("os", lua.LString(goruntime.GOOS))
	w.RawSetString("env", L.NewFunction(func(L *lua.LState) int {
		if v, ok := envMap[L.CheckString(1)]; ok {
			L.Push(lua.LString(v))
		} else {
			L.Push(L.Get(2)) // default, or nil
		}
		return 1
	}))
	w.RawSetString("exists", L.NewFunction(func(L *lua.LState) int {
		_, err := os.Stat(resolvePath(dir, L.CheckString(1)))
		L.Push(lua.LBool(err == nil))
		return 1
	}))
	L.SetGlobal("watchd", w)
	return L
}

func parseProject(top *lua.LTable, path, dir string) (*Project, error) {
	var errs errList
	t := newTable(top, "project", &errs)

	p := &Project{Path: path, Dir: dir}
	p.Name = t.str("name")
	if p.Name == "" {
		p.Name = filepath.Base(dir)
	}
	if !validName(p.Name) {
		errs.add("project name %q: use letters, digits, '.', '_' or '-'", p.Name)
	}

	projEnv := map[string]string{}
	for _, f := range t.strList("env_file") {
		loadEnvFile(resolvePath(dir, f), projEnv, &errs)
	}
	maps.Copy(projEnv, t.strMap("env"))

	appsVal := t.raw("apps")
	appsTbl, ok := appsVal.(*lua.LTable)
	if !ok {
		if appsVal == lua.LNil {
			errs.add("apps: required (a list of apps)")
		} else {
			errs.add("apps: expected a table, got %s", appsVal.Type())
		}
	} else {
		// apps may be a list ({ {name="api", ...} }) or a map keyed by name
		// ({ api = {...} }). Map entries are sorted for a stable order.
		type entry struct {
			key string
			tbl *lua.LTable
		}
		var list, keyed []entry
		appsTbl.ForEach(func(k, v lua.LValue) {
			vt, ok := v.(*lua.LTable)
			if !ok {
				errs.add("apps[%s]: expected a table, got %s", k, v.Type())
				return
			}
			if ks, ok := k.(lua.LString); ok {
				keyed = append(keyed, entry{string(ks), vt})
			} else {
				list = append(list, entry{"", vt})
			}
		})
		slices.SortFunc(keyed, func(a, b entry) int { return strings.Compare(a.key, b.key) })

		seen := map[string]bool{}
		for i, e := range append(list, keyed...) {
			where := fmt.Sprintf("apps[%d]", i+1)
			if e.key != "" {
				where = "apps." + e.key
			}
			a := parseApp(newTable(e.tbl, where, &errs), e.key, dir, projEnv, &errs)
			if a == nil {
				continue
			}
			if seen[a.Name] {
				errs.add("app %q is defined twice", a.Name)
				continue
			}
			seen[a.Name] = true
			p.Apps = append(p.Apps, a)
		}
		if len(p.Apps) == 0 && len(errs) == 0 {
			errs.add("apps: at least one app is required")
		}
	}
	t.checkUnknown()

	if len(errs) == 0 {
		order, err := sortByDeps(p.Apps)
		if err != nil {
			errs.add("%v", err)
		}
		p.Order = order
	}
	if err := errs.err(path); err != nil {
		return nil, err
	}
	return p, nil
}

func parseApp(t *table, key, dir string, projEnv map[string]string, errs *errList) *App {
	a := &App{Name: t.str("name")}
	if a.Name == "" {
		a.Name = key
	}
	if a.Name == "" {
		errs.add("%s: name is required", t.where)
		return nil
	}
	if key != "" && a.Name != key {
		errs.add("%s: name %q does not match its key", t.where, a.Name)
	}
	if !validName(a.Name) {
		errs.add("%s: name %q: use letters, digits, '.', '_' or '-'", t.where, a.Name)
	}
	t.where = fmt.Sprintf("app %q", a.Name)

	a.Cmd = t.str("cmd")
	if strings.TrimSpace(a.Cmd) == "" {
		errs.add("%s: cmd is required", t.where)
	}

	a.Cwd = dir
	if cwd := t.str("cwd"); cwd != "" {
		a.Cwd = resolvePath(dir, cwd)
	}
	if fi, err := os.Stat(a.Cwd); err != nil || !fi.IsDir() {
		errs.add("%s: cwd %s is not a directory", t.where, a.Cwd)
	}

	a.Env = maps.Clone(projEnv)
	for _, f := range t.strList("env_file") {
		loadEnvFile(resolvePath(dir, f), a.Env, errs)
	}
	maps.Copy(a.Env, t.strMap("env"))

	a.DependsOn = t.strList("depends_on")

	a.Watch = t.strList("watch")
	a.Ignore = t.strList("ignore")
	for _, pat := range append(slices.Clone(a.Watch), a.Ignore...) {
		if !doublestar.ValidatePattern(strings.TrimPrefix(pat, "./")) {
			errs.add("%s: invalid glob pattern %q", t.where, pat)
		}
	}
	a.Debounce = t.duration("debounce", DefaultDebounce)

	switch r := RestartPolicy(t.str("restart")); r {
	case "":
		a.Restart = DefaultRestart
	case RestartAlways, RestartOnFailure, RestartNever:
		a.Restart = r
	default:
		errs.add("%s: restart must be \"always\", \"on-failure\" or \"never\", got %q", t.where, r)
	}
	a.MaxRestarts = t.integer("max_restarts", DefaultMaxRestarts)
	if a.MaxRestarts < 0 {
		errs.add("%s: max_restarts must be >= 0", t.where)
	}

	sig := t.str("stop_signal")
	if sig == "" {
		sig = DefaultStopSignalNm
	}
	if s, err := parseSignal(sig); err != nil {
		errs.add("%s: stop_signal: %v", t.where, err)
	} else {
		a.StopSignal = s
	}
	a.StopTimeout = t.duration("stop_timeout", DefaultStopTimeout)
	a.Autostart = t.boolean("autostart", true)

	if ht := t.sub("healthcheck"); ht != nil {
		a.Health = parseHealth(ht, errs)
		ht.checkUnknown()
	}

	a.hooks.onChange = t.fn("on_change")
	a.hooks.onStart = t.fn("on_start")
	a.hooks.onExit = t.fn("on_exit")

	t.checkUnknown()
	return a
}

func parseHealth(t *table, errs *errList) *HealthCheck {
	h := &HealthCheck{
		Port:     t.integer("port", 0),
		Host:     t.str("host"),
		HTTP:     t.str("http"),
		Cmd:      t.str("cmd"),
		Interval: t.duration("interval", DefaultHCInterval),
		Timeout:  t.duration("timeout", DefaultHCTimeout),
		Retries:  t.integer("retries", DefaultHCRetries),
	}
	if h.Host == "" {
		h.Host = DefaultHCHost
	}
	set := 0
	for _, b := range []bool{h.Port != 0, h.HTTP != "", h.Cmd != ""} {
		if b {
			set++
		}
	}
	if set != 1 {
		errs.add("%s: set exactly one of port, http or cmd", t.where)
	}
	if h.Port < 0 || h.Port > 65535 {
		errs.add("%s: port %d out of range", t.where, h.Port)
	}
	if h.HTTP != "" && !strings.HasPrefix(h.HTTP, "http://") && !strings.HasPrefix(h.HTTP, "https://") {
		errs.add("%s: http must start with http:// or https://", t.where)
	}
	if h.Retries < 1 {
		errs.add("%s: retries must be >= 1", t.where)
	}
	return h
}

func loadEnvFile(path string, into map[string]string, errs *errList) {
	env, err := ParseEnvFile(path)
	if err != nil {
		errs.add("env_file: %v", err)
		return
	}
	maps.Copy(into, env)
}

// SetPrint redirects Lua print() output, e.g. into the app's log stream.
func (p *Project) SetPrint(fn func(app, msg string)) {
	p.rt.mu.Lock()
	p.rt.print = fn
	p.rt.mu.Unlock()
}

// Close releases the Lua state. Hooks become no-ops afterwards.
func (p *Project) Close() {
	if p.rt == nil {
		return
	}
	p.rt.mu.Lock()
	defer p.rt.mu.Unlock()
	if !p.rt.closed {
		p.rt.closed = true
		p.rt.L.Close()
	}
}

// HasOnChange reports whether the app defines an on_change hook.
func (a *App) HasOnChange() bool { return a.hooks.onChange != nil }

// OnChange runs the app's on_change(file, app) hook and reports whether the
// app should restart. Only an explicit `return false` skips the restart.
func (p *Project) OnChange(a *App, file string) (bool, error) {
	ret, err := p.call(a, a.hooks.onChange, 0, func(info *lua.LTable) []lua.LValue {
		return []lua.LValue{lua.LString(file), info}
	})
	if err != nil {
		return true, err
	}
	return ret != lua.LFalse, nil
}

// OnStart runs the app's on_start(app) hook.
func (p *Project) OnStart(a *App, pid int) error {
	_, err := p.call(a, a.hooks.onStart, pid, func(info *lua.LTable) []lua.LValue {
		return []lua.LValue{info}
	})
	return err
}

// OnExit runs the app's on_exit(app, code) hook.
func (p *Project) OnExit(a *App, pid, code int) error {
	_, err := p.call(a, a.hooks.onExit, pid, func(info *lua.LTable) []lua.LValue {
		return []lua.LValue{info, lua.LNumber(code)}
	})
	return err
}

// call runs fn with the arguments built by args, which receives the app
// info table ({ name, project, cwd, pid }).
func (p *Project) call(a *App, fn *lua.LFunction, pid int, args func(info *lua.LTable) []lua.LValue) (lua.LValue, error) {
	if fn == nil || p.rt == nil {
		return lua.LNil, nil
	}
	rt := p.rt
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.closed {
		return lua.LNil, nil
	}

	info := rt.L.NewTable()
	info.RawSetString("name", lua.LString(a.Name))
	info.RawSetString("project", lua.LString(p.Name))
	info.RawSetString("cwd", lua.LString(a.Cwd))
	if pid != 0 {
		info.RawSetString("pid", lua.LNumber(pid))
	}

	ctx, cancel := context.WithTimeout(context.Background(), rt.hookTimeout)
	defer cancel()
	rt.L.SetContext(ctx)
	defer rt.L.RemoveContext()
	rt.currentApp = a.Name
	defer func() { rt.currentApp = "" }()

	if err := rt.L.CallByParam(lua.P{Fn: fn, NRet: 1, Protect: true}, args(info)...); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return lua.LNil, fmt.Errorf("hook timed out after %s", rt.hookTimeout)
		}
		return lua.LNil, err
	}
	ret := rt.L.Get(-1)
	rt.L.Pop(1)
	return ret, nil
}
