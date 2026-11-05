package symbols

import "github.com/smacker/go-tree-sitter/php"

var phpSpec = &spec{
	exts:     []string{".php"},
	language: php.GetLanguage(),
	query: `
(namespace_definition name: (namespace_name) @name) @namespace
(class_declaration name: (name) @name) @class
(interface_declaration name: (name) @name) @interface
(trait_declaration name: (name) @name) @interface
(enum_declaration name: (name) @name) @enum
(enum_case name: (name) @name) @enum_value
(function_definition name: (name) @name) @function
(method_declaration name: (name) @name) @method
(property_declaration (property_element (variable_name (name) @name))) @field
(property_promotion_parameter name: (variable_name (name) @name)) @field
(const_declaration (const_element (name) @name)) @constant
`,
	// Blocks are local, except namespace bodies and conditional declarations
	// such as "if (!function_exists('f')) { function f() {} }".
	local:      set("compound_statement", "object_creation_expression"),
	namespaces: set("namespace_definition", "if_statement", "else_clause"),
}
