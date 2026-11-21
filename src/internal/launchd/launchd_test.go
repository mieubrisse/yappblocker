package launchd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const (
	// The recorded plist was rendered from these, so they are fixed rather than
	// derived from the machine running the test.
	recordedBinaryFilePath = "/opt/homebrew/bin/yappblocker"
	recordedLogFilePath    = "/Users/someone/Library/Application Support/yappblocker/yappblocker.log"
	recordedPlistFilePath  = "testdata/com.yappblocker.plist"

	fakeBinaryFilePath = "/fake/bin/yappblocker"
)

// mockRunner records every command instead of running it, so launchctl never
// touches a real agent.
type mockRunner struct {
	commands [][]string
	// errorsByVerb makes a launchctl subcommand ("load", "unload") fail.
	errorsByVerb map[string]error
}

func (m *mockRunner) Run(args []string) (string, error) {
	argsCopy := make([]string, len(args))
	copy(argsCopy, args)
	m.commands = append(m.commands, argsCopy)

	if len(args) >= 2 {
		if err, found := m.errorsByVerb[args[1]]; found {
			return "launchctl said no", err
		}
	}
	return "", nil
}

func newTestAgent(t *testing.T, runner *mockRunner) (*Agent, string) {
	t.Helper()
	dirPath := t.TempDir()
	plistFilePath := filepath.Join(dirPath, "LaunchAgents", plistFileName)
	logFilePath := filepath.Join(dirPath, "state", "yappblocker", "yappblocker.log")
	return NewAgent(plistFilePath, logFilePath, runner), plistFilePath
}

func TestRenderPlistMatchesRecording(t *testing.T) {
	expected, err := os.ReadFile(recordedPlistFilePath)
	if err != nil {
		t.Fatalf("reading recorded plist: %v", err)
	}

	rendered, err := renderPlist(recordedBinaryFilePath, recordedLogFilePath)
	if err != nil {
		t.Fatalf("rendering plist: %v", err)
	}

	if !bytes.Equal(rendered, expected) {
		t.Errorf("rendered plist differs from %s\n--- got ---\n%s\n--- want ---\n%s", recordedPlistFilePath, rendered, expected)
	}
}

func TestInstallWritesPlistAndLoadsIt(t *testing.T) {
	runner := &mockRunner{}
	agent, plistFilePath := newTestAgent(t, runner)

	var out bytes.Buffer
	if err := agent.Install(fakeBinaryFilePath, &out); err != nil {
		t.Fatalf("Install: %v", err)
	}

	written, err := os.ReadFile(plistFilePath)
	if err != nil {
		t.Fatalf("reading written plist: %v", err)
	}
	expected, err := renderPlist(fakeBinaryFilePath, agent.logFilePath)
	if err != nil {
		t.Fatalf("rendering plist: %v", err)
	}
	if !bytes.Equal(written, expected) {
		t.Errorf("written plist differs from the rendered one\n--- got ---\n%s\n--- want ---\n%s", written, expected)
	}

	if _, err := os.Stat(filepath.Dir(agent.logFilePath)); err != nil {
		t.Errorf("log directory was not created: %v", err)
	}

	// No plist existed beforehand, so there was nothing to unload.
	wantCommands := [][]string{{"launchctl", "load", plistFilePath}}
	if !reflect.DeepEqual(runner.commands, wantCommands) {
		t.Errorf("got commands %v, want %v", runner.commands, wantCommands)
	}

	if !strings.Contains(out.String(), plistFilePath) {
		t.Errorf("output %q does not name the plist", out.String())
	}
}

