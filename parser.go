package agentfile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"strings"
	"sync"

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

// maxAgentfileSize prevents OOM from oversized input (1 MB).
const maxAgentfileSize = 1 << 20

// ParseFile reads and validates an Agentfile from the filesystem.
func ParseFile(path string) (*ParseResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening agentfile: %w", err)
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(io.LimitReader(f, maxAgentfileSize+1))
	if err != nil {
		return nil, fmt.Errorf("reading agentfile: %w", err)
	}
	if len(data) > maxAgentfileSize {
		return nil, fmt.Errorf("agentfile exceeds maximum size of %d bytes", maxAgentfileSize)
	}
	return Parse(data)
}

// Parse validates raw YAML bytes against the Agentfile v1 schema
// and returns the parsed result.
func Parse(data []byte) (*ParseResult, error) {
	if len(data) > maxAgentfileSize {
		return nil, fmt.Errorf("agentfile data exceeds maximum size of %d bytes", maxAgentfileSize)
	}

	// YAML is parsed twice: first into yaml.Node (for line numbers in error
	// messages), then into any (for JSON Schema validation, which requires
	// json.Unmarshal-compatible types). Merging these would require walking the
	// yaml.Node tree manually.

	// Phase 1: Parse YAML into AST for line number resolution.
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parsing yaml: %w", err)
	}

	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing yaml to map: %w", err)
	}

	normalized := normalizeYAML(raw)

	// Phase 2: Validate against JSON Schema.
	validationErrors, err := validateSchema(normalized)
	if err != nil {
		return nil, fmt.Errorf("schema validation: %w", err)
	}

	// Resolve source line numbers from the YAML AST. Build a single dot-path
	// → line map up front so each error is an O(1) lookup instead of an
	// O(depth) walk from the document root. The semantic-validation phase
	// reuses the same index for the same reason.
	lineIdx := buildLineIndex(&doc)
	for i := range validationErrors {
		validationErrors[i].Line = lineIdx.lookup(&doc, validationErrors[i].Field)
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
	// Only runs on schema-valid input — semantic checks assume well-formed data.
	if len(validationErrors) == 0 {
		semanticErrors := validateSemantics(&af, &doc, lineIdx)
		result.Errors = append(result.Errors, semanticErrors...)
	}

	return result, nil
}

// compileSchema uses sync.Once: if compilation fails (e.g., corrupted embedded
// schema), the error is permanent and every subsequent Parse call returns it.
// This is intentional — a corrupted schema indicates a build-time defect, not a
// transient error.
var (
	compiledSchema     *jsonschema.Schema
	compiledSchemaOnce sync.Once
	compiledSchemaErr  error
)

// compileSchema lazily compiles the embedded JSON Schema exactly once.
func compileSchema() (*jsonschema.Schema, error) {
	compiledSchemaOnce.Do(func() {
		c := jsonschema.NewCompiler()
		schemaDoc, err := jsonschema.UnmarshalJSON(strings.NewReader(string(SchemaV1)))
		if err != nil {
			compiledSchemaErr = fmt.Errorf("unmarshaling schema: %w", err)
			return
		}
		if err := c.AddResource("agentfile-v1.json", schemaDoc); err != nil {
			compiledSchemaErr = fmt.Errorf("adding schema resource: %w", err)
			return
		}
		compiledSchema, compiledSchemaErr = c.Compile("agentfile-v1.json")
	})
	return compiledSchema, compiledSchemaErr
}

// validateSchema validates data against the embedded JSON Schema.
func validateSchema(data any) ([]ValidationError, error) {
	sch, err := compileSchema()
	if err != nil {
		return nil, err
	}

	// JSON round-trip: the jsonschema/v6 library requires values produced by
	// json.Unmarshal (not yaml.Unmarshal) for correct type matching. This
	// marshal+unmarshal step converts YAML-native types to JSON-native types.
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshaling to json: %w", err)
	}
	var jsonData any
	if err := json.Unmarshal(jsonBytes, &jsonData); err != nil {
		return nil, fmt.Errorf("unmarshaling json: %w", err)
	}

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
	var errs []ValidationError
	collectErrors(err, &errs)
	return errs
}

