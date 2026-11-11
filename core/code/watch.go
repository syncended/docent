package code

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/syncended/docent/core/config"
	"github.com/syncended/docent/core/source"
)

// The roots are watched with the platform's file system notifications
// (DESIGN.md, Technical decisions: File watching). watched holds the
// directories whose changes count: every directory holding an indexed file
// and those above it up to the root. Open and Reindex keep it in step with
// the roots; the platform's watches follow it (watch_*.go).

var (
	watchMu sync.Mutex
	// watched is the absolute path of every watched directory; nil while
	// no Watch runs.
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

// event is a change the platform reported at an absolute path.
type event struct {
	path string
	// dir is set when the platform knows the path is a directory.
	dir bool
	// created and removed are set for a path created, and for one removed
	// or renamed; a platform may report both for the same path.
	created, removed bool
	// lost is set when events were lost: anything in the root of path, or
	// in every root when path is empty, may have changed.
	lost bool
}

// Watch watches the roots' directories until ctx is done. Changes are
// reported per root once quiet has passed without further changes, so that
// a `git checkout` or an agent writing several files is one batch. Only one
// Watch runs at a time.
func Watch(ctx context.Context, quiet time.Duration, changed func(Change)) error {
	watchMu.Lock()
	events, err := startEvents()
	if err != nil {
		watchMu.Unlock()
		return err
	}
	watched = map[string]bool{}
	watchMu.Unlock()
	defer func() {
		watchMu.Lock()
		watched = nil
		stopEvents()
		watchMu.Unlock()
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
		case batch, ok := <-events:
			if !ok {
				return nil
			}
			noted := false
			for _, ev := range batch {
				if note(ev, pending) {
					noted = true
				}
			}
			if noted {
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

// note records ev in pending and reports whether it concerned a root. Only
// paths in watched directories count. A new directory is watched from then
// on; a watched directory gone puts its root in doubt, since the files
// below it went with it.
func note(ev event, pending map[string]map[string]bool) bool {
	if ev.lost {
		return doubt(ev.path, pending)
	}
	r, rel, ok := rootOf(ev.path)
	if !ok {
		return false
	}
	watchMu.Lock()
	inWatched, dir := watched[filepath.Dir(ev.path)], watched[ev.path]
	watchMu.Unlock()
	if !inWatched {
		return false
	}
	if _, doubt := pending[r.id]; doubt && pending[r.id] == nil {
		return true
	}
	info, err := os.Lstat(ev.path)
	exists := err == nil
	switch {
	case dir && !exists:
		unwatch(ev.path)
		pending[r.id] = nil
		return true
	case exists && info.IsDir():
		if dir || !(ev.created || ev.removed) || !source.Watchable(r.cfg, rel) {
			return false
		}
		watchTree(r.cfg, ev.path)
		pending[r.id] = nil
		return true
	case ev.dir:
		return false
	}
	if pending[r.id] == nil {
		pending[r.id] = map[string]bool{}
	}
	pending[r.id][rel] = true
	return true
}

// doubt puts the root of abs in doubt, or every root when abs is empty.
func doubt(abs string, pending map[string]map[string]bool) bool {
	if abs != "" {
		r, _, ok := rootOf(abs)
		if !ok {
			return false
		}
		pending[r.id] = nil
		return true
	}
	mu.RLock()
	defer mu.RUnlock()
	for _, r := range roots {
		pending[r.id] = nil
	}
	return len(roots) > 0
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
	want := map[string]bool{}
	var paths []string
	mu.RLock()
	for _, r := range roots {
		paths = append(paths, r.cfg.Path)
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
	if watched == nil {
		return
	}
	watchRoots(paths)
	for dir := range watched {
		if !want[dir] {
			removeWatch(dir)
			delete(watched, dir)
		}
	}
	for dir := range want {
		if !watched[dir] && addWatch(dir) {
			watched[dir] = true
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
		if watched != nil && !watched[p] && addWatch(p) {
			watched[p] = true
		}
		watchMu.Unlock()
		return nil
	})
}

func unwatch(dir string) {
	watchMu.Lock()
	defer watchMu.Unlock()
	prefix := dir + string(filepath.Separator)
	for d := range watched {
		if d == dir || strings.HasPrefix(d, prefix) {
			removeWatch(d)
			delete(watched, d)
		}
	}
}
