.PHONY: build check test

build:
	@echo "Building yappblocker..."
	@cd src && go build -o ../yappblocker .
	@echo "✓ Build complete: yappblocker"

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
	@echo "Running tests..."
	@cd src && go test -race ./...
	@echo "✓ Tests passed"

test:
	@echo "Running tests..."
	@cd src && go test -race ./...
	@echo "✓ Tests passed"
