// Package syntax highlights source code with tree-sitter, for languages the
// frontend has no good grammar for.
package syntax

import (
	"context"
	"fmt"
	"path"
	"slices"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/kotlin"
	"github.com/smacker/go-tree-sitter/protobuf"
)

// Span is a highlighted range. Offsets are in UTF-16 code units, as used by
// the editor.
type Span struct {
	From  int    `json:"from"`
	To    int    `json:"to"`
	Class string `json:"class"`
}

type language struct {
	exts     []string
	language *sitter.Language
	// query captures ranges with the class name, e.g. @keyword. Nested
	// captures are all returned; the innermost wins when rendered.
	query string

	once sync.Once
	q    *sitter.Query
	err  error
}

var languages = []*language{
	{
		exts:     []string{".kt", ".kts"},
		language: kotlin.GetLanguage(),
		query: `
(line_comment) @comment
(multiline_comment) @comment
(string_literal) @string
(character_literal) @string
(interpolated_identifier) @variable
(interpolated_expression) @variable
["$" "${"] @punctuation
[(integer_literal) (long_literal) (hex_literal) (bin_literal) (real_literal) (unsigned_literal)] @number
(boolean_literal) @constant
"null" @constant
(type_identifier) @type
(annotation) @annotation
(function_declaration (simple_identifier) @function)
(call_expression (simple_identifier) @function)
(call_expression (navigation_expression (navigation_suffix (simple_identifier) @function)))
(class_parameter (simple_identifier) @property)
(variable_declaration (simple_identifier) @property)
(parameter (simple_identifier) @parameter)
(enum_entry (simple_identifier) @constant)
(package_header (identifier) @namespace)
(import_header (identifier) @namespace)
(this_expression) @keyword
(super_expression) @keyword
[(class_modifier) (visibility_modifier) (function_modifier) (inheritance_modifier) (member_modifier)
 (property_modifier) (parameter_modifier) (platform_modifier) (variance_modifier) (reification_modifier)] @keyword
["package" "import" "class" "interface" "object" "fun" "val" "var" "if" "else" "when" "for" "while" "do"
 "return" "throw" "try" "catch" "finally" "in" "is" "as" "typealias" "companion" "break" "continue"
 "constructor" "init" "by" "get" "set" "where" "enum"] @keyword
`,
	},
	{
		exts:     []string{".proto"},
		language: protobuf.GetLanguage(),
		query: `
(comment) @comment
(string) @string
"\"proto3\"" @string
[(int_lit) (float_lit)] @number
[(true) (false)] @constant
[(message_name) (enum_name) (service_name) (message_or_enum_type) (key_type)] @type
["double" "float" "int32" "int64" "uint32" "uint64" "sint32" "sint64" "fixed32" "fixed64"
 "sfixed32" "sfixed64" "bool" "string" "bytes"] @type
(rpc_name) @function
(enum_field (identifier) @constant)
(field (identifier) @property)
(map_field (identifier) @property)
(oneof_field (identifier) @property)
(package (full_ident) @namespace)
["syntax" "package" "import" "option" "message" "enum" "service" "rpc" "returns" "stream"
 "repeated" "optional" "map" "oneof" "reserved"] @keyword
`,
	},
}

// Supported reports whether path is highlighted by this package.
func Supported(path string) bool {
	return find(path) != nil
}

func find(p string) *language {
	ext := strings.ToLower(path.Ext(p))
	for _, l := range languages {
		if slices.Contains(l.exts, ext) {
			return l
		}
	}
	return nil
}

func (l *language) compile() (*sitter.Query, error) {
	l.once.Do(func() {
		l.q, l.err = sitter.NewQuery([]byte(l.query), l.language)
		if l.err != nil {
			l.err = fmt.Errorf("compile highlight query: %w", l.err)
		}
	})
	return l.q, l.err
}

// Highlight returns highlighted spans of src, a file at path, sorted by From.
func Highlight(ctx context.Context, path string, src []byte) ([]Span, error) {
	l := find(path)
	if l == nil {
		return nil, fmt.Errorf("no highlighting for %s", path)
	}
	q, err := l.compile()
	if err != nil {
		return nil, err
	}
	parser := sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(l.language)
	tree, err := parser.ParseCtx(ctx, nil, src)
	if err != nil {
		return nil, err
	}
	defer tree.Close()
	qc := sitter.NewQueryCursor()
	defer qc.Close()
	qc.Exec(q, tree.RootNode())

	type byteSpan struct {
		from, to uint32
		class    string
	}
	seen := map[byteSpan]bool{}
	var spans []byteSpan
	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}
		for _, c := range m.Captures {
			s := byteSpan{c.Node.StartByte(), c.Node.EndByte(), q.CaptureNameForId(c.Index)}
			if s.from < s.to && !seen[s] {
				seen[s] = true
				spans = append(spans, s)
			}
		}
	}
	// Outer ranges first, so inner ones are rendered on top.
	sort.Slice(spans, func(i, j int) bool {
		if spans[i].from != spans[j].from {
			return spans[i].from < spans[j].from
		}
		return spans[i].to > spans[j].to
	})

	out := make([]Span, len(spans))
	toUTF16 := utf16Offsets(src)
	for i, s := range spans {
		out[i] = Span{From: toUTF16(s.from), To: toUTF16(s.to), Class: s.class}
	}
	return out, nil
}

// utf16Offsets converts byte offsets in src to UTF-16 offsets.
func utf16Offsets(src []byte) func(uint32) int {
	ascii := true
	for _, b := range src {
		if b >= utf8.RuneSelf {
			ascii = false
			break
		}
	}
	if ascii {
		return func(b uint32) int { return int(b) }
	}
	index := make([]int32, len(src)+1)
	n := int32(0)
	for i := 0; i < len(src); {
		r, size := utf8.DecodeRune(src[i:])
		for j := 0; j < size; j++ {
			index[i+j] = n
		}
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
		i += size
	}
	index[len(src)] = n
	return func(b uint32) int { return int(index[min(int(b), len(src))]) }
}
