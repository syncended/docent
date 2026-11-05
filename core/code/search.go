package code

import (
	"container/heap"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"regexp/syntax"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sourcegraph/zoekt"
	"github.com/sourcegraph/zoekt/query"
	"golang.org/x/sync/errgroup"

	"github.com/syncended/docent/core/fuzzy"
	"github.com/syncended/docent/core/symbols"
)

// Tab identifies a palette tab.
type Tab string

const (
	TabAll     Tab = "all"
	TabTypes   Tab = "types"
	TabSymbols Tab = "symbols"
	TabFiles   Tab = "files"
	TabText    Tab = "text"
)

// Hit is a search result.
type Hit struct {
	Tab      Tab    `json:"tab"`
	Root     string `json:"root"` // root ID
	RootName string `json:"rootName"`
	Path     string `json:"path"`
	// Line is 1-based; 0 if the hit is a whole file.
	Line   int `json:"line"`
	Column int `json:"column"`
	// Title is the symbol or file name; TitleMatches are matched byte offsets in it.
	Title        string `json:"title"`
	TitleMatches []int  `json:"titleMatches,omitempty"`
	Kind         string `json:"kind,omitempty"`
	// Container is the enclosing symbol, or the directory for files.
	Container string   `json:"container,omitempty"`
	Snippet   *Snippet `json:"snippet,omitempty"`

	score int
}

// Snippet is a fragment of a file around a hit.
type Snippet struct {
	StartLine int      `json:"startLine"` // 1-based line of Lines[0]
	Lines     []string `json:"lines"`
	Matches   []Range  `json:"matches,omitempty"`
}

// Range is a match within a snippet: Lines[Line][Start:End], byte offsets.
type Range struct {
	Line  int `json:"line"`
	Start int `json:"start"`
	End   int `json:"end"`
}

// Section is a group of hits of one tab, used by the All tab.
type Section struct {
	Tab  Tab   `json:"tab"`
	Hits []Hit `json:"hits"`
}

const (
	snippetBefore = 1
	snippetAfter  = 2
	maxLineLen    = 300
)

// SearchTypes finds classes, interfaces, enums, structs, proto messages and
// other types by fuzzy name. See SearchSymbols for the query syntax.
func SearchTypes(q string, limit int) []Hit {
	return searchSymbols(q, TabTypes, symbols.Kind.IsType, limit)
}

// SearchSymbols finds declarations of any kind by fuzzy name. A query of the
// form "Parent.name" also matches the enclosing declaration.
func SearchSymbols(q string, limit int) []Hit {
	return searchSymbols(q, TabSymbols, func(symbols.Kind) bool { return true }, limit)
}

// searchMembers finds declarations that are not types.
func searchMembers(q string, limit int) []Hit {
	return searchSymbols(q, TabSymbols, func(k symbols.Kind) bool { return !k.IsType() }, limit)
}

func searchSymbols(q string, tab Tab, keep func(symbols.Kind) bool, limit int) []Hit {
	q = strings.TrimSpace(q)
	parentQ, nameQ := "", q
	if i := strings.LastIndex(q, "."); i > 0 && i < len(q)-1 {
		parentQ, nameQ = q[:i], q[i+1:]
	}
	if nameQ == "" {
		return nil
	}

	top := newTopK(limit)
	mu.RLock()
	for _, r := range roots {
		for i := range r.syms {
			s := &r.syms[i]
			if !keep(s.Kind) {
				continue
			}
			score, pos := fuzzy.Match(nameQ, s.Name)
			if score == 0 {
				continue
			}
			if parentQ != "" {
				ps, _ := fuzzy.Match(parentQ, s.Parent)
				if ps == 0 {
					continue
				}
				score += ps / 100
			}
			if !top.accepts(int(score)) {
				continue
			}
			f := r.files[s.File]
			top.push(Hit{
				Tab: tab, Root: r.id, RootName: r.name, Path: f.Path,
				Line: s.Line, Column: s.Column, Title: s.Name, TitleMatches: pos,
				Kind: string(s.Kind), Container: s.Parent, score: int(score),
			})
		}
	}
	mu.RUnlock()

	hits := top.sorted()
	attachSnippets(hits)
	return hits
}

// SearchFiles finds files by fuzzy name, or by path if the query contains
// "/". A ":line" suffix is carried over to the hits.
func SearchFiles(q string, limit int) []Hit {
	q = strings.TrimSpace(q)
	line := 0
	if i := strings.LastIndex(q, ":"); i > 0 {
		if n, err := strconv.Atoi(q[i+1:]); err == nil {
			q, line = q[:i], n
		}
	}
	if q == "" {
		return nil
	}
	byPath := strings.Contains(q, "/")

	top := newTopK(limit)
	mu.RLock()
	for _, r := range roots {
		for _, f := range r.files {
			target := path.Base(f.Path)
			if byPath {
				target = f.Path
			}
			score, pos := fuzzy.Match(q, target)
			if score == 0 {
				continue
			}
			// Prefer shallower paths among equal names.
			s := int(score)*100 - min(strings.Count(f.Path, "/"), 99)
			if !top.accepts(s) {
				continue
			}
			h := Hit{
				Tab: TabFiles, Root: r.id, RootName: r.name, Path: f.Path, Line: line,
				Title: path.Base(f.Path), Container: path.Dir(f.Path), score: s,
			}
			if byPath {
				h.Title, h.Container = f.Path, ""
			}
			h.TitleMatches = pos
			top.push(h)
		}
	}
	mu.RUnlock()
	return top.sorted()
}

