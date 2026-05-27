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

	// Post-processing via the shared function (same as cmd/genschema/main.go).
	agentfile.ApplySchemaOverrides(schema)

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
		// Surface the first differing region so a CI failure is debuggable
		// without re-running `make schema` locally.
		t.Fatalf("schema/agentfile-v1.json is out of date with types.go — run `make schema` to regenerate.\nfirst-diff:\n  want: %q\n   got: %q",
			snippetAtFirstDiff(want, got, 200),
			snippetAtFirstDiff(got, want, 200))
	}
}

// snippetAtFirstDiff returns up to maxLen characters of a starting at the first
// byte that differs from b. If a and b are identical, the empty string is
// returned. Snippets are clipped to keep CI logs compact.
func snippetAtFirstDiff(a, b string, maxLen int) string {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	if i == len(a) && i == len(b) {
		return ""
	}
	end := i + maxLen
	if end > len(a) {
		end = len(a)
	}
	return a[i:end]
}
