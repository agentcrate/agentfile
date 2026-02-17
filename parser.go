package agentfile

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"gopkg.in/yaml.v3"
)

// ValidationError represents a single validation failure with location context.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Value   string `json:"value,omitempty"` // the offending value, if applicable
	Line    int    `json:"line,omitempty"`  // 1-indexed source line, 0 if unknown
}

func (e *ValidationError) Error() string {
	if e.Value != "" {
		return fmt.Sprintf("%s: %s (got %s)", e.Field, e.Message, e.Value)
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ParseResult contains the parsed Agentfile and any validation errors.
type ParseResult struct {
	Agentfile *Agentfile
	Errors    []ValidationError
}

// IsValid returns true if no validation errors were found.
func (r *ParseResult) IsValid() bool {
	return len(r.Errors) == 0
}

// ParseFile reads and validates an Agentfile from the filesystem.
func ParseFile(path string) (*ParseResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading agentfile: %w", err)
	}
	return Parse(data)
}

// Parse validates raw YAML bytes against the Agentfile v1 schema
// and returns the parsed result.
func Parse(data []byte) (*ParseResult, error) {
	// Phase 0: Parse into yaml.Node to preserve source line numbers.
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing yaml: %w", err)
	}

	// Phase 1: YAML -> generic map for JSON Schema validation.
	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing yaml: %w", err)
	}

	// Convert YAML map keys from map[string]any (yaml.v3 does this by default).
	normalized := normalizeYAML(raw)

	// Phase 2: Validate against JSON Schema.
	validationErrors, err := validateSchema(normalized)
	if err != nil {
		return nil, fmt.Errorf("schema validation: %w", err)
	}

	// Resolve source line numbers from the YAML AST.
	for i := range validationErrors {
		validationErrors[i].Line = resolveNodeLine(&doc, validationErrors[i].Field)
	}

	result := &ParseResult{
		Errors: validationErrors,
	}

	// Phase 3: Unmarshal into typed struct (even if schema validation fails,
	// we try to parse for partial results).
	var af Agentfile
	if err := yaml.Unmarshal(data, &af); err != nil {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "(root)",
			Message: fmt.Sprintf("failed to unmarshal: %s", err),
		})
		return result, nil
	}
	result.Agentfile = &af

	// Phase 4: Semantic validation (cross-field checks the schema can't express).
	semanticErrors := validateSemantics(&af, &doc)
	result.Errors = append(result.Errors, semanticErrors...)

	return result, nil
}

// validateSchema compiles and validates data against the embedded JSON Schema.
func validateSchema(data any) ([]ValidationError, error) {
	// Compile the schema.
	c := jsonschema.NewCompiler()
	schemaDoc, err := jsonschema.UnmarshalJSON(strings.NewReader(string(SchemaV1)))
	if err != nil {
		return nil, fmt.Errorf("unmarshaling schema: %w", err)
	}
	if err := c.AddResource("agentfile-v1.json", schemaDoc); err != nil {
		return nil, fmt.Errorf("adding schema resource: %w", err)
	}
	sch, err := c.Compile("agentfile-v1.json")
	if err != nil {
		return nil, fmt.Errorf("compiling schema: %w", err)
	}

	// Convert data to JSON-compatible form for the validator.
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshaling to json: %w", err)
	}
	var jsonData any
	if err := json.Unmarshal(jsonBytes, &jsonData); err != nil {
		return nil, fmt.Errorf("unmarshaling json: %w", err)
	}

	// Validate.
	err = sch.Validate(jsonData)
	if err == nil {
		return nil, nil
	}

	var validationErr *jsonschema.ValidationError
	if !errors.As(err, &validationErr) {
		return nil, fmt.Errorf("unexpected validation error type: %w", err)
	}

	return flattenValidationErrors(validationErr), nil
}

