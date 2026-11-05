package symbols

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/css"
)

var cssSpec = &spec{
	exts:     []string{".css"},
	language: css.GetLanguage(),
	query: `
(keyframes_statement (keyframes_name) @name) @constant
(declaration (property_name) @name) @variable
(at_rule (at_keyword) (keyword_query) @name) @variable
`,
	// Only custom properties ("--accent: …", "@property --accent") are kept.
	refine: func(n *sitter.Node, src []byte, k Kind) Kind {
		switch n.Type() {
		case "declaration":
			if !strings.HasPrefix(n.NamedChild(0).Content(src), "--") {
				return ""
			}
		case "at_rule":
			if n.NamedChild(0).Content(src) != "@property" {
				return ""
			}
		}
		return k
	},
}
