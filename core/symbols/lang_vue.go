package symbols

import "github.com/smacker/go-tree-sitter/html"

// Vue single-file components parse as HTML well enough to find their
// <script> blocks.
var vueSpec = &spec{
	exts:     []string{".vue"},
	language: html.GetLanguage(),
	scripts:  "typescript",
}
