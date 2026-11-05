package symbols

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/elixir"
)

// Elixir declarations are macro calls; the patterns match their shapes and
// refine keeps only the calls whose target is a defining macro.
var elixirSpec = &spec{
	exts:     []string{".ex", ".exs"},
	language: elixir.GetLanguage(),
	query: `
(call target: (identifier) (arguments . (alias) @name)) @class
(call target: (identifier) (arguments . (call target: (identifier) @name))) @function
(call target: (identifier) (arguments . (identifier) @name)) @function
(call target: (identifier) (arguments . (binary_operator left: (call target: (identifier) @name)))) @function
(call target: (identifier) (arguments . (binary_operator left: (identifier) @name))) @function
`,
	local: set("anonymous_function"),
	refine: func(n *sitter.Node, src []byte, k Kind) Kind {
		for p := n.Parent(); p != nil; p = p.Parent() {
			if p.Type() == "call" && elixirFunctions[elixirTarget(p, src)] {
				return ""
			}
		}
		switch t := elixirTarget(n, src); {
		case k == Class && t == "defmodule":
			return Class
		case k == Class && t == "defprotocol":
			return Interface
		case k == Function && elixirFunctions[t]:
			return Function
		}
		return ""
	},
}

var elixirFunctions = set("def", "defp", "defmacro", "defmacrop", "defguard", "defguardp", "defdelegate")

func elixirTarget(call *sitter.Node, src []byte) string {
	t := call.ChildByFieldName("target")
	if t == nil || t.Type() != "identifier" {
		return ""
	}
	return t.Content(src)
}