func TestInstallUnloadsAnExistingAgentFirst(t *testing.T) {
	runner := &mockRunner{}
	agent, plistFilePath := newTestAgent(t, runner)

	if err := os.MkdirAll(filepath.Dir(plistFilePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plistFilePath, []byte("an older plist"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := agent.Install(fakeBinaryFilePath, &bytes.Buffer{}); err != nil {
		t.Fatalf("Install: %v", err)
	}

	wantCommands := [][]string{
		{"launchctl", "unload", plistFilePath},
		{"launchctl", "load", plistFilePath},
	}
	if !reflect.DeepEqual(runner.commands, wantCommands) {
		t.Errorf("got commands %v, want %v", runner.commands, wantCommands)
	}

	written, err := os.ReadFile(plistFilePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) == "an older plist" {
		t.Error("the older plist was not replaced")
	}
}

func TestInstallIgnoresAFailedUnload(t *testing.T) {
	runner := &mockRunner{errorsByVerb: map[string]error{"unload": errors.New("not loaded")}}
	agent, plistFilePath := newTestAgent(t, runner)

	if err := os.MkdirAll(filepath.Dir(plistFilePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plistFilePath, []byte("an older plist"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := agent.Install(fakeBinaryFilePath, &bytes.Buffer{}); err != nil {
		t.Errorf("a failed unload of the old agent should not stop the install, got: %v", err)
	}
}

func TestInstallReportsAFailedLoad(t *testing.T) {
	runner := &mockRunner{errorsByVerb: map[string]error{"load": errors.New("exit status 5")}}
	agent, _ := newTestAgent(t, runner)

	err := agent.Install(fakeBinaryFilePath, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected an error when launchctl load fails")
	}
	if !strings.Contains(err.Error(), "launchctl said no") {
		t.Errorf("error %q does not carry launchctl's own output", err)
	}
}

func TestUninstallUnloadsAndRemovesThePlist(t *testing.T) {
	runner := &mockRunner{}
	agent, plistFilePath := newTestAgent(t, runner)
	if err := agent.Install(fakeBinaryFilePath, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	runner.commands = nil

	var out bytes.Buffer
	if err := agent.Uninstall(&out); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}

	if _, err := os.Stat(plistFilePath); !os.IsNotExist(err) {
		t.Errorf("plist still exists after uninstall (stat error: %v)", err)
	}
	wantCommands := [][]string{{"launchctl", "unload", plistFilePath}}
	if !reflect.DeepEqual(runner.commands, wantCommands) {
		t.Errorf("got commands %v, want %v", runner.commands, wantCommands)
	}
}

func TestUninstallRemovesThePlistEvenWhenUnloadFails(t *testing.T) {
	runner := &mockRunner{}
	agent, plistFilePath := newTestAgent(t, runner)
	if err := agent.Install(fakeBinaryFilePath, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	runner.errorsByVerb = map[string]error{"unload": errors.New("not loaded")}

	var out bytes.Buffer
	if err := agent.Uninstall(&out); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}

	if _, err := os.Stat(plistFilePath); !os.IsNotExist(err) {
		t.Errorf("plist still exists after uninstall (stat error: %v)", err)
	}
	if !strings.Contains(out.String(), "warning: launchctl unload failed") {
		t.Errorf("output %q does not warn about the failed unload", out.String())
	}
}

func TestUninstallWithNoAgentDoesNothing(t *testing.T) {
	runner := &mockRunner{}
	agent, _ := newTestAgent(t, runner)

	var out bytes.Buffer
	if err := agent.Uninstall(&out); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}

	if len(runner.commands) != 0 {
		t.Errorf("expected no launchctl calls, got %v", runner.commands)
	}
	if !strings.Contains(out.String(), "nothing to uninstall") {
		t.Errorf("output %q does not say there was nothing to uninstall", out.String())
	}
}

func TestNewUserAgentPointsAtTheUsersLaunchAgents(t *testing.T) {
	homeDirPath, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home directory to compare against: %v", err)
	}

	// Constructing the agent only computes paths; nothing is read or written.
	agent, err := NewUserAgent(&mockRunner{})
	if err != nil {
		t.Fatalf("NewUserAgent: %v", err)
	}

	wantPlistFilePath := filepath.Join(homeDirPath, "Library", "LaunchAgents", "com.yappblocker.plist")
	if agent.plistFilePath != wantPlistFilePath {
		t.Errorf("got plist path %q, want %q", agent.plistFilePath, wantPlistFilePath)
	}
	if filepath.Base(agent.logFilePath) != "yappblocker.log" {
		t.Errorf("got log path %q, want one ending in yappblocker.log", agent.logFilePath)
	}
}
