yappblocker Implementation Plan
================================

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Port the Python app-blocker to a standalone Go CLI tool that kills macOS apps on a schedule, distributed via Homebrew.

**Architecture:** Stateless Cobra CLI with three subcommands (`run`, `install`, `uninstall`). launchd calls `yappblocker run` every 120s. Config lives at `~/.config/yappblocker/config.yaml` (XDG). Logs at `~/.local/state/yappblocker/yappblocker.log`.

**Tech Stack:** Go 1.25, Cobra v1.10.2, gopkg.in/yaml.v3, github.com/mieubrisse/stacktrace, GoReleaser

---

Task 1: Project scaffolding
----------------------------

**Files:**
- Create: `go.mod`
- Create: `main.go`
- Create: `cmd/root.go`
- Create: `.gitignore`
- Create: `LICENSE`

**Step 1: Initialize Go module**

Run:
```bash
go mod init github.com/mieubrisse/yappblocker
```

**Step 2: Install Cobra dependency**

Run:
```bash
go get github.com/spf13/cobra@v1.10.2
```

**Step 3: Install other dependencies**

Run:
```bash
go get gopkg.in/yaml.v3@v3.0.1
go get github.com/mieubrisse/stacktrace@v0.1.0
```

**Step 4: Create main.go**

```go
package main

import (
	"os"

	"github.com/mieubrisse/yappblocker/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
```

**Step 5: Create cmd/root.go**

```go
package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "yappblocker",
	Short: "Kill distracting macOS apps on a schedule",
	Long:  "yappblocker automatically closes specified applications during configured time windows.\nUse 'yappblocker install' to set up automatic execution via launchd.",
}

func Execute() error {
	return rootCmd.Execute()
}
```

**Step 6: Create .gitignore**

```
yappblocker
dist/
```

**Step 7: Create LICENSE (MIT)**

Use the MIT license with copyright `2026 mieubrisse`.

**Step 8: Verify it builds**

Run:
```bash
go build -o yappblocker .
./yappblocker --help
```

Expected: help text showing `yappblocker` with description.

**Step 9: Commit**

```bash
git add go.mod go.sum main.go cmd/root.go .gitignore LICENSE
git commit -m "Scaffold Go project with Cobra CLI"
```

---

Task 2: Config package — types and parsing
-------------------------------------------

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

**Step 1: Write failing tests for config parsing**

Port the Python test cases. Use table-driven tests. Key test cases:

```go
package config

import (
	"testing"
)

func TestParseAppsSingleApp(t *testing.T) {
	yamlStr := `
apps:
  diablo3:
    match: "Diablo III"
    killType: pkillForce
`
	cfg, err := Load(yamlStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	app, ok := cfg.Apps["diablo3"]
	if !ok {
		t.Fatal("expected app 'diablo3' to exist")
	}
	if app.Match != "Diablo III" {
		t.Errorf("expected match 'Diablo III', got %q", app.Match)
	}
	if app.KillType != KillTypePkillForce {
		t.Errorf("expected killType pkillForce, got %q", app.KillType)
	}
}

func TestParseAppsMultiple(t *testing.T) {
	yamlStr := `
apps:
  diablo3:
    match: "Diablo III"
    killType: pkillForce
  chrome:
    match: "Google Chrome"
    killType: osascript
`
	cfg, err := Load(yamlStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Apps) != 2 {
		t.Errorf("expected 2 apps, got %d", len(cfg.Apps))
	}
	if cfg.Apps["chrome"].KillType != KillTypeOsascript {
		t.Errorf("expected osascript, got %q", cfg.Apps["chrome"].KillType)
	}
}

func TestParseAppInvalidKillType(t *testing.T) {
	yamlStr := `
apps:
  bad:
    match: "Bad"
    killType: nuke
`
	_, err := Load(yamlStr)
	if err == nil {
		t.Fatal("expected error for invalid killType")
	}
}

func TestParseAppMissingMatch(t *testing.T) {
	yamlStr := `
apps:
  bad:
    killType: pkillForce
`
	_, err := Load(yamlStr)
	if err == nil {
		t.Fatal("expected error for missing match")
	}
}

func TestParseAppDefaultKillType(t *testing.T) {
	yamlStr := `
apps:
  discord:
    match: "Discord"
`
	cfg, err := Load(yamlStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Apps["discord"].KillType != KillTypeOsascript {
		t.Errorf("expected default osascript, got %q", cfg.Apps["discord"].KillType)
	}
}
```

**Step 2: Run tests to verify they fail**

Run:
```bash
go test ./internal/config/ -v
```

