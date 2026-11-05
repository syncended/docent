package symbols

import "github.com/smacker/go-tree-sitter/svelte"

var svelteSpec = &spec{
	exts:     []string{".svelte"},
	language: svelte.GetLanguage(),
	scripts:  "typescript",
}
