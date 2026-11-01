yappblocker: YAML-configured guardrails for your sleep
======================================================
yappblocker (Yet Another Appblocker) is a YAML-configured app blocker that lets you kill distracting apps on a schedule.

For example, I have a "soft shutdown" schedule from 8:45pm to 6:00am that blocks games, Battle.net, Steam, and Discord, and a "hard shutdown" schedule from 9:45 to 6:00 that also blocks Chrome, Whatsapp, Messages, etc.

Why?
----
I got frustrated with the existing tooling:

MacOS's "Downtime" makes it trivial to skip the block, and doesn't recognize certain programs (Diablo 3, Chrome-installed apps).

Opal and JOMO also couldn't kill Diablo 3, and charge money to have more than one schedule.

Quick start
-----------
1. Install:

```bash
brew install mieubrisse/yappblocker/yappblocker
```

2. Run the setup wizard:

```bash
yappblocker init
```

This creates a config file at `~/Library/Application Support/yappblocker/config.yaml` and registers a launchd agent that enforces your schedules every 2 minutes.

3. Edit the config to define your blocked apps and schedules:

```bash
vim ~/Library/Application\ Support/yappblocker/config.yaml
```

That's it. Any app matching an active schedule window will be killed automatically.

Configuration
-------------

The config file lives at `~/Library/Application Support/yappblocker/config.yaml` and has three sections: `apps`, `appSets`, and `schedules`.

Here's my config:

```yaml
apps:
  diablo3:
    match: "Diablo III"
    killType: osascript
  battle-net:
    match: "Battle.net"
    killType: osascript
  discord:
    match: "Discord"
    killType: osascript
  whatsapp:
    match: "WhatsApp"
    killType: osascript
  chrome:
    match: "Google Chrome"
    killType: osascript
  gmail:
    match: "Gmail.app"
    killType: pkillGraceful
  messages:
    match: "Messages"
    killType: osascript
  crossover:
    match: "CrossOver"
    killType: osascript
  minecraft:
    match: "Minecraft"
    killType: osascript
  safari:
    match: "Safari"
    killType: osascript
  firefox:
    match: "Firefox"
    killType: osascript
  steam:
    match: "Steam"
    killType: osascript

appSets:
  soft_shutdown:
    apps: [diablo3, battle-net, discord, crossover, minecraft, steam]
  hard_shutdown:
    appSets: [soft_shutdown]
    apps: [chrome, whatsapp, gmail, messages, safari, firefox]

schedules:
  soft_shutdown:
    appSet: soft_shutdown
    windows:
      - days: [mon, tue, wed, thu]
        start: "20:45"
        end: "06:00"
      - days: [fri, sat]
        start: "21:30"
        end: "06:00"
  hard_shutdown:
    appSet: hard_shutdown
    windows:
      - days: [mon, tue, wed, thu]
        start: "21:45"
        end: "06:00"
      - days: [fri, sat]
        start: "22:30"
        end: "06:00"
```

### Apps

Each app entry has two fields:

- `match` — used both to detect the app (via `pgrep -f`) and to kill it. To find the right value, run `pgrep -f "AppName"` while the app is open.
- `killType` — how to terminate the process (optional, defaults to `osascript`).

The `match` value serves double duty: yappblocker first uses it to check if the app is running, then passes it to the kill command. How it gets passed depends on `killType`:

- **`osascript`** (default) — runs `quit app "<match>"` via AppleScript. This is a graceful quit that respects save dialogs. The `match` value must be the app's display name as macOS knows it (the name in the top-left menu bar, next to the Apple icon). Start here for most apps.
- **`pkillGraceful`** — sends `SIGTERM` via `pkill -f "<match>"`. Try this if `osascript` doesn't work for a particular app.
- **`pkillForce`** — sends `SIGKILL` via `pkill -f "<match>"`. Last resort for stubborn processes that ignore SIGTERM.

### App sets

App sets group apps together and can compose other app sets:

- `apps` — list of app names defined in the `apps` section.
- `appSets` — list of other app set names, for nesting.

Circular references are detected and rejected at load time.

### Schedules

Each schedule references an `appSet` and defines one or more time windows:

- `days` — which days the window is active: `mon`, `tue`, `wed`, `thu`, `fri`, `sat`, `sun`.
- `start` / `end` — times in `HH:MM` format.

Overnight windows are supported. A window like `start: "22:00"` / `end: "07:00"` on `[mon]` means Monday 22:00 through Tuesday 07:00. A window where `start` equals `end` is active 24 hours on the listed days.

Commands
--------

Run `yappblocker -h` for the full list of commands and flags.

Pausing
-------

