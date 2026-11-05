package symbols

import "github.com/smacker/go-tree-sitter/elm"

var elmSpec = &spec{
	exts:     []string{".elm"},
	language: elm.GetLanguage(),
	query: `
(type_declaration name: (upper_case_identifier) @name) @enum
(union_variant name: (upper_case_identifier) @name) @enum_value
(type_alias_declaration name: (upper_case_identifier) @name) @type
(value_declaration (function_declaration_left . (lower_case_identifier) @name)) @function
(port_annotation name: (lower_case_identifier) @name) @function
`,
	local: set("let_in_expr"),
}
