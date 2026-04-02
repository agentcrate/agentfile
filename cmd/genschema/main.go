// Command genschema generates the Agentfile v1 JSON Schema from the Go types.
//
// Usage:
//
//	go run ./cmd/genschema > schema/agentfile-v1.json
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/agentcrate/agentfile"
	"github.com/invopop/jsonschema"
)

func main() {
	r := &jsonschema.Reflector{
		// Don't wrap the root type in $defs — inline it at the top level.
		ExpandedStruct: true,
		// Don't generate $id from Go package path.
		Anonymous: true,
	}

	// Pull descriptions from Go comments on types.go.
	if err := r.AddGoComments("github.com/agentcrate/agentfile", "./"); err != nil {
		log.Fatalf("addGoComments: %v", err)
	}

	schema := r.Reflect(&agentfile.Agentfile{})

	// Override the auto-generated $schema and $id.
	schema.Version = "https://json-schema.org/draft/2020-12/schema"
	schema.ID = "https://agentcrate.ai/schemas/agentfile-v1.json"
	schema.Title = "Agentfile v1"
	schema.Description = "Declarative specification for packaging AI agents with AgentCrate."

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

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(data))
}
