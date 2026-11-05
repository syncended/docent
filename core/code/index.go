// Package code indexes the code roots of the active project and searches them.
// It is the shell-independent core of Docent.
//
// The index is package state: call Open once at startup, then use the
// package functions from any goroutine. Only the active project's roots are
// loaded; other projects keep their caches on disk.
package code

import (
	"context"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sourcegraph/zoekt"
	zindex "github.com/sourcegraph/zoekt/index"
	zsearch "github.com/sourcegraph/zoekt/search"
	"golang.org/x/sync/errgroup"

	"github.com/syncended/docent/core/config"
	"github.com/syncended/docent/core/source"
	"github.com/syncended/docent/core/symbols"
)

// snapshotVersion is bumped when the cached root format or symbol extraction
// changes, invalidating caches.
const snapshotVersion = 2

var (
	cacheDir string

	mu    sync.RWMutex
	roots []*root

	reindexMu sync.Mutex
)

type root struct {
	cfg config.Root
	// id is stable for a root's path, so editing its subdirectories keeps
	// open files pointing at it; it names the zoekt repository.
	id string
	// key names the cache: the same folder in two projects with different
	// subdirectories keeps two.
	key string
	// seed is another cache of the same folder, used for the symbols of
	// unchanged files when this one has none yet.
	seed *snapshot
	// name is shown to the user.
	name      string
	files     []source.File
	syms      []entry
	indexedAt time.Time
	// text searches the root's shards, which have a directory of their own
	// so that only the active project's shards are loaded.
	text zsearch.ReadySearcher
}

type entry struct {
	symbols.Symbol
	File int32 // index into root.files
}

type snapshot struct {
	Version   int
	Files     []source.File
	Syms      []entry
	IndexedAt time.Time
}

// RootInfo describes an indexed root.
type RootInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Files     int       `json:"files"`
	Symbols   int       `json:"symbols"`
	IndexedAt time.Time `json:"indexedAt"`
}

// Progress reports indexing progress.
type Progress struct {
	Root  string `json:"root"`
	Done  int    `json:"done"`
	Total int    `json:"total"`
	// Listed is set when a root indexed for the first time has its files
	// listed: its tree and file search work from then on, before symbols
	// and text search.
	Listed bool `json:"listed,omitempty"`
}

// DefaultCacheDir returns <user cache dir>/docent.
func DefaultCacheDir() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "docent"), nil
}

// Open loads the cached index of the active project's roots, keeping caches
// in dir, and deletes the caches of roots no project has. Roots without a
// cache are empty until Reindex. Calling Open again replaces the roots; it
// waits for a running Reindex to finish.
func Open(cfg *config.Config, dir string) error {
	reindexMu.Lock()
	defer reindexMu.Unlock()
	cacheDir = dir
	var loaded []*root
	for _, rc := range config.ActiveProject(cfg).Roots {
		r := &root{cfg: rc, id: rootID(rc.Path), key: cacheKey(rc), name: filepath.Base(rc.Path)}
		if s, err := loadSnapshot(r.key); err == nil {
			r.files, r.syms, r.indexedAt = s.Files, s.Syms, s.IndexedAt
		} else {
			r.seed = loadSeed(r.id)
		}
		if err := os.MkdirAll(shardDir(r.key), 0o755); err != nil {
			closeText(loaded)
			return err
		}
		// Shards load in the background; see TextReady.
		searcher, err := zsearch.NewDirectorySearcherFast(shardDir(r.key))
		if err != nil {
			closeText(loaded)
			return err
		}
		r.text = searcher
		loaded = append(loaded, r)
	}
	mu.Lock()
	old := roots
	roots = loaded
	mu.Unlock()
	closeText(old)
	// Best effort: a cache that cannot be deleted now goes at the next Open.
	removeStale(cfg)
	return nil
}

// TextReady reports whether the text index has finished loading.
func TextReady() bool {
	mu.RLock()
	defer mu.RUnlock()
	for _, r := range roots {
		if !r.text.Ready() {
			return false
		}
	}
	return true
}

