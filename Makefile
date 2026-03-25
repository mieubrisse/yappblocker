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

# Minimum per-package test coverage percentage. Packages below this threshold
# cause `make check` to fail. Raise over time as coverage improves.
COVERAGE_THRESHOLD := 60

# Packages excluded from coverage enforcement (one Go import-path grep pattern
# per line). Entry-point and code-generation packages that are not meaningfully
# unit-testable belong here.
COVERAGE_EXCLUDE_PATTERNS := \
	github.com/mieubrisse/yappblocker$$ \
	/cmd$$ \
	/cmd/version$$ \
	/internal/buildinfo$$ \
	/internal/launchd$$

.PHONY: build check clean compile fmt run setup test

setup:
	@if git rev-parse --git-dir >/dev/null 2>&1; then \
		current=$$(git config core.hooksPath 2>/dev/null); \
		if [ "$$current" != ".githooks" ]; then \
			git config core.hooksPath .githooks; \
			echo "Git hooks configured (.githooks/)"; \
		fi; \
	fi

check:
	@echo "Checking module tidiness..."
	@cd src && go mod tidy
	@dirty=$$(git diff -- src/go.mod src/go.sum); \
	untracked=$$(git ls-files --others --exclude-standard -- src/go.sum); \
	if [ -n "$$dirty" ] || [ -n "$$untracked" ]; then \
		echo "❌ go.mod or go.sum is not tidy:"; \
		if [ -n "$$dirty" ]; then echo "$$dirty"; fi; \
		if [ -n "$$untracked" ]; then echo "  New file: src/go.sum"; fi; \
		git checkout -- src/go.mod src/go.sum 2>/dev/null || true; \
		echo ""; \
		echo "Run: cd src && go mod tidy"; \
		exit 1; \
	fi
	@echo "✓ Modules OK"
	@echo "Checking code formatting..."
	@cd src && unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "❌ Files need formatting:"; \
		echo "$$unformatted"; \
		echo ""; \
		echo "Run: cd src && gofmt -w ."; \
		exit 1; \
	fi
	@echo "✓ Formatting OK"
	@echo "Running go vet..."
	@cd src && go vet ./...
	@echo "✓ Vet OK"
	@echo "Running linter..."
	@cd src && golangci-lint run ./...
	@echo "✓ Lint OK"
	@echo "Running govulncheck..."
	@cd src && govulncheck ./...
	@echo "✓ Vulncheck OK"
	@echo "Running deadcode analysis..."
	@cd src && output=$$(deadcode ./... 2>&1); \
	rc=$$?; \
	if [ "$$rc" -ne 0 ]; then \
		echo "❌ deadcode failed:"; \
		echo "$$output"; \
		exit 1; \
	fi; \
	if [ -n "$$output" ]; then \
		echo "⚠ Dead code found (informational — will become a hard error after cleanup):"; \
		echo "$$output"; \
	fi
	@echo "✓ Deadcode OK"
	@echo "Running tests with coverage..."
	@set -o pipefail; cd src && go test -race -coverprofile=coverage.out ./... 2>&1 | tee coverage-test.log
	@echo "✓ Tests passed"
	@echo "Checking per-package coverage (threshold: $(COVERAGE_THRESHOLD)%)..."
	@failed=0; \
	while IFS= read -r line; do \
		pkg=$$(echo "$$line" | awk '{for(i=1;i<=NF;i++) if($$i ~ /^github\.com\//) {print $$i; exit}}'); \
		if [ -z "$$pkg" ]; then continue; fi; \
		skip=0; \
		for pat in $(COVERAGE_EXCLUDE_PATTERNS); do \
			if echo "$$pkg" | grep -qE "$$pat"; then \
				skip=1; \
				break; \
			fi; \
		done; \
		if [ "$$skip" = "1" ]; then continue; fi; \
		if echo "$$line" | grep -q '\[no test files\]'; then \
			echo "  ✗ $$pkg: no test files"; \
			failed=1; \
			continue; \
		fi; \
		pct=$$(echo "$$line" | grep -oE '[0-9]+\.[0-9]+%' | tr -d '%'); \
		if [ -z "$$pct" ]; then continue; fi; \
		if [ "$$(echo "$$pct < $(COVERAGE_THRESHOLD)" | bc)" = "1" ]; then \
			echo "  ✗ $$pkg: $${pct}% < $(COVERAGE_THRESHOLD)%"; \
			failed=1; \
		fi; \
	done < src/coverage-test.log; \
	rm -f src/coverage.out src/coverage-test.log; \
	if [ "$$failed" = "1" ]; then \
		echo "❌ Some packages are below the $(COVERAGE_THRESHOLD)% coverage threshold"; \
		exit 1; \
	fi
	@echo "✓ Coverage OK"

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
