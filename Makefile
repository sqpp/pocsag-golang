# Makefile for POCSAG-GO
# Builds all tools with dynamic version information

VERSION ?= 2.1.0
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_ARCH := $(shell go env GOOS)/$(shell go env GOARCH)
GO_VERSION := $(shell go version | cut -d' ' -f3)

# Build flags for version information
LDFLAGS := -X 'github.com/sqpp/pocsag-golang.Version=$(VERSION)' \
           -X 'github.com/sqpp/pocsag-golang.BuildTime=$(BUILD_TIME)' \
           -X 'github.com/sqpp/pocsag-golang.GitCommit=$(GIT_COMMIT)' \
           -X 'github.com/sqpp/pocsag-golang.Author=marcell' \
           -X 'github.com/sqpp/pocsag-golang.ProjectURL=https://pagercast.com' \
           -X 'github.com/sqpp/pocsag-golang.BuildArch=$(BUILD_ARCH)' \
           -X 'github.com/sqpp/pocsag-golang.BuildGoVer=$(GO_VERSION)'

# Default target
.PHONY: all
all: build

# Build all tools
.PHONY: build
build:
	@echo "Building POCSAG-GO v$(VERSION)..."
	go build -ldflags "$(LDFLAGS)" -o bin/pocsag ./cmd/pocsag
	go build -ldflags "$(LDFLAGS)" -o bin/pocsag-decode ./cmd/pocsag-decode
	go build -ldflags "$(LDFLAGS)" -o bin/pocsag-burst ./cmd/pocsag-burst
	@echo "Build complete!"

# Install tools
.PHONY: install
install:
	go install -ldflags "$(LDFLAGS)" ./cmd/pocsag
	go install -ldflags "$(LDFLAGS)" ./cmd/pocsag-decode
	go install -ldflags "$(LDFLAGS)" ./cmd/pocsag-burst

# Test
.PHONY: test
test:
	go test -v ./...

# Clean build artifacts
.PHONY: clean
clean:
	rm -rf bin/

# Show version information
.PHONY: version
version:
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Git Commit: $(GIT_COMMIT)"

# Cross-compile for multiple platforms
.PHONY: cross-compile
cross-compile:
	@echo "Cross-compiling for multiple platforms..."
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/pocsag-linux-amd64 ./cmd/pocsag
	GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o bin/pocsag-linux-arm64 ./cmd/pocsag
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/pocsag-windows-amd64.exe ./cmd/pocsag
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/pocsag-darwin-amd64 ./cmd/pocsag
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o bin/pocsag-darwin-arm64 ./cmd/pocsag
	@echo "Cross-compilation complete!"

# Help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build        - Build all tools"
	@echo "  install      - Install tools to GOPATH/bin"
	@echo "  test         - Run tests"
	@echo "  clean        - Remove build artifacts"
	@echo "  version      - Show version information"
	@echo "  cross-compile - Build for multiple platforms"
	@echo "  help         - Show this help"
