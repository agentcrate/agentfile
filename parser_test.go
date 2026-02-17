package agentfile_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentcrate/agentfile"
)

func testdataPath(name string) string {
	return filepath.Join("testdata", name)
}

func mustReadTestdata(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(testdataPath(name))
	if err != nil {
		t.Fatalf("reading testdata/%s: %v", name, err)
	}
	return data
}

// --- Valid Agentfile tests ---

func TestParse_ValidFull(t *testing.T) {
	data := mustReadTestdata(t, "valid_full.yaml")
	result, err := agentfile.Parse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsValid() {
		t.Fatalf("expected valid, got %d errors:", len(result.Errors))
	}

	af := result.Agentfile
	if af.Version != "1" {
		t.Errorf("expected version '1', got %q", af.Version)
	}
	if af.Metadata.Name != "compliance-monitor" {
		t.Errorf("expected name 'compliance-monitor', got %q", af.Metadata.Name)
	}
	if af.Metadata.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %q", af.Metadata.Version)
	}
	if af.Brain.Default != "gpt" {
		t.Errorf("expected brain default 'gpt', got %q", af.Brain.Default)
	}
	if len(af.Brain.Models) != 2 {
		t.Errorf("expected 2 brain models, got %d", len(af.Brain.Models))
	}
	if af.Brain.Models[0].Name != "gpt" {
		t.Errorf("expected first model name 'gpt', got %q", af.Brain.Models[0].Name)
	}
	if af.Brain.Models[0].Model != "openai/gpt-4o" {
		t.Errorf("expected first model 'openai/gpt-4o', got %q", af.Brain.Models[0].Model)
	}
	if af.Brain.Models[0].Temperature == nil || *af.Brain.Models[0].Temperature != 0.3 {
		t.Errorf("expected first model temperature 0.3, got %v", af.Brain.Models[0].Temperature)
	}
	if len(af.Skills) != 3 {
		t.Errorf("expected 3 skills, got %d", len(af.Skills))
	}
	if af.Skills[0].Type != "http" {
		t.Errorf("expected first skill type 'http', got %q", af.Skills[0].Type)
	}
	if af.Policies == nil {
		t.Fatal("expected policies to be present")
	}
	if len(af.Policies.AllowedDomains) != 2 {
		t.Errorf("expected 2 allowed domains, got %d", len(af.Policies.AllowedDomains))
	}
	if len(af.Policies.ToolPermissions) != 2 {
		t.Errorf("expected 2 tool permissions, got %d", len(af.Policies.ToolPermissions))
	}
	if len(af.Profiles) != 2 {
		t.Errorf("expected 2 profiles, got %d", len(af.Profiles))
	}
	devProfile, ok := af.Profiles["dev"]
	if !ok {
		t.Fatal("expected 'dev' profile")
	}
	if devProfile.Brain == nil || devProfile.Brain.Default != "local" {
		t.Error("expected dev profile brain.default to be 'local'")
	}
}

func TestParse_ValidMinimal(t *testing.T) {
	data := mustReadTestdata(t, "valid_minimal.yaml")
	result, err := agentfile.Parse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsValid() {
		for _, e := range result.Errors {
			t.Logf("  error: %s: %s", e.Field, e.Message)
		}
		t.Fatalf("expected valid agentfile, got %d errors", len(result.Errors))
	}

	af := result.Agentfile
	if af.Metadata.Name != "minimal-agent" {
		t.Errorf("expected name 'minimal-agent', got %q", af.Metadata.Name)
	}
	if af.Policies != nil {
		t.Error("expected nil policies for minimal agentfile")
	}
	if len(af.Profiles) != 0 {
		t.Errorf("expected no profiles, got %d", len(af.Profiles))
	}
}

// --- Schema validation error tests ---

func TestParse_MissingMetadata(t *testing.T) {
	data := mustReadTestdata(t, "missing_metadata.yaml")
	result, err := agentfile.Parse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsValid() {
		t.Fatal("expected validation errors for missing metadata")
	}

	found := false
	for _, e := range result.Errors {
		if strings.Contains(e.Message, "metadata") || strings.Contains(e.Field, "metadata") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected an error mentioning 'metadata'")
		for _, e := range result.Errors {
			t.Logf("  got: %s: %s", e.Field, e.Message)
		}
	}
}

func TestParse_MissingMetadataVersion(t *testing.T) {
	data := mustReadTestdata(t, "missing_metadata_version.yaml")
	result, err := agentfile.Parse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsValid() {
		t.Fatal("expected validation errors for missing metadata.version")
	}

	found := false
	for _, e := range result.Errors {
		if strings.Contains(e.Field, "version") || strings.Contains(e.Message, "version") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected an error mentioning 'version'")
		for _, e := range result.Errors {
			t.Logf("  got: %s: %s", e.Field, e.Message)
		}
	}
}

// --- Semantic validation error tests ---

