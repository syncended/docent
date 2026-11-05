package symbols

import "github.com/smacker/go-tree-sitter/python"

var pythonSpec = &spec{
	exts:     []string{".py", ".pyi"},
	language: python.GetLanguage(),
}
