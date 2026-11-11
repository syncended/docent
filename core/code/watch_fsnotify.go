//go:build !darwin

package code

import (
	"errors"

	"github.com/fsnotify/fsnotify"
)

// Outside macOS each watched directory is a watch of its own.

// watchBuffer is the per-directory event buffer on Windows, where a watch
// costs its buffer: the default 64 KB over the thousands of directories of
// a repository would not be lightweight. Overflows put the root in doubt.
const watchBuffer = 4096

var (
	// watcher is the running watcher; guarded by watchMu.
	watcher *fsnotify.Watcher
	// watchDone stops the forwarding of its events.
	watchDone chan struct{}
)

// startEvents starts the watcher. Called with watchMu held.
func startEvents() (<-chan []event, error) {
	w, err := fsnotify.NewBufferedWatcher(4096)
	if err != nil {
		return nil, err
	}
	out := make(chan []event)
	done := make(chan struct{})
	watcher, watchDone = w, done
	go func() {
		defer close(out)
		for {
			var ev event
			select {
			case <-done:
				return
			case e, ok := <-w.Events:
				if !ok {
					return
				}
				ev = event{path: e.Name, created: e.Has(fsnotify.Create), removed: e.Has(fsnotify.Remove | fsnotify.Rename)}
			case err, ok := <-w.Errors:
				if !ok {
					return
				}
				if !errors.Is(err, fsnotify.ErrEventOverflow) {
					continue
				}
				ev = event{lost: true}
			}
			select {
			case out <- []event{ev}:
			case <-done:
				return
			}
		}
	}()
	return out, nil
}

// stopEvents stops the watcher. Called with watchMu held.
func stopEvents() {
	close(watchDone)
	watcher.Close()
	watcher, watchDone = nil, nil
}

// watchRoots does nothing here: the directories are watched one by one.
func watchRoots([]string) {}

// addWatch watches dir. Called with watchMu held.
func addWatch(dir string) bool {
	return watcher.AddWith(dir, fsnotify.WithBufferSize(watchBuffer)) == nil
}

// removeWatch stops watching dir. Called with watchMu held.
func removeWatch(dir string) {
	watcher.Remove(dir)
}
