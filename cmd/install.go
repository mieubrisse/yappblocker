package cmd

import (
	"github.com/mieubrisse/yappblocker/internal/launchd"
	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install launchd agent to run yappblocker automatically",
	Long:  "Creates a launchd plist and loads it so yappblocker runs every 2 minutes.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return launchd.Install()
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
}
