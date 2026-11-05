package symbols

import "github.com/smacker/go-tree-sitter/css"

var cssSpec = &spec{
	exts:     []string{".css"},
	language: css.GetLanguage(),
}
