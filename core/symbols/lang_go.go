package symbols

import (
	"github.com/smacker/go-tree-sitter/golang"
)

var goSpec = &spec{
	exts:     []string{".go"},
	language: golang.GetLanguage(),
	query: `
(type_spec name: (type_identifier) @name type: (struct_type)) @struct
(type_spec name: (type_identifier) @name type: (interface_type)) @interface
(type_spec name: (type_identifier) @name) @type
(function_declaration name: (identifier) @name) @function
(method_declaration name: (field_identifier) @name) @method
(method_elem name: (field_identifier) @name) @method
(field_declaration name: (field_identifier) @name) @field
(const_spec name: (identifier) @name) @constant
(var_spec name: (identifier) @name) @variable
`,
	local: set("block", "func_literal"),
}
