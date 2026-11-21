package cmd

import (
	"errors"
	"testing"
)

func TestTestReportsAnActiveScheduleAsActive(t *testing.T) {
	paths := newTestFilePaths(t)
	writeTestConfig(t, paths, testConfig)

	if err := runTest(testScheduleName, paths, timeInsideWindow); !errors.Is(err, errScheduleActive) {
		t.Errorf("got %v, want errScheduleActive", err)
	}
}

func TestTestReportsAnInactiveScheduleAsInactive(t *testing.T) {
	paths := newTestFilePaths(t)
	writeTestConfig(t, paths, testConfig)

	if err := runTest(testScheduleName, paths, timeOutsideWindow); err != nil {
		t.Errorf("got %v, want nil", err)
	}
}

func TestTestReportsAPausedScheduleAsInactive(t *testing.T) {
	paths := newTestFilePaths(t)
	writeTestConfig(t, paths, testConfig)
	writeActivePause(t, paths, timeInsideWindow)

	if err := runTest(testScheduleName, paths, timeInsideWindow); err != nil {
		t.Errorf("got %v, want nil while paused", err)
	}
}

func TestTestRejectsAnUnknownSchedule(t *testing.T) {
	paths := newTestFilePaths(t)
	writeTestConfig(t, paths, testConfig)

	err := runTest("no_such_schedule", paths, timeOutsideWindow)
	if err == nil || errors.Is(err, errScheduleActive) {
		t.Errorf("got %v, want an unknown-schedule error", err)
	}
}

func TestTestWithoutAConfigFails(t *testing.T) {
	paths := newTestFilePaths(t)

	if err := runTest(testScheduleName, paths, timeOutsideWindow); err == nil {
		t.Error("expected an error with no config file")
	}
}
