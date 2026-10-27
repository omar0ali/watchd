package config

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, DefaultFile)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func load(t *testing.T, body string) (*Project, error) {
	t.Helper()
	p, err := Load(writeConfig(t, body), Options{Env: []string{"FROM_CLI=yes"}, Print: func(string, string) {}})
	if err == nil {
		t.Cleanup(p.Close)
	}
	return p, err
}

func TestLoadFull(t *testing.T) {
	path := writeConfig(t, `
		return {
			name = "shop",
			env = { SHARED = "1" },
			apps = {
				{
					name = "web", cmd = "npm run dev", cwd = "./web",
					depends_on = { "api" },
					env = { PORT = 3000, DEBUG = true },
					watch = "src/**/*.{js,ts}",
					restart = "always", max_restarts = 3,
					stop_signal = "INT", stop_timeout = "500ms",
					healthcheck = { port = 3000, interval = 0.5 },
				},
				{ name = "api", cmd = "go run .", env = { CLI = watchd.env("FROM_CLI"), MISSING = watchd.env("NOPE", "dflt") } },
			},
		}`)
	if err := os.Mkdir(filepath.Join(filepath.Dir(path), "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	p, err := Load(path, Options{Env: []string{"FROM_CLI=yes"}})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	if p.Name != "shop" {
		t.Errorf("name = %q", p.Name)
	}
	if got := strings.Join(p.Order, ","); got != "api,web" {
		t.Errorf("order = %s, want api,web", got)
	}
	web := p.App("web")
	if web.Cwd != filepath.Join(filepath.Dir(path), "web") {
		t.Errorf("cwd = %s", web.Cwd)
	}
	if web.Env["PORT"] != "3000" || web.Env["DEBUG"] != "true" || web.Env["SHARED"] != "1" {
		t.Errorf("env = %v", web.Env)
	}
	if len(web.Watch) != 1 || web.Restart != RestartAlways || web.MaxRestarts != 3 {
		t.Errorf("watch/restart = %v %v %v", web.Watch, web.Restart, web.MaxRestarts)
	}
	if web.StopSignal != syscall.SIGINT || web.StopTimeout != 500*time.Millisecond {
		t.Errorf("stop = %v %v", web.StopSignal, web.StopTimeout)
	}
	if web.Health == nil || web.Health.Port != 3000 || web.Health.Interval != 500*time.Millisecond || web.Health.Host != DefaultHCHost {
		t.Errorf("health = %+v", web.Health)
	}
	api := p.App("api")
	if api.Env["CLI"] != "yes" || api.Env["MISSING"] != "dflt" {
		t.Errorf("watchd.env: %v", api.Env)
	}
	if api.Restart != DefaultRestart || api.StopTimeout != DefaultStopTimeout || !api.Autostart {
		t.Errorf("defaults not applied: %+v", api)
	}
}

func TestDefaultNameAndKeyedApps(t *testing.T) {
	p, err := load(t, `return { apps = { b = { cmd = "x" }, a = { cmd = "y" } } }`)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != filepath.Base(p.Dir) {
		t.Errorf("name = %q, want dir name", p.Name)
	}
	if p.Apps[0].Name != "a" || p.Apps[1].Name != "b" {
		t.Errorf("keyed apps not sorted: %s, %s", p.Apps[0].Name, p.Apps[1].Name)
	}
}

func TestValidationErrors(t *testing.T) {
	tests := []struct {
		name, body, want string
	}{
		{"no return", `x = 1`, "must end with `return"},
		{"no apps", `return { name = "x" }`, "apps: required"},
		{"missing cmd", `return { apps = { { name = "a" } } }`, "cmd is required"},
		{"typo", `return { apps = { { name = "a", cmd = "x", depend_on = {"b"} } } }`, "unknown field(s) depend_on"},
		{"bad restart", `return { apps = { { name = "a", cmd = "x", restart = "sometimes" } } }`, "restart must be"},
		{"bad type", `return { apps = { { name = "a", cmd = 5 } } }`, "cmd must be a string"},
		{"unknown dep", `return { apps = { { name = "a", cmd = "x", depends_on = {"b"} } } }`, `unknown app "b"`},
		{"cycle", `return { apps = {
			{ name = "a", cmd = "x", depends_on = {"b"} },
			{ name = "b", cmd = "x", depends_on = {"a"} } } }`, "dependency cycle"},
		{"duplicate", `return { apps = { { name = "a", cmd = "x" }, { name = "a", cmd = "y" } } }`, "defined twice"},
		{"bad name", `return { apps = { { name = "a/b", cmd = "x" } } }`, `name "a/b"`},
		{"bad cwd", `return { apps = { { name = "a", cmd = "x", cwd = "nope" } } }`, "is not a directory"},
		{"health two kinds", `return { apps = { { name = "a", cmd = "x", healthcheck = { port = 1, cmd = "true" } } } }`, "exactly one of"},
		{"bad duration", `return { apps = { { name = "a", cmd = "x", stop_timeout = "soon" } } }`, "invalid duration"},
		{"bad signal", `return { apps = { { name = "a", cmd = "x", stop_signal = "SIGFOO" } } }`, "unknown signal"},
		{"bad glob", `return { apps = { { name = "a", cmd = "x", watch = { "[" } } } }`, "invalid glob"},
		{"hook not fn", `return { apps = { { name = "a", cmd = "x", on_change = "yes" } } }`, "on_change must be a function"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := load(t, tt.body)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want it to contain %q", err, tt.want)
			}
		})
	}
}

