yappblocker: YAML-configured guardrails for your sleep
======================================================
yappblocker (Yet Another Appblocker) is a minimal app blocker that lets you define apps which will get repeatedly killed during configurable time periods.

For example, I have a "soft shutdown" window from 8:45pm to 6:00am that blocks games, Battle.net, Steam, and Discord, and a "hard shutdown" window from 9:45 to 6:00 that also blocks Chrome, Whatsapp, Messages, etc.

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

Further Tooling
---------------
If you liked this, you might like my other tools:

- [agenc](https://github.com/mieubrisse/agenc): the CEO command center for your fleet of Claudes
- [cmdk](https://github.com/mieubrisse/cmdk): access any file on your computer with ⌘-k in your terminal
- [safebrew](https://github.com/mieubrisse/safebrew): automated Homebrew backups to Github
- [fire-calculator](https://github.com/mieubrisse/fire-calculator): calculate when you'll achieve financial independence
