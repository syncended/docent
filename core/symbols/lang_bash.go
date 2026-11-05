package symbols

import "github.com/smacker/go-tree-sitter/bash"

// Only assignments directly at the top level are variables: ones in loops,
// conditionals and command prefixes are mostly updates and temporaries.
var bashSpec = &spec{
	exts:     []string{".sh", ".bash"},
	language: bash.GetLanguage(),
	query: `
(function_definition name: (word) @name) @function
(program (variable_assignment name: (variable_name) @name) @variable)
(program (variable_assignments (variable_assignment name: (variable_name) @name) @variable))
(program (declaration_command (variable_assignment name: (variable_name) @name) @variable))
`,
	local: set("function_definition"),
}