func TestAllErrorsReportedAtOnce(t *testing.T) {
	_, err := load(t, `return { apps = { { name = "a" }, { name = "b", cmd = "x", restart = "no" } } }`)
	if err == nil || !strings.Contains(err.Error(), "cmd is required") || !strings.Contains(err.Error(), "restart must be") {
		t.Fatalf("want both errors, got %v", err)
	}
}

func TestSandbox(t *testing.T) {
	for _, expr := range []string{"os.exit(1)", "os.execute('true')", "io.open('/etc/passwd')", "dofile('/etc/passwd')", "debug.getinfo(1)"} {
		_, err := load(t, expr+`; return { apps = { { name = "a", cmd = "x" } } }`)
		if err == nil {
			t.Errorf("%s: expected an error", expr)
		}
	}
	// Harmless os functions still work.
	if _, err := load(t, `local t = os.time(); local h = os.getenv("FROM_CLI"); assert(h == "yes")
		return { apps = { { name = "a", cmd = "x" } } }`); err != nil {
		t.Errorf("os.time/getenv: %v", err)
	}
}

func TestLoadTimeout(t *testing.T) {
	p := writeConfig(t, `while true do end`)
	_, err := Load(p, Options{LoadTimeout: 100 * time.Millisecond})
	if err == nil || !strings.Contains(err.Error(), "longer than") {
		t.Fatalf("error = %v", err)
	}
}

func TestRequireFromConfigDir(t *testing.T) {
	p := writeConfig(t, `local common = require("common"); return { apps = { { name = "a", cmd = common.cmd } } }`)
	if err := os.WriteFile(filepath.Join(filepath.Dir(p), "common.lua"), []byte(`return { cmd = "from-module" }`), 0o644); err != nil {
		t.Fatal(err)
	}
	proj, err := Load(p, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer proj.Close()
	if proj.Apps[0].Cmd != "from-module" {
		t.Errorf("cmd = %q", proj.Apps[0].Cmd)
	}
}

// The same folder reached through a symlink must resolve to the same config
// path, or the daemon would treat it as a different project.
func TestSymlinkedPathsResolveToSameConfig(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real", "proj")
	if err := os.MkdirAll(filepath.Join(real, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, DefaultFile), []byte(`return { apps = { { name = "a", cmd = "x" } } }`), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(filepath.Join(root, "real"), link); err != nil {
		t.Fatal(err)
	}

	viaReal, err := Find(filepath.Join(real, "sub"))
	if err != nil {
		t.Fatal(err)
	}
	viaLink, err := Find(filepath.Join(link, "proj", "sub"))
	if err != nil {
		t.Fatal(err)
	}
	if viaReal != viaLink {
		t.Errorf("Find: %s != %s", viaReal, viaLink)
	}

	p, err := Load(filepath.Join(link, "proj", DefaultFile), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if p.Path != viaReal {
		t.Errorf("Load path = %s, want %s", p.Path, viaReal)
	}
}

func TestRejectsWorldWritable(t *testing.T) {
	p := writeConfig(t, `return { apps = { { name = "a", cmd = "x" } } }`)
	if err := os.Chmod(p, 0o666); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(p, Options{}); err == nil || !strings.Contains(err.Error(), "world-writable") {
		t.Fatalf("error = %v", err)
	}
}

func TestHooks(t *testing.T) {
	var printed []string
	p, err := Load(writeConfig(t, `
		return { apps = { {
			name = "a", cmd = "x",
			on_change = function(file, app)
				print(app.name .. " saw " .. file)
				return not file:match("_test%.go$")
			end,
			on_exit = function(app, code) print("exit " .. code .. " pid " .. app.pid) end,
			on_start = function(app) while true do end end,
		} } }`), Options{
		Print:       func(app, msg string) { printed = append(printed, app+": "+msg) },
		HookTimeout: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	a := p.App("a")

	if ok, err := p.OnChange(a, "main.go"); err != nil || !ok {
		t.Errorf("main.go: restart=%v err=%v", ok, err)
	}
	if ok, err := p.OnChange(a, "main_test.go"); err != nil || ok {
		t.Errorf("main_test.go: restart=%v err=%v", ok, err)
	}
	if err := p.OnExit(a, 42, 1); err != nil {
		t.Error(err)
	}
	if err := p.OnStart(a, 42); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Errorf("on_start: want timeout, got %v", err)
	}
	// The state is still usable after a timed-out hook.
	if ok, err := p.OnChange(a, "x.go"); err != nil || !ok {
		t.Errorf("after timeout: restart=%v err=%v", ok, err)
	}

	want := []string{"a: a saw main.go", "a: a saw main_test.go", "a: exit 1 pid 42", "a: a saw x.go"}
	if strings.Join(printed, "|") != strings.Join(want, "|") {
		t.Errorf("printed = %q, want %q", printed, want)
	}

	p.Close()
	if ok, err := p.OnChange(a, "y.go"); err != nil || !ok {
		t.Errorf("after close: restart=%v err=%v", ok, err)
	}
}

func TestEnvFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	body := "# comment\nA=1\nexport B=\"two\\nlines\"\nC='lit $x'\nD=val # note\n\n"
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	env, err := ParseEnvFile(p)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"A": "1", "B": "two\nlines", "C": "lit $x", "D": "val"}
	for k, v := range want {
		if env[k] != v {
			t.Errorf("%s = %q, want %q", k, env[k], v)
		}
	}

	if err := os.WriteFile(p, []byte("not valid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseEnvFile(p); err == nil {
		t.Error("expected error for invalid line")
	}
}
