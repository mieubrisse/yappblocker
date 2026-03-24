yappblocker: YAML-configured guardrails for your sleep
======================================================
yappblocker (Yet Another Appblocker) is a minimal app blocker that lets you define apps which will get killed during configurable time periods.

For example, I have a "soft shutdown" window from 8:45pm to 6:00am that blocks games, Battle.net, Steam, and Discord, and a "hard shutdown" window from 9:45 to 6:00 that also blocks Chrome, Whatsapp, Messages, etc.

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

Here is a realistic example:

```yaml
# yappblocker configuration
#
# This file controls which applications get killed and when.
# Location: ~/Library/Application Support/yappblocker/config.yaml
#
# Three sections: apps, appSets, and schedules.

# ============================================================================
# apps — define each application you want to block
# ============================================================================
# Each app needs:
#   match:    a string that matches the process name (used with pgrep -f)
#   killType: how to kill it (optional, defaults to "osascript")
#
# Kill types:
#   osascript     - sends AppleScript "quit app" (graceful, for native macOS apps)
#   pkillGraceful - sends SIGTERM via pkill (for Chrome PWAs, CLI apps)
#   pkillForce    - sends SIGKILL via pkill (for stubborn processes)
#
# To find the right match string for an app, run:
#   pgrep -f "AppName"
# while the app is open.
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

# ============================================================================
# appSets — group apps together for use in schedules
# ============================================================================
# Each set can contain:
#   apps:    a list of app names from the apps section above
#   appSets: a list of other app set names (for composition)
appSets:
  soft_shutdown:
    apps: [diablo3, battle-net, discord, crossover, minecraft, steam]
  hard_shutdown:
    appSets: [soft_shutdown]
    apps: [chrome, whatsapp, gmail, messages, safari, firefox]

# ============================================================================
# schedules — define when to block app sets
# ============================================================================
# Each schedule references an appSet and defines time windows.
# Windows support overnight ranges (e.g., 21:00 to 06:00).
# Days: mon, tue, wed, thu, fri, sat, sun
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

- `match` — a string matched against running processes via `pgrep -f`. To find the right value, run `pgrep -f "AppName"` while the app is open.
- `killType` — how to terminate the process (optional, defaults to `osascript`).

There are three kill types:

- **`osascript`** (default) — sends an AppleScript `quit app` command. This is a graceful quit that respects save dialogs. The `match` value doubles as the AppleScript application name, so it must match exactly what macOS calls the app (e.g., `"Google Chrome"`, not `"chrome"`). Use this for native macOS apps.
- **`pkillGraceful`** — sends `SIGTERM` via `pkill -f`. Use this for Chrome PWAs, Electron apps, or processes where the `pgrep -f` match string differs from the macOS application name.
- **`pkillForce`** — sends `SIGKILL` via `pkill -KILL -f`. Last resort for stubborn processes that ignore SIGTERM.

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

Run `yappblocker --help` for the full list of commands and flags.

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
