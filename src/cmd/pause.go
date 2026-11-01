package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/adrg/xdg"
	"github.com/mieubrisse/yappblocker/internal/confirm"
	"github.com/mieubrisse/yappblocker/internal/pause"
	"github.com/spf13/cobra"
)

// pauseTimeFormat is how a pause boundary is shown to the user: the time of day,
// plus a date for anything that lands on another day.
const pauseTimeFormat = "15:04 on Mon 2 Jan"

// errPauseRefused is returned when a pause is turned down on purpose. It carries no
// message because the explanation is printed where the refusal happens.
var errPauseRefused = errors.New("")

var pauseCmd = &cobra.Command{
	Use:   "pause [DURATION]",
	Short: "Temporarily suspend blocking",
	Long: "Suspends blocking for a bounded stretch of time, after which it resumes on its own.\n\n" +
		"Taking a pause opens a 3-hour window during which you may pause again, and a\n" +
		"24-hour window after which pausing becomes possible again. The 3 hours are wall\n" +
		"clock from your first pause, not a budget of paused minutes, so splitting a pause\n" +
		"into several short ones only loses you time. 3 hours is the most you can have.\n\n" +
		"Run with no duration to see whether blocking is paused and how much is left.\n\n" +
		"Durations are written like 45m, 1h30m, or 2h.",
	Args: cobra.MaximumNArgs(1),
	RunE: executePause,
}

func init() {
	rootCmd.AddCommand(pauseCmd)
}

func executePause(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	err := runPause(args, time.Now, os.Stderr)
	if err != nil && !errors.Is(err, errPauseRefused) {
		fmt.Fprintln(os.Stderr, "Error:", err)
	}
	return err
}

// runPause takes a clock rather than a single timestamp because typing the
// confirmation phrase takes real time — at least 13 seconds, by design — and the
// pause has to be measured from when it was confirmed, not from when the command
// was typed. Someone who leaves the prompt open for twenty minutes should not get a
// pause that is twenty minutes shorter than they asked for, or one already expired.
func runPause(args []string, nowFunc func() time.Time, out io.Writer) error {
	now := nowFunc()
	stateFilePath := getPauseStateFilePath()

	state, err := pause.Load(stateFilePath)
	if err != nil {
		return fmt.Errorf("refusing to pause because the existing pause state could not be read, so delete that file to start over: %w", err)
	}

	if len(args) == 0 {
		printPauseStatus(out, state, now)
		return nil
	}

	requestedDuration, err := time.ParseDuration(args[0])
	if err != nil {
		fmt.Fprintf(out, "Could not read %q as a length of time. Write it like 45m, 1h30m, or 2h.\n", args[0])
		return errPauseRefused
	}
	if requestedDuration <= 0 {
		fmt.Fprintf(out, "A pause has to be longer than zero, and %v is not.\n", requestedDuration)
		return errPauseRefused
	}

	// Checked before the phrase is asked for, so a refusal never costs the typing.
	allowance := pause.AllowanceRemaining(state, now)
	if allowance <= 0 {
		fmt.Fprintf(out, "Your 3-hour pause window has closed, so pausing is not available until %s.\n",
			pause.CooldownEndsAt(state).Format(pauseTimeFormat))
		fmt.Fprintln(out, "That is the point: pauses are meant to be exceptional, and you have had this day's.")
		return errPauseRefused
	}
	if requestedDuration > allowance {
		// Worded to hold whether this is a first pause capped at the 3-hour maximum
		// or a later one capped by what is left of an open window.
		fmt.Fprintf(out, "You asked for %s, but the most you can pause right now is %s.\n",
			formatDuration(requestedDuration), formatDuration(allowance))
		return errPauseRefused
	}

	if pause.IsPaused(state, now) {
		proposedEnd := now.Add(requestedDuration)
		if proposedEnd.Before(state.PausedUntil) {
			fmt.Fprintf(out, "Blocking is already paused until %s, and pausing for %s would cut that short, to %s.\n",
				state.PausedUntil.Format(pauseTimeFormat), formatDuration(requestedDuration), proposedEnd.Format(pauseTimeFormat))
			fmt.Fprintln(out, "Ask for a longer pause to extend it, or run 'yappblocker resume' to end it now.")
			return errPauseRefused
		}
	}

	if err := confirm.Prompt(os.Stdin, out, confirm.PausePhrase); err != nil {
		return err
	}

	// The clock is read again here, because the pause runs from the moment it was
	// confirmed rather than from whenever the command was first typed.
	confirmedAt := nowFunc()

	newState, err := pause.Begin(state, requestedDuration, confirmedAt)
	if err != nil {
		fmt.Fprintf(out, "Too much time passed while you were typing, and %s no longer fits in your pause window.\n",
			formatDuration(requestedDuration))
		printRemainingAllowance(out, state, confirmedAt)
		return errPauseRefused
	}

	if err := pause.Save(stateFilePath, newState); err != nil {
		return err
	}

	fmt.Fprintf(out, "\nBlocking is paused until %s.\n", newState.PausedUntil.Format(pauseTimeFormat))
	printAllowanceAfterPauseEnds(out, newState)
	fmt.Fprintln(out, "Run 'yappblocker resume' to turn blocking back on sooner.")

	return nil
}