Expected: compilation error (types don't exist yet).

**Step 3: Write config.go with types and Load function**

```go
package config

import (
	"fmt"

	"github.com/mieubrisse/stacktrace"
	"gopkg.in/yaml.v3"
)

type KillType string

const (
	KillTypeOsascript    KillType = "osascript"
	KillTypePkillGraceful KillType = "pkillGraceful"
	KillTypePkillForce   KillType = "pkillForce"
)

var validKillTypes = map[KillType]bool{
	KillTypeOsascript:     true,
	KillTypePkillGraceful: true,
	KillTypePkillForce:    true,
}

type App struct {
	Name     string
	Match    string   `yaml:"match"`
	KillType KillType `yaml:"killType"`
}

type AppSetDef struct {
	Apps    []string `yaml:"apps"`
	AppSets []string `yaml:"appSets"`
}

type WindowDef struct {
	Days  []string `yaml:"days"`
	Start string   `yaml:"start"`
	End   string   `yaml:"end"`
}

type ScheduleDef struct {
	AppSet  string      `yaml:"appSet"`
	Windows []WindowDef `yaml:"windows"`
}

type Config struct {
	Apps      map[string]*App         `yaml:"apps"`
	AppSets   map[string]*AppSetDef   `yaml:"appSets"`
	Schedules map[string]*ScheduleDef `yaml:"schedules"`
}

func Load(yamlStr string) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal([]byte(yamlStr), &cfg); err != nil {
		return nil, stacktrace.Propagate(err, "failed to parse config YAML")
	}

	// Set app names from map keys and validate
	for name, app := range cfg.Apps {
		app.Name = name
		if app.Match == "" {
			return nil, stacktrace.NewError("app %q missing required field 'match'", name)
		}
		if app.KillType == "" {
			app.KillType = KillTypeOsascript
		}
		if !validKillTypes[app.KillType] {
			return nil, stacktrace.NewError("app %q has invalid killType %q", name, app.KillType)
		}
	}

	// Validate app set references
	for setName := range cfg.AppSets {
		if _, err := cfg.ResolveAppSet(setName); err != nil {
			return nil, err
		}
	}

	// Validate schedule references
	for schedName, sched := range cfg.Schedules {
		if _, ok := cfg.AppSets[sched.AppSet]; !ok {
			return nil, stacktrace.NewError("schedule %q references unknown appSet %q", schedName, sched.AppSet)
		}
	}

	return &cfg, nil
}
```

**Step 4: Run tests to verify they pass**

Run:
```bash
go test ./internal/config/ -v
```

Expected: all 5 tests pass.

**Step 5: Commit**

```bash
git add internal/config/
git commit -m "Add config package with YAML parsing and validation"
```

---

Task 3: Config package — app set resolution
--------------------------------------------

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Step 1: Write failing tests for app set resolution**

Add to `config_test.go`:

```go
func TestResolveAppSetAppsOnly(t *testing.T) {
	yamlStr := `
apps:
  diablo3:
    match: "Diablo III"
    killType: pkillForce
  steam:
    match: "Steam"
appSets:
  games:
    apps: [diablo3, steam]
schedules: {}
`
	cfg, err := Load(yamlStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	apps, err := cfg.ResolveAppSet("games")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(apps) != 2 {
		t.Errorf("expected 2 apps, got %d", len(apps))
	}
}

func TestResolveAppSetNested(t *testing.T) {
	yamlStr := `
apps:
  diablo3:
    match: "Diablo III"
  steam:
    match: "Steam"
  discord:
    match: "Discord"
appSets:
  games:
    apps: [diablo3, steam]
  social:
    apps: [discord]
  everything:
    appSets: [games, social]
schedules: {}
`
	cfg, err := Load(yamlStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	apps, err := cfg.ResolveAppSet("everything")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(apps) != 3 {
		t.Errorf("expected 3 apps, got %d", len(apps))
	}
}

func TestResolveAppSetBothAppsAndSets(t *testing.T) {
	yamlStr := `
apps:
  discord:
    match: "Discord"
  chrome:
    match: "Google Chrome"
appSets:
  soft:
    apps: [discord]
  hard:
    appSets: [soft]
    apps: [chrome]
schedules: {}
`
	cfg, err := Load(yamlStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	apps, err := cfg.ResolveAppSet("hard")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(apps) != 2 {
		t.Errorf("expected 2 apps, got %d", len(apps))
	}
}

func TestResolveAppSetCircularReference(t *testing.T) {
	yamlStr := `
apps: {}
appSets:
  a:
    appSets: [b]
  b:
    appSets: [a]
schedules: {}
`
	_, err := Load(yamlStr)
	if err == nil {
		t.Fatal("expected error for circular reference")
	}
}

func TestResolveAppSetUnknownApp(t *testing.T) {
	yamlStr := `
apps: {}
appSets:
  bad:
    apps: [nonexistent]
schedules: {}
`
	_, err := Load(yamlStr)
	if err == nil {
		t.Fatal("expected error for unknown app reference")
	}
}

func TestResolveAppSetUnknownSet(t *testing.T) {
	yamlStr := `
apps: {}
appSets:
  bad:
    appSets: [nonexistent]
schedules: {}
`
	_, err := Load(yamlStr)
	if err == nil {
		t.Fatal("expected error for unknown app set reference")
	}
}
```

**Step 2: Run tests to verify failures**

Run:
```bash
go test ./internal/config/ -v
```

Expected: `ResolveAppSet` method not found.

**Step 3: Implement ResolveAppSet on Config**

Add to `config.go`:

```go
func (c *Config) ResolveAppSet(setName string) ([]*App, error) {
	return c.resolveAppSetInner(setName, make(map[string]bool))
}

func (c *Config) resolveAppSetInner(setName string, visited map[string]bool) ([]*App, error) {
	if visited[setName] {
		return nil, stacktrace.NewError("circular reference detected: %q already visited", setName)
	}
	visited[setName] = true
	defer delete(visited, setName)

	setDef, ok := c.AppSets[setName]
	if !ok {
		return nil, stacktrace.NewError("unknown appSet %q", setName)
	}

	seen := make(map[string]bool)
	var result []*App

	for _, appName := range setDef.Apps {
		app, ok := c.Apps[appName]
		if !ok {
			return nil, stacktrace.NewError("appSet %q references unknown app %q", setName, appName)
		}
		if !seen[appName] {
			seen[appName] = true
			result = append(result, app)
		}
	}

	for _, nestedSetName := range setDef.AppSets {
		nested, err := c.resolveAppSetInner(nestedSetName, visited)
		if err != nil {
			return nil, err
		}
		for _, app := range nested {
			if !seen[app.Name] {
				seen[app.Name] = true
				result = append(result, app)
			}
		}
	}

	return result, nil
}
```

**Step 4: Run tests**

Run:
```bash
go test ./internal/config/ -v
```

Expected: all tests pass.

**Step 5: Commit**

```bash
git add internal/config/
git commit -m "Add app set resolution with cycle detection"
```

---

Task 4: Config package — full config load test
-----------------------------------------------

**Files:**
- Modify: `internal/config/config_test.go`

**Step 1: Write full config load test**

```go
func TestLoadFullConfig(t *testing.T) {
	yamlStr := `
apps:
  diablo3:
    match: "Diablo III"
    killType: pkillForce
  discord:
    match: "Discord"
appSets:
  games:
    apps: [diablo3]
  soft:
    appSets: [games]
    apps: [discord]
schedules:
  bedtime:
    appSet: soft
    windows:
      - days: [mon, tue, wed, thu]
        start: "20:45"
        end: "06:00"
`
	cfg, err := Load(yamlStr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Apps) != 2 {
		t.Errorf("expected 2 apps, got %d", len(cfg.Apps))
	}
	if len(cfg.AppSets) != 2 {
		t.Errorf("expected 2 app sets, got %d", len(cfg.AppSets))
	}
	if len(cfg.Schedules) != 1 {
		t.Errorf("expected 1 schedule, got %d", len(cfg.Schedules))
	}
	sched := cfg.Schedules["bedtime"]
	if sched.AppSet != "soft" {
		t.Errorf("expected appSet 'soft', got %q", sched.AppSet)
	}
	if len(sched.Windows) != 1 {
		t.Errorf("expected 1 window, got %d", len(sched.Windows))
	}
}

func TestLoadConfigInvalidScheduleRef(t *testing.T) {
	yamlStr := `
apps: {}
appSets: {}
schedules:
  bad:
    appSet: nonexistent
    windows: []
`
	_, err := Load(yamlStr)
	if err == nil {
		t.Fatal("expected error for unknown appSet in schedule")
	}
}
```

**Step 2: Run tests**

Run:
```bash
go test ./internal/config/ -v
```

Expected: pass.

**Step 3: Commit**

```bash
git add internal/config/
git commit -m "Add full config loading tests"
```

---

Task 5: Schedule package
------------------------

**Files:**
- Create: `internal/schedule/schedule.go`
- Create: `internal/schedule/schedule_test.go`

**Step 1: Write failing tests**

Port all Python schedule tests. Use table-driven format:

```go
package schedule

import (
	"testing"
	"time"

	"github.com/mieubrisse/yappblocker/internal/config"
)

func makeTime(year, month, day, hour, minute int) time.Time {
	return time.Date(year, time.Month(month), day, hour, minute, 0, 0, time.Local)
}

func TestIsWindowActive(t *testing.T) {
	tests := []struct {
		name   string
		window config.WindowDef
		now    time.Time
		want   bool
	}{
		{
			name:   "same-day window active within range",
			window: config.WindowDef{Days: []string{"mon", "tue", "wed", "thu"}, Start: "20:45", End: "23:00"},
			now:    makeTime(2026, 3, 25, 21, 0), // Wednesday
			want:   true,
		},
		{
			name:   "same-day window inactive before start",
			window: config.WindowDef{Days: []string{"mon", "tue", "wed", "thu"}, Start: "20:45", End: "23:00"},
			now:    makeTime(2026, 3, 25, 20, 30), // Wednesday
			want:   false,
		},
		{
			name:   "same-day window inactive wrong day",
			window: config.WindowDef{Days: []string{"mon", "tue", "wed", "thu"}, Start: "20:45", End: "23:00"},
			now:    makeTime(2026, 3, 28, 21, 0), // Saturday
			want:   false,
		},
		{
			name:   "overnight window active before midnight",
			window: config.WindowDef{Days: []string{"mon", "tue", "wed", "thu"}, Start: "20:45", End: "06:00"},
			now:    makeTime(2026, 3, 25, 23, 0), // Wednesday night
			want:   true,
		},
		{
			name:   "overnight window active after midnight",
			window: config.WindowDef{Days: []string{"wed"}, Start: "20:45", End: "06:00"},
			now:    makeTime(2026, 3, 26, 2, 0), // Thursday 2am, window started Wed
			want:   true,
		},
		{
			name:   "overnight window inactive after end",
			window: config.WindowDef{Days: []string{"wed"}, Start: "20:45", End: "06:00"},
			now:    makeTime(2026, 3, 26, 7, 0), // Thursday 7am
			want:   false,
		},
		{
			name:   "overnight window inactive wrong start day",
			window: config.WindowDef{Days: []string{"mon"}, Start: "20:45", End: "06:00"},
			now:    makeTime(2026, 3, 26, 2, 0), // Thursday 2am, but window starts Mon only
			want:   false,
		},
		{
			name:   "sunday keyword works",
			window: config.WindowDef{Days: []string{"sun"}, Start: "10:00", End: "22:00"},
			now:    makeTime(2026, 3, 29, 15, 0), // Sunday
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsWindowActive(tt.window, tt.now)
			if got != tt.want {
				t.Errorf("IsWindowActive() = %v, want %v", got, tt.want)
			}
		})
	}
}
```

**Step 2: Run tests to verify failure**

Run:
```bash
go test ./internal/schedule/ -v
```

Expected: compilation error.

**Step 3: Implement schedule.go**

```go
package schedule

import (
	"strings"
	"strconv"
	"time"

	"github.com/mieubrisse/yappblocker/internal/config"
)

var dayNames = []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}

func dayName(t time.Time) string {
	// time.Weekday: Sunday=0, Monday=1 ... Saturday=6
	// We want: mon=0, tue=1 ... sun=6
	wd := int(t.Weekday())
	idx := (wd + 6) % 7
	return dayNames[idx]
}

func prevDayName(t time.Time) string {
	wd := int(t.Weekday())
	idx := (wd + 5) % 7
	return dayNames[idx]
}

func parseTime(timeStr string) (hour, minute int) {
	parts := strings.Split(timeStr, ":")
	hour, _ = strconv.Atoi(parts[0])
	minute, _ = strconv.Atoi(parts[1])
	return
}

func timeOfDay(t time.Time) (hour, minute int) {
	return t.Hour(), t.Minute()
}

func timeBefore(h1, m1, h2, m2 int) bool {
	return h1 < h2 || (h1 == h2 && m1 < m2)
}

func timeBeforeOrEqual(h1, m1, h2, m2 int) bool {
	return h1 < h2 || (h1 == h2 && m1 <= m2)
}

func containsDay(days []string, day string) bool {
	for _, d := range days {
		if strings.ToLower(d) == day {
			return true
		}
	}
	return false
}

func IsWindowActive(window config.WindowDef, now time.Time) bool {
	startH, startM := parseTime(window.Start)
	endH, endM := parseTime(window.End)
	curH, curM := timeOfDay(now)
	today := dayName(now)
	yesterday := prevDayName(now)

	if timeBefore(startH, startM, endH, endM) {
		// Same-day window
		return containsDay(window.Days, today) &&
			timeBeforeOrEqual(startH, startM, curH, curM) &&
			timeBefore(curH, curM, endH, endM)
	}

	// Overnight window (start >= end)
	if containsDay(window.Days, today) && timeBeforeOrEqual(startH, startM, curH, curM) {
		return true
	}
	if containsDay(window.Days, yesterday) && timeBefore(curH, curM, endH, endM) {
		return true
	}
	return false
}
```

**Step 4: Run tests**

Run:
```bash
go test ./internal/schedule/ -v
```

Expected: all 8 tests pass.

**Step 5: Commit**

```bash
git add internal/schedule/
git commit -m "Add schedule package with window activation logic"
```

---

Task 6: Killer package
----------------------

**Files:**
- Create: `internal/killer/killer.go`
- Create: `internal/killer/killer_test.go`

**Step 1: Write failing tests with CommandRunner interface**

```go
package killer

import (
	"testing"

	"github.com/mieubrisse/yappblocker/internal/config"
)

type mockRunner struct {
	commands [][]string
	pgrepResults map[string]string // match -> stdout
}

func newMockRunner() *mockRunner {
	return &mockRunner{
		pgrepResults: make(map[string]string),
	}
}

func (m *mockRunner) Run(args []string) (string, error) {
	m.commands = append(m.commands, args)
	// If this is a pgrep call, return configured result
	if len(args) >= 3 && args[0] == "pgrep" {
		match := args[2]
		if stdout, ok := m.pgrepResults[match]; ok {
			return stdout, nil
		}
		return "", &ExitError{Code: 1}
	}
	return "", nil
}

func TestKillForceApp(t *testing.T) {
	runner := newMockRunner()
	runner.pgrepResults["Diablo III"] = "123\n456\n"
	app := &config.App{Name: "diablo3", Match: "Diablo III", KillType: config.KillTypePkillForce}
	count := FindAndKillApps([]*config.App{app}, false, false, runner)
	if count != 2 {
		t.Errorf("expected 2 matched, got %d", count)
	}
	// Verify pkill -KILL -f was called
	found := false
	for _, cmd := range runner.commands {
		if len(cmd) == 4 && cmd[0] == "pkill" && cmd[1] == "-KILL" && cmd[3] == "Diablo III" {
			found = true
		}
	}
	if !found {
		t.Error("expected pkill -KILL -f call")
	}
}

func TestKillOsascript(t *testing.T) {
	runner := newMockRunner()
	runner.pgrepResults["Discord"] = "789\n"
	app := &config.App{Name: "discord", Match: "Discord", KillType: config.KillTypeOsascript}
	count := FindAndKillApps([]*config.App{app}, false, false, runner)
	if count != 1 {
		t.Errorf("expected 1 matched, got %d", count)
	}
	found := false
	for _, cmd := range runner.commands {
		if len(cmd) == 3 && cmd[0] == "osascript" {
			found = true
		}
	}
	if !found {
		t.Error("expected osascript call")
	}
}

func TestKillPkillGraceful(t *testing.T) {
	runner := newMockRunner()
	runner.pgrepResults["Gmail.app"] = "321\n"
	app := &config.App{Name: "gmail", Match: "Gmail.app", KillType: config.KillTypePkillGraceful}
	count := FindAndKillApps([]*config.App{app}, false, false, runner)
	if count != 1 {
		t.Errorf("expected 1 matched, got %d", count)
	}
	found := false
	for _, cmd := range runner.commands {
		if len(cmd) == 3 && cmd[0] == "pkill" && cmd[1] == "-f" {
			found = true
		}
	}
	if !found {
		t.Error("expected pkill -f call")
	}
}

func TestDryRunDoesNotKill(t *testing.T) {
	runner := newMockRunner()
	runner.pgrepResults["Diablo III"] = "123\n"
	app := &config.App{Name: "diablo3", Match: "Diablo III", KillType: config.KillTypePkillForce}
	count := FindAndKillApps([]*config.App{app}, true, false, runner)
	if count != 1 {
		t.Errorf("expected 1 matched, got %d", count)
	}
	// Should only have pgrep call, no pkill
	for _, cmd := range runner.commands {
		if cmd[0] == "pkill" || cmd[0] == "osascript" {
			t.Errorf("unexpected kill command in dry-run: %v", cmd)
		}
	}
}

func TestNoMatchingProcesses(t *testing.T) {
	runner := newMockRunner()
	// No pgrepResults configured = pgrep returns exit 1
	app := &config.App{Name: "diablo3", Match: "Diablo III", KillType: config.KillTypePkillForce}
	count := FindAndKillApps([]*config.App{app}, false, false, runner)
	if count != 0 {
		t.Errorf("expected 0 matched, got %d", count)
	}
}
```

**Step 2: Run tests to verify failure**

Run:
```bash
go test ./internal/killer/ -v
```

Expected: compilation error.

**Step 3: Implement killer.go**

```go
package killer

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mieubrisse/yappblocker/internal/config"
)

type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("exit code %d", e.Code)
}

type CommandRunner interface {
	Run(args []string) (string, error)
}

type RealRunner struct{}

func (r *RealRunner) Run(args []string) (string, error) {
	cmd := exec.Command(args[0], args[1:]...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return string(out), &ExitError{Code: exitErr.ExitCode()}
		}
		return string(out), err
	}
	return string(out), nil
}

func FindAndKillApps(apps []*config.App, dryRun bool, verbose bool, runner CommandRunner) int {
	totalMatched := 0
	for _, app := range apps {
		stdout, err := runner.Run([]string{"pgrep", "-f", app.Match})
		if err != nil {
			continue
		}

		pids := parsePIDs(stdout)
		totalMatched += len(pids)

		if dryRun {
			fmt.Fprintf(os.Stderr, "[dry-run] Would kill %q (%d process(es))\n", app.Name, len(pids))
			continue
		}

		killApp(app, runner, verbose)
	}
	return totalMatched
}

func killApp(app *config.App, runner CommandRunner, verbose bool) {
	var args []string
	switch app.KillType {
	case config.KillTypeOsascript:
		args = []string{"osascript", "-e", fmt.Sprintf(`quit app "%s"`, app.Match)}
	case config.KillTypePkillGraceful:
		args = []string{"pkill", "-f", app.Match}
	case config.KillTypePkillForce:
		args = []string{"pkill", "-KILL", "-f", app.Match}
	}

	_, err := runner.Run(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to kill %q: %v\n", app.Name, err)
	}
	if verbose {
		fmt.Fprintf(os.Stderr, "Killed %q via %s\n", app.Name, app.KillType)
	}
}

func parsePIDs(stdout string) []string {
	var pids []string
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			pids = append(pids, line)
		}
	}
	return pids
}
```

**Step 4: Run tests**

Run:
```bash
go test ./internal/killer/ -v
```

Expected: all 5 tests pass.

**Step 5: Commit**

```bash
git add internal/killer/
git commit -m "Add killer package with process detection and kill dispatch"
```

---

Task 7: Default config template
--------------------------------

**Files:**
- Create: `internal/config/default.go`

**Step 1: Create default config template**

```go
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
```

**Step 2: Commit**

```bash
git add internal/config/default.go
git commit -m "Add default config template with documentation"
```

---

Task 8: Run command
-------------------

**Files:**
- Create: `cmd/run.go`

**Step 1: Implement the run command**

```go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mieubrisse/stacktrace"
	"github.com/mieubrisse/yappblocker/internal/config"
	"github.com/mieubrisse/yappblocker/internal/killer"
	"github.com/mieubrisse/yappblocker/internal/schedule"
	"github.com/spf13/cobra"
)

var (
	dryRun  bool
	verbose bool
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Check schedules and kill blocked apps",
	Long:  "Checks all configured schedules against the current time and kills any apps that should be blocked right now.",
	RunE:  executeRun,
}

func init() {
	runCmd.Flags().BoolVar(&dryRun, "dry-run", false, "print what would be killed without killing")
	runCmd.Flags().BoolVar(&verbose, "verbose", false, "print detailed output")
	rootCmd.AddCommand(runCmd)
}

func getConfigFilePath() string {
	configDirPath, err := os.UserConfigDir()
	if err != nil {
		configDirPath = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(configDirPath, "yappblocker", "config.yaml")
}

func ensureConfigExists(configFilePath string) error {
	if _, err := os.Stat(configFilePath); err == nil {
		return nil
	}

	dirPath := filepath.Dir(configFilePath)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return stacktrace.Propagate(err, "failed to create config directory %q", dirPath)
	}

	if err := os.WriteFile(configFilePath, []byte(config.DefaultConfigTemplate), 0644); err != nil {
		return stacktrace.Propagate(err, "failed to write default config")
	}

	fmt.Fprintf(os.Stderr, "Created default config at %s\nEdit it to configure your blocked apps and schedules.\n", configFilePath)
	return nil
}

func executeRun(cmd *cobra.Command, args []string) error {
	configFilePath := getConfigFilePath()

	if err := ensureConfigExists(configFilePath); err != nil {
		return err
	}

	data, err := os.ReadFile(configFilePath)
	if err != nil {
		return stacktrace.Propagate(err, "failed to read config file %q", configFilePath)
	}

	cfg, err := config.Load(string(data))
	if err != nil {
		return stacktrace.Propagate(err, "failed to load config")
	}

	now := time.Now()
	runner := &killer.RealRunner{}

	var activeApps []*config.App
	seenApps := make(map[string]bool)

	for schedName, sched := range cfg.Schedules {
		for _, window := range sched.Windows {
			if schedule.IsWindowActive(window, now) {
				resolved, err := cfg.ResolveAppSet(sched.AppSet)
				if err != nil {
					return err
				}
				if verbose {
					names := make([]string, 0, len(resolved))
					for _, a := range resolved {
						names = append(names, a.Name)
					}
					fmt.Fprintf(os.Stderr, "Schedule %q active — apps: %v\n", schedName, names)
				}
				for _, app := range resolved {
					if !seenApps[app.Name] {
						seenApps[app.Name] = true
						activeApps = append(activeApps, app)
					}
				}
				break // Only need one active window per schedule
			}
		}
	}

	if len(activeApps) == 0 {
		if verbose {
			fmt.Fprintln(os.Stderr, "No active schedules")
		}
		return nil
	}

	killed := killer.FindAndKillApps(activeApps, dryRun, verbose, runner)
	if verbose || dryRun {
		action := "Killed"
		if dryRun {
			action = "Would kill"
		}
		fmt.Fprintf(os.Stderr, "%s %d process(es)\n", action, killed)
	}

	return nil
}
```

**Step 2: Verify it builds**

Run:
```bash
go build -o yappblocker .
./yappblocker run --help
```

Expected: help text for run command showing --dry-run and --verbose flags.

**Step 3: Commit**

```bash
git add cmd/run.go
git commit -m "Add run command with config auto-creation"
```

---

Task 9: launchd package
-----------------------

**Files:**
- Create: `internal/launchd/launchd.go`

**Step 1: Implement plist generation and launchctl management**

```go
package launchd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"

	"github.com/mieubrisse/stacktrace"
)

const (
	plistLabel    = "com.yappblocker"
	plistFileName = plistLabel + ".plist"
	runInterval   = 120
)

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>{{.Label}}</string>
    <key>ProgramArguments</key>
    <array>
        <string>{{.BinaryFilePath}}</string>
        <string>run</string>
    </array>
    <key>StartInterval</key>
    <integer>{{.RunInterval}}</integer>
    <key>StandardOutPath</key>
    <string>{{.LogFilePath}}</string>
    <key>StandardErrorPath</key>
    <string>{{.LogFilePath}}</string>
    <key>RunAtLoad</key>
    <true/>
</dict>
</plist>
`

type plistData struct {
	Label          string
	BinaryFilePath string
	RunInterval    int
	LogFilePath    string
}

func getPlistFilePath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, "Library", "LaunchAgents", plistFileName)
}

