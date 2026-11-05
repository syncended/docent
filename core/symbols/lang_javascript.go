package symbols

import (
	"github.com/smacker/go-tree-sitter/javascript"
)

const jsQuery = `
(class_declaration name: (_) @name) @class
(function_declaration name: (identifier) @name) @function
(generator_function_declaration name: (identifier) @name) @function
(method_definition name: (_) @name) @method
(variable_declarator name: (identifier) @name value: [(arrow_function) (function_expression)]) @function
(lexical_declaration (variable_declarator name: (identifier) @name)) @variable
(variable_declaration (variable_declarator name: (identifier) @name)) @variable
`

var javascriptSpec = &spec{
	exts:     []string{".js", ".jsx", ".mjs", ".cjs"},
	language: javascript.GetLanguage(),
	query: jsQuery + `
(field_definition property: (property_identifier) @name) @field
`,
	local: set("statement_block", "arrow_function", "class_static_block"),
}
