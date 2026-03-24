package cmd

import (
	"github.com/mieubrisse/yappblocker/internal/launchd"
	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove launchd agent",
	Long:  "Unloads and removes the launchd plist, stopping automatic execution.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return launchd.Uninstall()
	},
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
}
