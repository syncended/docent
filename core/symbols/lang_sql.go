package symbols

import "github.com/smacker/go-tree-sitter/sql"

var sqlSpec = &spec{
	exts:     []string{".sql"},
	language: sql.GetLanguage(),
}
