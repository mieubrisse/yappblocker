yappblocker Design
==================

A Go re-implementation of the Python app-blocker from mieubrisse/dotfiles. Kills
specified macOS applications during configured time windows. Designed as a standalone
tool publishable via Homebrew.

Goals
-----

- 1:1 feature port of the Python app-blocker
- YAML config with camelCase keys
- Cobra CLI with `run`, `install`, `uninstall` subcommands
- GoReleaser for builds, personal Homebrew tap for distribution
- README suitable for HN launch

Architecture
------------

Single stateless binary. launchd calls `yappblocker run` every 120 seconds. Each
invocation loads config, checks schedules against current time, kills matching
processes, and exits.

### CLI Subcommands

**`yappblocker run`**
- Loads config from `~/.config/yappblocker/config.yaml`
- If config missing, creates default with commented example, prints message, exits 0
- Checks all schedule windows against current time
- Collects union of apps from active schedules
- For each app: `pgrep -f <match>`, then dispatches kill
- Flags: `--dry-run`, `--verbose`

**`yappblocker install`**
- Writes plist to `~/Library/LaunchAgents/com.yappblocker.plist`
- Uses `exec.LookPath("yappblocker")` for the binary path (Homebrew symlink, survives upgrades)
- Creates log directory at `~/.local/state/yappblocker/` if needed
- Runs `launchctl load <path>`
- Idempotent: overwrites existing plist

**`yappblocker uninstall`**
- Runs `launchctl unload <path>`
- Removes plist file
- Idempotent: warns if plist doesn't exist, exits 0

### Config Format

Located at `~/.config/yappblocker/config.yaml`. camelCase keys.

```yaml
apps:
  discord:
    match: "Discord"
    killType: osascript
  gmail:
    match: "Gmail.app"
    killType: pkillGraceful

appSets:
  distractions:
    apps: [discord, gmail]
  everything:
    appSets: [distractions]
    apps: [chrome]

schedules:
  evening:
    appSet: distractions
    windows:
      - days: [mon, tue, wed, thu]
        start: "20:45"
        end: "06:00"
```

Kill types: `osascript` (AppleScript quit), `pkillGraceful` (SIGTERM), `pkillForce` (SIGKILL).

### Logging

launchd plist routes stdout and stderr to `~/.local/state/yappblocker/yappblocker.log`.
Documented in README.

### Package Layout

```
yappblocker/
├── cmd/
│   ├── root.go
│   ├── run.go
│   ├── install.go
│   └── uninstall.go
├── internal/
│   ├── config/
│   │   └── config.go
│   ├── schedule/
│   │   └── schedule.go
│   ├── killer/
│   │   └── killer.go
│   └── launchd/
│       └── launchd.go
├── main.go
├── .goreleaser.yaml
├── go.mod
├── LICENSE
└── README.md
```

- `internal/` keeps packages private (CLI tool, not library)
- config, schedule, killer map 1:1 to Python modules
- launchd package isolates macOS-specific plist/launchctl logic

Data Flow
---------

### `yappblocker run`

1. Resolve config path: `~/.config/yappblocker/config.yaml`
2. If missing: write default config with comments, print message, exit 0
3. Parse YAML into typed structs. Validate all references and detect cycles
4. For each schedule, check if any window is active (handles overnight windows)
5. Collect union of all apps from active schedules
6. For each app: `pgrep -f <match>`. If found, dispatch kill
7. Exit 0

### Schedule Window Logic

Same-day window (start < end): active if today is in days list and current time
is between start and end.

Overnight window (start >= end): active if today is in days list and time >= start,
OR if yesterday is in days list and time < end.

Error Handling
--------------

- Config parse/validation errors: stderr, exit 1
- Invalid killType, unknown app/appSet ref, circular ref: exit 1 with message
- `pgrep` no matches (exit 1): skip silently (normal)
- Kill command failures: log warning to stderr, continue to next app
- `install` when plist exists: overwrite (idempotent)
- `uninstall` when plist missing: warn, exit 0

Testing
-------

- **config**: table-driven tests for YAML parsing, appSet resolution (simple, nested,
  circular), validation. Ported from Python tests
- **schedule**: table-driven tests for window logic — same-day, overnight, day
  boundaries. Ported from Python tests
- **killer**: `CommandRunner` interface for mocking pgrep/osascript/pkill
- **integration**: top-level test wiring config + schedule + mocked killer
- stdlib `testing` only, no external test deps

Release & Distribution
----------------------

- GoReleaser: builds darwin/amd64 + darwin/arm64
- Homebrew tap: `mieubrisse/homebrew-yappblocker`
- GoReleaser auto-publishes formula to tap repo on tag push
