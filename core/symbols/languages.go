package symbols

import (
	"fmt"
	"sync"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/cpp"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/java"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/kotlin"
	"github.com/smacker/go-tree-sitter/protobuf"
	"github.com/smacker/go-tree-sitter/typescript/tsx"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

// spec describes how to extract declarations for one language.
//
// Each query pattern captures the declared name as @name and the whole
// declaration with a capture named after its Kind. Patterns listed earlier
// take precedence when several capture the same name.
type spec struct {
	exts     []string
	language *sitter.Language
	query    string
	// local lists node types of function bodies and similar scopes;
	// declarations inside them are skipped.
	local map[string]bool
	// namespaces lists node types whose body is not a local scope even if
	// its node type is listed in local (TypeScript namespaces).
	namespaces map[string]bool
	// refine adjusts the kind using the declaration node, when the grammar
	// does not distinguish kinds by node type.
	refine func(n *sitter.Node, k Kind) Kind

	once sync.Once
	q    *sitter.Query
	err  error
}

func (s *spec) compile() (*sitter.Query, error) {
	s.once.Do(func() {
		s.q, s.err = sitter.NewQuery([]byte(s.query), s.language)
		if s.err != nil {
			s.err = fmt.Errorf("compile query: %w", s.err)
		}
	})
	return s.q, s.err
}

func (s *spec) isLocal(n *sitter.Node) bool {
	for p := n.Parent(); p != nil; p = p.Parent() {
		if s.local[p.Type()] && !(p.Parent() != nil && s.namespaces[p.Parent().Type()]) {
			return true
		}
	}
	return false
}

func set(types ...string) map[string]bool {
	m := make(map[string]bool, len(types))
	for _, t := range types {
		m[t] = true
	}
	return m
}

const jsQuery = `
(class_declaration name: (_) @name) @class
(function_declaration name: (identifier) @name) @function
(generator_function_declaration name: (identifier) @name) @function
(method_definition name: (_) @name) @method
(variable_declarator name: (identifier) @name value: [(arrow_function) (function_expression)]) @function
(lexical_declaration (variable_declarator name: (identifier) @name)) @variable
(variable_declaration (variable_declarator name: (identifier) @name)) @variable
`

const tsQuery = jsQuery + `
(abstract_class_declaration name: (_) @name) @class
(interface_declaration name: (_) @name) @interface
(enum_declaration name: (_) @name) @enum
(type_alias_declaration name: (_) @name) @type
(enum_body (property_identifier) @name @enum_value)
(enum_assignment name: (property_identifier) @name) @enum_value
(method_signature name: (_) @name) @method
(abstract_method_signature name: (_) @name) @method
(public_field_definition name: (_) @name) @field
(property_signature name: (_) @name) @field
(function_signature name: (identifier) @name) @function
(internal_module name: (_) @name) @namespace
(module name: (_) @name) @namespace
`

var (
	tsLocal      = set("statement_block", "arrow_function", "class_static_block")
	tsNamespaces = set("internal_module", "module")
)

var specs = map[string]*spec{
	"go": {
		exts:     []string{".go"},
		language: golang.GetLanguage(),
		query: `
(type_spec name: (type_identifier) @name type: (struct_type)) @struct
(type_spec name: (type_identifier) @name type: (interface_type)) @interface
(type_spec name: (type_identifier) @name) @type
(function_declaration name: (identifier) @name) @function
(method_declaration name: (field_identifier) @name) @method
(method_elem name: (field_identifier) @name) @method
(field_declaration name: (field_identifier) @name) @field
(const_spec name: (identifier) @name) @constant
(var_spec name: (identifier) @name) @variable
`,
		local: set("block", "func_literal"),
	},
	"java": {
		exts:     []string{".java"},
		language: java.GetLanguage(),
		query: `
(class_declaration name: (identifier) @name) @class
(record_declaration name: (identifier) @name) @class
(interface_declaration name: (identifier) @name) @interface
(annotation_type_declaration name: (identifier) @name) @interface
(enum_declaration name: (identifier) @name) @enum
(enum_constant name: (identifier) @name) @enum_value
(method_declaration name: (identifier) @name) @method
(field_declaration declarator: (variable_declarator name: (identifier) @name)) @field
(constant_declaration declarator: (variable_declarator name: (identifier) @name)) @constant
`,
		local: set("block", "constructor_body", "lambda_expression"),
	},
	"kotlin": {
		exts:     []string{".kt", ".kts"},
		language: kotlin.GetLanguage(),
		query: `
(class_declaration (type_identifier) @name) @class
(object_declaration (type_identifier) @name) @object
(type_alias (type_identifier) @name) @type
(function_declaration (simple_identifier) @name) @function
(property_declaration (variable_declaration (simple_identifier) @name)) @property
(enum_entry (simple_identifier) @name) @enum_value
`,
		local: set("function_body", "lambda_literal", "anonymous_initializer", "getter", "setter"),
		// class_declaration covers classes, interfaces and enum classes.
		refine: func(n *sitter.Node, k Kind) Kind {
			if k != Class {
				return k
			}
			for i := 0; i < int(n.ChildCount()); i++ {
				switch n.Child(i).Type() {
				case "interface":
					return Interface
				case "enum_class_body":
					return Enum
				}
			}
			return k
		},
	},
	"cpp": {
		exts:     []string{".c", ".cc", ".cpp", ".cxx", ".h", ".hh", ".hpp", ".hxx", ".ipp", ".inl"},
		language: cpp.GetLanguage(),
		query: `
(class_specifier name: (_) @name body: (_)) @class
(struct_specifier name: (_) @name body: (_)) @struct
(union_specifier name: (_) @name body: (_)) @union
(enum_specifier name: (_) @name body: (_)) @enum
(enumerator name: (identifier) @name) @enum_value
(namespace_definition name: (_) @name) @namespace
(type_definition declarator: (type_identifier) @name) @type
(alias_declaration name: (type_identifier) @name) @type
(function_definition declarator: (function_declarator declarator: (_) @name)) @function
(function_definition declarator: (pointer_declarator declarator: (function_declarator declarator: (_) @name))) @function
(function_definition declarator: (reference_declarator (function_declarator declarator: (_) @name))) @function
(declaration declarator: (function_declarator declarator: (_) @name)) @function
(declaration declarator: (pointer_declarator declarator: (function_declarator declarator: (_) @name))) @function
(declaration declarator: (reference_declarator (function_declarator declarator: (_) @name))) @function
(field_declaration declarator: (function_declarator declarator: (_) @name)) @method
(field_declaration declarator: (field_identifier) @name) @field
`,
		local: set("compound_statement", "lambda_expression"),
	},
	"proto": {
		exts:     []string{".proto"},
		language: protobuf.GetLanguage(),
		query: `
(message (message_name (identifier) @name)) @message
(enum (enum_name (identifier) @name)) @enum
(service (service_name (identifier) @name)) @service
(rpc (rpc_name (identifier) @name)) @rpc
(field (identifier) @name) @field
(map_field (identifier) @name) @field
(oneof_field (identifier) @name) @field
(enum_field (identifier) @name) @enum_value
`,
	},
	"javascript": {
		exts:     []string{".js", ".jsx", ".mjs", ".cjs"},
		language: javascript.GetLanguage(),
		query: jsQuery + `
(field_definition property: (property_identifier) @name) @field
`,
		local: set("statement_block", "arrow_function", "class_static_block"),
	},
	"typescript": {
		exts:       []string{".ts", ".mts", ".cts"},
		language:   typescript.GetLanguage(),
		query:      tsQuery,
		local:      tsLocal,
		namespaces: tsNamespaces,
	},
	"tsx": {
		exts:       []string{".tsx"},
		language:   tsx.GetLanguage(),
		query:      tsQuery,
		local:      tsLocal,
		namespaces: tsNamespaces,
	},
}
