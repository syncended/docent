package symbols

import "github.com/smacker/go-tree-sitter/lua"

var luaSpec = &spec{
	exts:     []string{".lua"},
	language: lua.GetLanguage(),
}