func getLogFilePath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".local", "state", "yappblocker", "yappblocker.log")
}

func Install() error {
	binaryFilePath, err := exec.LookPath("yappblocker")
	if err != nil {
		return stacktrace.Propagate(err, "could not find yappblocker in PATH")
	}

	logFilePath := getLogFilePath()
	logDirPath := filepath.Dir(logFilePath)
	if err := os.MkdirAll(logDirPath, 0755); err != nil {
		return stacktrace.Propagate(err, "failed to create log directory %q", logDirPath)
	}

	data := plistData{
		Label:          plistLabel,
		BinaryFilePath: binaryFilePath,
		RunInterval:    runInterval,
		LogFilePath:    logFilePath,
	}

	tmpl, err := template.New("plist").Parse(plistTemplate)
	if err != nil {
		return stacktrace.Propagate(err, "failed to parse plist template")
	}

	plistFilePath := getPlistFilePath()
	plistDirPath := filepath.Dir(plistFilePath)
	if err := os.MkdirAll(plistDirPath, 0755); err != nil {
		return stacktrace.Propagate(err, "failed to create LaunchAgents directory")
	}

	// Unload existing plist if present (ignore errors)
	if _, err := os.Stat(plistFilePath); err == nil {
		exec.Command("launchctl", "unload", plistFilePath).Run()
	}

	f, err := os.Create(plistFilePath)
	if err != nil {
		return stacktrace.Propagate(err, "failed to create plist file %q", plistFilePath)
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return stacktrace.Propagate(err, "failed to write plist")
	}

	cmd := exec.Command("launchctl", "load", plistFilePath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return stacktrace.Propagate(err, "launchctl load failed: %s", string(out))
	}

	fmt.Fprintf(os.Stderr, "Installed launchd agent: %s\n", plistFilePath)
	fmt.Fprintf(os.Stderr, "yappblocker will run every %d seconds.\n", runInterval)
	fmt.Fprintf(os.Stderr, "Logs: %s\n", logFilePath)
	return nil
}

