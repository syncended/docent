package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/syncended/docent/core/code"
)

// Docent's log goes to docent.log in the cache directory as well as stderr:
// launched from the desktop, a background app has no console, and a startup
// failure would otherwise leave no trace (DESIGN.md, Technical decisions).

// logLimit is the size past which the log file is set aside at startup.
const logLimit = 1 << 20

// openLog sends the standard log to the log file and stderr. The file is
// appended to, and once it exceeds logLimit the previous one is kept as
// docent.log.old. It stays open for the life of the process.
func openLog() error {
	dir, err := code.DefaultCacheDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, "docent.log")
	if info, err := os.Stat(path); err == nil && info.Size() > logLimit {
		// Best effort: a running Docent may hold the file on Windows.
		_ = os.Rename(path, path+".old")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open log: %w", err)
	}
	// The file first: without a console, writes to stderr fail and stop
	// the MultiWriter.
	log.SetOutput(io.MultiWriter(f, os.Stderr))
	return nil
}
