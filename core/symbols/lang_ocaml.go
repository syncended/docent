package symbols

import "github.com/smacker/go-tree-sitter/ocaml"

var ocamlSpec = &spec{
	exts:     []string{".ml"},
	language: ocaml.GetLanguage(),
}
