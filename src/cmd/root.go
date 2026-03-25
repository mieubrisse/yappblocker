package cmd

import (
	"github.com/mieubrisse/yappblocker/cmd/version"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "yappblocker",
	Short: "Kill distracting macOS apps on a schedule",
	Long:  "yappblocker automatically closes specified applications during configured time windows.\nRun 'yappblocker init' to get started.",
}

func init() {
	rootCmd.AddCommand(version.Cmd)
}

func Execute() error {
	return rootCmd.Execute()
}
