package symbols

import "github.com/smacker/go-tree-sitter/csharp"

var csharpSpec = &spec{
	exts:     []string{".cs"},
	language: csharp.GetLanguage(),
	query: `
(namespace_declaration name: (_) @name) @namespace
(file_scoped_namespace_declaration name: (_) @name) @namespace
(class_declaration name: (identifier) @name) @class
(record_declaration "struct" name: (identifier) @name) @struct
(record_declaration name: (identifier) @name) @class
(struct_declaration name: (identifier) @name) @struct
(interface_declaration name: (identifier) @name) @interface
(enum_declaration name: (identifier) @name) @enum
(enum_member_declaration name: (identifier) @name) @enum_value
(delegate_declaration name: (identifier) @name) @type
(method_declaration name: (identifier) @name) @method
(property_declaration name: (identifier) @name) @property
(field_declaration (modifier "const") (variable_declaration (variable_declarator name: (identifier) @name))) @constant
(field_declaration (variable_declaration (variable_declarator name: (identifier) @name))) @field
(event_field_declaration (variable_declaration (variable_declarator name: (identifier) @name))) @field
`,
	local: set("block", "arrow_expression_clause", "lambda_expression", "anonymous_method_expression", "global_statement"),
	toEnd: set("file_scoped_namespace_declaration"),
}
