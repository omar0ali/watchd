// Package detect guesses the apps of a project from well-known files, to
// pre-fill a new watchd.lua. It deliberately knows only a handful of common
// stacks; anything else gets the commented template.
package detect

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/omar0ali/watchd/internal/config"
)

// App is a detected app. Cwd is relative to the project root ("" for the
// root itself).
type App struct {
	Name   string
	Kind   string // shown to the user: "Procfile", "Go", "Node (pnpm)", ...
	Cmd    string
	Cwd    string
	Watch  []string
	Ignore []string
}

// Directories never scanned for apps.
var skipDirs = map[string]bool{
	"node_modules": true, "vendor": true, "dist": true, "build": true, "target": true,
	"tmp": true, "out": true, "bin": true, "venv": true, "__pycache__": true,
}

// Detect looks at root and its immediate subdirectories. A Procfile in root
// wins: it already lists the project's processes.
func Detect(root string) []App {
	if apps := procfile(root); len(apps) > 0 {
		return apps
	}

	var apps []App
	apps = append(apps, detectDir(root, "")...)

	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || strings.HasPrefix(name, ".") || skipDirs[name] {
			continue
		}
		apps = append(apps, detectDir(filepath.Join(root, name), name)...)
	}
	return uniqueNames(apps)
}

// detectDir checks one directory. rel is its path relative to the root.
func detectDir(dir, rel string) []App {
	var apps []App
	// Apps in the root are named after their stack, apps in a subdirectory
	// after the directory.
	name := func(kind string) string {
		if rel != "" {
			return config.SanitizeName(rel)
		}
		return kind
	}

	if exists(dir, "go.mod") {
		if hasMainPackage(dir) {
			apps = append(apps, App{Name: name("app"), Kind: "Go", Cmd: "go run .", Cwd: rel,
				Watch: []string{"*.go", "go.mod"}})
		}
		// The common cmd/<name>/main.go layout.
		cmds, _ := os.ReadDir(filepath.Join(dir, "cmd"))
		for _, c := range cmds {
			if c.IsDir() && hasMainPackage(filepath.Join(dir, "cmd", c.Name())) {
				apps = append(apps, App{Name: config.SanitizeName(c.Name()), Kind: "Go",
					Cmd: "go run ./cmd/" + c.Name(), Cwd: rel, Watch: []string{"*.go", "go.mod"}})
			}
		}
	}

	if script, ok := packageScript(dir); ok {
		pm := packageManager(dir)
		kind := "Node"
		if pm != "npm" {
			kind += " (" + pm + ")"
		}
		// Dev servers reload themselves; only restart when dependencies change.
		apps = append(apps, App{Name: name("web"), Kind: kind, Cmd: pm + " run " + script, Cwd: rel,
			Watch: []string{"package.json"}, Ignore: []string{"node_modules"}})
	}

	if exists(dir, "Cargo.toml") {
		apps = append(apps, App{Name: name("app"), Kind: "Rust", Cmd: "cargo run", Cwd: rel,
			Watch: []string{"*.rs", "Cargo.toml"}, Ignore: []string{"target"}})
	}

	if exists(dir, "manage.py") {
		python := "python"
		for _, venv := range []string{".venv", "venv"} {
			if exists(dir, venv+"/bin/python") {
				python = venv + "/bin/python"
				break
			}
		}
		apps = append(apps, App{Name: name("django"), Kind: "Django",
			Cmd: python + " manage.py runserver --noreload", Cwd: rel,
			Watch: []string{"*.py"}, Ignore: []string{".venv", "venv", "__pycache__"}})
	}
	return apps
}

var procLine = regexp.MustCompile(`^([A-Za-z0-9_.-]+):\s*(.+)$`)

func procfile(root string) []App {
	f, err := os.Open(filepath.Join(root, "Procfile"))
	if err != nil {
		return nil
	}
	defer f.Close()
	var apps []App
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if m := procLine.FindStringSubmatch(line); m != nil && !strings.HasPrefix(line, "#") {
			apps = append(apps, App{Name: config.SanitizeName(m[1]), Kind: "Procfile", Cmd: m[2]})
		}
	}
	return uniqueNames(apps)
}

