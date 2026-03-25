package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "yappblocker",
	Short: "Kill distracting macOS apps on a schedule",
	Long:  "yappblocker automatically closes specified applications during configured time windows.\nRun 'yappblocker init' to get started.",
}

func Execute() error {
	return rootCmd.Execute()
}
