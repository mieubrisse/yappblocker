package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/adrg/xdg"
	"github.com/mieubrisse/stacktrace"
	"github.com/mieubrisse/yappblocker/internal/config"
	"github.com/mieubrisse/yappblocker/internal/killer"
	"github.com/mieubrisse/yappblocker/internal/schedule"
	"github.com/spf13/cobra"
)

var (
	dryRun  bool
	verbose bool
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Check schedules and kill blocked apps",
	Long:  "Checks all configured schedules against the current time and kills any apps that should be blocked right now.",
	RunE:  executeRun,
}

func init() {
	runCmd.Flags().BoolVar(&dryRun, "dry-run", false, "print what would be killed without killing")
	runCmd.Flags().BoolVar(&verbose, "verbose", false, "print detailed output")
	rootCmd.AddCommand(runCmd)
}

func getConfigFilePath() string {
	return filepath.Join(xdg.ConfigHome, "yappblocker", "config.yaml")
}

func ensureConfigExists(configFilePath string) error {
	if _, err := os.Stat(configFilePath); err == nil {
		return nil
	}

	dirPath := filepath.Dir(configFilePath)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return stacktrace.Propagate(err, "failed to create config directory %q", dirPath)
	}

	if err := os.WriteFile(configFilePath, []byte(config.DefaultConfigTemplate), 0644); err != nil {
		return stacktrace.Propagate(err, "failed to write default config")
	}

	fmt.Fprintf(os.Stderr, "Created default config at %s\nEdit it to configure your blocked apps and schedules.\n", configFilePath)
	return nil
}

func executeRun(cmd *cobra.Command, args []string) error {
	configFilePath := getConfigFilePath()

	if err := ensureConfigExists(configFilePath); err != nil {
		return err
	}

	data, err := os.ReadFile(configFilePath)
	if err != nil {
		return stacktrace.Propagate(err, "failed to read config file %q", configFilePath)
	}

	cfg, err := config.Load(string(data))
	if err != nil {
		return stacktrace.Propagate(err, "failed to load config")
	}

	now := time.Now()
	runner := &killer.RealRunner{}

	var activeApps []*config.App
	seenApps := make(map[string]bool)

	for schedName, sched := range cfg.Schedules {
		for _, window := range sched.Windows {
			if schedule.IsWindowActive(window, now) {
				resolved, resolveErr := cfg.ResolveAppSet(sched.AppSet)
				if resolveErr != nil {
					return resolveErr
				}
				if verbose {
					names := make([]string, 0, len(resolved))
					for _, a := range resolved {
						names = append(names, a.Name)
					}
					fmt.Fprintf(os.Stderr, "Schedule %q active — apps: %v\n", schedName, names)
				}
				for _, app := range resolved {
					if !seenApps[app.Name] {
						seenApps[app.Name] = true
						activeApps = append(activeApps, app)
					}
				}
				break
			}
		}
	}

	if len(activeApps) == 0 {
		if verbose {
			fmt.Fprintln(os.Stderr, "No active schedules")
		}
		return nil
	}

	killed := killer.FindAndKillApps(activeApps, dryRun, verbose, runner)
	if verbose || dryRun {
		action := "Killed"
		if dryRun {
			action = "Would kill"
		}
		fmt.Fprintf(os.Stderr, "%s %d process(es)\n", action, killed)
	}

	return nil
}
