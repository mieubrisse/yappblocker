yappblocker
===========

A Go CLI that kills distracting macOS apps on a schedule. This file is the repo's agent guidance: the non-obvious context, commands and workflow rules that reading the code will not give you. `CLAUDE.md` is a symlink to it, so there is one file to keep current.

Where Work Is Tracked
---------------------

Work is tracked in **Linear**, workspace [Kevin Today](https://linear.app/todayendeavors), team Autowork Factory (issue ids `KEV-N`). This repo's issues are found two ways, and a search needs both: the `yappblocker` label, and the project [Make yappblocker agent-ready](https://linear.app/todayendeavors/project/make-yappblocker-agent-ready-85bfbf5c92e0), whose issues often carry no label. Label a new issue for this repo `yappblocker`. Nothing is tracked inside the repo: no issue database, no TODO files. Work you discover but were not asked to do goes to Linear as a new issue.

The repo used Beads until KEV-136 removed it. Bead ids (`yappblk-*`) in old commits resolve through the git history of `.beads/issues.jsonl`.

The Check
---------

The full check is one command, run from the repo root:

```bash
go -C tools run ./check
```

CI and the pre-commit hook run exactly this string; `make check` is an alias for it. It needs only `go` and `git`: it re-runs itself under the Go toolchain pinned by the `toolchain` line in `src/go.mod`, builds golangci-lint, govulncheck and deadcode at the versions pinned in `tools/*.mod`, and fails if any version it ran with differs from its pin, so its verdict matches CI's. It then checks modules → formatting → vet → lint → govulncheck → deadcode → tests (`-race`) → per-package coverage. In a fresh environment, run it before anything else.

The first run on a machine downloads the pinned toolchain and modules from `proxy.golang.org`, and so may a first `make build` or `make test`. The sandbox allows writes to the Go build cache but not that download, so run the first one outside the sandbox.

How a Change Lands
------------------

Every change reaches `main` through a pull request. Never push to `main` directly, never force-push, and never skip the hooks with `--no-verify`.

1. Branch off `main`. Parallel work goes in its own git worktree; the build and hooks work from one.
2. Run the check and get it green before pushing.
3. Open a pull request against `main` that names the Linear issue it closes.
4. CI (`.github/workflows/ci.yml`) runs the check on the pull request and must be green. The CI retry budget is **two rounds**: if CI is still red after a second push of fixes, stop and write what is failing on the Linear issue rather than pushing more attempts.
5. Before merging, have a reviewer with fresh context (not the agent that wrote the change) read the diff.
6. Merge only with CI green and the review passed. If the task you were given does not say you may merge, stop at a green pull request and say so on the Linear issue.

No ruleset protects `main` yet, so these rules are the gate until [KEV-135](https://linear.app/todayendeavors/issue/KEV-135) adds one; do not treat a push that GitHub accepts as permission.

Merging does not ship anything. Releases come from pushing a `v*` tag, which runs goreleaser (`.github/workflows/release.yml`) and publishes binaries and the Homebrew formula. Tagging is a separate, deliberate step: do not tag as part of landing a change.

Commands
--------

Always build through the Makefile: it injects the version string via ldflags from git state. A bare `go build` produces a binary that reports version "dev".

```bash
make setup      # One-time per clone: points core.hooksPath at .githooks/
make check      # Alias for the check: go -C tools run ./check
make build      # setup → check → compile
make compile    # Build the binary only, into _build/ (skips checks)
make run        # Build and run; pass arguments via ARGS="..."
make test       # Tests only (-race)
make fmt        # Auto-format code
make clean      # Remove _build/
```

The pre-commit hook runs the check, but only in a clone where `make setup` has run; a worktree inherits its clone's setting.

To bump a pin, change its source of truth: `go -C src mod edit -toolchain=go1.X.Y` for Go, `go -C tools get -tool -modfile=<tool>.mod <package>@<version>` for a check tool.

Project Structure
-----------------

```
src/                    Go source code (module: github.com/mieubrisse/yappblocker)
  cmd/                  Cobra commands
  internal/             Internal packages (config, schedule, killer, launchd, pause, notify, confirm, buildinfo)
  main.go               Entry point — calls cmd.Execute()
tools/                  The check (its own module) and one modfile per pinned check tool
_build/                 Build output (gitignored)
docs/plans/             Original design and implementation plans
.githooks/              Git hooks (wired by make setup)
.claude/                Claude Code settings
.github/workflows/      CI and release automation
```

All Go code lives under `src/`, and `go.mod` is there, so run Go commands from `src/` (or use the Makefile, which does).

Each subcommand is a file in package `cmd` that defines its `*cobra.Command` and registers it on `rootCmd` with `AddCommand()` in `init()`. `version` is the one subcommand in its own package (`cmd/version/`), exporting `CmdStr` and `Cmd`.

Dependencies
------------

Add library dependencies with `go get` (not `go install`, which is for CLI tools), then run `go mod tidy`; the check fails on an untidy module.

```bash
cd src && go get github.com/some/library
```

Non-Interactive Shell Commands
------------------------------

**Always use non-interactive flags** with file operations to avoid hanging on confirmation prompts. Commands like `cp`, `mv` and `rm` may be aliased to `-i` on some systems, which hangs an agent waiting for y/n input.

```bash
cp -f source dest           # NOT: cp source dest
mv -f source dest           # NOT: mv source dest
rm -f file                  # NOT: rm file
cp -rf source dest          # NOT: cp -r source dest
```

Other commands that may prompt: `scp` and `ssh` (use `-o BatchMode=yes`), `apt-get` (use `-y`), `brew` (set `HOMEBREW_NO_AUTO_UPDATE=1`).
