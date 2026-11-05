package symbols

import "github.com/smacker/go-tree-sitter/scala"

var scalaSpec = &spec{
	exts:     []string{".scala", ".sc"},
	language: scala.GetLanguage(),
}
