package killer

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"github.com/mieubrisse/yappblocker/internal/config"
)

// noGracePeriodSoTestsDoNotSleep keeps the post-kill check immediate. The mock
// runner answers from a table rather than from the real process list, so there is
// nothing for a wait to change.
const noGracePeriodSoTestsDoNotSleep = 0

type mockRunner struct {
	commands     [][]string
	pgrepResults map[string]string // bracketed match pattern -> stdout
	// pgrepResultsAfterKill overrides pgrepResults once a kill has been sent, which
	// is how a test says whether the app actually died.
	pgrepResultsAfterKill map[string]string
	// killOutput and killError are what a kill command reports back.
	killOutput string
	killError  error

	killHasBeenSent bool
}

func (m *mockRunner) Run(args []string) (string, error) {
	argsCopy := make([]string, len(args))
	copy(argsCopy, args)
	m.commands = append(m.commands, argsCopy)

	if len(args) >= 3 && args[0] == "pgrep" {
		match := args[2]
		if m.killHasBeenSent {
			if stdout, ok := m.pgrepResultsAfterKill[match]; ok {
				return stdout, nil
			}
		}
		if stdout, ok := m.pgrepResults[match]; ok {
			return stdout, nil
		}
		return "", &ExitError{Code: 1}
	}

	m.killHasBeenSent = true
	return m.killOutput, m.killError
}

// captureLogOutput collects everything written through the standard logger while
// the given function runs, since the warnings this package emits are its only
// externally visible behaviour.
func captureLogOutput(t *testing.T, run func()) string {
	t.Helper()

	var buffer bytes.Buffer
	originalOutput := log.Writer()
	log.SetOutput(&buffer)
	defer log.SetOutput(originalOutput)

	run()

	return buffer.String()
}