// TODO(i18n): Hardcoded to English for v1.
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
// lineIdx is a pre-built dot-path → source line index used to attach a 1-indexed
// line number to every error without re-walking the YAML AST per call.
func validateSemantics(af *Agentfile, doc *yaml.Node, lineIdx *lineIndex) []ValidationError {
	var errs []ValidationError

	// Build a set of declared model names.
	modelNames := make(map[string]struct{}, len(af.Brain.Models))
	for _, m := range af.Brain.Models {
		modelNames[m.Name] = struct{}{}
	}

	// Check that brain.default references a declared model name.
	if _, ok := modelNames[af.Brain.Default]; !ok {
		field := "brain.default"
		errs = append(errs, ValidationError{
			Field:   field,
			Message: "references undeclared model name",
			Value:   fmt.Sprintf("%q", af.Brain.Default),
			Line:    lineIdx.lookup(doc, field),
		})
	}

	// Check duplicate model names in brain.models.
	seen := make(map[string]struct{}, len(af.Brain.Models))
	for i, m := range af.Brain.Models {
		if _, dup := seen[m.Name]; dup {
			field := fmt.Sprintf("brain.models[%d].name", i)
			errs = append(errs, ValidationError{
				Field:   field,
				Message: "duplicate model name",
				Value:   fmt.Sprintf("%q", m.Name),
				Line:    lineIdx.lookup(doc, field),
			})
		}
		seen[m.Name] = struct{}{}
	}

	// Check that profile brain.default references a declared model name.
	// The nil guard must wrap the map lookup: accessing profile.Brain.Default
	// before checking profile.Brain != nil would panic on nil Brain pointers.
	for name, profile := range af.Profiles {
		if profile.Brain != nil {
			if _, ok := modelNames[profile.Brain.Default]; !ok {
				field := fmt.Sprintf("profiles.%s.brain.default", name)
				errs = append(errs, ValidationError{
					Field:   field,
					Message: "references undeclared model name",
					Value:   fmt.Sprintf("%q", profile.Brain.Default),
					Line:    lineIdx.lookup(doc, field),
				})
			}
		}
	}

	// Check duplicate skill names while building the declared-name set.
	// One pass: detect dupes, populate the lookup used by the policy-reference
	// checks below. Two separate maps would carry identical values.
	skillNames := make(map[string]struct{}, len(af.Skills))
	for i := range af.Skills {
		if _, dup := skillNames[af.Skills[i].Name]; dup {
			field := fmt.Sprintf("skills[%d].name", i)
			errs = append(errs, ValidationError{
				Field:   field,
				Message: "duplicate skill name",
				Value:   fmt.Sprintf("%q", af.Skills[i].Name),
				Line:    lineIdx.lookup(doc, field),
			})
		}
		skillNames[af.Skills[i].Name] = struct{}{}
	}

	if af.Policies != nil {
		// Check that tool_permissions reference declared skills.
		for i, tp := range af.Policies.ToolPermissions {
			if _, ok := skillNames[tp.Skill]; !ok {
				field := fmt.Sprintf("policies.tool_permissions[%d].skill", i)
				errs = append(errs, ValidationError{
					Field:   field,
					Message: "references undeclared skill",
					Value:   fmt.Sprintf("%q", tp.Skill),
					Line:    lineIdx.lookup(doc, field),
				})
			}
		}

		// Check that human_in_the_loop skills reference declared skills.
		for i, hitl := range af.Policies.HumanInTheLoop {
			if _, ok := skillNames[hitl.Skill]; !ok {
				field := fmt.Sprintf("policies.human_in_the_loop[%d].skill", i)
				errs = append(errs, ValidationError{
					Field:   field,
					Message: "references undeclared skill",
					Value:   fmt.Sprintf("%q", hitl.Skill),
					Line:    lineIdx.lookup(doc, field),
				})
			}
		}
	}

	// Validate skill source formats per type.
	for i := range af.Skills {
		field := fmt.Sprintf("skills[%d].source", i)
		switch af.Skills[i].Type {
		case "http", "sse":
			// Must be a valid URL with http/https scheme.
			if !isValidURL(af.Skills[i].Source) {
				errs = append(errs, ValidationError{
					Field:   field,
					Message: fmt.Sprintf("%s skill source must be a valid http/https URL", af.Skills[i].Type),
					Value:   fmt.Sprintf("%q", af.Skills[i].Source),
					Line:    lineIdx.lookup(doc, field),
				})
			}
		case "stdio":
			// stdio transports invoke a local binary; command identifies what to
			// run and args carry at minimum the server package or entrypoint.
			if strings.TrimSpace(af.Skills[i].Command) == "" {
				errs = append(errs, ValidationError{
					Field:   fmt.Sprintf("skills[%d].command", i),
					Message: "stdio skill must have a command",
					Line:    lineIdx.lookup(doc, fmt.Sprintf("skills[%d].name", i)),
				})
			}
			if len(af.Skills[i].Args) == 0 {
				errs = append(errs, ValidationError{
					Field:   fmt.Sprintf("skills[%d].args", i),
					Message: "stdio skill must have non-empty args",
					Line:    lineIdx.lookup(doc, fmt.Sprintf("skills[%d].name", i)),
				})
			}
		case "mcp":
			// Registry identifier -- must be non-empty.
			if strings.TrimSpace(af.Skills[i].Source) == "" {
				errs = append(errs, ValidationError{
					Field:   field,
					Message: "mcp skill source must be a non-empty registry identifier",
					Line:    lineIdx.lookup(doc, field),
				})
			}
		}
	}

	return errs
}

