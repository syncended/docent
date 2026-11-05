package symbols

import "github.com/smacker/go-tree-sitter/swift"

// class_declaration covers classes, structs, enums, actors and extensions;
// declaration_kind tells them apart.
var swiftSpec = &spec{
	exts:     []string{".swift"},
	language: swift.GetLanguage(),
	query: `
(class_declaration declaration_kind: "class" name: (type_identifier) @name) @class
(class_declaration declaration_kind: "actor" name: (type_identifier) @name) @class
(class_declaration declaration_kind: "struct" name: (type_identifier) @name) @struct
(class_declaration declaration_kind: "enum" name: (type_identifier) @name) @enum
(class_declaration declaration_kind: "extension" name: (user_type (type_identifier) @name .)) @scope
(protocol_declaration name: (type_identifier) @name) @interface
(enum_entry name: (simple_identifier) @name) @enum_value
(function_declaration name: (simple_identifier) @name) @function
(protocol_function_declaration name: (simple_identifier) @name) @function
(property_declaration name: (pattern bound_identifier: (simple_identifier) @name)) @property
(protocol_property_declaration name: (pattern bound_identifier: (simple_identifier) @name)) @property
(typealias_declaration name: (type_identifier) @name) @type
`,
	local: set("function_body", "lambda_literal", "computed_property", "willset_didset_block"),
}
