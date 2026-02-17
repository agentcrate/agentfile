package agentfile_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/agentcrate/agentfile"
	"github.com/invopop/jsonschema"
)

// TestSchemaUpToDate ensures the committed agentfile-v1.json matches
// what would be generated from the current Go types. If this test
// fails, run `make schema` to regenerate.
func TestSchemaUpToDate(t *testing.T) {
	// Generate schema from Go types (same logic as cmd/genschema).
	r := &jsonschema.Reflector{
		ExpandedStruct: true,
		Anonymous:      true,
	}
	if err := r.AddGoComments("github.com/agentcrate/agentfile", "./"); err != nil {
		t.Fatalf("AddGoComments: %v", err)
	}

	schema := r.Reflect(&agentfile.Agentfile{})
	schema.Version = "https://json-schema.org/draft/2020-12/schema"
	schema.ID = "https://agentcrate.ai/schemas/agentfile-v1.json"
	schema.Title = "Agentfile v1"
	schema.Description = "Declarative specification for packaging AI agents with AgentCrate."

	// Post-processing (must match cmd/genschema/main.go).
	if v, ok := schema.Properties.Get("version"); ok {
		v.Const = "1"
	}
	if metaDef, ok := schema.Definitions["Metadata"]; ok {
		if tags, ok := metaDef.Properties.Get("tags"); ok {
			tags.Items = &jsonschema.Schema{
				Type:    "string",
				Pattern: "^[a-z0-9-]{1,50}$",
			}
		}
	}

	generated, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	// Add trailing newline to match file output from fmt.Println.
	generated = append(generated, '\n')

	// Normalize line endings so the test passes on Windows (\r\n) and Unix (\n).
	want := strings.ReplaceAll(string(generated), "\r\n", "\n")
	got := strings.ReplaceAll(string(agentfile.SchemaV1), "\r\n", "\n")

	if want != got {
		t.Fatal("schema/agentfile-v1.json is out of date with types.go — run `make schema` to regenerate")
	}
}
