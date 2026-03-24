package killer

import (
	"testing"

	"github.com/mieubrisse/yappblocker/internal/config"
)

type mockRunner struct {
	commands     [][]string
	pgrepResults map[string]string // match string -> stdout
}

func (m *mockRunner) Run(args []string) (string, error) {
	argsCopy := make([]string, len(args))
	copy(argsCopy, args)
	m.commands = append(m.commands, argsCopy)

	if len(args) >= 3 && args[0] == "pgrep" {
		match := args[2]
		if stdout, ok := m.pgrepResults[match]; ok {
			return stdout, nil
		}
		return "", &ExitError{Code: 1}
	}
	return "", nil
}

func TestKillForceApp(t *testing.T) {
	runner := &mockRunner{
		pgrepResults: map[string]string{
			"SomeApp": "123\n456\n",
		},
	}
	apps := []*config.App{
		{Name: "someapp", Match: "SomeApp", KillType: config.KillTypePkillForce},
	}

	count := FindAndKillApps(apps, false, false, runner)

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
			"MyApp": "789\n",
		},
	}
	apps := []*config.App{
		{Name: "myapp", Match: "MyApp", KillType: config.KillTypeOsascript},
	}

	count := FindAndKillApps(apps, false, false, runner)

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
			"GracefulApp": "321\n",
		},
	}
	apps := []*config.App{
		{Name: "graceful", Match: "GracefulApp", KillType: config.KillTypePkillGraceful},
	}

	count := FindAndKillApps(apps, false, false, runner)

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
			"DryApp": "123\n",
		},
	}
	apps := []*config.App{
		{Name: "dryapp", Match: "DryApp", KillType: config.KillTypePkillForce},
	}

	count := FindAndKillApps(apps, true, false, runner)

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

	count := FindAndKillApps(apps, false, false, runner)

	if count != 0 {
		t.Errorf("expected count 0, got %d", count)
	}
}
