package symbols

import "github.com/smacker/go-tree-sitter/groovy"

var groovySpec = &spec{
	exts:     []string{".groovy", ".gradle"},
	language: groovy.GetLanguage(),
}
