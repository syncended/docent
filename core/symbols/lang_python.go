package symbols

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"
)

var pythonSpec = &spec{
	exts:     []string{".py", ".pyi"},
	language: python.GetLanguage(),
	query: `
(class_definition name: (identifier) @name) @class
(function_definition name: (identifier) @name) @function
(assignment left: (identifier) @name) @property
(assignment left: (pattern_list (identifier) @name)) @property
`,
	// block is also the body of classes and if/try statements, so locals
	// are found by their enclosing function instead.
	local: set("function_definition", "lambda"),
	// By convention, an UPPER_CASE name is a constant.
	refine: func(n *sitter.Node, src []byte, k Kind) Kind {
		if k != Property {
			return k
		}
		left := n.ChildByFieldName("left")
		if left == nil || left.Type() != "identifier" {
			return k
		}
		name := left.Content(src)
		if strings.ToUpper(name) == name && strings.ContainsAny(name, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
			return Constant
		}
		return k
	},
}
