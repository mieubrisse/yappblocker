yappblocker
===========

A Go CLI that kills distracting macOS apps on a schedule. This file contains the non-obvious context an agent needs to work effectively in this repo.

Project Structure
-----------------

```
src/                    Go source code (module: github.com/mieubrisse/yappblocker)
  cmd/                  Cobra command definitions
  internal/             Internal packages
    config/             YAML config loading and validation
    killer/             App killing logic (osascript, pkill)
    launchd/            launchd agent install/uninstall
    schedule/           Schedule matching and time window logic
  main.go              Entry point
.github/workflows/      CI and release automation
.claude/                Claude Code settings
```

All Go code lives under `src/`. The `go.mod` file is in `src/`, so run Go commands from that directory (or use the Makefile, which handles this).

Building and Running
--------------------

```bash
make build      # Build the binary
make check      # Modules → formatting → vet → lint → govulncheck → deadcode → tests (with -race)
make test       # Tests only (with -race)
```

The sandbox is configured to allow writes to the Go build cache, so `make build`, `make check`, and `make test` work without disabling the sandbox.

Quality Checks
--------------

Run `make check` before committing. It runs the full pipeline:

1. `go mod tidy` — fails if go.mod/go.sum drift
2. `gofmt` — fails if any file is unformatted
3. `go vet ./...` — static analysis
4. `golangci-lint run ./...` — linters (config in `src/.golangci.yml`)
5. `govulncheck ./...` — known vulnerability scan
6. `deadcode ./...` — unreachable code detection (informational, does not fail the build)
7. `go test -race ./...` — tests with race detector
