package symbols

import "github.com/smacker/go-tree-sitter/swift"

var swiftSpec = &spec{
	exts:     []string{".swift"},
	language: swift.GetLanguage(),
}
