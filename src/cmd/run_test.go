package cmd

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/mieubrisse/yappblocker/internal/notify"
)

const (
	isDryRunForTest    = true
	isNotDryRunForTest = false
	isVerboseForTest   = true
)

func TestRunKillsRunningAppsInsideAWindow(t *testing.T) {
	paths := newTestFilePaths(t)
	writeTestConfig(t, paths, testConfig)
	machine := newFakeMachine(t, "Safari", "SomeHelper")

	if err := runBlocking(paths, machine, noKillGracePeriod, timeInsideWindow, isNotDryRunForTest, isVerboseForTest); err != nil {
		t.Fatalf("runBlocking: %v", err)
	}

	if len(machine.runningMatches) != 0 {
		t.Errorf("apps still running after the run: %v", machine.runningMatches)
	}
	wantPkills := [][]string{{"pkill", "-f", "SomeHelper"}}
	if got := machine.commandsStartingWith("pkill"); !reflect.DeepEqual(got, wantPkills) {
		t.Errorf("got pkill commands %v, want %v", got, wantPkills)
	}
	if len(machine.notifications) != 0 {
		t.Errorf("no app refused, yet notifications were raised: %v", machine.notifications)
	}
	// Nothing was ever reported, so there is no state worth writing.
	if _, err := os.Stat(paths.ignoredKillStateFilePath); !os.IsNotExist(err) {
		t.Errorf("expected no ignored-kill state file, stat said: %v", err)
	}
}

func TestRunNotifiesOnceWhenAnAppIgnoresItsKill(t *testing.T) {
	paths := newTestFilePaths(t)
	writeTestConfig(t, paths, testConfig)
	machine := newFakeMachine(t, "Safari")
	machine.matchesIgnoringKills["Safari"] = true

	for range 2 {
		if err := runBlocking(paths, machine, noKillGracePeriod, timeInsideWindow, isNotDryRunForTest, isVerboseForTest); err != nil {
			t.Fatalf("runBlocking: %v", err)
		}
	}

	if len(machine.notifications) != 1 {
		t.Fatalf("got %d notifications over two refusing runs, want exactly 1: %v", len(machine.notifications), machine.notifications)
	}
	if !strings.Contains(machine.notifications[0], "safari") {
		t.Errorf("notification %q does not name the app", machine.notifications[0])
	}

	state, err := notify.Load(paths.ignoredKillStateFilePath)
	if err != nil {
		t.Fatalf("loading ignored-kill state: %v", err)
	}
	if state == nil || !reflect.DeepEqual(state.ReportedAppNames, []string{"safari"}) {
		t.Errorf("got ignored-kill state %+v, want safari recorded", state)
	}

	// Once the window closes, the episode is forgotten so the next one is reported.
	if err := runBlocking(paths, machine, noKillGracePeriod, timeOutsideWindow, isNotDryRunForTest, isVerboseForTest); err != nil {
		t.Fatalf("runBlocking: %v", err)
	}
	state, err = notify.Load(paths.ignoredKillStateFilePath)
	if err != nil {
		t.Fatalf("loading ignored-kill state: %v", err)
	}
	if state == nil || len(state.ReportedAppNames) != 0 {
		t.Errorf("got ignored-kill state %+v after the window closed, want it emptied", state)
	}
}

func TestRunOutsideAWindowRunsNothing(t *testing.T) {
	paths := newTestFilePaths(t)
	writeTestConfig(t, paths, testConfig)
	machine := newFakeMachine(t, "Safari")

	if err := runBlocking(paths, machine, noKillGracePeriod, timeOutsideWindow, isNotDryRunForTest, isVerboseForTest); err != nil {
		t.Fatalf("runBlocking: %v", err)
	}

	if len(machine.commands) != 0 {
		t.Errorf("expected no commands outside the window, got %v", machine.commands)
	}
}

func TestRunDryRunKillsNothing(t *testing.T) {
	paths := newTestFilePaths(t)
	writeTestConfig(t, paths, testConfig)
	machine := newFakeMachine(t, "Safari", "SomeHelper")

	if err := runBlocking(paths, machine, noKillGracePeriod, timeInsideWindow, isDryRunForTest, isVerboseForTest); err != nil {
		t.Fatalf("runBlocking: %v", err)
	}

	for _, command := range machine.commands {
		if command[0] != "pgrep" {
			t.Errorf("a dry run ran %v; it should only look", command)
		}
	}
	if len(machine.runningMatches) != 2 {
		t.Errorf("a dry run stopped apps: still running %v", machine.runningMatches)
	}
}

func TestRunWhilePausedKillsNothing(t *testing.T) {
	paths := newTestFilePaths(t)
	writeTestConfig(t, paths, testConfig)
	writeActivePause(t, paths, timeInsideWindow)
	machine := newFakeMachine(t, "Safari")

	if err := runBlocking(paths, machine, noKillGracePeriod, timeInsideWindow, isNotDryRunForTest, isVerboseForTest); err != nil {
		t.Fatalf("runBlocking: %v", err)
	}

	if len(machine.commands) != 0 {
		t.Errorf("expected no commands while paused, got %v", machine.commands)
	}
}

func TestRunWithoutAConfigFails(t *testing.T) {
	paths := newTestFilePaths(t)
	machine := newFakeMachine(t)

	err := runBlocking(paths, machine, noKillGracePeriod, timeInsideWindow, isNotDryRunForTest, isVerboseForTest)
	if err == nil {
		t.Fatal("expected an error with no config file")
	}
	if !strings.Contains(err.Error(), paths.configFilePath) {
		t.Errorf("error %q does not name the missing config %q", err, paths.configFilePath)
	}
}

func TestRunWithAnInvalidConfigFails(t *testing.T) {
	paths := newTestFilePaths(t)
	writeTestConfig(t, paths, "schedules: [this is not a map")
	machine := newFakeMachine(t)

	if err := runBlocking(paths, machine, noKillGracePeriod, timeInsideWindow, isNotDryRunForTest, isVerboseForTest); err == nil {
		t.Fatal("expected an error with an unparseable config")
	}
}
