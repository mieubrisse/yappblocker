yappblocker
===========

A macOS CLI tool that automatically kills distracting applications on a schedule. Define which apps to block and when, and yappblocker enforces it — no willpower required.

Quick start
-----------

1. Install the binary:

```bash
brew install mieubrisse/yappblocker/yappblocker
```

2. Edit the config (auto-created on first run):

```bash
yappblocker run        # creates ~/.config/yappblocker/config.yaml
vim ~/.config/yappblocker/config.yaml
```

3. Enable automatic enforcement:

```bash
yappblocker install    # registers a launchd agent that runs every 2 minutes
```

That's it. Any app matching an active schedule window will be killed automatically.

Configuration
-------------

The config file lives at `~/.config/yappblocker/config.yaml` and has three sections: `apps`, `appSets`, and `schedules`.

Here is a realistic example:

```yaml
apps:
  discord:
    match: "Discord"
    killType: osascript
  chrome:
    match: "Google Chrome"
    killType: osascript
  gmail:
    match: "Gmail.app"
    killType: pkillGraceful
  steam:
    match: "Steam"
    killType: osascript
  minecraft:
    match: "minecraft"
    killType: pkillForce

appSets:
  social:
    apps: [discord, gmail]
  gaming:
    apps: [steam, minecraft]
  allDistractions:
    apps: [chrome]
    appSets: [social, gaming]

schedules:
  workday:
    appSet: allDistractions
    windows:
      - days: [mon, tue, wed, thu, fri]
        start: "09:00"
        end: "17:00"
  bedtime:
    appSet: social
    windows:
      - days: [sun, mon, tue, wed, thu]
        start: "22:00"
        end: "07:00"
      - days: [fri, sat]
        start: "23:00"
        end: "09:00"
```

### Apps

Each app entry defines a process to target:

- `match` — a string matched against running processes via `pgrep -f`. To find the right value, run `pgrep -f "AppName"` while the app is open.
- `killType` — how to terminate the process (optional, defaults to `osascript`).

**Note:** When using `osascript` kill type, the `match` value is also used as the application name in the AppleScript quit command. For native macOS apps, the process name and app name are usually the same (e.g., "Discord", "Google Chrome"). If they differ, use `pkillGraceful` or `pkillForce` instead.

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

Kill types
----------

| Kill type | Method | Use when |
|---|---|---|
| `osascript` | Sends an AppleScript "quit app" command | Native macOS apps (Discord, Chrome, Steam). Graceful quit that respects save dialogs. This is the default. |
| `pkillGraceful` | Sends `SIGTERM` via `pkill -f` | Chrome PWAs, Electron apps, or CLI processes that don't respond to AppleScript. |
| `pkillForce` | Sends `SIGKILL` via `pkill -KILL -f` | Stubborn processes that ignore SIGTERM. Use as a last resort. |

Commands
--------

### `yappblocker run`

Check all schedules against the current time and kill any blocked apps that are running.

Flags:

- `--dry-run` — print what would be killed without actually killing anything.
- `--verbose` — print detailed output about active schedules and matched processes.

### `yappblocker install`

Register a launchd agent that runs `yappblocker run` every 2 minutes. The agent starts immediately and persists across reboots.

### `yappblocker uninstall`

Remove the launchd agent, stopping automatic execution.

How it works
------------

`yappblocker install` creates a launchd plist at `~/Library/LaunchAgents/com.yappblocker.plist` that invokes `yappblocker run` every 120 seconds. Each invocation is stateless: it reads the config, checks which schedule windows are currently active, finds matching processes with `pgrep -f`, and kills them using the configured kill type. There is no daemon, no background process, and no state between runs.

File locations
--------------

| File | Path |
|---|---|
| Config | `~/.config/yappblocker/config.yaml` |
| Log | `~/.local/state/yappblocker/yappblocker.log` |
| Launchd plist | `~/Library/LaunchAgents/com.yappblocker.plist` |

The config file is auto-created with a commented-out example on first run.

Uninstall
---------

To fully remove yappblocker:

```bash
yappblocker uninstall
brew uninstall yappblocker
rm -rf ~/.config/yappblocker
rm -rf ~/.local/state/yappblocker
```

License
-------

MIT
