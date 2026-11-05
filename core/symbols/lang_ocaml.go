package symbols

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/ocaml"
)

// The grammar parses implementations only: top-level "val" in .mli files is
// an error, so interface files are not indexed.
var ocamlSpec = &spec{
	exts:     []string{".ml"},
	language: ocaml.GetLanguage(),
	query: `
(module_type_definition name: (module_type_name) @name) @interface
(module_binding name: (module_name) @name) @namespace
(class_binding name: (class_name) @name) @class
(type_binding name: (type_constructor) @name) @type
(exception_definition (constructor_declaration (constructor_name) @name)) @class
(constructor_declaration (constructor_name) @name) @enum_value
(field_declaration (field_name) @name) @field
(let_binding pattern: (value_name) @name) @variable
(external (value_name) @name) @function
(value_specification (value_name) @name) @variable
(method_definition name: (method_name) @name) @method
(instance_variable_definition name: (instance_variable_name) @name) @field
`,
	local: set("let_binding", "let_expression", "method_definition"),
	refine: func(n *sitter.Node, src []byte, k Kind) Kind {
		switch {
		case n.Type() == "type_binding":
			// Variants and records are containers of their constructors and fields.
			switch b := n.ChildByFieldName("body"); {
			case b != nil && b.Type() == "variant_declaration":
				return Enum
			case b != nil && b.Type() == "record_declaration":
				return Struct
			}
		case n.Type() == "let_binding":
			if b := n.ChildByFieldName("body"); b != nil && (b.Type() == "fun_expression" || b.Type() == "function_expression") {
				return Function
			}
			for i := 0; i < int(n.NamedChildCount()); i++ {
				if n.NamedChild(i).Type() == "parameter" {
					return Function
				}
			}
		case n.Type() == "value_specification":
			for i := 0; i < int(n.NamedChildCount()); i++ {
				if n.NamedChild(i).Type() == "function_type" {
					return Function
				}
			}
		}
		return k
	},
}