A pause suspends blocking for a bounded stretch of time, after which it resumes on its own. It exists so that a legitimate need during a blocking window has an answer other than uninstalling.

```bash
yappblocker pause 45m    # suspend blocking for 45 minutes
yappblocker pause        # show whether blocking is paused, and how much pausing is left
yappblocker resume       # end a pause early
```

The rules are deliberately tight, because a pause you can take freely is not a guard at all:

- **Three hours maximum**, and the three hours are *wall clock from your first pause*, not a budget of paused minutes. Taking several short pauses therefore loses you time against taking one long one — there is nothing to gain by splitting them up.
- **Pausing opens a 24-hour window.** For the first three hours of it you may pause again, up to whatever is left of those three hours. After that, pausing is unavailable until the 24 hours are up.
- **A confirmation phrase has to be typed** before a pause starts. `resume` needs no phrase — turning blocking back on is never the thing worth discouraging — and gives no allowance back.

While a pause is running, `yappblocker run` kills nothing and `yappblocker test` reports every schedule as inactive, so anything built on `test` (such as a shell hook) sees the pause too.

Pause state lives in `~/Library/Application Support/yappblocker/pause.json`. If that file is missing, damaged, or unreadable, blocking stays **on** and a warning goes to the log.

Upgrading
---------

```bash
brew upgrade yappblocker
```

The launchd agent points to the Homebrew symlink, so upgrades take effect automatically — no need to re-run `yappblocker install`.

Uninstall
---------

To fully remove yappblocker:

```bash
yappblocker uninstall
brew uninstall yappblocker
rm -rf ~/Library/Application\ Support/yappblocker
```

`yappblocker uninstall` is deliberately awkward, for the same reason the tool exists at all — the moment you want it gone is the moment it is doing its job:

- **It refuses to run while any schedule's window is open.** Inside a window the only move is `yappblocker pause`; the uninstall has to wait until blocking lifts.
- **Outside a window it requires a long confirmation phrase, typed by hand.** Pasting it does not work: input that arrives faster than anyone could type is rejected, as is a phrase piped or redirected in rather than typed at a terminal. Capitalization and spacing are ignored; every word is not.

None of this is security, and it is not meant to be — `launchctl unload`, `brew uninstall`, and deleting the plist all walk around it. It is there to make the impulsive path slow and deliberate.

Development
-----------

### Prerequisites

- [Go](https://go.dev/dl/) (version specified in `src/go.mod`)
- [GNU Make](https://www.gnu.org/software/make/)
- [golangci-lint](https://golangci-lint.run/welcome/install/) (`brew install golangci-lint`)
- [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck) (`go install golang.org/x/vuln/cmd/govulncheck@latest`)
- [deadcode](https://pkg.go.dev/golang.org/x/tools/cmd/deadcode) (`go install golang.org/x/tools/cmd/deadcode@latest`)

### Getting started

Clone the repository and run the full build pipeline:

```bash
git clone https://github.com/mieubrisse/yappblocker.git
cd yappblocker
make build
```

`make build` configures Git hooks, runs formatting and linting checks, runs the test suite, and compiles the binary. The output binary is at `_build/yappblocker`.

### Common tasks

| Command | What it does |
|---|---|
| `make build` | Full pipeline: configure hooks, check, compile |
| `make compile` | Build the binary only (skip checks) |
| `make check` | Run formatting, vet, lint, govulncheck, deadcode, and tests |
| `make test` | Run the test suite only |
| `make run` | Build and run the binary (pass args via `ARGS="..."`) |
| `make fmt` | Auto-format code |
| `make clean` | Remove build artifacts |

### Running the binary

Use `make run` to build and run in one step:

```bash
make run
make run ARGS="version"
make run ARGS="--help"
```

Or run the compiled binary directly:

```bash
_build/yappblocker
_build/yappblocker version
```

### Project layout

```
src/                    Go source code
  cmd/                  CLI commands (one directory per command)
    version/            "version" subcommand
  internal/             Internal packages
    buildinfo/          Build-time version injection
    config/             YAML config loading and validation
    killer/             App killing logic
    launchd/            launchd agent management
    schedule/           Schedule matching
  main.go              Entry point
_build/                 Compiled binary (gitignored)
.githooks/              Git hooks (activated by make setup)
```

Further Tooling
---------------
If you liked this, you might like my other tools:

- [agenc](https://github.com/mieubrisse/agenc): the CEO command center for your fleet of Claudes
- [cmdk](https://github.com/mieubrisse/cmdk): access any file on your computer with ⌘-k in your terminal
- [safebrew](https://github.com/mieubrisse/safebrew): automated Homebrew backups to Github
- [fire-calculator](https://github.com/mieubrisse/fire-calculator): calculate when you'll achieve financial independence
