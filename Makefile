.PHONY: build check test

GO_MODULE := github.com/mieubrisse/yappblocker

build:
	go build -o yappblocker .

check:
	@echo "==> Checking go.mod tidiness..."
	@cp go.mod go.mod.bak && cp go.sum go.sum.bak
	@go mod tidy
	@diff go.mod go.mod.bak > /dev/null 2>&1 && diff go.sum go.sum.bak > /dev/null 2>&1 || \
		(echo "ERROR: go.mod/go.sum are not tidy — run 'go mod tidy' and commit the result" && rm go.mod.bak go.sum.bak && exit 1)
	@rm go.mod.bak go.sum.bak

	@echo "==> Running gofmt..."
	@test -z "$$(gofmt -l .)" || (echo "ERROR: gofmt found unformatted files:" && gofmt -l . && exit 1)

	@echo "==> Running go vet..."
	go vet ./...

	@echo "==> Running golangci-lint..."
	golangci-lint run ./...

	@echo "==> Running govulncheck..."
	govulncheck ./...

	@echo "==> Running deadcode (informational)..."
	-deadcode ./...

	@echo "==> Running tests..."
	go test -race ./...

test:
	go test -race ./...
