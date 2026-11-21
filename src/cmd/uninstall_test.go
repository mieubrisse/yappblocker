package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestUninstallIsRefusedInsideAWindow(t *testing.T) {
	paths := newTestFilePaths(t)
	writeTestConfig(t, paths, testConfig)
	var out bytes.Buffer

	err := requireNoBlockingWindowOpen(timeInsideWindow, &out, paths.configFilePath)
	if !errors.Is(err, errUninstallRefused) {
		t.Errorf("got %v, want errUninstallRefused", err)
	}
	if !strings.Contains(out.String(), testScheduleName) {
		t.Errorf("output %q does not name the active schedule", out.String())
	}

	// The command itself refuses before asking for the phrase.
	if err := runUninstall(fixedClock(timeInsideWindow), paths.configFilePath); !errors.Is(err, errUninstallRefused) {
		t.Errorf("runUninstall: got %v, want errUninstallRefused", err)
	}
}

func TestUninstallIsAllowedOutsideAWindow(t *testing.T) {
	paths := newTestFilePaths(t)
	writeTestConfig(t, paths, testConfig)

	if err := requireNoBlockingWindowOpen(timeOutsideWindow, &bytes.Buffer{}, paths.configFilePath); err != nil {
		t.Errorf("got %v, want nil", err)
	}
}

func TestUninstallIsAllowedWithNoConfig(t *testing.T) {
	paths := newTestFilePaths(t)

	if err := requireNoBlockingWindowOpen(timeInsideWindow, &bytes.Buffer{}, paths.configFilePath); err != nil {
		t.Errorf("got %v, want nil", err)
	}
}

func TestUninstallIsRefusedWithABrokenConfig(t *testing.T) {
	paths := newTestFilePaths(t)
	writeTestConfig(t, paths, "schedules: [this is not a map")

	err := requireNoBlockingWindowOpen(timeOutsideWindow, &bytes.Buffer{}, paths.configFilePath)
	if err == nil || errors.Is(err, errUninstallRefused) {
		t.Errorf("got %v, want a config error", err)
	}
}
