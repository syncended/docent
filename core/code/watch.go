package code

import (
	"context"
	"errors"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/syncended/docent/core/config"
	"github.com/syncended/docent/core/source"
)

// The directories of the indexed files are watched with the platform's file
// system notifications; Open and Reindex keep the watched directories in step
// with the roots (DESIGN.md, Technical decisions: File watching).

var (
	watchMu sync.Mutex
	watcher *fsnotify.Watcher
	// watched is the absolute path of every watched directory.
	watched map[string]bool
)

// Change is a batch of files that changed in a root: created, written,
// deleted or renamed. Paths are slash-separated, relative to the root, and
// sorted. Paths is nil when the changes could not be tracked, such as after
// events were lost: anything in the root may have changed.
type Change struct {
	Root  string   `json:"root"`
	Paths []string `json:"paths"`
}

// watchBuffer is the per-directory event buffer on Windows, where a watch
// costs its buffer: the default 64 KB over the thousands of directories of
// a repository would not be lightweight. Overflows put the root in doubt.
const watchBuffer = 4096

// Watch watches the roots' directories until ctx is done. Changes are
// reported per root once quiet has passed without further changes, so that
// a `git checkout` or an agent writing several files is one batch. Only one
// Watch runs at a time.
func Watch(ctx context.Context, quiet time.Duration, changed func(Change)) error {
	w, err := fsnotify.NewBufferedWatcher(4096)
	if err != nil {
		return err
	}
	watchMu.Lock()
	watcher, watched = w, map[string]bool{}
	watchMu.Unlock()
	defer func() {
		watchMu.Lock()
		watcher, watched = nil, nil
		watchMu.Unlock()
		w.Close()
	}()
	rewatch()

	// pending holds the changed paths by root ID; a nil set means the
	// whole root is in doubt.
	pending := map[string]map[string]bool{}
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-w.Events:
			if !ok {
				return nil
			}
			if note(ev, pending) {
				timer.Reset(quiet)
			}
		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}
			if errors.Is(err, fsnotify.ErrEventOverflow) {
				mu.RLock()
				for _, r := range roots {
					pending[r.id] = nil
				}
				mu.RUnlock()
				timer.Reset(quiet)
			}
		case <-timer.C:
			for id, set := range pending {
				c := Change{Root: id}
				if set != nil {
					for p := range set {
						c.Paths = append(c.Paths, p)
					}
					sort.Strings(c.Paths)
				}
				changed(c)
			}
			pending = map[string]map[string]bool{}
		}
	}
}

// note records ev in pending and reports whether it concerned a root. A new
// directory is watched from then on; a directory removed or renamed puts its
// root in doubt, since the files below it moved with it.
func note(ev fsnotify.Event, pending map[string]map[string]bool) bool {
	r, rel, ok := rootOf(ev.Name)
	if !ok {
		return false
	}
	if _, doubt := pending[r.id]; doubt && pending[r.id] == nil {
		return true
	}
	watchMu.Lock()
	dir := watched[ev.Name]
	watchMu.Unlock()
	switch {
	case dir && ev.Has(fsnotify.Remove|fsnotify.Rename):
		unwatch(ev.Name)
		pending[r.id] = nil
		return true
	case ev.Has(fsnotify.Create):
		if info, err := os.Lstat(ev.Name); err == nil && info.IsDir() {
			if !source.Watchable(r.cfg, rel) {
				return false
			}
			watchTree(r.cfg, ev.Name)
			pending[r.id] = nil
			return true
		}
	}
	if pending[r.id] == nil {
		pending[r.id] = map[string]bool{}
	}
	pending[r.id][rel] = true
	return true
}

// rootOf finds the root an absolute path is in and the path relative to it,
// slash-separated, if the path is one the index could hold.
func rootOf(abs string) (*root, string, bool) {
	mu.RLock()
	defer mu.RUnlock()
	for _, r := range roots {
		rel, err := filepath.Rel(r.cfg.Path, abs)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			rel = ""
		}
		if !source.Watchable(r.cfg, path.Dir(rel)) {
			return nil, "", false
		}
		return r, rel, true
	}
	return nil, "", false
}

// rewatch brings the watched directories in step with the roots' files:
// every directory holding an indexed file, and the directories above it up
// to the root. Without a Watch running it does nothing.
func rewatch() {
	watchMu.Lock()
	w := watcher
	watchMu.Unlock()
	if w == nil {
		return
	}
	want := map[string]bool{}
	mu.RLock()
	for _, r := range roots {
		want[r.cfg.Path] = true
		for _, inc := range r.cfg.Include {
			for d := filepath.ToSlash(filepath.Clean(inc)); d != "." && d != "/" && d != ""; d = path.Dir(d) {
				want[filepath.Join(r.cfg.Path, filepath.FromSlash(d))] = true
			}
		}
		for _, f := range r.files {
			for d := path.Dir(f.Path); d != "." && d != "/"; d = path.Dir(d) {
				abs := filepath.Join(r.cfg.Path, filepath.FromSlash(d))
				if want[abs] {
					break
				}
				want[abs] = true
			}
		}
	}
	mu.RUnlock()
	watchMu.Lock()
	defer watchMu.Unlock()
	if watcher != w {
		return
	}
	for dir := range watched {
		if !want[dir] {
			w.Remove(dir)
			delete(watched, dir)
		}
	}
	for dir := range want {
		if !watched[dir] {
			if w.AddWith(dir, fsnotify.WithBufferSize(watchBuffer)) == nil {
				watched[dir] = true
			}
		}
	}
}

// watchTree watches a new directory of a root and everything below it that
// the index could hold.
func watchTree(rc config.Root, dir string) {
	filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(rc.Path, p)
		if err != nil || !source.Watchable(rc, filepath.ToSlash(rel)) {
			return filepath.SkipDir
		}
		watchMu.Lock()
		if watcher != nil && !watched[p] && watcher.AddWith(p, fsnotify.WithBufferSize(watchBuffer)) == nil {
			watched[p] = true
		}
		watchMu.Unlock()
		return nil
	})
}

func unwatch(dir string) {
	watchMu.Lock()
	defer watchMu.Unlock()
	if watcher == nil {
		return
	}
	prefix := dir + string(filepath.Separator)
	for d := range watched {
		if d == dir || strings.HasPrefix(d, prefix) {
			watcher.Remove(d)
			delete(watched, d)
		}
	}
}
