package cmd

import (
	"os"
	"os/exec"

	"github.com/mieubrisse/stacktrace"
	"github.com/mieubrisse/yappblocker/internal/killer"
	"github.com/mieubrisse/yappblocker/internal/launchd"
	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install launchd agent to run yappblocker automatically",
	Long:  "Creates a launchd plist and loads it so yappblocker runs every 2 minutes.",
	RunE: func(cmd *cobra.Command, args []string) error {
		agent, binaryFilePath, err := getUserAgentAndBinary()
		if err != nil {
			return err
		}
		return agent.Install(binaryFilePath, os.Stderr)
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
}

// getUserAgentAndBinary returns the user's real launchd agent and the yappblocker
// binary on PATH that it should run.
func getUserAgentAndBinary() (*launchd.Agent, string, error) {
	binaryFilePath, err := exec.LookPath("yappblocker")
	if err != nil {
		return nil, "", stacktrace.Propagate(err, "could not find yappblocker in PATH")
	}

	agent, err := launchd.NewUserAgent(&killer.RealRunner{})
	if err != nil {
		return nil, "", err
	}

	return agent, binaryFilePath, nil
}
