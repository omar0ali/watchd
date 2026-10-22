package detect

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/omar0ali/watchd/internal/config"
)

func write(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func names(apps []App) string {
	var n []string
	for _, a := range apps {
		n = append(n, a.Name+"="+a.Cmd+"@"+a.Cwd)
	}
	return strings.Join(n, " | ")
}

func TestDetectMonorepo(t *testing.T) {
	root := t.TempDir()
	write(t, root, map[string]string{
		"api/go.mod":                    "module api",
		"api/main.go":                   "package main\nfunc main() {}",
		"web/package.json":              `{"scripts": {"dev": "vite", "build": "vite build"}}`,
		"web/pnpm-lock.yaml":            "",
		"tools/go.mod":                  "module tools",
		"tools/cmd/migrate/main.go":     "package main",
		"tools/lib.go":                  "package tools",
		"engine/Cargo.toml":             "",
		"site/manage.py":                "",
		"site/.venv/bin/python":         "",
		"docs/README.md":                "nothing to run",
		"node_modules/x/package.json":   `{"scripts": {"dev": "x"}}`,
		".hidden/package.json":          `{"scripts": {"dev": "x"}}`,
		"lib/package.json":              `{"scripts": {"test": "jest"}}`,
		"api/cmd/worker/main.go":        "package main",
		"api/cmd/worker/worker_test.go": "package main",
	})
	got := names(Detect(root))
	// Subdirectories are scanned alphabetically.
	want := "api=go run .@api | worker=go run ./cmd/worker@api | engine=cargo run@engine | " +
		"site=.venv/bin/python manage.py runserver --noreload@site | migrate=go run ./cmd/migrate@tools | web=pnpm run dev@web"
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

func TestDetectRootAndDuplicates(t *testing.T) {
	root := t.TempDir()
	write(t, root, map[string]string{
		"go.mod":           "module x",
		"main.go":          "package main",
		"package.json":     `{"scripts": {"start": "node server.js"}}`,
		"web/package.json": `{"scripts": {"dev": "next dev"}}`,
	})
	got := names(Detect(root))
	want := "app=go run .@ | web=npm run start@ | web-2=npm run dev@web"
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

func TestProcfileWins(t *testing.T) {
	root := t.TempDir()
	write(t, root, map[string]string{
		"Procfile": "# comment\nweb: bundle exec rails s -p $PORT\nworker:  bundle exec sidekiq\n\n",
		"go.mod":   "module x",
		"main.go":  "package main",
	})
	got := names(Detect(root))
	want := "web=bundle exec rails s -p $PORT@ | worker=bundle exec sidekiq@"
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

func TestDetectNothing(t *testing.T) {
	if apps := Detect(t.TempDir()); len(apps) != 0 {
		t.Errorf("got %v", apps)
	}
}

// Rendered configs must load cleanly, with or without detected apps.
func TestRenderLoads(t *testing.T) {
	root := t.TempDir()
	write(t, root, map[string]string{"api/main.go": "", "web/x": ""})
	for _, apps := range [][]App{
		nil,
		{
			{Name: "api", Cmd: `go run . -msg "hi \ there"`, Cwd: "api", Watch: []string{"*.go"}},
			{Name: "web", Cmd: "npm run dev", Cwd: "web", Watch: []string{"package.json"}, Ignore: []string{"node_modules"}},
		},
	} {
		path := filepath.Join(root, config.DefaultFile)
		if err := os.WriteFile(path, []byte(Render("my-project", apps)), 0o644); err != nil {
			t.Fatal(err)
		}
		p, err := config.Load(path, config.Options{})
		if err != nil {
			t.Fatalf("rendered config does not load: %v\n%s", err, Render("my-project", apps))
		}
		if p.Name != "my-project" {
			t.Errorf("name = %q", p.Name)
		}
		if len(apps) > 0 && p.App("api").Cmd != apps[0].Cmd {
			t.Errorf("cmd round-trip: %q", p.App("api").Cmd)
		}
		p.Close()
	}
}

func TestSanitizeName(t *testing.T) {
	for in, want := range map[string]string{
		"My App (old)": "my-app-old",
		"shop":         "shop",
		"  --x--  ":    "x",
		"日本":           "project",
		"api_v2.1":     "api_v2.1",
	} {
		if got := config.SanitizeName(in); got != want {
			t.Errorf("SanitizeName(%q) = %q, want %q", in, got, want)
		}
	}
}
