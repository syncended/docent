package symbols

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/ruby"
)

var rubySpec = &spec{
	exts:     []string{".rb", ".rake", ".gemspec"},
	language: ruby.GetLanguage(),
	query: `
(class name: (constant) @name) @class
(class name: (scope_resolution name: (constant) @name)) @class
(module name: (constant) @name) @namespace
(module name: (scope_resolution name: (constant) @name)) @namespace
(method name: (_) @name) @function
(singleton_method name: (_) @name) @method
(assignment left: (constant) @name) @constant
(assignment left: (scope_resolution name: (constant) @name)) @constant
(assignment left: (left_assignment_list (constant) @name)) @constant
`,
	local: set("method", "singleton_method"),
	// Ruby has only methods: a def in a module or class << self is one too.
	refine: func(n *sitter.Node, _ []byte, k Kind) Kind {
		if k != Function {
			return k
		}
		for p := n.Parent(); p != nil; p = p.Parent() {
			switch p.Type() {
			case "class", "module", "singleton_class":
				return Method
			}
		}
		return k
	},
}