// isValidURL checks that a source string is a valid http/https URL
// with a non-empty host. Go's url.Parse is extremely lenient, so we
// also verify scheme and host explicitly.
func isValidURL(source string) bool {
	parsed, err := url.Parse(source)
	if err != nil {
		return false
	}
	return (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

// normalizeYAML deep-copies the value tree so the caller can mutate it
// without affecting the original yaml.Unmarshal output.
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

// lineIndex maps dot-notation field paths to their 1-indexed source line
// numbers. Building it once amortizes the cost of locating every error's
// source line, replacing an O(N×depth) per-error walk with O(1) lookups.
type lineIndex struct {
	rootLine int
	paths    map[string]int
}

// lookup returns the 1-indexed source line for dotPath. The (*yaml.Node)
// argument is kept as a safety fallback for any path that wasn't materialized
// into the index (defensive — the index is exhaustive in practice).
func (l *lineIndex) lookup(doc *yaml.Node, dotPath string) int {
	if l == nil {
		return resolveNodeLine(doc, dotPath)
	}
	if dotPath == "" || dotPath == "(root)" {
		return l.rootLine
	}
	if line, ok := l.paths[dotPath]; ok {
		return line
	}
	// Fallback for paths the walker didn't visit (e.g., unexpected shapes).
	return resolveNodeLine(doc, dotPath)
}

// buildLineIndex walks the YAML AST once, recording the source line for
// every reachable mapping key and sequence element keyed by the same
// dot-notation syntax used in ValidationError.Field.
func buildLineIndex(doc *yaml.Node) *lineIndex {
	idx := &lineIndex{paths: make(map[string]int)}
	if doc == nil {
		return idx
	}
	root := doc
	if root.Kind == yaml.DocumentNode {
		if len(root.Content) == 0 {
			return idx
		}
		root = root.Content[0]
	}
	idx.rootLine = root.Line
	walkLineIndex(root, "", idx.paths)
	return idx
}

// walkLineIndex recursively records (path → line) for every key in a
// mapping and every element in a sequence under prefix.
func walkLineIndex(node *yaml.Node, prefix string, out map[string]int) {
	if node == nil {
		return
	}
	switch node.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i].Value
			val := node.Content[i+1]
			var path string
			if prefix == "" {
				path = key
			} else {
				path = prefix + "." + key
			}
			out[path] = val.Line
			walkLineIndex(val, path, out)
		}
	case yaml.SequenceNode:
		for i, child := range node.Content {
			path := fmt.Sprintf("%s[%d]", prefix, i)
			out[path] = child.Line
			walkLineIndex(child, path, out)
		}
	}
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

// parseIndex converts a string to an integer index, returning -1 on failure
// or overflow. The ceiling is set to math.MaxInt32 to guard against crafted input.
func parseIndex(s string) int {
	if s == "" {
		return -1
	}
	idx := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return -1
		}
		idx = idx*10 + int(c-'0')
		if idx > math.MaxInt32 {
			return -1
		}
	}
	return idx
}
