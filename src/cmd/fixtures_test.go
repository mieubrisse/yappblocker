package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mieubrisse/yappblocker/internal/killer"
	"github.com/mieubrisse/yappblocker/internal/pause"
)

// testConfig blocks Safari (an osascript quit) and a pkill-killed helper every
// evening from 20:00 to 23:00.
const testConfig = `
apps:
  safari:
    match: "Safari"
    killType: osascript
  helper:
    match: "SomeHelper"
    killType: pkillGraceful
appSets:
  evening_apps:
    apps: [safari, helper]
schedules:
  evening:
    appSet: evening_apps
    windows:
      - days: [mon, tue, wed, thu, fri, sat, sun]
        start: "20:00"
        end: "23:00"
`

const testScheduleName = "evening"

var (
	timeInsideWindow  = time.Date(2026, time.November, 18, 21, 0, 0, 0, time.Local)
	timeOutsideWindow = time.Date(2026, time.November, 18, 12, 0, 0, 0, time.Local)
)

// noKillGracePeriod keeps the post-kill check immediate. The fake machine answers
// from a table, so there is nothing for a wait to change.
const noKillGracePeriod = 0

// newTestFilePaths roots every file the commands touch in a fresh temporary
// directory.
func newTestFilePaths(t *testing.T) *filePaths {
	t.Helper()
	dirPath := t.TempDir()
	return newFilePaths(filepath.Join(dirPath, "config"), filepath.Join(dirPath, "state"))
}

func writeTestConfig(t *testing.T, paths *filePaths, contents string) {
	t.Helper()
	writeTestFile(t, paths.configFilePath, contents)
}

func writeTestFile(t *testing.T, filePath string, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
}

func fixedClock(now time.Time) func() time.Time {
	return func() time.Time { return now }
}

func writeActivePause(t *testing.T, paths *filePaths, now time.Time) {
	t.Helper()
	state, err := pause.Begin(nil, time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := pause.Save(paths.pauseStateFilePath, state); err != nil {
		t.Fatal(err)
	}
}

// fakeMachine stands in for the process table, AppleScript and launchctl. It
// answers every command the commands are known to run, and fails the test on any
// other, so an unexpected command is never silently treated as success.
type fakeMachine struct {
	t        *testing.T
	commands [][]string

	// runningMatches holds the match string of every app currently running.
	runningMatches map[string]bool
	// matchesIgnoringKills holds apps that stay running after being told to quit.
	matchesIgnoringKills map[string]bool
	notifications        []string
}

func newFakeMachine(t *testing.T, runningMatches ...string) *fakeMachine {
	machine := &fakeMachine{
		t:                    t,
		runningMatches:       map[string]bool{},
		matchesIgnoringKills: map[string]bool{},
	}
	for _, match := range runningMatches {
		machine.runningMatches[match] = true
	}
	return machine
}

func (m *fakeMachine) Run(args []string) (string, error) {
	argsCopy := make([]string, len(args))
	copy(argsCopy, args)
	m.commands = append(m.commands, argsCopy)

	switch {
	case len(args) == 3 && args[0] == "pgrep":
		// The pattern arrives bracketed, e.g. "[S]afari".
		match := strings.Replace(strings.Replace(args[2], "[", "", 1), "]", "", 1)
		if m.runningMatches[match] {
			return "4242\n", nil
		}
		return "", &killer.ExitError{Code: 1}

	case len(args) >= 3 && args[0] == "pkill":
		m.receiveKill(args[len(args)-1])
		return "", nil

	case len(args) == 3 && args[0] == "osascript":
		script := args[2]
		if strings.HasPrefix(script, "display notification") {
			m.notifications = append(m.notifications, script)
			return "", nil
		}
		for match := range m.runningMatches {
			if script == fmt.Sprintf("application %q is running", match) {
				return "true\n", nil
			}
			if script == fmt.Sprintf("quit app %q", match) {
				m.receiveKill(match)
				return "", nil
			}
		}
		if strings.HasSuffix(script, "is running") {
			return "false\n", nil
		}
		// Quitting an app that is not running succeeds, as the real osascript does.
		if strings.HasPrefix(script, "quit app") {
			return "", nil
		}

	case len(args) == 3 && args[0] == "launchctl":
		return "", nil
	}

	m.t.Errorf("unexpected command run: %v", args)
	return "", fmt.Errorf("the fake machine does not know the command %v", args)
}

func (m *fakeMachine) receiveKill(match string) {
	if !m.matchesIgnoringKills[match] {
		delete(m.runningMatches, match)
	}
}

// commandsStartingWith returns the commands whose first argument is the given one.
func (m *fakeMachine) commandsStartingWith(name string) [][]string {
	var matching [][]string
	for _, command := range m.commands {
		if command[0] == name {
			matching = append(matching, command)
		}
	}
	return matching
}
