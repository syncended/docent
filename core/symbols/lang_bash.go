package symbols

import "github.com/smacker/go-tree-sitter/bash"

var bashSpec = &spec{
	exts:     []string{".sh", ".bash"},
	language: bash.GetLanguage(),
}
