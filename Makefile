.PHONY: all build test lint clean run install

# Version from git tag or default
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags="-H windowsgui -s -w -X main.Version=$(VERSION)"

# Default target
all: test build

# Build the application
build:
	go build $(LDFLAGS) -o home-sentry.exe

# Build without GUI flags (for CLI testing)
build-cli:
	go build -ldflags="-X main.Version=$(VERSION)" -o home-sentry-cli.exe

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Run linter (requires golangci-lint installed)
lint:
	golangci-lint run ./...

# Format code
fmt:
	go fmt ./...

# Tidy dependencies
tidy:
	go mod tidy

# Clean build artifacts
clean:
	rm -f home-sentry.exe home-sentry-cli.exe
	rm -f coverage.out coverage.html

# Run the application
run: build
	./home-sentry.exe

# Install to GOPATH/bin
install:
	go install $(LDFLAGS)

# Show version
version:
	@echo $(VERSION)

# Help
help:
	@echo "Home Sentry Build System"
	@echo ""
	@echo "Targets:"
	@echo "  all           - Run tests and build (default)"
	@echo "  build         - Build production executable"
	@echo "  build-cli     - Build CLI version for testing"
	@echo "  test          - Run all tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  lint          - Run golangci-lint"
	@echo "  fmt           - Format code"
	@echo "  tidy          - Tidy go.mod"
	@echo "  clean         - Remove build artifacts"
	@echo "  run           - Build and run"
	@echo "  install       - Install to GOPATH/bin"
	@echo "  version       - Show current version"
