SHELL := /bin/bash

VERSION_PKG := github.com/mieubrisse/yappblocker/internal/buildinfo

GIT_DIRTY := $(shell git diff --quiet 2>/dev/null && echo clean || echo dirty)
GIT_HASH  := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
GIT_TAG   := $(shell git describe --tags --exact-match HEAD 2>/dev/null)

ifeq ($(GIT_DIRTY),clean)
  ifneq ($(GIT_TAG),)
    VERSION := $(patsubst v%,%,$(GIT_TAG))
  else
    VERSION := $(GIT_HASH)
  endif
else
  VERSION := $(GIT_HASH)-dirty
endif

LDFLAGS   := -X $(VERSION_PKG).Version=$(VERSION)
BINARY    := yappblocker
BUILD_DIR := _build

.PHONY: build check clean compile fmt run setup test

setup:
	@if git rev-parse --git-dir >/dev/null 2>&1; then \
		current=$$(git config core.hooksPath 2>/dev/null); \
		if [ "$$current" != ".githooks" ]; then \
			git config core.hooksPath .githooks; \
			echo "Git hooks configured (.githooks/)"; \
		fi; \
	fi

# The full check pipeline. For what it needs on the host, see tools/check/main.go.
check:
	go -C tools run ./check

compile:
	@echo "Building $(BINARY)..."
	@mkdir -p $(BUILD_DIR)
	@cd src && go build -ldflags "$(LDFLAGS)" -o ../$(BUILD_DIR)/$(BINARY) .
	@echo "✓ Build complete: $(BUILD_DIR)/$(BINARY)"

fmt:
	@cd src && gofmt -w .
	@echo "✓ Formatted"

build: setup check compile

run: compile
	@$(BUILD_DIR)/$(BINARY) $(ARGS)

test:
	@echo "Running tests..."
	@cd src && go test -race ./...
	@echo "✓ Tests passed"

clean:
	rm -rf $(BUILD_DIR)