// Close releases the engine's resources.
func Close() {
	mu.Lock()
	old := roots
	roots = nil
	mu.Unlock()
	closeText(old)
}

func closeText(rs []*root) {
	for _, r := range rs {
		r.text.Close()
	}
}

// removeStale deletes the caches of roots that are in no project, such as a
// removed root or one whose subdirectories changed, and shards left in the
// shared directory used before projects.
func removeStale(cfg *config.Config) error {
	keep := map[string]bool{}
	for _, p := range cfg.Projects {
		for _, rc := range p.Roots {
			keep[cacheKey(rc)] = true
		}
	}
	var errs []error
	shards, _ := os.ReadDir(filepath.Join(cacheDir, "shards"))
	for _, e := range shards {
		if !keep[e.Name()] {
			errs = append(errs, os.RemoveAll(filepath.Join(cacheDir, "shards", e.Name())))
		}
	}
	snapshots, _ := os.ReadDir(filepath.Join(cacheDir, "roots"))
	for _, e := range snapshots {
		if id, ok := strings.CutSuffix(e.Name(), ".gob"); ok && !keep[id] {
			errs = append(errs, os.Remove(filepath.Join(cacheDir, "roots", e.Name())))
		}
	}
	return errors.Join(errs...)
}

// Roots describes the indexed roots.
func Roots() []RootInfo {
	mu.RLock()
	defer mu.RUnlock()
	infos := make([]RootInfo, len(roots))
	for i, r := range roots {
		infos[i] = RootInfo{ID: r.id, Name: r.name, Path: r.cfg.Path, Files: len(r.files), Symbols: len(r.syms), IndexedAt: r.indexedAt}
	}
	return infos
}

