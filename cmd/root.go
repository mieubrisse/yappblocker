package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "yappblocker",
	Short: "Kill distracting macOS apps on a schedule",
	Long:  "yappblocker automatically closes specified applications during configured time windows.\nUse 'yappblocker install' to set up automatic execution via launchd.",
}

func Execute() error {
	return rootCmd.Execute()
}
