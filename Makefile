# Makefile for agentfile

.PHONY: test lint clean coverage

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
	@rm -rf dist/

## help: Show this help message
help:
	@echo "Available targets:"
	@grep -E '^## ' Makefile | sed 's/## /  /'
