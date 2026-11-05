package symbols

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/groovy"
)

// The grammar is script-oriented: an "implements" clause right after the
// class name parses as an error that swallows the name, constructors are not
// recognized, and enums and traits parse as declarations of a variable typed
// "enum" or "trait" followed by a closure.
var groovySpec = &spec{
	exts:     []string{".groovy", ".gradle"},
	language: groovy.GetLanguage(),
	query: `
(class_definition ["class" "interface"] . name: (identifier) @name) @class
(ERROR . (identifier) @name . (identifier)) @class
(function_definition function: (identifier) @name) @function
(function_declaration function: (identifier) @name) @function
(declaration type: (identifier) name: (identifier) @name) @enum
(declaration name: (identifier) @name) @property
`,
	// Closures are both blocks and class bodies.
	local:      set("closure"),
	namespaces: set("class_definition"),
	refine: func(n *sitter.Node, src []byte, k Kind) Kind {
		switch k {
		case Class:
			if n.Type() == "ERROR" {
				// "class Foo implements Bar": the error right after the
				// keyword holds the name; the class body is not its child.
				prev := n.PrevSibling()
				if n.Parent() == nil || n.Parent().Type() != "class_definition" || prev == nil ||
					prev.Type() != "class" && prev.Type() != "interface" ||
					n.NamedChild(1).Content(src) != "implements" {
					return ""
				}
			}
			for i := 0; i < int(n.ChildCount()); i++ {
				if n.Child(i).Type() == "interface" {
					return Interface
				}
			}
		case Enum:
			switch n.ChildByFieldName("type").Content(src) {
			case "enum":
				return Enum
			case "trait":
				return Interface
			}
			return ""
		case Property:
			// "task build { ... }" in Gradle scripts.
			if t := n.ChildByFieldName("type"); t != nil && t.Content(src) == "task" {
				return ""
			}
		}
		return k
	},
}
