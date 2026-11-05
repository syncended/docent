package symbols

import (
	"fmt"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
)

// spec describes how to extract declarations for one language.
//
// Each query pattern captures the declared name as @name and the whole
// declaration with a capture named after its Kind. Patterns listed earlier
// take precedence when several capture the same name.
type spec struct {
	exts     []string
	language *sitter.Language
	query    string
	// local lists node types of function bodies and similar scopes;
	// declarations inside them are skipped.
	local map[string]bool
	// namespaces lists node types whose body is not a local scope even if
	// its node type is listed in local (TypeScript namespaces).
	namespaces map[string]bool
	// toEnd lists node types of declarations that enclose the rest of the
	// file although their node does not (C# file-scoped namespaces).
	toEnd map[string]bool
	// refine adjusts the kind using the declaration node and the source,
	// when the grammar does not distinguish kinds by node type; "" skips
	// the declaration.
	refine func(n *sitter.Node, src []byte, k Kind) Kind
	// scripts names the language of the <script> blocks of component files
	// (Svelte, Vue): the file is parsed only to find them, and the
	// declarations are those of the blocks.
	scripts string

	once sync.Once
	q    *sitter.Query
	err  error
}

func (s *spec) compile() (*sitter.Query, error) {
	s.once.Do(func() {
		s.q, s.err = sitter.NewQuery([]byte(s.query), s.language)
		if s.err != nil {
			s.err = fmt.Errorf("compile query: %w", s.err)
		}
	})
	return s.q, s.err
}

func (s *spec) isLocal(n *sitter.Node) bool {
	for p := n.Parent(); p != nil; p = p.Parent() {
		if s.local[p.Type()] && !(p.Parent() != nil && s.namespaces[p.Parent().Type()]) {
			return true
		}
	}
	return false
}

func set(types ...string) map[string]bool {
	m := make(map[string]bool, len(types))
	for _, t := range types {
		m[t] = true
	}
	return m
}

// specs are the supported languages, by name; each is in lang_<name>.go.
var specs = map[string]*spec{
	"go":         goSpec,
	"java":       javaSpec,
	"kotlin":     kotlinSpec,
	"cpp":        cppSpec,
	"proto":      protoSpec,
	"javascript": javascriptSpec,
	"typescript": typescriptSpec,
	"tsx":        tsxSpec,
	"python":     pythonSpec,
	"ruby":       rubySpec,
	"lua":        luaSpec,
	"bash":       bashSpec,
	"rust":       rustSpec,
	"swift":      swiftSpec,
	"csharp":     csharpSpec,
	"scala":      scalaSpec,
	"php":        phpSpec,
	"elixir":     elixirSpec,
	"groovy":     groovySpec,
	"ocaml":      ocamlSpec,
	"elm":        elmSpec,
	"sql":        sqlSpec,
	"css":        cssSpec,
	"hcl":        hclSpec,
	"svelte":     svelteSpec,
	"vue":        vueSpec,
}
