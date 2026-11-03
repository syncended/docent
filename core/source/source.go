// Package source enumerates the files of a code root.
package source

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/syncended/docent/core/config"
)

// MaxFileSize is the largest file that is indexed.
const MaxFileSize = 1 << 20

// File is a file inside a root.
type File struct {
	// Path is relative to the root, slash-separated.
	Path    string
	Size    int64
	ModTime time.Time
}

// skipDirs are never descended into when walking without git, in addition
// to hidden directories.
var skipDirs = map[string]bool{
	"node_modules": true, "__pycache__": true,
	"build": true, "target": true, "dist": true, "out": true,
}

// List returns the files of root that should be indexed. Git checkouts are
// listed with git, honoring .gitignore; other directories are walked.
func List(ctx context.Context, root config.Root) ([]File, error) {
	paths, err := gitFiles(ctx, root)
	if err != nil {
		paths, err = walkFiles(ctx, root)
		if err != nil {
			return nil, err
		}
	}
	files := make([]File, 0, len(paths))
	for _, p := range paths {
		if excluded(root, p) {
			continue
		}
		info, err := os.Lstat(filepath.Join(root.Path, filepath.FromSlash(p)))
		if err != nil || !info.Mode().IsRegular() || info.Size() > MaxFileSize || info.Size() == 0 {
			continue
		}
		files = append(files, File{Path: p, Size: info.Size(), ModTime: info.ModTime()})
	}
	return files, nil
}

func gitFiles(ctx context.Context, root config.Root) ([]string, error) {
	args := []string{"-C", root.Path, "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--"}
	args = append(args, root.Include...)
	out, err := exec.CommandContext(ctx, "git", args...).Output()
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, p := range bytes.Split(out, []byte{0}) {
		if len(p) > 0 {
			paths = append(paths, string(p))
		}
	}
	return paths, nil
}

func walkFiles(ctx context.Context, root config.Root) ([]string, error) {
	starts := root.Include
	if len(starts) == 0 {
		starts = []string{"."}
	}
	var paths []string
	for _, start := range starts {
		dir := filepath.Join(root.Path, filepath.FromSlash(start))
		err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if d.IsDir() {
				if p != dir && (skipDirs[d.Name()] || strings.HasPrefix(d.Name(), ".")) {
					return filepath.SkipDir
				}
				return nil
			}
			rel, err := filepath.Rel(root.Path, p)
			if err != nil {
				return nil
			}
			paths = append(paths, filepath.ToSlash(rel))
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return paths, nil
}

func excluded(root config.Root, p string) bool {
	for _, pattern := range root.Exclude {
		if ok, _ := doublestar.Match(pattern, p); ok {
			return true
		}
		// A pattern naming a directory excludes everything below it.
		if ok, _ := doublestar.Match(path.Join(pattern, "**"), p); ok {
			return true
		}
	}
	return false
}

// IsBinary reports whether content looks like a binary file.
func IsBinary(content []byte) bool {
	n := min(len(content), 8000)
	return bytes.IndexByte(content[:n], 0) >= 0
}
