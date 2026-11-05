package symbols

import (
	"github.com/smacker/go-tree-sitter/java"
)

var javaSpec = &spec{
	exts:     []string{".java"},
	language: java.GetLanguage(),
	query: `
(class_declaration name: (identifier) @name) @class
(record_declaration name: (identifier) @name) @class
(interface_declaration name: (identifier) @name) @interface
(annotation_type_declaration name: (identifier) @name) @interface
(enum_declaration name: (identifier) @name) @enum
(enum_constant name: (identifier) @name) @enum_value
(method_declaration name: (identifier) @name) @method
(field_declaration declarator: (variable_declarator name: (identifier) @name)) @field
(constant_declaration declarator: (variable_declarator name: (identifier) @name)) @constant
`,
	local: set("block", "constructor_body", "lambda_expression"),
}
