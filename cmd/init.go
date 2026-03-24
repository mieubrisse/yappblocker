package cmd

import (
	"fmt"
	"os"

	"github.com/mieubrisse/yappblocker/internal/launchd"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Set up yappblocker for the first time",
	Long:  "Creates the default config file, installs the launchd agent, and walks you through next steps.",
	RunE:  executeInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func executeInit(cmd *cobra.Command, args []string) error {
	configFilePath := getConfigFilePath()

	// Step 1: Create config file
	fmt.Fprintln(os.Stderr, "✅ Step 1/2: Config file")
	if err := ensureConfigExists(configFilePath); err != nil {
		return err
	}
	if _, err := os.Stat(configFilePath); err == nil {
		fmt.Fprintf(os.Stderr, "   Config ready at %s\n", configFilePath)
	}

	// Step 2: Install launchd agent
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "✅ Step 2/2: Launchd agent")
	if err := launchd.Install(); err != nil {
		return err
	}

	// Next steps
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "🎉 yappblocker is installed and running!")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintf(os.Stderr, "📝 Next: edit your config to add apps and schedules:\n")
	fmt.Fprintf(os.Stderr, "   %s\n", configFilePath)
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "🔍 Test your config without killing anything:")
	fmt.Fprintln(os.Stderr, "   yappblocker run --dry-run --verbose")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "🗑️  To uninstall later:")
	fmt.Fprintln(os.Stderr, "   yappblocker uninstall")

	return nil
}
