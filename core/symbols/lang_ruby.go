package symbols

import "github.com/smacker/go-tree-sitter/ruby"

var rubySpec = &spec{
	exts:     []string{".rb", ".rake", ".gemspec"},
	language: ruby.GetLanguage(),
}