func TestBuildPgrepPattern(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Discord", "[D]iscord"},
		{"Steam", "[S]team"},
		{"a", "[a]"},
		{"", ""},
	}
	for _, tt := range tests {
		got := buildPgrepPattern(tt.input)
		if got != tt.expected {
			t.Errorf("buildPgrepPattern(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestKillForceApp(t *testing.T) {
	runner := &mockRunner{
		pgrepResults: map[string]string{
			"[S]omeApp": "123\n456\n",
		},
		pgrepResultsAfterKill: map[string]string{
			"[S]omeApp": "",
		},
	}
	apps := []*config.App{
		{Name: "someapp", Match: "SomeApp", KillType: config.KillTypePkillForce},
	}

	count := FindAndKillApps(apps, false, false, runner, noGracePeriodSoTestsDoNotSleep)

	if count != 2 {
		t.Errorf("expected count 2, got %d", count)
	}

	foundKill := false
	for _, cmd := range runner.commands {
		if len(cmd) >= 4 && cmd[0] == "pkill" && cmd[1] == "-KILL" && cmd[2] == "-f" && cmd[3] == "SomeApp" {
			foundKill = true
		}
	}
	if !foundKill {
		t.Errorf("expected pkill -KILL -f SomeApp command, got commands: %v", runner.commands)
	}
}

func TestKillOsascript(t *testing.T) {
	runner := &mockRunner{
		pgrepResults: map[string]string{
			"[M]yApp": "789\n",
		},
		pgrepResultsAfterKill: map[string]string{
			"[M]yApp": "",
		},
	}
	apps := []*config.App{
		{Name: "myapp", Match: "MyApp", KillType: config.KillTypeOsascript},
	}

	count := FindAndKillApps(apps, false, false, runner, noGracePeriodSoTestsDoNotSleep)

	if count != 1 {
		t.Errorf("expected count 1, got %d", count)
	}

	foundKill := false
	for _, cmd := range runner.commands {
		if len(cmd) >= 3 && cmd[0] == "osascript" && cmd[1] == "-e" && cmd[2] == "quit app \"MyApp\"" {
			foundKill = true
		}
	}
	if !foundKill {
		t.Errorf("expected osascript quit command, got commands: %v", runner.commands)
	}
}

func TestKillPkillGraceful(t *testing.T) {
	runner := &mockRunner{
		pgrepResults: map[string]string{
			"[G]racefulApp": "321\n",
		},
		pgrepResultsAfterKill: map[string]string{
			"[G]racefulApp": "",
		},
	}
	apps := []*config.App{
		{Name: "graceful", Match: "GracefulApp", KillType: config.KillTypePkillGraceful},
	}

	count := FindAndKillApps(apps, false, false, runner, noGracePeriodSoTestsDoNotSleep)

	if count != 1 {
		t.Errorf("expected count 1, got %d", count)
	}

	foundKill := false
	for _, cmd := range runner.commands {
		if len(cmd) >= 3 && cmd[0] == "pkill" && cmd[1] == "-f" && cmd[2] == "GracefulApp" {
			foundKill = true
		}
	}
	if !foundKill {
		t.Errorf("expected pkill -f GracefulApp command, got commands: %v", runner.commands)
	}
}

func TestDryRunDoesNotKill(t *testing.T) {
	runner := &mockRunner{
		pgrepResults: map[string]string{
			"[D]ryApp": "123\n",
		},
	}
	apps := []*config.App{
		{Name: "dryapp", Match: "DryApp", KillType: config.KillTypePkillForce},
	}

	count := FindAndKillApps(apps, true, false, runner, noGracePeriodSoTestsDoNotSleep)

	if count != 1 {
		t.Errorf("expected count 1, got %d", count)
	}

	if len(runner.commands) != 1 {
		t.Errorf("expected only 1 command (pgrep), got %d commands: %v", len(runner.commands), runner.commands)
	}
	if runner.commands[0][0] != "pgrep" {
		t.Errorf("expected only pgrep command, got: %v", runner.commands[0])
	}
}

func TestNoMatchingProcesses(t *testing.T) {
	runner := &mockRunner{
		pgrepResults: map[string]string{},
	}
	apps := []*config.App{
		{Name: "noapp", Match: "NoApp", KillType: config.KillTypePkillForce},
	}

	count := FindAndKillApps(apps, false, false, runner, noGracePeriodSoTestsDoNotSleep)

	if count != 0 {
		t.Errorf("expected count 0, got %d", count)
	}
}

// TestKillReportedAsSuccessfulButIgnoredIsLogged covers the defect that let a
// blocking window pass with every app still running and an empty log: osascript's
// "quit app" exits 0 even when the app ignores it.
func TestKillReportedAsSuccessfulButIgnoredIsLogged(t *testing.T) {
	runner := &mockRunner{
		pgrepResults: map[string]string{
			"[S]tubbornApp": "4242\n",
		},
		// Still running after the kill, and the kill itself reported success.
		pgrepResultsAfterKill: map[string]string{
			"[S]tubbornApp": "4242\n",
		},
	}
	apps := []*config.App{
		{Name: "stubborn", Match: "StubbornApp", KillType: config.KillTypeOsascript},
	}

	logged := captureLogOutput(t, func() {
		FindAndKillApps(apps, false, false, runner, noGracePeriodSoTestsDoNotSleep)
	})

	if !strings.Contains(logged, "StubbornApp") || !strings.Contains(logged, "still running") {
		t.Errorf("expected a warning that the app is still running, got log output: %q", logged)
	}
}

func TestKillThatWorksIsNotLogged(t *testing.T) {
	runner := &mockRunner{
		pgrepResults: map[string]string{
			"[O]bedientApp": "4242\n",
		},
		pgrepResultsAfterKill: map[string]string{
			"[O]bedientApp": "",
		},
	}
	apps := []*config.App{
		{Name: "obedient", Match: "ObedientApp", KillType: config.KillTypeOsascript},
	}

	logged := captureLogOutput(t, func() {
		FindAndKillApps(apps, false, false, runner, noGracePeriodSoTestsDoNotSleep)
	})

	if logged != "" {
		t.Errorf("expected no log output for a kill that worked, got: %q", logged)
	}
}

// TestFailedKillLogsTheCommandOutput covers the other half of the blindness: the
// kill commands explain themselves on stderr, and that explanation was captured
// and then discarded, leaving "exit code 1" as the whole diagnosis.
func TestFailedKillLogsTheCommandOutput(t *testing.T) {
	runner := &mockRunner{
		pgrepResults: map[string]string{
			"[A]ngryApp": "999\n",
		},
		killOutput: "execution error: Not authorized to send Apple events to AngryApp. (-1743)\n",
		killError:  &ExitError{Code: 1},
	}
	apps := []*config.App{
		{Name: "angry", Match: "AngryApp", KillType: config.KillTypeOsascript},
	}

	logged := captureLogOutput(t, func() {
		FindAndKillApps(apps, false, false, runner, noGracePeriodSoTestsDoNotSleep)
	})

	if !strings.Contains(logged, "-1743") {
		t.Errorf("expected the command's own error text in the log, got: %q", logged)
	}
	if strings.Count(logged, "\n") != 1 {
		t.Errorf("expected the output collapsed onto a single log line, got: %q", logged)
	}
}

func TestFormatOutputForLog(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty stays empty", "", ""},
		{"whitespace only stays empty", "  \n\t ", ""},
		{"multiline collapses", "first line\nsecond line\n", ": first line second line"},
	}
	for _, tt := range tests {
		got := formatOutputForLog(tt.input)
		if got != tt.expected {
			t.Errorf("%s: formatOutputForLog(%q) = %q, want %q", tt.name, tt.input, got, tt.expected)
		}
	}
}
