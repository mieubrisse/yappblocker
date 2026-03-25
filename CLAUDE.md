yappblocker
===========

A Go CLI that kills distracting macOS apps on a schedule. This file contains the non-obvious context an agent needs to work effectively in this repo — conventions, build commands, and workflow rules that cannot be inferred from reading the code alone.

Project Structure
-----------------

```
src/                    Go source code (module: github.com/mieubrisse/yappblocker)
  cmd/                  Cobra command definitions (directory-per-command)
    version/            "version" subcommand
  internal/             Internal packages
    buildinfo/          Build-time version injection via ldflags
    config/             YAML config loading and validation
    killer/             App killing logic (osascript, pkill)
    launchd/            launchd agent install/uninstall
    schedule/           Schedule matching and time window logic
  main.go              Entry point — calls cmd.Execute()
_build/                 Build output (gitignored)
.githooks/              Git hooks (configured via make setup)
.beads/                 Beads issue tracking config
.claude/                Claude Code settings
.github/workflows/      CI and release automation
```

All Go code lives under `src/`. The module path is `github.com/mieubrisse/yappblocker`. The `go.mod` file is in `src/`, so run Go commands from that directory (or use the Makefile, which handles this).

Command Structure
-----------------

Each command lives in its own package under `src/cmd/`, one directory per level of the command hierarchy:

```
src/cmd/
  root.go                  Root command (package cmd)
  version/version.go       "version" subcommand (package version)
  init.go                  "init" subcommand
  install.go               "install" subcommand
  uninstall.go             "uninstall" subcommand
  run.go                   "run" subcommand
```

Each command package exports:
- `CmdStr` — the command name as a constant
- `Cmd` — the `*cobra.Command` instance

Parent commands wire children via `init()` using `AddCommand()`.

Building and Running
--------------------

Always build via the Makefile — it injects the version string via ldflags from git state. Running `go build` directly produces a binary that reports version "dev".

```bash
make build      # Full pipeline: configure hooks → check → compile
make compile    # Build the binary only (skip checks)
make run        # Build and run (pass args via ARGS="...")
make check      # Modules → formatting → vet → lint → govulncheck → deadcode → tests (with -race)
make test       # Tests only (with -race)
make fmt        # Auto-format code
make clean      # Remove _build/
```

Use `make run` to build and run the binary in one step:

```bash
make run
make run ARGS="version"
```

The sandbox is configured to allow writes to the Go build cache, so `make build`, `make check`, and `make test` work without disabling the sandbox.

Dependencies
------------

Add library dependencies with `go get` (not `go install`, which is for CLI tools):

```bash
cd src && go get github.com/some/library@latest
```

Run `go mod tidy` after adding or removing dependencies.
