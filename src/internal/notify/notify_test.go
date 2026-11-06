package notify

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

type mockRunner struct {
	commands [][]string
	output   string
	err      error
}

func (m *mockRunner) Run(args []string) (string, error) {
	argsCopy := make([]string, len(args))
	copy(argsCopy, args)
	m.commands = append(m.commands, argsCopy)
	return m.output, m.err
}

func TestFirstRefusalIsReportedAndRepeatsAreNot(t *testing.T) {
	ignored := []string{"chrome"}

	firstRun := AppNamesNeedingNotification(nil, ignored)
	if len(firstRun) != 1 || firstRun[0] != "chrome" {
		t.Fatalf("expected the first refusal to be reported, got %v", firstRun)
	}

	// The state saved by that run is what the next run sees.
	secondRun := AppNamesNeedingNotification(StateAfterRun(ignored), ignored)
	if len(secondRun) != 0 {
		t.Errorf("expected a repeat refusal to stay quiet, got %v", secondRun)
	}
}

func TestAppIsReportedAgainAfterItHasBeenSeenGone(t *testing.T) {
	reported := StateAfterRun([]string{"chrome"})

	// The app finally died, so the episode is over and nothing is carried forward.
	afterItDied := StateAfterRun(nil)
	if len(afterItDied.ReportedAppNames) != 0 {
		t.Fatalf("expected the episode to be forgotten once the app died, got %v", afterItDied.ReportedAppNames)
	}

	// It starts refusing again in some later window.
	again := AppNamesNeedingNotification(afterItDied, []string{"chrome"})
	if len(again) != 1 {
		t.Errorf("expected a fresh episode to be reported again, got %v", again)
	}

	// Guard against the opposite mistake: carrying the old state forward must not
	// report it twice within the same episode.
	stillSame := AppNamesNeedingNotification(reported, []string{"chrome"})
	if len(stillSame) != 0 {
		t.Errorf("expected no repeat within one episode, got %v", stillSame)
	}
}

func TestOnlyTheNewlyRefusingAppsAreReported(t *testing.T) {
	state := StateAfterRun([]string{"chrome"})

	needing := AppNamesNeedingNotification(state, []string{"chrome", "discord", "slack"})

	if strings.Join(needing, ",") != "discord,slack" {
		t.Errorf("expected only the unreported apps, sorted, got %v", needing)
	}
}

func TestSendRaisesOneNotificationNamingEveryApp(t *testing.T) {
	runner := &mockRunner{}

	if err := Send(runner, []string{"chrome", "discord"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.commands) != 1 {
		t.Fatalf("expected one notification for the whole set, got %d: %v", len(runner.commands), runner.commands)
	}

	script := strings.Join(runner.commands[0], " ")
	if !strings.Contains(script, "osascript") || !strings.Contains(script, "display notification") {
		t.Errorf("expected an osascript display-notification call, got %q", script)
	}
	if !strings.Contains(script, "chrome") || !strings.Contains(script, "discord") {
		t.Errorf("expected both apps named in the notification, got %q", script)
	}
}

func TestSendDoesNothingWhenThereIsNothingToReport(t *testing.T) {
	runner := &mockRunner{}

	if err := Send(runner, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(runner.commands) != 0 {
		t.Errorf("expected no notification, got %v", runner.commands)
	}
}

// TestSendReportsItsOwnFailure matters more here than in most places: a
// notification channel that fails quietly leaves the tool exactly as blind as it
// was before any of this existed.
func TestSendReportsItsOwnFailure(t *testing.T) {
	runner := &mockRunner{
		output: "execution error: something went wrong\n",
		err:    errors.New("exit code 1"),
	}

	err := Send(runner, []string{"chrome"})

	if err == nil {
		t.Fatal("expected an error when the notification could not be raised")
	}
	if !strings.Contains(err.Error(), "chrome") || !strings.Contains(err.Error(), "something went wrong") {
		t.Errorf("expected the app and the command's own output in the error, got %q", err.Error())
	}
}

func TestLoadMissingFileIsNotAnError(t *testing.T) {
	state, err := Load(filepath.Join(t.TempDir(), "nope.json"))

	if err != nil {
		t.Fatalf("unexpected error for a missing state file: %v", err)
	}
	if state != nil {
		t.Errorf("expected no state for a missing file, got %v", state)
	}
}

func TestSaveThenLoadRoundTrips(t *testing.T) {
	stateFilePath := filepath.Join(t.TempDir(), "nested", "ignored-kills.json")

	if err := Save(stateFilePath, StateAfterRun([]string{"discord", "chrome"})); err != nil {
		t.Fatalf("unexpected error saving: %v", err)
	}

	loaded, err := Load(stateFilePath)
	if err != nil {
		t.Fatalf("unexpected error loading: %v", err)
	}
	if strings.Join(loaded.ReportedAppNames, ",") != "chrome,discord" {
		t.Errorf("expected the sorted app names to round-trip, got %v", loaded.ReportedAppNames)
	}
}
