package watcher

import (
	"path"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// DefaultIgnore is always ignored: VCS directories and editor temp files.
var DefaultIgnore = []string{
	".git", ".hg", ".svn",
	"*.swp", "*.swx", "*~", ".#*", "#*#", "4913", ".DS_Store",
}

// Match reports whether rel (a slash-separated path relative to the watch
// root) matches pattern. Patterns without a "/" match the file name at any
// depth, like .gitignore: "*.go" matches "cmd/api/main.go". Patterns with a
// "/" match the whole path: "src/**/*.ts".
func Match(pattern, rel string) bool {
	pattern = strings.TrimSuffix(strings.TrimPrefix(pattern, "./"), "/")
	if !strings.Contains(pattern, "/") {
		ok, _ := doublestar.Match(pattern, path.Base(rel))
		return ok
	}
	ok, _ := doublestar.Match(pattern, rel)
	return ok
}

// Ignored reports whether rel or any of its parent directories matches one
// of the patterns, so "node_modules" ignores everything below it.
func Ignored(patterns []string, rel string) bool {
	if rel == "." || rel == "" {
		return false
	}
	parts := strings.Split(rel, "/")
	for i := range parts {
		prefix := strings.Join(parts[:i+1], "/")
		for _, p := range patterns {
			if Match(p, prefix) {
				return true
			}
		}
	}
	return false
}
