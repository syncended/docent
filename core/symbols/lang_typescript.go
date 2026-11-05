package symbols

import (
	"github.com/smacker/go-tree-sitter/typescript/tsx"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

const tsQuery = jsQuery + `
(abstract_class_declaration name: (_) @name) @class
(interface_declaration name: (_) @name) @interface
(enum_declaration name: (_) @name) @enum
(type_alias_declaration name: (_) @name) @type
(enum_body (property_identifier) @name @enum_value)
(enum_assignment name: (property_identifier) @name) @enum_value
(method_signature name: (_) @name) @method
(abstract_method_signature name: (_) @name) @method
(public_field_definition name: (_) @name) @field
(property_signature name: (_) @name) @field
(function_signature name: (identifier) @name) @function
(internal_module name: (_) @name) @namespace
(module name: (_) @name) @namespace
`

var (
	tsLocal      = set("statement_block", "arrow_function", "class_static_block")
	tsNamespaces = set("internal_module", "module")
)

var typescriptSpec = &spec{
	exts:       []string{".ts", ".mts", ".cts"},
	language:   typescript.GetLanguage(),
	query:      tsQuery,
	local:      tsLocal,
	namespaces: tsNamespaces,
}

var tsxSpec = &spec{
	exts:       []string{".tsx"},
	language:   tsx.GetLanguage(),
	query:      tsQuery,
	local:      tsLocal,
	namespaces: tsNamespaces,
}