// Reindex brings the index of every root up to date. Unchanged files keep
// their symbols; the text index of a root is rebuilt only if it changed.
func Reindex(ctx context.Context, progress func(Progress)) error {
	reindexMu.Lock()
	defer reindexMu.Unlock()

	mu.RLock()
	current := append([]*root(nil), roots...)
	mu.RUnlock()

	var errs []error
	for i, old := range current {
		// A root without an index shows its files while the rest is built.
		listed := func(files []source.File) {
			if len(old.files) > 0 {
				return
			}
			mu.Lock()
			roots[i] = &root{cfg: old.cfg, id: old.id, key: old.key, name: old.name, files: files, text: old.text, seed: old.seed}
			mu.Unlock()
			if progress != nil {
				progress(Progress{Root: old.name, Total: len(files), Listed: true})
			}
		}
		r, err := reindexRoot(ctx, old, listed, progress)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", old.cfg.Path, err))
			continue
		}
		mu.Lock()
		roots[i] = r
		mu.Unlock()
		if err := saveSnapshot(r); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func reindexRoot(ctx context.Context, old *root, listed func([]source.File), progress func(Progress)) (*root, error) {
	files, err := source.List(ctx, old.cfg)
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	listed(files)

	type prevFile struct {
		file source.File
		syms []symbols.Symbol
	}
	prevFiles, prevSyms := old.files, old.syms
	if len(prevFiles) == 0 && old.seed != nil {
		prevFiles, prevSyms = old.seed.Files, old.seed.Syms
	}
	prev := make(map[string]*prevFile, len(prevFiles))
	for _, f := range prevFiles {
		prev[f.Path] = &prevFile{file: f}
	}
	for _, s := range prevSyms {
		p := prev[prevFiles[s.File].Path]
		p.syms = append(p.syms, s.Symbol)
	}

	unchanged := func(f source.File) bool {
		p, ok := prev[f.Path]
		return ok && p.file.Size == f.Size && p.file.ModTime.Equal(f.ModTime)
	}
	changed := len(files) != len(old.files) || !hasShards(old.key)
	for _, f := range files {
		if !changed && !unchanged(f) {
			changed = true
		}
	}
	r := &root{cfg: old.cfg, id: old.id, key: old.key, name: old.name, files: files, indexedAt: time.Now(), text: old.text}
	if !changed {
		r.syms = old.syms
		return r, nil
	}

	builder, err := zindex.NewBuilder(zindex.Options{
		IndexDir:              shardDir(r.key),
		ShardPrefixOverride:   r.key,
		RepositoryDescription: zoekt.Repository{Name: r.id},
		DisableCTags:          true,
		SizeMax:               source.MaxFileSize,
	})
	if err != nil {
		return nil, err
	}

	// Workers read and parse files; a single goroutine feeds the builder,
	// which is not safe for concurrent use.
	type result struct {
		i    int
		doc  zindex.Document
		syms []symbols.Symbol
	}
	jobs := make(chan int)
	results := make(chan result)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		defer close(jobs)
		for i := range files {
			select {
			case jobs <- i:
			case <-gctx.Done():
				return gctx.Err()
			}
		}
		return nil
	})
	var workers sync.WaitGroup
	for range runtime.NumCPU() {
		workers.Add(1)
		g.Go(func() error {
			defer workers.Done()
			for i := range jobs {
				f := files[i]
				content, err := os.ReadFile(filepath.Join(r.cfg.Path, filepath.FromSlash(f.Path)))
				if err != nil || source.IsBinary(content) {
					continue
				}
				var syms []symbols.Symbol
				if unchanged(f) {
					syms = prev[f.Path].syms
				} else if lang := symbols.Language(f.Path); lang != "" {
					syms, _ = symbols.Extract(gctx, lang, content)
				}
				select {
				case results <- result{i: i, doc: document(f.Path, content, syms), syms: syms}:
				case <-gctx.Done():
					return gctx.Err()
				}
			}
			return nil
		})
	}
	go func() {
		workers.Wait()
		close(results)
	}()

	// Keep draining results after a builder error so workers can exit.
	perFile := make([][]symbols.Symbol, len(files))
	done := 0
	var addErr error
	for res := range results {
		if addErr == nil {
			addErr = builder.Add(res.doc)
		}
		perFile[res.i] = res.syms
		done++
		if progress != nil && done%500 == 0 {
			progress(Progress{Root: r.name, Done: done, Total: len(files)})
		}
	}
	if err := errors.Join(g.Wait(), addErr); err != nil {
		return nil, err
	}
	if err := builder.Finish(); err != nil {
		return nil, err
	}
	if progress != nil {
		progress(Progress{Root: r.name, Done: len(files), Total: len(files)})
	}

	for i, syms := range perFile {
		for _, s := range syms {
			r.syms = append(r.syms, entry{Symbol: s, File: int32(i)})
		}
	}
	return r, nil
}

func document(path string, content []byte, syms []symbols.Symbol) zindex.Document {
	doc := zindex.Document{Name: path, Content: content}
	sorted := append([]symbols.Symbol(nil), syms...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Start < sorted[j].Start })
	for _, s := range sorted {
		if len(doc.Symbols) > 0 && doc.Symbols[len(doc.Symbols)-1].End > s.Start {
			continue // zoekt requires non-overlapping sections
		}
		doc.Symbols = append(doc.Symbols, zindex.DocumentSection{Start: s.Start, End: s.End})
		doc.SymbolsMetaData = append(doc.SymbolsMetaData, &zoekt.Symbol{Sym: s.Name, Kind: string(s.Kind), Parent: s.Parent})
	}
	return doc
}

func rootID(path string) string {
	sum := sha256.Sum256([]byte(path))
	return filepath.Base(path) + "-" + hex.EncodeToString(sum[:4])
}

// cacheKey names the cache of a root: its ID and a hash of its filters.
func cacheKey(rc config.Root) string {
	h := sha256.New()
	fmt.Fprintf(h, "%q\x00%q", rc.Include, rc.Exclude)
	return rootID(rc.Path) + "." + hex.EncodeToString(h.Sum(nil)[:4])
}

