package symbols

import "github.com/smacker/go-tree-sitter/php"

var phpSpec = &spec{
	exts:     []string{".php"},
	language: php.GetLanguage(),
}
