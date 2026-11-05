package symbols

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/kotlin"
)

var kotlinSpec = &spec{
	exts:     []string{".kt", ".kts"},
	language: kotlin.GetLanguage(),
	query: `
(class_declaration (type_identifier) @name) @class
(object_declaration (type_identifier) @name) @object
(type_alias (type_identifier) @name) @type
(function_declaration (simple_identifier) @name) @function
(property_declaration (variable_declaration (simple_identifier) @name)) @property
(enum_entry (simple_identifier) @name) @enum_value
`,
	local: set("function_body", "lambda_literal", "anonymous_initializer", "getter", "setter"),
	// class_declaration covers classes, interfaces and enum classes.
	refine: func(n *sitter.Node, k Kind) Kind {
		if k != Class {
			return k
		}
		for i := 0; i < int(n.ChildCount()); i++ {
			switch n.Child(i).Type() {
			case "interface":
				return Interface
			case "enum_class_body":
				return Enum
			}
		}
		return k
	},
}
