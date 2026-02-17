# Contributing to agentfile

Thank you for your interest in contributing to agentfile! This document provides guidelines and instructions for contributing.

## Code of Conduct

This project adheres to the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code.

## How to Contribute

### Reporting Bugs

Before creating a bug report, please check existing issues to avoid duplicates.

When filing a bug report, include:

- **agentfile version** (Go module version)
- **Go version** (`go version`)
- **Input YAML** that triggers the issue (sanitized of secrets)
- **Expected behavior** vs. **actual behavior**

### Suggesting Features

Feature requests are welcome! Please open an issue with:

- A clear description of the feature
- The use case it addresses
- Any proposed schema changes

### Pull Requests

1. **Fork** the repository
2. **Create a branch** from `main`: `git checkout -b feat/my-feature`
3. **Make your changes** following the coding standards below
4. **Add tests** for any new functionality
5. **Run the test suite**: `make test`
6. **Run the linter**: `make lint`
7. **Commit** using conventional commit messages
8. **Push** and open a Pull Request

## Development Setup

### Prerequisites

- Go 1.22+
- [golangci-lint](https://golangci-lint.run/) (for linting)

### Getting Started

```bash
git clone https://github.com/agentcrate/agentfile.git
cd agentfile
make test
```

## Coding Standards

### Go Code

- Follow [Effective Go](https://go.dev/doc/effective_go) and [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- Use `gofmt` / `goimports` for formatting
- All exported functions must have doc comments
- Error variables use `Err` prefix: `ErrInvalidAgentfile`
- Tests are co-located: `foo.go` -> `foo_test.go`

### Commit Messages

We follow [Conventional Commits](https://www.conventionalcommits.org/):

```text
type(scope): description

feat(schema): add build section with base_image
fix(parser): handle missing metadata section gracefully
docs(readme): add usage examples
test(policy): add domain validation tests
```

Types: `feat`, `fix`, `docs`, `test`, `chore`, `refactor`, `perf`, `ci`

### Branch Naming

- `feat/description` for features
- `fix/description` for bug fixes
- `docs/description` for documentation
- `refactor/description` for refactoring

## Testing

- All new code must have accompanying tests
- Tests must pass with race detection: `go test -race ./...`
- Aim for meaningful coverage, not 100% line coverage
- Use table-driven tests where appropriate

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.
