package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMatch(t *testing.T) {
	tests := []struct {
		pattern, rel string
		want         bool
	}{
		{"*.go", "main.go", true},
		{"*.go", "cmd/api/main.go", true},
		{"*.go", "main.go.bak", false},
		{"**/*.go", "cmd/api/main.go", true},
		{"src/**/*.{js,ts}", "src/a/b.ts", true},
		{"src/**/*.{js,ts}", "lib/b.ts", false},
		{"./src/*.js", "src/a.js", true},
		{"dist/", "dist", true},
	}
	for _, tt := range tests {
		if got := Match(tt.pattern, tt.rel); got != tt.want {
			t.Errorf("Match(%q, %q) = %v, want %v", tt.pattern, tt.rel, got, tt.want)
		}
	}
}

func TestIgnored(t *testing.T) {
	pats := []string{"node_modules", "build/out", "*.tmp"}
	tests := []struct {
		rel  string
		want bool
	}{
		{"node_modules/x/index.js", true},
		{"web/node_modules/x.js", true},
		{"build/out/a.o", true},
		{"build/src/a.c", false},
		{"a.tmp", true},
		{"src/main.go", false},
		{".", false},
	}
	for _, tt := range tests {
		if got := Ignored(pats, tt.rel); got != tt.want {
			t.Errorf("Ignored(%q) = %v, want %v", tt.rel, got, tt.want)
		}
	}
}

func TestWatcherDebouncesAndFilters(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := make(chan []string, 4)
	w, err := New(root, []string{"*.go"}, []string{"node_modules"}, 100*time.Millisecond,
		func(files []string) { got <- files },
		func(err error) { t.Log("watch error:", err) })
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	write := func(rel string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.go")
	write("b.go")
	write("notes.txt")
	write("node_modules/c.go")

	select {
	case files := <-got:
		if len(files) != 2 || files[0] != "a.go" || files[1] != "b.go" {
			t.Fatalf("got %v, want [a.go b.go]", files)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no change reported")
	}

	// New directories are watched as they appear.
	write("pkg/deep/d.go")
	select {
	case files := <-got:
		found := false
		for _, f := range files {
			if f == "pkg/deep/d.go" {
				found = true
			}
		}
		if !found {
			t.Fatalf("got %v, want pkg/deep/d.go", files)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("change in new directory not reported")
	}
}
