package symbols

import "github.com/smacker/go-tree-sitter/rust"

var rustSpec = &spec{
	exts:     []string{".rs"},
	language: rust.GetLanguage(),
}
