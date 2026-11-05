package symbols

import "github.com/smacker/go-tree-sitter/scala"

var scalaSpec = &spec{
	exts:     []string{".scala", ".sc"},
	language: scala.GetLanguage(),
	query: `
(package_clause name: (_) @name body: (_)) @namespace
(package_object name: (identifier) @name) @object
(class_definition name: (identifier) @name) @class
(object_definition name: (identifier) @name) @object
(trait_definition name: (identifier) @name) @interface
(enum_definition name: (identifier) @name) @enum
(simple_enum_case name: (identifier) @name) @enum_value
(full_enum_case name: (identifier) @name) @enum_value
(function_definition name: (identifier) @name) @function
(function_declaration name: (identifier) @name) @function
(type_definition name: (type_identifier) @name) @type
(class_parameter "val" name: (identifier) @name) @property
(class_parameter "var" name: (identifier) @name) @property
(val_definition pattern: (identifier) @name) @property
(val_definition pattern: (identifiers (identifier) @name)) @property
(var_definition pattern: (identifier) @name) @property
(var_definition pattern: (identifiers (identifier) @name)) @property
(val_declaration name: (identifier) @name) @property
(var_declaration name: (identifier) @name) @property
(given_definition name: (identifier) @name) @property
`,
	local: set("block", "indented_block", "lambda_expression", "instance_expression", "case_clause"),
}
