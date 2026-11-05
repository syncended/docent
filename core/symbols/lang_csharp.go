package symbols

import "github.com/smacker/go-tree-sitter/csharp"

var csharpSpec = &spec{
	exts:     []string{".cs"},
	language: csharp.GetLanguage(),
}
