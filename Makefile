# Makefile for agentfile

.PHONY: test lint clean coverage schema

## schema: Regenerate JSON Schema from Go types
schema:
	@go run ./cmd/genschema > schema/agentfile-v1.json
	@echo "schema/agentfile-v1.json regenerated"

## test: Run all tests
test:
	@go test -race -v ./...

## test-short: Run tests without verbose output
test-short:
	@go test -race ./...

## lint: Run linters
lint:
	@golangci-lint run ./...

## coverage: Generate and view test coverage report
coverage:
	@go test -race -coverprofile=coverage.out -covermode=atomic ./...
	@go tool cover -func=coverage.out
	@echo "\nTo view in browser: go tool cover -html=coverage.out"

## clean: Remove build artifacts
clean:
	@rm -f coverage.out coverage.html

.PHONY: setup
setup: ## Install pre-commit hooks
	@pre-commit install
	@echo "pre-commit hooks installed"

## help: Show this help message
help:
	@echo "Available targets:"
	@grep -E '^## ' Makefile | sed 's/## /  /'
