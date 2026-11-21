package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

// The pause tests stop short of the confirmation phrase, which needs a real
// terminal; every refusal below is decided before it is asked for.

func TestPauseWithNoArgumentsReportsStatus(t *testing.T) {
	paths := newTestFilePaths(t)
	var out bytes.Buffer

	if err := runPause(nil, fixedClock(timeInsideWindow), &out, paths.pauseStateFilePath); err != nil {
		t.Fatalf("runPause: %v", err)
	}
	if !strings.Contains(out.String(), "Blocking is not paused.") {
		t.Errorf("output %q does not report the unpaused state", out.String())
	}

	writeActivePause(t, paths, timeInsideWindow)
	out.Reset()
	if err := runPause(nil, fixedClock(timeInsideWindow), &out, paths.pauseStateFilePath); err != nil {
		t.Fatalf("runPause: %v", err)
	}
	if !strings.Contains(out.String(), "Blocking is paused until") {
		t.Errorf("output %q does not report the pause", out.String())
	}
}

func TestPauseRefusesBeforeAskingForThePhrase(t *testing.T) {
	tests := []struct {
		name          string
		requested     string
		isPausedFirst bool
		wantOutput    string
	}{
		{name: "unreadable duration", requested: "soon", wantOutput: "Could not read"},
		{name: "zero duration", requested: "0s", wantOutput: "longer than zero"},
		{name: "beyond the allowance", requested: "4h", wantOutput: "the most you can pause right now"},
		{name: "shorter than the pause already running", requested: "10m", isPausedFirst: true, wantOutput: "would cut that short"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			paths := newTestFilePaths(t)
			if test.isPausedFirst {
				writeActivePause(t, paths, timeInsideWindow)
			}
			var out bytes.Buffer

			err := runPause([]string{test.requested}, fixedClock(timeInsideWindow), &out, paths.pauseStateFilePath)
			if !errors.Is(err, errPauseRefused) {
				t.Errorf("got %v, want errPauseRefused", err)
			}
			if !strings.Contains(out.String(), test.wantOutput) {
				t.Errorf("output %q does not contain %q", out.String(), test.wantOutput)
			}
		})
	}
}

func TestPauseRefusesOnceTheWindowIsUsedUp(t *testing.T) {
	paths := newTestFilePaths(t)
	writeActivePause(t, paths, timeInsideWindow)
	var out bytes.Buffer

	// Past the 3-hour window the first pause opened, but inside its 24-hour cooldown.
	later := timeInsideWindow.Add(5 * time.Hour)
	err := runPause([]string{"10m"}, fixedClock(later), &out, paths.pauseStateFilePath)
	if !errors.Is(err, errPauseRefused) {
		t.Errorf("got %v, want errPauseRefused", err)
	}
	if !strings.Contains(out.String(), "pause window has closed") {
		t.Errorf("output %q does not say the window has closed", out.String())
	}
}

func TestPauseRefusesWithADamagedStateFile(t *testing.T) {
	paths := newTestFilePaths(t)
	writeTestFile(t, paths.pauseStateFilePath, "{not json")

	err := runPause([]string{"10m"}, fixedClock(timeInsideWindow), &bytes.Buffer{}, paths.pauseStateFilePath)
	if err == nil || errors.Is(err, errPauseRefused) {
		t.Errorf("got %v, want a state-file error", err)
	}
}

func TestADamagedPauseStateLeavesBlockingOn(t *testing.T) {
	paths := newTestFilePaths(t)
	writeTestFile(t, paths.pauseStateFilePath, "{not json")
	var out bytes.Buffer

	if isBlockingPaused(timeInsideWindow, &out, paths.pauseStateFilePath) {
		t.Error("a damaged pause state switched blocking off")
	}
	if !strings.Contains(out.String(), "blocking stays on") {
		t.Errorf("output %q does not warn about the damaged state", out.String())
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{
			name:     "whole hours drop the minutes",
			duration: 3 * time.Hour,
			want:     "3h",
		},
		{
			name:     "hours and minutes together",
			duration: 2*time.Hour + 15*time.Minute,
			want:     "2h15m",
		},
		{
			name:     "under an hour is minutes alone",
			duration: 45 * time.Minute,
			want:     "45m",
		},
		{
			name:     "seconds round to the nearest minute",
			duration: 44*time.Minute + 40*time.Second,
			want:     "45m",
		},
		{
			name:     "a single minute",
			duration: time.Minute,
			want:     "1m",
		},
		{
			name:     "anything under a minute is spelled out",
			duration: 20 * time.Second,
			want:     "less than a minute",
		},
		{
			name:     "zero is spelled out too",
			duration: 0,
			want:     "less than a minute",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := formatDuration(test.duration); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}
