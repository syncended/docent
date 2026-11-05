package symbols

import (
	"github.com/smacker/go-tree-sitter/cpp"
)

var cppSpec = &spec{
	exts:     []string{".c", ".cc", ".cpp", ".cxx", ".h", ".hh", ".hpp", ".hxx", ".ipp", ".inl"},
	language: cpp.GetLanguage(),
	query: `
(class_specifier name: (_) @name body: (_)) @class
(struct_specifier name: (_) @name body: (_)) @struct
(union_specifier name: (_) @name body: (_)) @union
(enum_specifier name: (_) @name body: (_)) @enum
(enumerator name: (identifier) @name) @enum_value
(namespace_definition name: (_) @name) @namespace
(type_definition declarator: (type_identifier) @name) @type
(alias_declaration name: (type_identifier) @name) @type
(function_definition declarator: (function_declarator declarator: (_) @name)) @function
(function_definition declarator: (pointer_declarator declarator: (function_declarator declarator: (_) @name))) @function
(function_definition declarator: (reference_declarator (function_declarator declarator: (_) @name))) @function
(declaration declarator: (function_declarator declarator: (_) @name)) @function
(declaration declarator: (pointer_declarator declarator: (function_declarator declarator: (_) @name))) @function
(declaration declarator: (reference_declarator (function_declarator declarator: (_) @name))) @function
(field_declaration declarator: (function_declarator declarator: (_) @name)) @method
(field_declaration declarator: (field_identifier) @name) @field
`,
	local: set("compound_statement", "lambda_expression"),
}
