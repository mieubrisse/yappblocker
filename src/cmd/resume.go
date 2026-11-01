package cmd

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mieubrisse/stacktrace"
	"github.com/mieubrisse/yappblocker/internal/pause"
	"github.com/spf13/cobra"
)

var resumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "End a pause early and turn blocking back on",
	Long: "Ends the current pause immediately.\n\n" +
		"Requires no confirmation phrase — turning blocking back on is never the thing\n" +
		"that needs discouraging. It gives you no allowance back either: the 3-hour pause\n" +
		"window runs on wall clock from your first pause, so stopping early is a kindness\n" +
		"to yourself rather than a way to save up pause time.",
	RunE: executeResume,
}

func init() {
	rootCmd.AddCommand(resumeCmd)
}

func executeResume(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	err := runResume(time.Now(), os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
	}
	return err
}

func runResume(now time.Time, out io.Writer) error {
	stateFilePath := getPauseStateFilePath()

	state, err := pause.Load(stateFilePath)
	if err != nil {
		return stacktrace.Propagate(err, "could not read the pause state at %s", stateFilePath)
	}

	if !pause.IsPaused(state, now) {
		fmt.Fprintln(out, "Blocking is not paused, so there is nothing to resume.")
		printRemainingAllowance(out, state, now)
		return nil
	}

	if err := pause.Save(stateFilePath, pause.Resume(state, now)); err != nil {
		return err
	}

	fmt.Fprintln(out, "Blocking is back on.")

	return nil
}