func Uninstall() error {
	plistFilePath := getPlistFilePath()

	if _, err := os.Stat(plistFilePath); os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "No launchd agent found — nothing to uninstall.")
		return nil
	}

	cmd := exec.Command("launchctl", "unload", plistFilePath)
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: launchctl unload failed: %s\n", string(out))
	}

	if err := os.Remove(plistFilePath); err != nil {
		return stacktrace.Propagate(err, "failed to remove plist file %q", plistFilePath)
	}

	fmt.Fprintf(os.Stderr, "Uninstalled launchd agent: %s\n", plistFilePath)
	return nil
}
```

**Step 2: Commit**

```bash
git add internal/launchd/
git commit -m "Add launchd package for plist management"
```

---

Task 10: Install and uninstall commands
---------------------------------------

**Files:**
- Create: `cmd/install.go`
- Create: `cmd/uninstall.go`

**Step 1: Create install command**

```go
package cmd

import (
	"github.com/mieubrisse/yappblocker/internal/launchd"
	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install launchd agent to run yappblocker automatically",
	Long:  "Creates a launchd plist and loads it so yappblocker runs every 2 minutes.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return launchd.Install()
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
}
```

**Step 2: Create uninstall command**

```go
package cmd

import (
	"github.com/mieubrisse/yappblocker/internal/launchd"
	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove launchd agent",
	Long:  "Unloads and removes the launchd plist, stopping automatic execution.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return launchd.Uninstall()
	},
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
}
```

**Step 3: Verify build and help**

Run:
```bash
go build -o yappblocker .
./yappblocker --help
./yappblocker install --help
./yappblocker uninstall --help
```

Expected: all three subcommands visible in help.

**Step 4: Commit**

```bash
git add cmd/install.go cmd/uninstall.go
git commit -m "Add install and uninstall commands"
```

---

Task 11: Run all tests end-to-end
----------------------------------

**Step 1: Run entire test suite**

Run:
```bash
go test ./... -v
```

Expected: all tests pass across config, schedule, and killer packages.

**Step 2: Build and smoke test**

Run:
```bash
go build -o yappblocker .
./yappblocker run --dry-run --verbose
```

Expected: creates default config if needed, prints "No active schedules" or similar.

**Step 3: Commit any fixes**

---

Task 12: GoReleaser configuration
----------------------------------

**Files:**
- Create: `.goreleaser.yaml`

**Step 1: Create GoReleaser config**

```yaml
version: 2