func shardDir(key string) string { return filepath.Join(cacheDir, "shards", key) }

func snapshotPath(key string) string {
	return filepath.Join(cacheDir, "roots", key+".gob")
}

func hasShards(key string) bool {
	m, _ := filepath.Glob(filepath.Join(shardDir(key), key+"_v*.zoekt"))
	return len(m) > 0
}

// loadSeed loads any cache of the root with the given ID, with other
// subdirectories or from before projects, or nil if there is none.
func loadSeed(id string) *snapshot {
	m, _ := filepath.Glob(filepath.Join(cacheDir, "roots", id+".*gob"))
	for _, path := range m {
		if s, err := loadSnapshot(strings.TrimSuffix(filepath.Base(path), ".gob")); err == nil {
			return s
		}
	}
	return nil
}

func loadSnapshot(id string) (*snapshot, error) {
	f, err := os.Open(snapshotPath(id))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var s snapshot
	if err := gob.NewDecoder(f).Decode(&s); err != nil {
		return nil, err
	}
	if s.Version != snapshotVersion {
		return nil, fs.ErrNotExist
	}
	return &s, nil
}

func saveSnapshot(r *root) error {
	path := snapshotPath(r.key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".snapshot-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	s := snapshot{Version: snapshotVersion, Files: r.files, Syms: r.syms, IndexedAt: r.indexedAt}
	if err := gob.NewEncoder(tmp).Encode(&s); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// resolve returns the root with the given ID and the absolute path of a
// file inside it. It rejects paths that escape the root.
func resolve(id, path string) (*root, string, error) {
	mu.RLock()
	defer mu.RUnlock()
	for _, r := range roots {
		if r.id != id {
			continue
		}
		abs := filepath.Join(r.cfg.Path, filepath.FromSlash(path))
		rel, err := filepath.Rel(r.cfg.Path, abs)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, "", fmt.Errorf("path %q is outside root %q", path, id)
		}
		return r, abs, nil
	}
	return nil, "", fmt.Errorf("unknown root %q", id)
}

// ReadFile returns the content of a file in a root.
func ReadFile(root, path string) (string, error) {
	_, abs, err := resolve(root, path)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	if source.IsBinary(b) {
		return "", fmt.Errorf("%s is a binary file", path)
	}
	return string(b), nil
}

// Entry is a file or folder directly inside a folder of a root.
type Entry struct {
	Name string `json:"name"`
	Dir  bool   `json:"dir"`
}

// ListDir returns the indexed files and folders directly inside dir, a
// slash-separated path relative to the root ("" is the root itself).
// Folders come first; only folders with indexed files are listed.
func ListDir(id, dir string) ([]Entry, error) {
	mu.RLock()
	defer mu.RUnlock()
	var r *root
	for _, c := range roots {
		if c.id == id {
			r = c
		}
	}
	if r == nil {
		return nil, fmt.Errorf("unknown root %q", id)
	}
	prefix := strings.Trim(dir, "/")
	if prefix != "" {
		prefix += "/"
	}
	seen := map[string]bool{}
	var entries []Entry
	for _, f := range r.files {
		rest, ok := strings.CutPrefix(f.Path, prefix)
		if !ok {
			continue
		}
		name, _, sub := strings.Cut(rest, "/")
		if seen[name] {
			continue
		}
		seen[name] = true
		entries = append(entries, Entry{Name: name, Dir: sub})
	}
	if entries == nil && prefix != "" {
		return nil, fmt.Errorf("no indexed files in %q", dir)
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Dir != entries[j].Dir {
			return entries[i].Dir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	return entries, nil
}

// WriteFile replaces the content of an existing file in a root.
func WriteFile(root, path, content string) error {
	_, abs, err := resolve(root, path)
	if err != nil {
		return err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return err
	}
	return os.WriteFile(abs, []byte(content), info.Mode().Perm())
}
