package agentfile

import _ "embed"

// SchemaV1 contains the raw JSON Schema for Agentfile v1.
//
//go:embed schema/agentfile-v1.json
var SchemaV1 []byte
