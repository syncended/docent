package symbols

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/hcl"
)

// Only top-level Terraform blocks are matched; the block type is an
// identifier, so refine picks the kind from its text. Quoted labels are
// captured by their template_literal, without the quotes.
var hclSpec = &spec{
	exts:     []string{".hcl", ".tf", ".tfvars"},
	language: hcl.GetLanguage(),
	query: `
(config_file (body (block (identifier) . (string_lit) . (string_lit (template_literal) @name) . (block_start)) @object))
(config_file (body (block (identifier) . (identifier) . (identifier) @name . (block_start)) @object))
(config_file (body (block (identifier) . (string_lit (template_literal) @name) . (block_start)) @variable))
(config_file (body (block (identifier) . (identifier) @name . (block_start)) @variable))
(config_file (body (block (body (attribute (identifier) @name) @variable))))
`,
	refine: func(n *sitter.Node, src []byte, k Kind) Kind {
		if n.Type() == "attribute" {
			if n.Parent().Parent().NamedChild(0).Content(src) == "locals" {
				return Variable
			}
			return ""
		}
		switch n.NamedChild(0).Content(src) {
		case "resource", "data":
			if k == Object {
				return Object
			}
		case "module":
			if k == Variable {
				return Namespace
			}
		case "variable":
			if k == Variable {
				return Variable
			}
		case "output":
			if k == Variable {
				return Constant
			}
		}
		return ""
	},
}
