package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show installed version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("PureWin version %s\n", appVersion)
		fmt.Printf("Commit: %s\n", appCommit)
		fmt.Printf("Built: %s\n", appDate)
		fmt.Printf("Go: %s\n", runtime.Version())
		fmt.Printf("OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	},
}