// SearchText finds lines containing q, ignoring case.
func SearchText(ctx context.Context, q string, limit int) ([]Hit, error) {
	return searchLiteral(ctx, q, false, limit)
}

// SearchTextMatchCase finds lines containing q, matching case.
func SearchTextMatchCase(ctx context.Context, q string, limit int) ([]Hit, error) {
	return searchLiteral(ctx, q, true, limit)
}

// SearchRegex finds lines matching the regular expression pattern, ignoring case.
func SearchRegex(ctx context.Context, pattern string, limit int) ([]Hit, error) {
	return searchPattern(ctx, pattern, false, limit)
}

// SearchRegexMatchCase finds lines matching the regular expression pattern,
// matching case.
func SearchRegexMatchCase(ctx context.Context, pattern string, limit int) ([]Hit, error) {
	return searchPattern(ctx, pattern, true, limit)
}

func searchLiteral(ctx context.Context, q string, caseSensitive bool, limit int) ([]Hit, error) {
	if q == "" {
		return nil, nil
	}
	return searchText(ctx, &query.Substring{Pattern: q, Content: true, CaseSensitive: caseSensitive}, limit, 3)
}

func searchPattern(ctx context.Context, pattern string, caseSensitive bool, limit int) ([]Hit, error) {
	if pattern == "" {
		return nil, nil
	}
	zq, err := regexQuery(pattern, caseSensitive)
	if err != nil {
		return nil, err
	}
	return searchText(ctx, zq, limit, 3)
}

func regexQuery(pattern string, caseSensitive bool) (query.Q, error) {
	re, err := syntax.Parse(pattern, syntax.ClassNL|syntax.PerlX|syntax.UnicodeGroups)
	if err != nil {
		return nil, fmt.Errorf("invalid regular expression: %w", err)
	}
	return &query.Regexp{Regexp: re, Content: true, CaseSensitive: caseSensitive}, nil
}

