package symbols

import "github.com/smacker/go-tree-sitter/rust"

var rustSpec = &spec{
	exts:     []string{".rs"},
	language: rust.GetLanguage(),
	query: `
(struct_item name: (type_identifier) @name) @struct
(struct_item body: (field_declaration_list (field_declaration name: (field_identifier) @name) @field))
(union_item name: (type_identifier) @name) @union
(union_item body: (field_declaration_list (field_declaration name: (field_identifier) @name) @field))
(enum_item name: (type_identifier) @name) @enum
(enum_variant name: (identifier) @name) @enum_value
(trait_item name: (type_identifier) @name) @interface
(impl_item type: (type_identifier) @name) @scope
(impl_item type: (generic_type type: (type_identifier) @name)) @scope
(impl_item type: (scoped_type_identifier name: (type_identifier) @name)) @scope
(impl_item type: (generic_type type: (scoped_type_identifier name: (type_identifier) @name))) @scope
(impl_item type: (reference_type type: (type_identifier) @name)) @scope
(impl_item type: (reference_type type: (generic_type type: (type_identifier) @name))) @scope
(function_item name: (identifier) @name) @function
(function_signature_item name: (identifier) @name) @function
(const_item name: (identifier) @name) @constant
(static_item name: (identifier) @name) @constant
(type_item name: (type_identifier) @name) @type
(associated_type name: (type_identifier) @name) @type
(mod_item name: (identifier) @name body: (_)) @namespace
(macro_definition name: (identifier) @name) @function
`,
	local: set("block", "closure_expression"),
}
