// Package fuzzy matches queries against names the way IDE "go to symbol"
// dialogs do: exact and prefix matches first, then camel-hump matches
// ("OrSt" → "OrderStatus"), then substrings. Loose subsequences that skip
// across word boundaries ("OrSt" → "OrderRequest") do not match.
package fuzzy

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Match tiers; a higher tier always ranks above a lower one.
const (
	tierSubstring = 1 + iota
	tierHumps
	tierWordSubstring
	tierHumpsFromStart
	tierPrefix
	tierExact
)

// Score of a match; higher is better. Zero means no match.
type Score int

// Match matches query against name, case-insensitively. It returns the score
// and the byte offsets in name of the matched characters, for highlighting.
func Match(query, name string) (Score, []int) {
	if query == "" {
		return 0, nil
	}
	if !isSubsequence(query, name) {
		return 0, nil
	}
	lq, ln := strings.ToLower(query), strings.ToLower(name)

	switch {
	case lq == ln:
		return score(tierExact, name, query == name), span(0, len(query))
	case strings.HasPrefix(ln, lq):
		return score(tierPrefix, name, strings.HasPrefix(name, query)), span(0, len(query))
	}
	starts := wordStarts(name)
	if pos := humps(lq, ln, starts); pos != nil {
		if pos[0] == 0 {
			return score(tierHumpsFromStart, name, sameCase(query, name, pos)), pos
		}
		return score(tierHumps, name, sameCase(query, name, pos)), pos
	}
	if i := indexAtWordStart(ln, lq, starts); i >= 0 {
		pos := span(i, len(query))
		return score(tierWordSubstring, name, sameCase(query, name, pos)), pos
	}
	if i := strings.Index(ln, lq); i >= 0 {
		pos := span(i, len(query))
		return score(tierSubstring, name, sameCase(query, name, pos)), pos
	}
	return 0, nil
}

// sameCase reports whether the matched characters have the query's case.
func sameCase(query, name string, pos []int) bool {
	for i, p := range pos {
		if query[i] != name[p] {
			return false
		}
	}
	return true
}

// score combines the tier with tie-breakers: exact case, then shorter names.
func score(tier int, name string, exactCase bool) Score {
	s := tier * 10000
	if exactCase {
		s += 1000
	}
	s += 999 - min(len(name), 999)
	return Score(s)
}

func span(start, n int) []int {
	pos := make([]int, n)
	for i := range pos {
		pos[i] = start + i
	}
	return pos
}

// isSubsequence is a cheap, allocation-free prefilter.
func isSubsequence(query, name string) bool {
	j := 0
	for i := 0; i < len(query); i++ {
		c := lower(query[i])
		for j < len(name) && lower(name[j]) != c {
			j++
		}
		if j == len(name) {
			return false
		}
		j++
	}
	return true
}

func lower(b byte) byte {
	if 'A' <= b && b <= 'Z' {
		return b + 'a' - 'A'
	}
	return b
}

// wordStarts marks byte offsets where a word begins: the first character,
// an upper-case letter after a lower-case one or a digit, the last upper-case
// letter of an acronym followed by a lower-case one ("HTTPServer" → H, S),
// and any letter or digit after a separator.
func wordStarts(name string) []bool {
	starts := make([]bool, len(name))
	var prev rune = -1
	for i, r := range name {
		next, _ := utf8.DecodeRuneInString(name[i+utf8.RuneLen(r):])
		switch {
		case !unicode.IsLetter(r) && !unicode.IsDigit(r):
		case prev == -1:
			starts[i] = true
		case !unicode.IsLetter(prev) && !unicode.IsDigit(prev):
			starts[i] = true
		case unicode.IsUpper(r) && (unicode.IsLower(prev) || unicode.IsDigit(prev)):
			starts[i] = true
		case unicode.IsUpper(r) && unicode.IsUpper(prev) && unicode.IsLower(next):
			starts[i] = true
		case unicode.IsDigit(r) && !unicode.IsDigit(prev):
			starts[i] = true
		}
		prev = r
	}
	return starts
}

// humps matches lq against ln where every query character either continues
// the current match or starts at a word start. The first character must be
// at a word start. It returns matched positions, or nil.
func humps(lq, ln string, starts []bool) []int {
	pos := make([]int, 0, len(lq))
	var try func(qi, from int) bool
	try = func(qi, from int) bool {
		if qi == len(lq) {
			return true
		}
		// Continue the current word.
		if qi > 0 && from < len(ln) && ln[from] == lq[qi] {
			pos = append(pos, from)
			if try(qi+1, from+1) {
				return true
			}
			pos = pos[:len(pos)-1]
		}
		// Jump to a later word start. Separators in the query ("." in
		// "order.proto") may jump to the same separator anywhere.
		sep := !isAlnum(lq[qi])
		for i := from; i < len(ln); i++ {
			if (qi > 0 && i == from) || !starts[i] && !sep || ln[i] != lq[qi] {
				continue
			}
			pos = append(pos, i)
			if try(qi+1, i+1) {
				return true
			}
			pos = pos[:len(pos)-1]
		}
		return false
	}
	if try(0, 0) {
		return pos
	}
	return nil
}

func indexAtWordStart(ln, lq string, starts []bool) int {
	for i := 0; i+len(lq) <= len(ln); i++ {
		if starts[i] && ln[i:i+len(lq)] == lq {
			return i
		}
	}
	return -1
}

func isAlnum(b byte) bool {
	return 'a' <= b && b <= 'z' || 'A' <= b && b <= 'Z' || '0' <= b && b <= '9' || b >= utf8.RuneSelf
}
