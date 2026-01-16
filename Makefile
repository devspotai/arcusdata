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
	$(GO) build -v ./...

## Run with coverage + race detector
test:
	$(GO) test -race -cover -coverprofile=coverage.out ./...

test-html:
	$(GO) tool cover -html=coverage.out -o coverage.html

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

## ---------- Tagging helpers ---------- ##
# You can also do:
#   make tag VERSION=v1.2.3
# or:
#   make tag-patch      # v0.0.0 -> v0.0.1
#   make tag-minor      # v0.0.3 -> v0.1.0
#   make tag-major      # v0.2.5 -> v1.0.0
#
# Release shortcuts (runs tests + tidy first):
#   make release-patch
#   make release-minor
#   make release-major

# Manual tag: VERSION=vX.Y.Z
tag:
ifndef VERSION
	$(error VERSION is not set. Usage: make tag VERSION=v1.2.3)
endif
	@git tag $(VERSION)
	@git push origin $(VERSION)
	@echo "Tagged and pushed $(VERSION)"

# Auto bump patch version: vX.Y.Z -> vX.Y.(Z+1)
tag-patch:
	@old=$$(git describe --tags --abbrev=0 2>/dev/null || echo v0.0.0); \
	echo "Current tag: $$old"; \
	ver=$${old#v}; \
	major=$$(echo $$ver | cut -d. -f1); \
	minor=$$(echo $$ver | cut -d. -f2); \
	patch=$$(echo $$ver | cut -d. -f3); \
	patch=$$((patch+1)); \
	new=v$$major.$$minor.$$patch; \
	echo "Tagging $$new"; \
	git tag $$new; \
	git push origin $$new; \
	echo "Done."

# Auto bump minor version: vX.Y.Z -> vX.(Y+1).0
tag-minor:
	@old=$$(git describe --tags --abbrev=0 2>/dev/null || echo v0.0.0); \
	echo "Current tag: $$old"; \
	ver=$${old#v}; \
	major=$$(echo $$ver | cut -d. -f1); \
	minor=$$(echo $$ver | cut -d. -f2); \
	minor=$$((minor+1)); \
	new=v$$major.$$minor.0; \
	echo "Tagging $$new"; \
	git tag $$new; \
	git push origin $$new; \
	echo "Done."

# Auto bump major version: vX.Y.Z -> v(X+1).0.0
tag-major:
	@old=$$(git describe --tags --abbrev=0 2>/dev/null || echo v0.0.0); \
	echo "Current tag: $$old"; \
	ver=$${old#v}; \
	major=$$(echo $$ver | cut -d. -f1); \
	major=$$((major+1)); \
	new=v$$major.0.0; \
	echo "Tagging $$new"; \
	git tag $$new; \
	git push origin $$new; \
	echo "Done."

## ---------- Release shortcuts ---------- ##
# These ensure tests/tidy pass before tagging.

release-patch: test tidy tag-patch
	@echo "Patch release complete."

release-minor: test tidy tag-minor
	@echo "Minor release complete."

release-major: test tidy tag-major
	@echo "Major release complete."


