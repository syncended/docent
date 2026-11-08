package code

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// startWatch runs Watch for the test and returns its changes.
func startWatch(t *testing.T) <-chan Change {
	t.Helper()
	changes := make(chan Change, 16)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := Watch(ctx, 100*time.Millisecond, func(c Change) { changes <- c }); err != nil {
			t.Error(err)
		}
	}()
	t.Cleanup(func() { cancel(); <-done })
	// Watching starts asynchronously; give it a moment before changing files.
	deadline := time.Now().Add(2 * time.Second)
	for {
		watchMu.Lock()
		n := len(watched)
		watchMu.Unlock()
		if n > 0 || time.Now().After(deadline) {
			return changes
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func next(t *testing.T, changes <-chan Change) Change {
	t.Helper()
	select {
	case c := <-changes:
		return c
	case <-time.After(5 * time.Second):
		t.Fatal("no change reported")
		return Change{}
	}
}

func TestWatch(t *testing.T) {
	id := setup(t)
	root := Roots()[0].Path
	changes := startWatch(t)

	// A new file and an edited one come as one batch.
	writeFiles(t, root, map[string]string{"shop/cart.go": "package shop\n\ntype Cart struct{}\n", "README.md": "# changed\n"})
	c := next(t, changes)
	if c.Root != id || !slices.Equal(c.Paths, []string{"README.md", "shop/cart.go"}) {
		t.Errorf("change = %+v", c)
	}

	// A deleted file is reported by its path.
	if err := os.Remove(filepath.Join(root, "shop", "cart.go")); err != nil {
		t.Fatal(err)
	}
	if c := next(t, changes); !slices.Equal(c.Paths, []string{"shop/cart.go"}) {
		t.Errorf("after delete: %+v", c)
	}

	// Files in a new directory are watched too; a directory appearing puts
	// the root in doubt (Paths nil) since its files may predate the watch.
	writeFiles(t, root, map[string]string{"pay/pay.go": "package pay\n"})
	if c := next(t, changes); c.Paths != nil {
		t.Errorf("new directory: %+v, want the root in doubt", c)
	}
	writeFiles(t, root, map[string]string{"pay/card.go": "package pay\n"})
	if c := next(t, changes); !slices.Equal(c.Paths, []string{"pay/card.go"}) {
		t.Errorf("in new directory: %+v", c)
	}

	// Hidden directories are not watched.
	writeFiles(t, root, map[string]string{".git/HEAD": "ref: refs/heads/trunk\n"})
	select {
	case c := <-changes:
		t.Errorf("hidden directory reported: %+v", c)
	case <-time.After(300 * time.Millisecond):
	}
}

func TestWriteFileCreates(t *testing.T) {
	id := setup(t)
	root := Roots()[0].Path
	if err := os.RemoveAll(filepath.Join(root, "shop")); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(id, "shop/order.go", "package shop\n"); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadFile(id, "shop/order.go"); err != nil || got != "package shop\n" {
		t.Errorf("ReadFile = %q, %v", got, err)
	}
}
