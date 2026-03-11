# Vertex Traffic Generator - Build Makefile

# Build information
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -w -s -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)

# Default target
.PHONY: all
all: build

# Development build
.PHONY: build
build:
	go build -ldflags="$(LDFLAGS)" -o vertex .

# Run in development mode
.PHONY: run
run:
	go run -ldflags="$(LDFLAGS)" . --port 8080 --log debug

# Cross-platform builds (Linux only - requires MACVLAN kernel support)
.PHONY: build-all
build-all:
	@mkdir -p bin
	GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o bin/vertex-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o bin/vertex-linux-arm64 .
	@echo "Built binaries:"
	@ls -la bin/

# Create checksums
.PHONY: checksums
checksums: build-all
	cd bin && sha256sum * > checksums.txt
	@echo "Checksums created:"
	@cat bin/checksums.txt

# Clean build artifacts
.PHONY: clean
clean:
	rm -f vertex
	rm -rf bin/

# Test
.PHONY: test
test:
	go test -v ./...

# Format code
.PHONY: fmt
fmt:
	go fmt ./...

# Vet code
.PHONY: vet
vet:
	go vet ./...

# Lint (requires golangci-lint)
.PHONY: lint
lint:
	golangci-lint run

# Go mod tidy
.PHONY: tidy
tidy:
	go mod tidy

# Development checks
.PHONY: check
check: fmt vet test

# Install development tools
.PHONY: tools
tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Help
.PHONY: help
help:
	@echo "Vertex Traffic Generator - Build Commands"
	@echo ""
	@echo "  build       Build binary for current platform"
	@echo "  build-all   Build binaries for all platforms"
	@echo "  run         Run in development mode"
	@echo "  test        Run tests"
	@echo "  check       Run format, vet, and test"
	@echo "  clean       Clean build artifacts"
	@echo "  checksums   Create checksums for binaries"
	@echo "  help        Show this help"