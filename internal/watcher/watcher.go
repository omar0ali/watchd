// Package watcher watches a directory tree and reports batches of changed
// files that match include patterns, after a quiet period (debounce).
package watcher

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

type Watcher struct {
	root     string
	include  []string
	ignore   []string
	debounce time.Duration
	onChange func(files []string)
	onError  func(error)

	fsw       *fsnotify.Watcher
	done      chan struct{}
	closeOnce sync.Once
}

// New starts watching root recursively. onChange receives sorted paths
// relative to root; onError receives non-fatal errors. Both are called from
// the watcher's goroutine, one at a time.
func New(root string, include, ignore []string, debounce time.Duration, onChange func([]string), onError func(error)) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	w := &Watcher{
		root:     root,
		include:  include,
		ignore:   append(slices.Clone(DefaultIgnore), ignore...),
		debounce: debounce,
		onChange: onChange,
		onError:  onError,
		fsw:      fsw,
		done:     make(chan struct{}),
	}
	if err := w.addTree(root, nil); err != nil {
		fsw.Close()
		return nil, err
	}
	go w.loop()
	return w, nil
}

func (w *Watcher) Close() {
	w.closeOnce.Do(func() {
		close(w.done)
		w.fsw.Close()
	})
}

func (w *Watcher) rel(p string) string {
	r, err := filepath.Rel(w.root, p)
	if err != nil {
		return p
	}
	return filepath.ToSlash(r)
}

// addTree watches dir and every non-ignored directory below it. found, if
// set, receives every file inside: files created together with a new
// directory may land before the directory is watched.
func (w *Watcher) addTree(dir string, found func(rel string)) error {
	return filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == dir {
				return err
			}
			return nil // vanished or unreadable; skip
		}
		if !d.IsDir() {
			if found != nil {
				found(w.rel(p))
			}
			return nil
		}
		if Ignored(w.ignore, w.rel(p)) {
			return filepath.SkipDir
		}
		if err := w.fsw.Add(p); err != nil {
			if errors.Is(err, syscall.ENOSPC) {
				return fmt.Errorf("too many directories to watch under %s; add large directories to `ignore` or raise fs.inotify.max_user_watches", w.root)
			}
			return fmt.Errorf("watch %s: %w", p, err)
		}
		return nil
	})
}

func (w *Watcher) loop() {
	pending := map[string]struct{}{}
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	add := func(rel string) {
		if !Ignored(w.ignore, rel) && w.matches(rel) {
			pending[rel] = struct{}{}
			timer.Reset(w.debounce)
		}
	}

	for {
		select {
		case <-w.done:
			timer.Stop()
			return

		case ev, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			rel := w.rel(ev.Name)
			if Ignored(w.ignore, rel) {
				continue
			}
			if ev.Has(fsnotify.Create) {
				if fi, err := os.Lstat(ev.Name); err == nil && fi.IsDir() {
					if err := w.addTree(ev.Name, add); err != nil {
						w.onError(err)
					}
					continue
				}
			}
			// Chmod events are kept: `touch file` only produces one, and
			// it's a common way to force a restart.
			add(rel)

		case <-timer.C:
			if len(pending) == 0 {
				continue
			}
			files := make([]string, 0, len(pending))
			for f := range pending {
				files = append(files, f)
			}
			slices.Sort(files)
			clear(pending)
			w.onChange(files)

		case err, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			w.onError(err)
		}
	}
}

func (w *Watcher) matches(rel string) bool {
	for _, p := range w.include {
		if Match(p, rel) {
			return true
		}
	}
	return false
}
