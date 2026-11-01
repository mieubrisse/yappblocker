package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mieubrisse/stacktrace"
	"github.com/mieubrisse/yappblocker/internal/confirm"
	"github.com/mieubrisse/yappblocker/internal/launchd"
	"github.com/mieubrisse/yappblocker/internal/schedule"
	"github.com/spf13/cobra"
)

// errUninstallRefused is returned when the uninstall is turned down on purpose. It
// carries no message because the explanation is printed where the refusal happens,
// as several lines of guidance rather than a one-line error.
var errUninstallRefused = errors.New("")

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove launchd agent",
	Long: "Unloads and removes the launchd plist, stopping automatic execution.\n\n" +
		"Refuses to run while any schedule's window is open, and otherwise requires a long\n" +
		"confirmation phrase to be typed out by hand. Use 'yappblocker pause' if you need\n" +
		"temporary access during a window.",
	RunE: executeUninstall,
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
}

func executeUninstall(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	err := runUninstall(time.Now())
	if err != nil && !errors.Is(err, errUninstallRefused) {
		fmt.Fprintln(os.Stderr, "Error:", err)
	}
	return err
}

func runUninstall(now time.Time) error {
	if err := requireNoBlockingWindowOpen(now, os.Stderr); err != nil {
		return err
	}

	if err := confirm.Prompt(os.Stdin, os.Stderr, confirm.UninstallPhrase); err != nil {
		return err
	}

	return launchd.Uninstall()
}

// requireNoBlockingWindowOpen refuses the uninstall while apps are actively being
// blocked. The impulse to uninstall arrives mid-window, so making the user wait for
// the window to close defeats it far more reliably than any amount of typing does.
func requireNoBlockingWindowOpen(now time.Time, out *os.File) error {
	configFilePath := getConfigFilePath()

	// No config means no schedules, so nothing can be blocked and this is just
	// leftover cleanup.
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		return nil
	}

	cfg, err := loadConfigFile(configFilePath)
	if err != nil {
		// Whether a window is open is unknowable with a broken config, and assuming
		// "not open" would turn a typo in the config into a way around this check.
		return stacktrace.Propagate(err, "refusing to uninstall because the config could not be read, so there is no way to tell whether blocking is active right now")
	}

	active := schedule.FindActiveBlocking(cfg, now)
	if active == nil {
		return nil
	}

	fmt.Fprintf(out, "Blocking is active right now (%s) and lifts at %s.\n",
		strings.Join(active.ScheduleNames, ", "),
		active.EndsAt.Format("15:04 on Mon 2 Jan"),
	)
	fmt.Fprintln(out, "yappblocker does not uninstall inside a blocking window — that moment is the whole reason it exists.")
	fmt.Fprintln(out, "If you need access for a legitimate reason, use 'yappblocker pause <duration>' instead.")

	return errUninstallRefused
}
