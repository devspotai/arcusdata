# ---- Go library Makefile ----

# Auto-detect module path from go.mod
MODULE := $(shell awk '/^module /{print $$2}' go.mod)
PKGS   := $(shell go list ./...)
GO     ?= go

# Optional: version info from git (useful for logs/tests)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

## Compile all packages (no binary). Good sanity check for a library.
build:
	@echo "📦 Building (compile only) $(MODULE)"
	$(GO) build -v $(PKGS)

## Run with coverage + race detector
test:
	$(GO) test -race -cover -coverprofile=coverage.out ./...

## Show coverage summary
cover: test
	$(GO) tool cover -func=coverage.out

## Format, vet, tidy, and verify modules
fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

tidy:
	$(GO) mod tidy
	$(GO) mod verify

## Lint (requires golangci-lint)
lint:
	golangci-lint run ./...

## Generate (if you use go:generate)
generate:
	$(GO) generate ./...

## Clean build/test artifacts
clean:
	rm -f coverage.out

## Everything you'd usually want in CI
.PHONY: all
all: fmt vet tidy build test lint cover