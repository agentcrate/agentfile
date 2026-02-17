package agentfile

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// parseDotPath
// ---------------------------------------------------------------------------

func TestParseDotPath(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{"metadata", []string{"metadata"}},
		{"metadata.name", []string{"metadata", "name"}},
		{"skills[0].source", []string{"skills", "0", "source"}},
		{"policies.tool_permissions[0].skill", []string{"policies", "tool_permissions", "0", "skill"}},
		{"a[0][1].b", []string{"a", "0", "1", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseDotPath(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("parseDotPath(%q) = %v, want %v", tt.input, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseDotPath(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseIndex
// ---------------------------------------------------------------------------

func TestParseIndex(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"0", 0},
		{"1", 1},
		{"42", 42},
		{"", -1},
		{"abc", -1},
		{"1a", -1},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseIndex(tt.input)
			if got != tt.want {
				t.Errorf("parseIndex(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// findMappingValue
// ---------------------------------------------------------------------------

func TestFindMappingValue(t *testing.T) {
	// Build a simple mapping: {foo: bar, baz: qux}
	node := &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "foo"},
			{Kind: yaml.ScalarNode, Value: "bar"},
			{Kind: yaml.ScalarNode, Value: "baz"},
			{Kind: yaml.ScalarNode, Value: "qux"},
		},
	}

	if v := findMappingValue(node, "foo"); v == nil || v.Value != "bar" {
		t.Errorf("expected 'bar' for key 'foo', got %v", v)
	}
	if v := findMappingValue(node, "baz"); v == nil || v.Value != "qux" {
		t.Errorf("expected 'qux' for key 'baz', got %v", v)
	}
	if v := findMappingValue(node, "missing"); v != nil {
		t.Errorf("expected nil for missing key, got %v", v)
	}
}

// ---------------------------------------------------------------------------
// resolveNodeLine
// ---------------------------------------------------------------------------

func TestResolveNodeLine(t *testing.T) {
	yamlContent := `metadata:
  name: test-agent
  version: "1.0.0"
skills:
  - name: search
    source: cratehub.ai/tools/web-search
  - name: mcp-tool
    source: not-a-uri
`
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(yamlContent), &doc); err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}

	tests := []struct {
		path     string
		wantLine int
	}{
		{"(root)", 1},
		{"", 1},
		{"metadata", 2},       // value of metadata key starts at line 2
		{"metadata.name", 2},  // name: test-agent is on line 2
		{"skills", 5},         // skills sequence starts at line 5
		{"skills[0].name", 5}, // first skill's name
		{"skills[1].source", 8},
		{"nonexistent", 0},
		{"skills[99]", 0},
		{"skills[0].nonexistent", 0},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := resolveNodeLine(&doc, tt.path)
			if got != tt.wantLine {
				t.Errorf("resolveNodeLine(%q) = %d, want %d", tt.path, got, tt.wantLine)
			}
		})
	}
}

func TestResolveNodeLine_EmptyDocument(t *testing.T) {
	doc := &yaml.Node{Kind: yaml.DocumentNode}
	if got := resolveNodeLine(doc, "(root)"); got != 0 {
		t.Errorf("expected 0 for empty document, got %d", got)
	}
	if got := resolveNodeLine(doc, "anything"); got != 0 {
		t.Errorf("expected 0 for empty document path, got %d", got)
	}
}

func TestResolveNodeLine_ScalarRoot(t *testing.T) {
	// A document whose root is a scalar (unusual but valid YAML).
	doc := &yaml.Node{
		Kind: yaml.DocumentNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "hello", Line: 1},
		},
	}
	// Trying to walk a path on a scalar should return 0.
	if got := resolveNodeLine(doc, "foo"); got != 0 {
		t.Errorf("expected 0 for scalar root, got %d", got)
	}
}