func exists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

var mainPkg = regexp.MustCompile(`(?m)^package main\b`)

func hasMainPackage(dir string) bool {
	files, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err == nil && mainPkg.Match(b) {
			return true
		}
	}
	return false
}

// packageScript returns the script to run: "dev" if present, else "start".
func packageScript(dir string) (string, bool) {
	b, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return "", false
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if json.Unmarshal(b, &pkg) != nil {
		return "", false
	}
	for _, s := range []string{"dev", "start"} {
		if _, ok := pkg.Scripts[s]; ok {
			return s, true
		}
	}
	return "", false
}

func packageManager(dir string) string {
	switch {
	case exists(dir, "pnpm-lock.yaml"):
		return "pnpm"
	case exists(dir, "yarn.lock"):
		return "yarn"
	case exists(dir, "bun.lock"), exists(dir, "bun.lockb"):
		return "bun"
	default:
		return "npm"
	}
}

// uniqueNames suffixes repeated names: web, web-2, ...
func uniqueNames(apps []App) []App {
	seen := map[string]int{}
	for i := range apps {
		n := apps[i].Name
		seen[n]++
		if seen[n] > 1 {
			apps[i].Name = n + "-" + strconv.Itoa(seen[n])
		}
	}
	return apps
}

// Render writes a watchd.lua for the project. With no apps it returns the
// commented starter template.
func Render(project string, apps []App) string {
	var b strings.Builder
	b.WriteString("-- watchd config. Reference: https://github.com/omar0ali/watchd#configuration\n")
	b.WriteString("-- Check it with: watchd validate      Start it with: watchd up\n")
	b.WriteString("--\n-- Other app fields: env, env_file, depends_on, restart (\"always\" | \"on-failure\" | \"never\"),\n")
	b.WriteString("-- healthcheck = { port = 8080 }, stop_signal, stop_timeout, autostart, on_change.\n")
	b.WriteString("return {\n")
	b.WriteString("\tname = " + luaQuote(project) + ",\n\n")
	b.WriteString("\tapps = {\n")
	if len(apps) == 0 {
		b.WriteString(starterApps)
	}
	for i, a := range apps {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("\t\t{\n")
		b.WriteString("\t\t\tname = " + luaQuote(a.Name) + ",\n")
		b.WriteString("\t\t\tcmd = " + luaQuote(a.Cmd) + ",\n")
		if a.Cwd != "" {
			b.WriteString("\t\t\tcwd = " + luaQuote("./"+filepath.ToSlash(a.Cwd)) + ",\n")
		}
		if len(a.Watch) > 0 {
			b.WriteString("\t\t\twatch = " + luaList(a.Watch) + ",\n")
		}
		if len(a.Ignore) > 0 {
			b.WriteString("\t\t\tignore = " + luaList(a.Ignore) + ",\n")
		}
		b.WriteString("\t\t},\n")
	}
	b.WriteString("\t},\n}\n")
	return b.String()
}

const starterApps = `		{
			name = "app",
			cmd = "echo 'replace me with your command' && sleep 3600",
			-- cwd = "./backend",           -- relative to this file
			-- watch = { "**/*.go" },       -- restart when these change
			-- healthcheck = { port = 8080 },
		},

		-- {
		-- 	name = "web",
		-- 	cmd = "npm run dev",
		-- 	cwd = "./web",
		-- 	depends_on = { "app" },      -- starts after app is ready
		-- 	env = { PORT = 3000 },
		-- },
`

func luaList(items []string) string {
	q := make([]string, len(items))
	for i, s := range items {
		q[i] = luaQuote(s)
	}
	return "{ " + strings.Join(q, ", ") + " }"
}

// luaQuote returns a Lua 5.1 double-quoted string literal.
func luaQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, c := range []byte(s) {
		switch {
		case c == '"' || c == '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		case c == '\n':
			b.WriteString(`\n`)
		case c < 0x20 || c == 0x7f:
			fmt.Fprintf(&b, `\%03d`, c)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}
