// Package symbols extracts named declarations from source code with tree-sitter.
package symbols

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

// Kind is the kind of a declaration.
type Kind string

const (
	Class     Kind = "class"
	Interface Kind = "interface"
	Enum      Kind = "enum"
	Struct    Kind = "struct"
	Union     Kind = "union"
	Object    Kind = "object"
	Message   Kind = "message"
	Service   Kind = "service"
	TypeAlias Kind = "type"
	Namespace Kind = "namespace"
	Function  Kind = "function"
	Method    Kind = "method"
	RPC       Kind = "rpc"
	Field     Kind = "field"
	Property  Kind = "property"
	Constant  Kind = "constant"
	Variable  Kind = "variable"
	EnumValue Kind = "enum_value"
)

// IsType reports whether declarations of this kind are shown in the Types tab.
func (k Kind) IsType() bool {
	switch k {
	case Class, Interface, Enum, Struct, Union, Object, Message, Service, TypeAlias:
		return true
	}
	return false
}

// isContainer reports whether declarations of this kind become the Parent
// of declarations nested in them.
func (k Kind) isContainer() bool {
	return k.IsType() && k != TypeAlias || k == Namespace
}

// Symbol is a named declaration.
type Symbol struct {
	Name string
	Kind Kind
	// Parent is the name of the enclosing type or namespace, if any.
	Parent string
	// Line is the 1-based line of the name; Column is its 0-based byte offset in the line.
	Line   int
	Column int
	// Start and End are the byte offsets of the name in the file.
	Start, End uint32
}

// Language returns the language of the file at p, or "" if it is not supported.
func Language(p string) string {
	ext := strings.ToLower(path.Ext(p))
	for name, spec := range specs {
		for _, e := range spec.exts {
			if e == ext {
				return name
			}
		}
	}
	return ""
}

// Extract returns the declarations in src, written in lang (see Language).
func Extract(ctx context.Context, lang string, src []byte) ([]Symbol, error) {
	spec, ok := specs[lang]
	if !ok {
		return nil, fmt.Errorf("unsupported language %q", lang)
	}
	q, err := spec.compile()
	if err != nil {
		return nil, err
	}

	parser := sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(spec.language)
	tree, err := parser.ParseCtx(ctx, nil, src)
	if err != nil {
		return nil, err
	}
	defer tree.Close()

	qc := sitter.NewQueryCursor()
	defer qc.Close()
	qc.Exec(q, tree.RootNode())

	// The same name may be captured by several patterns (e.g. a Go struct is
	// also a type spec); the pattern listed first wins.
	byStart := map[uint32]def{}
	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}
		var d def
		for _, c := range m.Captures {
			if q.CaptureNameForId(c.Index) == "name" {
				d.name = c.Node
			} else {
				d.node = c.Node
				d.kind = Kind(q.CaptureNameForId(c.Index))
			}
		}
		if d.name == nil || d.node == nil || spec.isLocal(d.node) {
			continue
		}
		d.pattern = m.PatternIndex
		if spec.refine != nil {
			d.kind = spec.refine(d.node, d.kind)
		}
		start := d.name.StartByte()
		if prev, ok := byStart[start]; ok && prev.pattern <= d.pattern {
			continue
		}
		byStart[start] = d
	}

	defs := make([]def, 0, len(byStart))
	for _, d := range byStart {
		defs = append(defs, d)
	}
	return resolve(defs, src), nil
}

type def struct {
	name, node *sitter.Node
	kind       Kind
	pattern    uint16
}

// resolve converts defs to symbols, assigning parents by range nesting.
func resolve(defs []def, src []byte) []Symbol {
	sort.Slice(defs, func(i, j int) bool {
		a, b := defs[i].node, defs[j].node
		if a.StartByte() != b.StartByte() {
			return a.StartByte() < b.StartByte()
		}
		return a.EndByte() > b.EndByte()
	})

	type open struct {
		name string
		kind Kind
		end  uint32
	}
	var stack []open
	out := make([]Symbol, 0, len(defs))
	for _, d := range defs {
		for len(stack) > 0 && stack[len(stack)-1].end <= d.node.StartByte() {
			stack = stack[:len(stack)-1]
		}
		s := Symbol{
			Name:   d.name.Content(src),
			Kind:   d.kind,
			Line:   int(d.name.StartPoint().Row) + 1,
			Column: int(d.name.StartPoint().Column),
			Start:  d.name.StartByte(),
			End:    d.name.EndByte(),
		}
		if len(stack) > 0 {
			s.Parent = stack[len(stack)-1].name
		}
		hasTypeParent := s.Parent != "" && stack[len(stack)-1].kind != Namespace
		// C++ out-of-line definitions: "Foo::bar" is bar with parent Foo.
		if i := strings.LastIndex(s.Name, "::"); i >= 0 {
			s.Parent, s.Name = s.Name[:i], s.Name[i+2:]
			hasTypeParent = true
		}
		switch {
		case s.Kind == Function && hasTypeParent:
			s.Kind = Method
		case s.Kind == Property && hasTypeParent:
			s.Kind = Field
		case s.Kind == Property:
			s.Kind = Variable
		}
		out = append(out, s)
		if d.kind.isContainer() {
			stack = append(stack, open{name: s.Name, end: d.node.EndByte(), kind: d.kind})
		}
	}
	return out
}
