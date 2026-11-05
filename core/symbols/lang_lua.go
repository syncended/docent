package symbols

import "github.com/smacker/go-tree-sitter/lua"

// Functions are named by their last name segment: "function M.a:foo" is foo.
var luaSpec = &spec{
	exts:     []string{".lua"},
	language: lua.GetLanguage(),
	query: `
(function_statement name: (function_name (identifier) @name .)) @function
(function_statement name: (identifier) @name) @function
(variable_declaration name: (variable_declarator (identifier) @name .) value: (function)) @function
(field name: (identifier) @name value: (function)) @function
`,
	local: set("function_body"),
}
