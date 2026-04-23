package cmd

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/mieubrisse/stacktrace"
	"github.com/mieubrisse/yappblocker/internal/config"
	"github.com/mieubrisse/yappblocker/internal/schedule"
	"github.com/spf13/cobra"
)

// errScheduleActive is returned when the tested schedule is currently active.
// It carries no message because the command must be silent in this case.
var errScheduleActive = errors.New("")

var testCmd = &cobra.Command{
	Use:   "test SCHEDULE",
	Short: "Test whether a schedule is currently active",
	Long:  "Exits 0 if the named schedule is not active, exits 1 if it is active. Designed for use in shell hooks and conditionals.",
	Args:  cobra.ExactArgs(1),
	RunE:  executeTest,
}

func init() {
	rootCmd.AddCommand(testCmd)
}

func executeTest(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	err := runTest(args[0])
	if err != nil && !errors.Is(err, errScheduleActive) {
		fmt.Fprintln(os.Stderr, "Error:", err)
	}
	return err
}

func runTest(scheduleName string) error {
	configFilePath := getConfigFilePath()

	data, err := os.ReadFile(configFilePath)
	if err != nil {
		return stacktrace.Propagate(err, "failed to read config file %q", configFilePath)
	}

	cfg, err := config.Load(string(data))
	if err != nil {
		return stacktrace.Propagate(err, "failed to load config")
	}

	sched, ok := cfg.Schedules[scheduleName]
	if !ok {
		return stacktrace.NewError("unknown schedule %q", scheduleName)
	}

	now := time.Now()
	for _, window := range sched.Windows {
		if schedule.IsWindowActive(window, now) {
			return errScheduleActive
		}
	}

	return nil
}
