package symbols

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/sql"
)

var sqlSpec = &spec{
	exts:     []string{".sql"},
	language: sql.GetLanguage(),
	query: `
(create_table (object_reference name: (identifier) @name)) @table
(create_view (object_reference name: (identifier) @name)) @table
(create_materialized_view (object_reference name: (identifier) @name)) @table
(create_function (object_reference name: (identifier) @name)) @function
(create_type (object_reference name: (identifier) @name)) @type
(create_schema (identifier) @name) @namespace
(create_index column: (identifier) @name) @constant
(create_sequence (object_reference name: (identifier) @name)) @constant
(create_table (column_definitions (column_definition name: (identifier) @name) @field))
(create_type (column_definitions (column_definition name: (identifier) @name) @field))
(create_table (column_definitions (constraints (constraint name: (identifier) @name) @constant)))
`,
	local: set("function_body"),
	// CREATE TYPE covers enum, composite and other types.
	refine: func(n *sitter.Node, _ []byte, k Kind) Kind {
		if k != TypeAlias {
			return k
		}
		for i := 0; i < int(n.NamedChildCount()); i++ {
			switch n.NamedChild(i).Type() {
			case "enum_elements":
				return Enum
			case "column_definitions":
				return Struct
			}
		}
		return k
	},
}
