Quality Checks
==============

Run `make check` before committing. It runs the full pipeline:

1. `go mod tidy` — fails if go.mod/go.sum drift
2. `gofmt` — fails if any file is unformatted
3. `go vet ./...` — static analysis
4. `golangci-lint run ./...` — linters (config in `.golangci.yml`)
5. `govulncheck ./...` — known vulnerability scan
6. `deadcode ./...` — unreachable code detection (informational, does not fail the build)
7. `go test -race ./...` — tests with race detector

Run `make test` for tests only (with `-race`).
