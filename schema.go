package agentfile

import (
	_ "embed"

	"github.com/invopop/jsonschema"
)

// SchemaV1 contains the raw JSON Schema for Agentfile v1.
//
//go:embed schema/agentfile-v1.json
var SchemaV1 []byte

// ApplySchemaOverrides applies post-processing overrides to the reflected schema.
// This function is the single source of truth for schema post-processing steps
// shared between cmd/genschema (which writes the schema file) and schema_test.go
// (which verifies the committed schema is up to date). Both call this function so
// changes are applied consistently in one place.
func ApplySchemaOverrides(schema *jsonschema.Schema) {
	// Set version const (not supported via struct tags).
	if v, ok := schema.Properties.Get("version"); ok {
		v.Const = "1"
	}
	// Set tag item pattern on the Metadata $def.
	if metaDef, ok := schema.Definitions["Metadata"]; ok {
		if tags, ok := metaDef.Properties.Get("tags"); ok {
			tags.Items = &jsonschema.Schema{
				Type:    "string",
				Pattern: "^[a-z0-9-]{1,50}$",
			}
		}
	}
}