func searchText(ctx context.Context, zq query.Q, limit, maxPerFile int) ([]Hit, error) {
	mu.RLock()
	active := append([]*root(nil), roots...)
	mu.RUnlock()

	opts := &zoekt.SearchOptions{
		MaxDocDisplayCount:   limit,
		MaxMatchDisplayCount: limit * maxPerFile,
		ShardMaxMatchCount:   10000,
		TotalMaxMatchCount:   100000,
		NumContextLines:      snippetBefore,
		MaxWallTime:          3 * time.Second,
	}
	// Each root has its own searcher; their best files are merged by score.
	type rootFile struct {
		fm   zoekt.FileMatch
		name string
	}
	results := make([][]rootFile, len(active))
	g, gctx := errgroup.WithContext(ctx)
	for i, r := range active {
		g.Go(func() error {
			res, err := r.text.Search(gctx, zq, opts)
			if err != nil {
				return err
			}
			for _, fm := range res.Files {
				results[i] = append(results[i], rootFile{fm: fm, name: r.name})
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	var files []rootFile
	for _, rs := range results {
		files = append(files, rs...)
	}
	sort.SliceStable(files, func(i, j int) bool { return files[i].fm.Score > files[j].fm.Score })

	var hits []Hit
	for _, f := range files {
		n := 0
		for _, lm := range f.fm.LineMatches {
			if lm.FileName || n == maxPerFile {
				continue
			}
			n++
			hits = append(hits, textHit(f.fm, lm, f.name))
		}
	}
	return hits[:min(len(hits), limit)], nil
}

func textHit(fm zoekt.FileMatch, lm zoekt.LineMatch, rootName string) Hit {
	before := splitLines(lm.Before)
	sn := &Snippet{StartLine: lm.LineNumber - len(before)}
	sn.Lines = append(sn.Lines, before...)
	sn.Lines = append(sn.Lines, strings.TrimRight(string(lm.Line), "\r\n"))
	sn.Lines = append(sn.Lines, splitLines(lm.After)...)
	for _, f := range lm.LineFragments {
		sn.Matches = append(sn.Matches, Range{Line: len(before), Start: f.LineOffset, End: f.LineOffset + f.MatchLength})
	}
	clampSnippet(sn)
	column := 0
	if len(lm.LineFragments) > 0 {
		column = lm.LineFragments[0].LineOffset
	}
	return Hit{
		Tab: TabText, Root: fm.Repository, RootName: rootName, Path: fm.FileName,
		Line: lm.LineNumber, Column: column, Title: path.Base(fm.FileName),
		Container: path.Dir(fm.FileName), Snippet: sn,
	}
}

func splitLines(b []byte) []string {
	s := strings.TrimRight(string(b), "\n")
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], "\r")
	}
	return lines
}

// SearchAll runs the Types, Symbols, Files and Text searches together,
// returning a section per tab with at most limit hits each.
func SearchAll(ctx context.Context, q string, limit int) []Section {
	sections := []Section{{Tab: TabTypes}, {Tab: TabSymbols}, {Tab: TabFiles}, {Tab: TabText}}
	var wg sync.WaitGroup
	wg.Go(func() { sections[0].Hits = SearchTypes(q, limit) })
	wg.Go(func() { sections[1].Hits = searchMembers(q, limit) })
	wg.Go(func() { sections[2].Hits = SearchFiles(q, limit) })
	// Trigram search needs at least three characters to be selective.
	if len(strings.TrimSpace(q)) >= 3 {
		wg.Go(func() {
			sections[3].Hits, _ = searchText(ctx, &query.Substring{Pattern: q, Content: true}, limit, 1)
		})
	}
	wg.Wait()
	out := sections[:0]
	for _, s := range sections {
		if len(s.Hits) > 0 {
			out = append(out, s)
		}
	}
	return out
}

// Definitions returns declarations named exactly ident.
func Definitions(ident string, limit int) []Hit {
	var hits []Hit
	for _, h := range SearchSymbols(ident, limit*4) {
		if h.Title == ident {
			hits = append(hits, h)
		}
	}
	// Types first: "go to definition" on a type name should land on the type.
	sort.SliceStable(hits, func(i, j int) bool {
		return symbols.Kind(hits[i].Kind).IsType() && !symbols.Kind(hits[j].Kind).IsType()
	})
	return hits[:min(len(hits), limit)]
}

// Usages returns lines where ident occurs as a whole word.
func Usages(ctx context.Context, ident string, limit int) ([]Hit, error) {
	zq, err := regexQuery(`\b`+regexp.QuoteMeta(ident)+`\b`, true)
	if err != nil {
		return nil, err
	}
	return searchText(ctx, zq, limit, 20)
}

func rootPath(id string) string {
	mu.RLock()
	defer mu.RUnlock()
	for _, r := range roots {
		if r.id == id {
			return r.cfg.Path
		}
	}
	return ""
}

// attachSnippets reads the lines around each hit.
func attachSnippets(hits []Hit) {
	cache := map[string][]string{}
	for i := range hits {
		h := &hits[i]
		abs := filepath.Join(rootPath(h.Root), filepath.FromSlash(h.Path))
		lines, ok := cache[abs]
		if !ok {
			b, err := os.ReadFile(abs)
			if err == nil {
				lines = strings.Split(string(b), "\n")
			}
			cache[abs] = lines
		}
		if h.Line < 1 || h.Line > len(lines) {
			continue
		}
		from := max(h.Line-1-snippetBefore, 0)
		to := min(h.Line+snippetAfter, len(lines))
		sn := &Snippet{StartLine: from + 1}
		for _, l := range lines[from:to] {
			sn.Lines = append(sn.Lines, strings.TrimRight(l, "\r"))
		}
		sn.Matches = []Range{{Line: h.Line - 1 - from, Start: h.Column, End: h.Column + len(h.Title)}}
		clampSnippet(sn)
		h.Snippet = sn
	}
}

// clampSnippet truncates very long lines (minified code) and drops matches
// past the cut.
func clampSnippet(sn *Snippet) {
	for i, l := range sn.Lines {
		if len(l) > maxLineLen {
			sn.Lines[i] = l[:maxLineLen]
		}
	}
	ms := sn.Matches[:0]
	for _, m := range sn.Matches {
		if m.End <= maxLineLen {
			ms = append(ms, m)
		}
	}
	sn.Matches = ms
}

// topK keeps the best hits by score.
type topK struct {
	limit int
	h     hitHeap
}

func newTopK(limit int) *topK { return &topK{limit: limit} }

func (t *topK) accepts(score int) bool {
	if t.limit <= 0 {
		return false
	}
	return len(t.h) < t.limit || score > t.h[0].score
}

func (t *topK) push(h Hit) {
	heap.Push(&t.h, h)
	if len(t.h) > t.limit {
		heap.Pop(&t.h)
	}
}

func (t *topK) sorted() []Hit {
	hits := []Hit(t.h)
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		return hits[i].Path < hits[j].Path
	})
	return hits
}

type hitHeap []Hit

func (h hitHeap) Len() int           { return len(h) }
func (h hitHeap) Less(i, j int) bool { return h[i].score < h[j].score }
func (h hitHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *hitHeap) Push(x any)        { *h = append(*h, x.(Hit)) }
func (h *hitHeap) Pop() any {
	old := *h
	x := old[len(old)-1]
	*h = old[:len(old)-1]
	return x
}
