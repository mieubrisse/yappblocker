package version

import (
	"fmt"

	"github.com/mieubrisse/yappblocker/internal/buildinfo"
	"github.com/spf13/cobra"
)

const CmdStr = "version"

var Cmd = &cobra.Command{
	Use:   CmdStr,
	Short: "Print the version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(buildinfo.Version)
	},
}