func TestParse_BadPolicyRefs(t *testing.T) {
	data := mustReadTestdata(t, "bad_policy_refs.yaml")
	result, err := agentfile.Parse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsValid() {
		t.Fatal("expected validation errors for bad policy refs")
	}

	// Should have errors for both tool_permissions and human_in_the_loop.
	var foundToolPerm, foundHITL bool
	for _, e := range result.Errors {
		if strings.Contains(e.Field, "tool_permissions") && strings.Contains(e.Value, "nonexistent-skill") {
			foundToolPerm = true
		}
		if strings.Contains(e.Field, "human_in_the_loop") && strings.Contains(e.Value, "also-nonexistent") {
			foundHITL = true
		}
	}
	if !foundToolPerm {
		t.Error("expected error for tool_permissions referencing undeclared skill")
	}
	if !foundHITL {
		t.Error("expected error for human_in_the_loop referencing undeclared skill")
	}
}

func TestParse_BadMCPSource(t *testing.T) {
	data := mustReadTestdata(t, "bad_mcp_source.yaml")
	result, err := agentfile.Parse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsValid() {
		t.Fatal("expected validation errors for bad MCP source")
	}

	found := false
	for _, e := range result.Errors {
		if strings.Contains(e.Message, "http skill source must be a valid") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error about invalid http skill source URL")
		for _, e := range result.Errors {
			t.Logf("  got: %s: %s", e.Field, e.Message)
		}
	}
}

// --- ParseFile tests ---

func TestParseFile_ValidFile(t *testing.T) {
	result, err := agentfile.ParseFile(testdataPath("valid_minimal.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsValid() {
		t.Fatalf("expected valid agentfile, got %d errors", len(result.Errors))
	}
}

func TestParseFile_NonexistentFile(t *testing.T) {
	_, err := agentfile.ParseFile("testdata/does_not_exist.yaml")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

// --- Edge case tests ---

func TestParse_InvalidYAML(t *testing.T) {
	data := []byte("{{not valid yaml")
	_, err := agentfile.Parse(data)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestParse_EmptyDocument(t *testing.T) {
	data := []byte("")
	result, err := agentfile.Parse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsValid() {
		t.Fatal("expected validation errors for empty document")
	}
}

func TestValidationError_Error(t *testing.T) {
	e := agentfile.ValidationError{
		Field:   "metadata.name",
		Message: "is required",
	}
	got := e.Error()
	if got != "metadata.name: is required" {
		t.Errorf("unexpected Error() output: %q", got)
	}
}

func TestValidationError_Error_WithValue(t *testing.T) {
	e := agentfile.ValidationError{
		Field:   "skills[0].source",
		Message: "MCP source must be a valid URI",
		Value:   `"not-a-uri"`,
	}
	got := e.Error()
	expected := `skills[0].source: MCP source must be a valid URI (got "not-a-uri")`
	if got != expected {
		t.Errorf("unexpected Error() output:\n  got:  %q\n  want: %q", got, expected)
	}
}

func TestParse_SkillTypes(t *testing.T) {
	// Valid skills with different types.
	yaml := `
version: "1"
metadata:
  name: test-agent
  version: "1.0.0"
  description: "test"
brain:
  default: gpt
  models:
    - name: gpt
      model: openai/gpt-4o
persona:
  system_prompt: "test"
skills:
  - name: my-stdio
    type: stdio
    command: npx
    args:
      - "-y"
      - "@example/my-server"
  - name: my-registry
    type: mcp
    source: cratehub.ai/tools/calculator
`
	result, err := agentfile.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsValid() {
		for _, e := range result.Errors {
			t.Logf("error: %s: %s", e.Field, e.Message)
		}
		t.Fatalf("expected valid, got %d errors", len(result.Errors))
	}
	if len(result.Agentfile.Skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(result.Agentfile.Skills))
	}
}

// --- Line number tests ---

func TestParse_LineNumbers_SchemaErrors(t *testing.T) {
	// Missing required fields should report line numbers > 0.
	data := mustReadTestdata(t, "missing_metadata.yaml")
	result, err := agentfile.Parse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsValid() {
		t.Fatal("expected validation errors")
	}
	for _, e := range result.Errors {
		if e.Line == 0 {
			t.Errorf("expected non-zero line number for error %q (field=%s)", e.Message, e.Field)
		}
	}
}

func TestParse_LineNumbers_SemanticErrors(t *testing.T) {
	data := mustReadTestdata(t, "bad_mcp_source.yaml")
	result, err := agentfile.Parse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsValid() {
		t.Fatal("expected validation errors")
	}

	for _, e := range result.Errors {
		if strings.Contains(e.Message, "MCP source") {
			if e.Line == 0 {
				t.Errorf("expected non-zero line number for MCP source error, got 0")
			}
			if e.Line > 0 {
				t.Logf("MCP source error correctly resolved to line %d", e.Line)
			}
		}
	}
}