// flattenValidationErrors converts the nested jsonschema.ValidationError tree
// into a flat list of ValidationError values.
func flattenValidationErrors(err *jsonschema.ValidationError) []ValidationError {
	var result []ValidationError
	collectErrors(err, &result)
	return result
}

// printer is an English message printer for localizing JSON Schema errors.
var printer = message.NewPrinter(language.English)

func collectErrors(err *jsonschema.ValidationError, out *[]ValidationError) {
	if len(err.Causes) == 0 {
		field := pointerToDot(err.InstanceLocation)
		msg := err.ErrorKind.LocalizedString(printer)
		*out = append(*out, ValidationError{
			Field:   field,
			Message: msg,
		})
		return
	}
	for _, cause := range err.Causes {
		collectErrors(cause, out)
	}
}

// pointerToDot converts a JSON Pointer path slice like ["skills", "0", "source"]
// into human-readable dot notation like "skills[0].source".
func pointerToDot(parts []string) string {
	if len(parts) == 0 {
		return "(root)"
	}
	var b strings.Builder
	for i, part := range parts {
		// If the part is numeric, render as array index.
		if isNumeric(part) {
			fmt.Fprintf(&b, "[%s]", part)
		} else {
			if i > 0 && !isNumeric(parts[i-1]) {
				b.WriteByte('.')
			}
			b.WriteString(part)
		}
	}
	return b.String()
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return s != ""
}

// validateSemantics performs cross-field validation that JSON Schema can't express.
func validateSemantics(af *Agentfile, doc *yaml.Node) []ValidationError {
	var errs []ValidationError

	// Build a set of declared model names.
	modelNames := make(map[string]bool, len(af.Brain.Models))
	for _, m := range af.Brain.Models {
		modelNames[m.Name] = true
	}

	// Check that brain.default references a declared model name.
	if !modelNames[af.Brain.Default] {
		field := "brain.default"
		errs = append(errs, ValidationError{
			Field:   field,
			Message: "references undeclared model name",
			Value:   fmt.Sprintf("%q", af.Brain.Default),
			Line:    resolveNodeLine(doc, field),
		})
	}

	// Check duplicate model names in brain.models.
	seen := make(map[string]bool, len(af.Brain.Models))
	for i, m := range af.Brain.Models {
		if seen[m.Name] {
			field := fmt.Sprintf("brain.models[%d].name", i)
			errs = append(errs, ValidationError{
				Field:   field,
				Message: "duplicate model name",
				Value:   fmt.Sprintf("%q", m.Name),
				Line:    resolveNodeLine(doc, field),
			})
		}
		seen[m.Name] = true
	}

	// Check that profile brain.default references a declared model name.
	for name, profile := range af.Profiles {
		if profile.Brain != nil && !modelNames[profile.Brain.Default] {
			field := fmt.Sprintf("profiles.%s.brain.default", name)
			errs = append(errs, ValidationError{
				Field:   field,
				Message: "references undeclared model name",
				Value:   fmt.Sprintf("%q", profile.Brain.Default),
				Line:    resolveNodeLine(doc, field),
			})
		}
	}

	// Build a set of declared skill names.
	skillNames := make(map[string]bool, len(af.Skills))
	for _, s := range af.Skills {
		skillNames[s.Name] = true
	}

	if af.Policies != nil {
		// Check that tool_permissions reference declared skills.
		for i, tp := range af.Policies.ToolPermissions {
			if !skillNames[tp.Skill] {
				field := fmt.Sprintf("policies.tool_permissions[%d].skill", i)
				errs = append(errs, ValidationError{
					Field:   field,
					Message: "references undeclared skill",
					Value:   fmt.Sprintf("%q", tp.Skill),
					Line:    resolveNodeLine(doc, field),
				})
			}
		}

		// Check that human_in_the_loop tools reference declared skills.
		for i, hitl := range af.Policies.HumanInTheLoop {
			if !skillNames[hitl.Tool] {
				field := fmt.Sprintf("policies.human_in_the_loop[%d].tool", i)
				errs = append(errs, ValidationError{
					Field:   field,
					Message: "references undeclared skill",
					Value:   fmt.Sprintf("%q", hitl.Tool),
					Line:    resolveNodeLine(doc, field),
				})
			}
		}
	}

	// Validate skill source formats per type.
	for i, s := range af.Skills {
		field := fmt.Sprintf("skills[%d].source", i)
		switch s.Type {
		case "http", "sse":
			// Must be a valid URL with http/https scheme.
			if !isValidURL(s.Source) {
				errs = append(errs, ValidationError{
					Field:   field,
					Message: fmt.Sprintf("%s skill source must be a valid http/https URL", s.Type),
					Value:   fmt.Sprintf("%q", s.Source),
					Line:    resolveNodeLine(doc, field),
				})
			}
		case "stdio":
			// Must be a non-empty path.
			if strings.TrimSpace(s.Source) == "" {
				errs = append(errs, ValidationError{
					Field:   field,
					Message: "stdio skill source must be a non-empty path",
					Line:    resolveNodeLine(doc, field),
				})
			}
		case "mcp":
			// Registry identifier -- must be non-empty.
			if strings.TrimSpace(s.Source) == "" {
				errs = append(errs, ValidationError{
					Field:   field,
					Message: "mcp skill source must be a non-empty registry identifier",
					Line:    resolveNodeLine(doc, field),
				})
			}
		}
	}

	return errs
}

