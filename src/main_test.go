package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These tests run the real binary in a subprocess, because the contract they pin is
// a process exit code: the Claude Code prompt hook in Kevin's dotfiles reads
// `yappblocker test SCHEDULE` exiting 0 as "the window is closed" and 1 as "the
// window is open". An in-process test of the command could pass while main's
// mapping of errors to exit codes broke it.

// subprocessConfig has one schedule that is open at every moment and one that is
// never open, so the tests do not depend on the time of day they run at. A window
// whose start equals its end is overnight, and covers the whole day on every listed
// day; a window with no days listed never opens.
const subprocessConfig = `
apps:
  safari:
    match: "Safari"
appSets:
  browsers:
    apps: [safari]
schedules:
  always_open_for_test:
    appSet: browsers
    windows:
      - days: [mon, tue, wed, thu, fri, sat, sun]
        start: "00:00"
        end: "00:00"
  never_open_for_test:
    appSet: browsers
    windows:
      - days: []
        start: "00:00"
        end: "00:00"
`

const (
	exitCodeScheduleInactive = 0
	exitCodeScheduleActive   = 1
)

var binaryFilePath string

func TestMain(m *testing.M) {
	os.Exit(buildBinaryAndRunTests(m))
}

func buildBinaryAndRunTests(m *testing.M) int {
	buildDirPath, err := os.MkdirTemp("", "yappblocker-binary-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not create a directory for the test binary: %v\n", err)
		return 1
	}
	defer func() {
		if err := os.RemoveAll(buildDirPath); err != nil {
			fmt.Fprintf(os.Stderr, "could not remove the test binary directory '%s', so delete it by hand: %v\n", buildDirPath, err)
		}
	}()

	binaryFilePath = filepath.Join(buildDirPath, "yappblocker")
	buildOutput, err := exec.Command("go", "build", "-o", binaryFilePath, ".").CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not build yappblocker for the subprocess tests: %v\n%s\n", err, buildOutput)
		return 1
	}

	return m.Run()
}

// runYappblockerTest runs `yappblocker test SCHEDULE` against a config in a fresh
// temporary home and returns its exit code and stderr. HOME and both XDG
// directories are pointed into that home, so the binary can reach neither the real
// config nor the real pause state, which a pause in progress would otherwise turn
// into a false "inactive".
func runYappblockerTest(t *testing.T, scheduleName string, configContents string) (int, string) {
	t.Helper()
	homeDirPath := t.TempDir()
	configDirPath := filepath.Join(homeDirPath, "config")
	stateDirPath := filepath.Join(homeDirPath, "state")

	if configContents != "" {
		configFilePath := filepath.Join(configDirPath, "yappblocker", "config.yaml")
		if err := os.MkdirAll(filepath.Dir(configFilePath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(configFilePath, []byte(configContents), 0644); err != nil {
			t.Fatal(err)
		}
	}

	command := exec.Command(binaryFilePath, "test", scheduleName)
	command.Env = append(os.Environ(),
		"HOME="+homeDirPath,
		"XDG_CONFIG_HOME="+configDirPath,
		"XDG_STATE_HOME="+stateDirPath,
	)
	var stderr strings.Builder
	command.Stderr = &stderr

	err := command.Run()
	if err == nil {
		return 0, stderr.String()
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("could not run %v: %v", command.Args, err)
	}
	return exitErr.ExitCode(), stderr.String()
}

func TestTestExitsOneForAnActiveSchedule(t *testing.T) {
	exitCode, stderr := runYappblockerTest(t, "always_open_for_test", subprocessConfig)

	if exitCode != exitCodeScheduleActive {
		t.Errorf("got exit code %d, want %d; stderr: %s", exitCode, exitCodeScheduleActive, stderr)
	}
	// The hook depends on the active case being silent.
	if stderr != "" {
		t.Errorf("expected no output for an active schedule, got %q", stderr)
	}
}

func TestTestExitsZeroForAnInactiveSchedule(t *testing.T) {
	// This also proves the temporary config was the one read: no real config has a
	// schedule by this name, and an unknown schedule exits 1.
	exitCode, stderr := runYappblockerTest(t, "never_open_for_test", subprocessConfig)

	if exitCode != exitCodeScheduleInactive {
		t.Errorf("got exit code %d, want %d; stderr: %s", exitCode, exitCodeScheduleInactive, stderr)
	}
}

func TestTestExitsOneForAnUnknownSchedule(t *testing.T) {
	exitCode, stderr := runYappblockerTest(t, "no_such_schedule", subprocessConfig)

	if exitCode != exitCodeScheduleActive {
		t.Errorf("got exit code %d, want %d", exitCode, exitCodeScheduleActive)
	}
	if !strings.Contains(stderr, "unknown schedule") {
		t.Errorf("stderr %q does not report the unknown schedule", stderr)
	}
}

func TestTestExitsOneWithNoConfig(t *testing.T) {
	exitCode, stderr := runYappblockerTest(t, "always_open_for_test", "")

	if exitCode != exitCodeScheduleActive {
		t.Errorf("got exit code %d, want %d", exitCode, exitCodeScheduleActive)
	}
	if !strings.Contains(stderr, "config file not found") {
		t.Errorf("stderr %q does not report the missing config", stderr)
	}
}
