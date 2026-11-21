package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mieubrisse/yappblocker/internal/config"
	"github.com/mieubrisse/yappblocker/internal/launchd"
)

func TestInitCreatesTheConfigAndInstallsTheAgent(t *testing.T) {
	paths := newTestFilePaths(t)
	agentDirPath := t.TempDir()
	plistFilePath := filepath.Join(agentDirPath, "LaunchAgents", "com.yappblocker.plist")
	machine := newFakeMachine(t)
	agent := launchd.NewAgent(plistFilePath, filepath.Join(agentDirPath, "yappblocker.log"), machine)

	if err := runInit(paths.configFilePath, agent, "/fake/bin/yappblocker"); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	written, err := os.ReadFile(paths.configFilePath)
	if err != nil {
		t.Fatalf("reading created config: %v", err)
	}
	if string(written) != config.DefaultConfigTemplate {
		t.Error("the created config is not the default template")
	}
	if _, err := os.Stat(plistFilePath); err != nil {
		t.Errorf("plist was not written: %v", err)
	}
	wantLaunchctl := [][]string{{"launchctl", "load", plistFilePath}}
	if got := machine.commandsStartingWith("launchctl"); !reflect.DeepEqual(got, wantLaunchctl) {
		t.Errorf("got launchctl commands %v, want %v", got, wantLaunchctl)
	}
}

func TestInitKeepsAnExistingConfig(t *testing.T) {
	paths := newTestFilePaths(t)
	writeTestConfig(t, paths, testConfig)
	agentDirPath := t.TempDir()
	agent := launchd.NewAgent(
		filepath.Join(agentDirPath, "com.yappblocker.plist"),
		filepath.Join(agentDirPath, "yappblocker.log"),
		newFakeMachine(t),
	)

	if err := runInit(paths.configFilePath, agent, "/fake/bin/yappblocker"); err != nil {
		t.Fatalf("runInit: %v", err)
	}

	written, err := os.ReadFile(paths.configFilePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != testConfig {
		t.Error("init overwrote an existing config")
	}
}
