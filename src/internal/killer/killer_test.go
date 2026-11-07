package killer

import (
	"bytes"
	"errors"
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
	// is how a test says whether the app's processes actually went away.
	pgrepResultsAfterKill map[string]string
	// isRunningAnswer is what LaunchServices reports for `application "X" is
	// running`. Empty behaves like "false", which is the ordinary case of an app
	// that obeyed its quit.
	isRunningAnswer string
	// isRunningError makes that check unanswerable.
	isRunningError error
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

	// The survival check is also an osascript call, so it must not be mistaken for
	// a kill.
	if len(args) >= 3 && args[0] == "osascript" && strings.Contains(args[2], "is running") {
		return m.isRunningAnswer, m.isRunningError
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

	count, _ := FindAndKillApps(apps, false, false, runner, noGracePeriodSoTestsDoNotSleep)

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
		isRunningAnswer: "false\n",
	}
	apps := []*config.App{
		{Name: "myapp", Match: "MyApp", KillType: config.KillTypeOsascript},
	}

	count, _ := FindAndKillApps(apps, false, false, runner, noGracePeriodSoTestsDoNotSleep)

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

	count, _ := FindAndKillApps(apps, false, false, runner, noGracePeriodSoTestsDoNotSleep)

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

	count, _ := FindAndKillApps(apps, true, false, runner, noGracePeriodSoTestsDoNotSleep)

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

	count, _ := FindAndKillApps(apps, false, false, runner, noGracePeriodSoTestsDoNotSleep)

	if count != 0 {
		t.Errorf("expected count 0, got %d", count)
	}
}

// TestSurvivingHelperProcessesAreNotAFailedQuit is the regression test for the
// false alarms shipped in v0.4.1. Messages, Safari, Steam and the Claude desktop
// app all obeyed their quits while leaving XPC or helper processes behind, and all
// four were reported as refusing.
func TestSurvivingHelperProcessesAreNotAFailedQuit(t *testing.T) {
	runner := &mockRunner{
		pgrepResults: map[string]string{
			"[M]essages": "100\n",
		},
		// The helper process outlives the quit, as an XPC service does.
		pgrepResultsAfterKill: map[string]string{
			"[M]essages": "101\n",
		},
		// But the application itself is gone, which is what the quit acted on.
		isRunningAnswer: "false\n",
	}
	apps := []*config.App{
		{Name: "messages", Match: "Messages", KillType: config.KillTypeOsascript},
	}

	var ignored []*config.App
	logged := captureLogOutput(t, func() {
		_, ignored = FindAndKillApps(apps, false, false, runner, noGracePeriodSoTestsDoNotSleep)
	})

	if logged != "" {
		t.Errorf("expected no warning when only helper processes survive, got: %q", logged)
	}
	if len(ignored) != 0 {
		t.Errorf("expected nothing reported to the caller, got %v", ignored)
	}
}

func TestOsascriptQuitThatIsIgnoredIsStillReported(t *testing.T) {
	runner := &mockRunner{
		pgrepResults: map[string]string{
			"[G]oogle Chrome": "66775\n",
		},
		isRunningAnswer: "true\n",
	}
	apps := []*config.App{
		{Name: "chrome", Match: "Google Chrome", KillType: config.KillTypeOsascript},
	}

	var ignored []*config.App
	logged := captureLogOutput(t, func() {
		_, ignored = FindAndKillApps(apps, false, false, runner, noGracePeriodSoTestsDoNotSleep)
	})

	if !strings.Contains(logged, "Google Chrome") || !strings.Contains(logged, "the application is still running") {
		t.Errorf("expected a warning that the application is still running, got: %q", logged)
	}
	if len(ignored) != 1 || ignored[0].Name != "chrome" {
		t.Errorf("expected the refusing app returned for the caller to notify about, got %v", ignored)
	}
}

// TestPkillSurvivorsAreStillCheckedAgainstProcesses keeps the process-level path
// honest: a pkill signals processes, so surviving processes are a real failure
// there. Chrome PWAs such as the gmail entry have no application identity and rely
// on this path.
func TestPkillSurvivorsAreStillCheckedAgainstProcesses(t *testing.T) {
	runner := &mockRunner{
		pgrepResults: map[string]string{
			"[G]mail.app": "900\n",
		},
		pgrepResultsAfterKill: map[string]string{
			"[G]mail.app": "900\n901\n",
		},
	}
	apps := []*config.App{
		{Name: "gmail", Match: "Gmail.app", KillType: config.KillTypePkillGraceful},
	}

	var ignored []*config.App
	logged := captureLogOutput(t, func() {
		_, ignored = FindAndKillApps(apps, false, false, runner, noGracePeriodSoTestsDoNotSleep)
	})

	if !strings.Contains(logged, "2 process(es) are still running") {
		t.Errorf("expected a process-level warning, got: %q", logged)
	}
	if len(ignored) != 1 {
		t.Errorf("expected the app reported, got %v", ignored)
	}
}

// TestUnanswerableSurvivalCheckIsLoggedWithoutManufacturingAFailure covers the
// middle case: a check that cannot be answered must neither pass silently nor
// invent a failure.
func TestUnanswerableSurvivalCheckIsLoggedWithoutManufacturingAFailure(t *testing.T) {
	runner := &mockRunner{
		pgrepResults: map[string]string{
			"[O]ddApp": "500\n",
		},
		isRunningAnswer: "execution error: something broke\n",
		isRunningError:  &ExitError{Code: 1},
	}
	apps := []*config.App{
		{Name: "odd", Match: "OddApp", KillType: config.KillTypeOsascript},
	}

	var ignored []*config.App
	logged := captureLogOutput(t, func() {
		_, ignored = FindAndKillApps(apps, false, false, runner, noGracePeriodSoTestsDoNotSleep)
	})

	if !strings.Contains(logged, "could not tell whether") || !strings.Contains(logged, "something broke") {
		t.Errorf("expected the unanswerable check logged with its own output, got: %q", logged)
	}
	if len(ignored) != 0 {
		t.Errorf("expected no manufactured failure, got %v", ignored)
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
		killError:  errors.New("exit code 1"),
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
