package symbols

import "github.com/smacker/go-tree-sitter/elixir"

var elixirSpec = &spec{
	exts:     []string{".ex", ".exs"},
	language: elixir.GetLanguage(),
}
