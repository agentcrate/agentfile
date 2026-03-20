<div align="center">

# agentfile

**The spec for AI agent packaging.**

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue?style=flat-square)](LICENSE)
[![CI](https://img.shields.io/github/actions/workflow/status/agentcrate/agentfile/ci.yml?branch=main&style=flat-square&label=CI)](https://github.com/agentcrate/agentfile/actions)
[![codecov](https://codecov.io/gh/agentcrate/agentfile/graph/badge.svg)](https://codecov.io/gh/agentcrate/agentfile)
[![Go Report Card](https://goreportcard.com/badge/github.com/agentcrate/agentfile?style=flat-square)](https://goreportcard.com/report/github.com/agentcrate/agentfile)

The canonical Go implementation of the Agentfile v1 specification.

[Getting Started](#getting-started) · [Documentation](https://agentcrate.ai) · [Contributing](CONTRIBUTING.md)

</div>

---

## What is agentfile?

`agentfile` is the Go library that defines the **Agentfile v1** specification — the declarative YAML format for packaging AI agents with [AgentCrate](https://agentcrate.ai). It provides type definitions, YAML parsing with JSON Schema validation, environment profile resolution, and policy checking.

Both [`crate`](https://github.com/agentcrate/crate) (the CLI) and [`crated`](https://github.com/agentcrate/crated) (the runtime daemon) depend on this package as their shared contract.

## Features

- **Type definitions** for the Agentfile v1 spec (`Agentfile`, `Metadata`, `Brain`, `Skill`, etc.)
- **YAML parsing** with JSON Schema validation and line-level error reporting
- **Semantic validation** for cross-field consistency (policy refs, model defaults, URL formats)
- **Profile resolution** for environment-specific overrides (dev/staging/prod)
- **Policy checking** for security constraint validation (domains, permissions, HITL rules)
- **Embedded JSON Schema** for compile-time schema access via `go:embed`

## Getting Started

### Install

```bash
go get github.com/agentcrate/agentfile@latest
```

### Quick Start

```go
package main

import (
    "fmt"
    "log"
    "os"

    "github.com/agentcrate/agentfile"
)

func main() {
    data, err := os.ReadFile("Agentfile")
    if err != nil {
        log.Fatal(err)
    }

    result, err := agentfile.Parse(data)
    if err != nil {
        log.Fatal(err)
    }

    if !result.IsValid() {
        for _, e := range result.Errors {
            fmt.Fprintf(os.Stderr, "  %s: %s\n", e.Field, e.Message)
        }
        os.Exit(1)
    }

    af := result.Agentfile
    fmt.Printf("Agent: %s v%s\n", af.Metadata.Name, af.Metadata.Version)
    fmt.Printf("Model: %s\n", af.Brain.Default)
    fmt.Printf("Skills: %d\n", len(af.Skills))
}
```

### Profile Resolution

```go
// Resolve a named profile (merges overrides onto base config).
resolved, err := agentfile.ResolveProfile(af, "prod")
```

### Policy Checking

```go
// Validate policy consistency.
result := agentfile.CheckPolicies(af)
if result.HasErrors() {
    for _, e := range result.Errors() {
        fmt.Println(e)
    }
}
```

## Building from Source

```bash
git clone https://github.com/agentcrate/agentfile.git
cd agentfile

# Run tests
make test

# Run linter
make lint
```

### Requirements

- Go 1.24+

## Architecture

`agentfile` is part of the AgentCrate ecosystem:

| Component | Description | License |
| --------- | ----------- | ------- |
| **`agentfile`** (this repo) | Agentfile v1 spec, types, parsing, and validation | Apache 2.0 |
| [`api`](https://github.com/agentcrate/api) | Protocol Buffer definitions for all AgentCrate services | Apache 2.0 |
| [`crate`](https://github.com/agentcrate/crate) | CLI for building, validating, and publishing agent images | Apache 2.0 |
| [`crated`](https://github.com/agentcrate/crated) | Agent runtime daemon (container entrypoint) | Apache 2.0 |

### Project Structure

```text
agentfile/
├── doc.go           # Package documentation
├── types.go         # Agentfile v1 type definitions
├── parser.go        # YAML parsing + JSON Schema + semantic validation
├── profile.go       # Environment profile resolution
├── policy.go        # Security policy checking
├── schema.go        # Embedded JSON Schema (go:embed)
└── schema/
    └── agentfile-v1.json   # Agentfile v1 JSON Schema
```

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

### Development Workflow

```bash
# Run tests with race detection
make test

# Run linter
make lint

# Generate coverage report
make coverage
```

## Security

If you discover a security vulnerability, please report it responsibly. See [SECURITY.md](SECURITY.md) for details.

## License

Licensed under the [Apache License, Version 2.0](LICENSE).

Copyright 2026 AgentCrate Contributors.
