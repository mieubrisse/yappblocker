package config

const DefaultConfigTemplate = `# yappblocker configuration
#
# This file controls which applications get killed and when.
# Location: ~/.config/yappblocker/config.yaml
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
  # discord:
  #   match: "Discord"
  #   killType: osascript
  # chrome:
  #   match: "Google Chrome"
  #   killType: osascript
  # gmail:
  #   match: "Gmail.app"
  #   killType: pkillGraceful

# ============================================================================
# appSets — group apps together for use in schedules
# ============================================================================
# Each set can contain:
#   apps:    a list of app names from the apps section above
#   appSets: a list of other app set names (for composition)
appSets:
  # distractions:
  #   apps: [discord, chrome, gmail]
  # everything:
  #   appSets: [distractions]
  #   apps: [safari]

# ============================================================================
# schedules — define when to block app sets
# ============================================================================
# Each schedule references an appSet and defines time windows.
# Windows support overnight ranges (e.g., 21:00 to 06:00).
# Days: mon, tue, wed, thu, fri, sat, sun
schedules:
  # bedtime:
  #   appSet: distractions
  #   windows:
  #     - days: [mon, tue, wed, thu]
  #       start: "21:00"
  #       end: "06:00"
  #     - days: [fri, sat]
  #       start: "22:00"
  #       end: "08:00"
`
