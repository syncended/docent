package symbols

import (
	"github.com/smacker/go-tree-sitter/protobuf"
)

var protoSpec = &spec{
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
}