project_name: yappblocker

before:
  hooks:
    - go mod tidy
    - go generate ./...

builds:
  - env:
      - CGO_ENABLED=0
    goos:
      - darwin
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w

archives:
  - format: tar.gz

brews:
  - repository:
      owner: mieubrisse
      name: homebrew-yappblocker
    homepage: "https://github.com/mieubrisse/yappblocker"
    description: "Kill distracting macOS apps on a schedule"
    license: "MIT"
    install: |
      bin.install "yappblocker"
    test: |
      system "#{bin}/yappblocker", "--help"

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
```

**Step 2: Verify config**

Run (requires goreleaser installed):
```bash
brew install goreleaser
goreleaser check
```

Expected: config is valid.

**Step 3: Commit**

```bash
git add .goreleaser.yaml
git commit -m "Add GoReleaser config with Homebrew tap"
```

---

Task 13: README
---------------

**Files:**
- Create: `README.md`

**Step 1: Write README**

The README should include:
- Project description (what it does, why)
- Installation via Homebrew
- Quick start (install, edit config, enable)
- Config reference (apps, appSets, schedules, kill types, overnight windows)
- Example config
- Log location (`~/.local/state/yappblocker/yappblocker.log`)
- Commands reference (run, install, uninstall)
- How to verify it's working
- Uninstall instructions
- License

Target audience: HN readers who want to try it.

**Step 2: Commit**

```bash
git add README.md
git commit -m "Add README"
```

---

Task 14: Create GitHub repo and push
-------------------------------------

**Step 1: Create GitHub repo**

Run:
```bash
gh repo create mieubrisse/yappblocker --public --source=. --push
```

**Step 2: Also create the tap repo**

Run:
```bash
gh repo create mieubrisse/homebrew-yappblocker --public --description "Homebrew tap for yappblocker"
```

**Step 3: Set up GitHub Actions for GoReleaser**

Create `.github/workflows/release.yml`:

```yaml
name: release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write

jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version: stable
      - uses: goreleaser/goreleaser-action@v6
        with:
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          HOMEBREW_TAP_GITHUB_TOKEN: ${{ secrets.HOMEBREW_TAP_GITHUB_TOKEN }}
```

Note: The `HOMEBREW_TAP_GITHUB_TOKEN` secret needs to be a personal access token with `repo` scope for the tap repo. This must be configured manually in GitHub repo settings.

**Step 4: Commit and push**

```bash
git add .github/
git commit -m "Add GitHub Actions release workflow"
git push
```

---

Task 15: Create beads issues for tracking
------------------------------------------

Create beads issues for each task above so progress can be tracked in the shared Dolt DB.

Run:
```bash
bd create -t task -s "Scaffold Go project with Cobra CLI" -d "Task 1 from implementation plan"
bd create -t task -s "Config package: types and parsing" -d "Task 2"
bd create -t task -s "Config package: app set resolution" -d "Task 3"
bd create -t task -s "Config package: full config load test" -d "Task 4"
bd create -t task -s "Schedule package" -d "Task 5"
bd create -t task -s "Killer package" -d "Task 6"
bd create -t task -s "Default config template" -d "Task 7"
bd create -t task -s "Run command" -d "Task 8"
bd create -t task -s "launchd package" -d "Task 9"
bd create -t task -s "Install and uninstall commands" -d "Task 10"
bd create -t task -s "Run all tests end-to-end" -d "Task 11"
bd create -t task -s "GoReleaser configuration" -d "Task 12"
bd create -t task -s "README" -d "Task 13"
bd create -t task -s "Create GitHub repo and push" -d "Task 14"
```
