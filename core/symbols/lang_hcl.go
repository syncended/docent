package symbols

import "github.com/smacker/go-tree-sitter/hcl"

var hclSpec = &spec{
	exts:     []string{".hcl", ".tf", ".tfvars"},
	language: hcl.GetLanguage(),
}
