@AGENTS.md

# Agentfile — Schema & Validation Library

Go library for parsing, validating, and resolving Agentfile v1 YAML specs. Used by `crate` (CLI) and `crated` (runtime).

## Build & Dev

All commands go through `make`:

| Target | Purpose |
|--------|---------|
| `make test` | Tests with `-race` and verbose |
| `make test-short` | Tests with `-race` (no verbose) |
| `make lint` | golangci-lint |
| `make coverage` | Coverage report |
| `make schema` | Regenerate JSON Schema from Go types |

Never run `go test` directly — always use `make`.

## Architecture

```text
types.go           All Agentfile v1 type definitions
parser.go          YAML parsing, JSON Schema validation, semantic checks
profile.go         Environment-specific config resolution
policy.go          Security policy validation
schema.go          Embedded JSON Schema (go:embed)
cmd/genschema/     CLI tool: reflect types → JSON Schema
schema/            Embedded agentfile-v1.json
testdata/          14 YAML fixtures for testing
```

## Parse Pipeline (4 phases)

```text
YAML bytes → Phase 1: YAML AST (preserves line numbers)
           → Phase 2: JSON Schema validation (Draft 2020-12)
           → Phase 3: Typed unmarshal into Agentfile struct
           → Phase 4: Semantic checks (cross-field refs, URL validation)
```

All phases run even if earlier ones fail (partial recovery). Errors include 1-indexed source line numbers.

## Public API

```go
// Parse raw YAML
result, err := agentfile.Parse(data)
if !result.IsValid() {
    for _, e := range result.Errors {
        fmt.Printf("%s (line %d): %s\n", e.Field, e.Line, e.Message)
    }
}

// Parse from file
result, err := agentfile.ParseFile("Agentfile")

// Check security policies
policyResult := agentfile.CheckPolicies(result.Agentfile)

// Resolve environment profile
resolved, err := agentfile.ResolveProfile(result.Agentfile, "staging")
```

## Schema Maintenance

Types in `types.go` are the source of truth. When you change types:

1. Update `types.go`
2. Run `make schema` to regenerate `schema/agentfile-v1.json`
3. `TestSchemaUpToDate` will fail if you forget step 2

The JSON Schema is embedded at build time via `go:embed`.

## Error Types

- `ValidationError` — parsing failures (Field, Message, Value, Line)
- `PolicyFinding` — policy check results (Severity, Rule, Field, Message, Value)
- `ProfileNotFoundError` — unknown profile name (includes available profiles)
- `Parse()` returns `(*ParseResult, error)` — the `error` is for I/O failures, validation issues go in `ParseResult.Errors`

## Profile Resolution

- Empty name or `"default"` → returns base config unchanged
- Profiles can only switch `brain.default` (not add models) and fully replace `policies`
- Returns a shallow copy — never mutates the original

## Testing

- **Fixture-driven** — 14 YAML files in `testdata/` (valid, schema errors, semantic errors, policy errors)
- **Black-box** (`package agentfile_test`) for parser, policy, schema tests
- **White-box** (`package agentfile`) for profile resolution and YAML helpers
- `TestSchemaUpToDate` ensures Go types and JSON Schema stay in sync

## Anti-Patterns

- **Never** edit `schema/agentfile-v1.json` by hand — always regenerate with `make schema`
- **Never** add validation logic that JSON Schema can express — put it in the schema, not in Go
- **Never** add fields to types.go without updating both the schema and test fixtures
- **Never** mutate the original `Agentfile` struct during profile resolution
- **Never** return raw `error` from `Parse()` for validation issues — use `ParseResult.Errors`
- **Max input size** is 1 MB — enforced in parser, do not increase

## Key References

- Architecture: `ARCHITECTURE.md` in this repo
- Agentfile spec: `types.go` (authoritative)
- JSON Schema: `schema/agentfile-v1.json`
