package killer

import (
	"fmt"
	"log"
	"os/exec"
	"strings"

	"github.com/mieubrisse/yappblocker/internal/config"
)

// CommandRunner abstracts command execution for testability.
type CommandRunner interface {
	Run(args []string) (stdout string, err error)
}

// RealRunner executes commands via os/exec.
type RealRunner struct{}

func (r *RealRunner) Run(args []string) (string, error) {
	cmd := exec.Command(args[0], args[1:]...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return string(out), &ExitError{Code: exitErr.ExitCode()}
		}
		return string(out), err
	}
	return string(out), nil
}

// ExitError represents a non-zero exit code from a command.
type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("exit code %d", e.Code)
}

// FindAndKillApps finds running instances of the given apps and kills them.
// Returns the total number of matched PIDs across all apps.
func FindAndKillApps(apps []*config.App, dryRun bool, verbose bool, runner CommandRunner) int {
	totalPIDs := 0

	for _, app := range apps {
		stdout, err := runner.Run([]string{"pgrep", "-f", app.Match})
		if err != nil {
			if verbose {
				log.Printf("No processes found matching %q (%s)", app.Match, app.Name)
			}
			continue
		}

		pids := parsePIDs(stdout)
		pidCount := len(pids)
		if pidCount == 0 {
			continue
		}
		totalPIDs += pidCount

		if dryRun {
			log.Printf("[dry-run] Would kill %d process(es) matching %q (%s)", pidCount, app.Match, app.Name)
			continue
		}

		killArgs := buildKillArgs(app)
		if verbose {
			log.Printf("Killing %d process(es) matching %q (%s) with %v", pidCount, app.Match, app.Name, killArgs)
		}

		if _, err := runner.Run(killArgs); err != nil {
			log.Printf("Warning: failed to kill %q (%s): %v", app.Match, app.Name, err)
		}
	}

	return totalPIDs
}

func parsePIDs(stdout string) []string {
	var pids []string
	for _, line := range strings.Split(stdout, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			pids = append(pids, trimmed)
		}
	}
	return pids
}

func buildKillArgs(app *config.App) []string {
	switch app.KillType {
	case config.KillTypeOsascript:
		return []string{"osascript", "-e", fmt.Sprintf("quit app %q", app.Match)}
	case config.KillTypePkillGraceful:
		return []string{"pkill", "-f", app.Match}
	case config.KillTypePkillForce:
		return []string{"pkill", "-KILL", "-f", app.Match}
	default:
		return []string{"pkill", "-f", app.Match}
	}
}