// isValidURL checks that a source string is a valid http/https URL.
func isValidURL(source string) bool {
	if !strings.HasPrefix(source, "http://") && !strings.HasPrefix(source, "https://") {
		return false
	}
	_, err := url.Parse(source)
	return err == nil
}

// normalizeYAML converts YAML-specific types (map[string]any is default for
// yaml.v3 so this is mostly a pass-through, but handles edge cases).
func normalizeYAML(v any) any {
	switch val := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(val))
		for k, v := range val {
			m[k] = normalizeYAML(v)
		}
		return m
	case []any:
		s := make([]any, len(val))
		for i, v := range val {
			s[i] = normalizeYAML(v)
		}
		return s
	default:
		return v
	}
}

// resolveNodeLine walks the yaml.Node tree to find the node at the given
// dot-notation path and returns its 1-indexed source line number.
// Returns 0 if the path cannot be resolved.
func resolveNodeLine(doc *yaml.Node, dotPath string) int {
	if dotPath == "" || dotPath == "(root)" {
		if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
			return doc.Content[0].Line
		}
		return 0
	}

	// Parse dot-notation path into segments: "skills[0].source" -> ["skills", "0", "source"]
	parts := parseDotPath(dotPath)

	// Start at the document root.
	node := doc
	if node.Kind == yaml.DocumentNode {
		if len(node.Content) == 0 {
			return 0
		}
		node = node.Content[0]
	}

	for _, part := range parts {
		if node == nil {
			return 0
		}
		switch node.Kind {
		case yaml.MappingNode:
			node = findMappingValue(node, part)
		case yaml.SequenceNode:
			idx := parseIndex(part)
			if idx < 0 || idx >= len(node.Content) {
				return 0
			}
			node = node.Content[idx]
		default:
			return 0
		}
	}

	if node != nil {
		return node.Line
	}
	return 0
}

// parseDotPath splits a dot-notation path like "skills[0].source" into
// individual segments: ["skills", "0", "source"].
func parseDotPath(path string) []string {
	var parts []string
	var current strings.Builder
	for i := 0; i < len(path); i++ {
		switch path[i] {
		case '.':
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		case '[':
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		case ']':
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(path[i])
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

// findMappingValue finds a key in a yaml.MappingNode and returns its value node.
func findMappingValue(node *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

// parseIndex converts a string to an integer index, returning -1 on failure.
func parseIndex(s string) int {
	idx := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return -1
		}
		idx = idx*10 + int(c-'0')
	}
	if s == "" {
		return -1
	}
	return idx
}
