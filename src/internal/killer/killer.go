package killer

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/mieubrisse/yappblocker/internal/config"
)

// DefaultKillGracePeriod is how long an app is given to act on a kill before it is
// judged to have ignored it. A graceful quit is not instant, since the app has to
// finish what it is doing first, but this wait sits inside a run that launchd
// repeats every two minutes — so it is the shortest span that does not report a
// merely slow quit as a failed one.
const DefaultKillGracePeriod = 3 * time.Second

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
//
// Kills that report success are checked afterwards, because a reported success is
// not evidence the app died. `osascript -e 'quit app "X"'` exits 0 whether or not
// anything was quit — it does so even for an application that does not exist — so
// its exit code cannot tell a quit that worked from one the app ignored. Taking
// that code at face value let a blocking window pass with every app still on
// screen and not one line written to the log.
func FindAndKillApps(
	apps []*config.App,
	dryRun bool,
	verbose bool,
	runner CommandRunner,
	gracePeriod time.Duration,
) int {
	totalPIDs := 0
	var appsSentAKill []*config.App

	for _, app := range apps {
		pids := findPIDs(app, runner)
		if len(pids) == 0 {
			if verbose {
				log.Printf("No processes found matching %q (%s)", app.Match, app.Name)
			}
			continue
		}
		totalPIDs += len(pids)

		if dryRun {
			log.Printf("[dry-run] Would kill %d process(es) matching %q (%s)", len(pids), app.Match, app.Name)
			continue
		}

		killArgs := buildKillArgs(app)
		if verbose {
			log.Printf("Killing %d process(es) matching %q (%s) with %v", len(pids), app.Match, app.Name, killArgs)
		}

		output, err := runner.Run(killArgs)
		if err != nil {
			// The command's own output is logged alongside the exit code. The kill
			// commands explain themselves on stderr, and that explanation used to
			// be captured and then discarded, leaving "exit code 1" as the entire
			// diagnosis for every failure the tool has ever recorded.
			log.Printf("Warning: failed to kill %q (%s): %v%s", app.Match, app.Name, err, formatOutputForLog(output))
			continue
		}

		appsSentAKill = append(appsSentAKill, app)
	}

	if len(appsSentAKill) > 0 {
		reportAppsThatIgnoredTheirKill(appsSentAKill, runner, gracePeriod)
	}

	return totalPIDs
}

// findPIDs returns the PIDs of every process matching the app. The result is empty
// when the app is not running, because pgrep exits non-zero in that ordinary case.
func findPIDs(app *config.App, runner CommandRunner) []string {
	stdout, err := runner.Run([]string{"pgrep", "-f", buildPgrepPattern(app.Match)})
	if err != nil {
		return nil
	}
	return parsePIDs(stdout)
}

// reportAppsThatIgnoredTheirKill waits out the grace period and then logs every app
// that is still running. These are the silent failures the exit code hides: the
// kill command said it worked, so without this nothing in the tool would ever
// mention them, and a window would appear to have been enforced when it was not.
//
// The wait is taken once for the whole set rather than per app, because each app
// would otherwise add its own grace period to a run that repeats every two minutes.
func reportAppsThatIgnoredTheirKill(apps []*config.App, runner CommandRunner, gracePeriod time.Duration) {
	time.Sleep(gracePeriod)

	for _, app := range apps {
		survivingPIDs := findPIDs(app, runner)
		if len(survivingPIDs) == 0 {
			continue
		}
		log.Printf(
			"Warning: kill of %q (%s) reported success but %d process(es) are still running %v later — the app is ignoring it",
			app.Match, app.Name, len(survivingPIDs), gracePeriod,
		)
	}
}

// formatOutputForLog renders a command's output as a suffix for a log line,
// collapsed onto one line: this log is read with tail, where a wrapped multi-line
// entry is easy to misread as belonging to the line above it.
func formatOutputForLog(output string) string {
	collapsed := strings.Join(strings.Fields(output), " ")
	if collapsed == "" {
		return ""
	}
	return ": " + collapsed
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

// buildPgrepPattern transforms a match string so that pgrep -f won't match its
// own process. It wraps the first character in a bracket character class, e.g.
// "Discord" becomes "[D]iscord". The pgrep argv contains the literal bracket
// form, which does not match itself as a regex.
func buildPgrepPattern(match string) string {
	if len(match) == 0 {
		return match
	}
	return "[" + match[:1] + "]" + match[1:]
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