func printPauseStatus(out io.Writer, state *pause.State, now time.Time) {
	if pause.IsPaused(state, now) {
		fmt.Fprintf(out, "Blocking is paused until %s, which is %s from now.\n",
			state.PausedUntil.Format(pauseTimeFormat),
			formatDuration(state.PausedUntil.Sub(now)),
		)
	} else {
		fmt.Fprintln(out, "Blocking is not paused.")
	}

	printRemainingAllowance(out, state, now)
}

// printAllowanceAfterPauseEnds reports what will be left once the pause just taken
// runs out, which is the figure someone wants after taking one. Said in the future
// tense, because saying it in the present would describe a moment that has not
// arrived — right now the pause itself is still using the window up.
func printAllowanceAfterPauseEnds(out io.Writer, state *pause.State) {
	allowance := pause.AllowanceRemaining(state, state.PausedUntil)
	if allowance <= 0 {
		fmt.Fprintf(out, "That uses up your window, so there will be no more pausing until %s.\n",
			pause.CooldownEndsAt(state).Format(pauseTimeFormat))
		return
	}

	fmt.Fprintf(out, "Once it ends, you will be able to pause again for up to %s.\n", formatDuration(allowance))
}

// printRemainingAllowance says how much more pausing is available as of the given
// moment.
func printRemainingAllowance(out io.Writer, state *pause.State, asOf time.Time) {
	allowance := pause.AllowanceRemaining(state, asOf)
	if allowance <= 0 {
		fmt.Fprintf(out, "No more pausing until %s.\n", pause.CooldownEndsAt(state).Format(pauseTimeFormat))
		return
	}

	fmt.Fprintf(out, "You can pause for up to %s.\n", formatDuration(allowance))
}

// formatDuration renders a length of time the way a person would say it, since Go's
// own formatting produces things like "3h0m0s" that nobody wants to read.
func formatDuration(duration time.Duration) string {
	rounded := duration.Round(time.Minute)
	if rounded < time.Minute {
		return "less than a minute"
	}

	hours := int(rounded / time.Hour)
	minutes := int((rounded % time.Hour) / time.Minute)

	if hours == 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	if minutes == 0 {
		return fmt.Sprintf("%dh", hours)
	}

	return fmt.Sprintf("%dh%dm", hours, minutes)
}

func getPauseStateFilePath() string {
	return filepath.Join(xdg.StateHome, "yappblocker", "pause.json")
}

// isBlockingPaused reports whether a pause is in force. An unreadable state file
// counts as "not paused" so that damaging the file cannot switch blocking off, and
// says so rather than passing over it in silence.
func isBlockingPaused(now time.Time, out io.Writer) bool {
	state, err := pause.Load(getPauseStateFilePath())
	if err != nil {
		fmt.Fprintf(out, "Warning: the pause state could not be read, so blocking stays on: %v\n", err)
		return false
	}

	return pause.IsPaused(state, now)
}
