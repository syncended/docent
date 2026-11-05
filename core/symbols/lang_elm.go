package symbols

import "github.com/smacker/go-tree-sitter/elm"

var elmSpec = &spec{
	exts:     []string{".elm"},
	language: elm.GetLanguage(),
}
