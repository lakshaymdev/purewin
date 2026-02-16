package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/lakshaymaurya-felt/purewin/internal/core"
	"github.com/lakshaymaurya-felt/purewin/internal/shell"
	"github.com/lakshaymaurya-felt/purewin/internal/ui"
)

var (
	// Global flags
	debug    bool
	dryRun   bool
	runAdmin bool

	// Version info populated from main
	appVersion = "dev"
	appCommit  = "none"
	appDate    = "unknown"
)

// SetVersionInfo sets build-time version information.
func SetVersionInfo(version, commit, date string) {
	appVersion = version
	appCommit = commit
	appDate = date
}

var rootCmd = &cobra.Command{
	Use:   "pw",
	Short: "Deep clean and optimize your Windows",
	Long: `PureWin - Deep clean and optimize your Windows.

All-in-one toolkit for system cleanup, app uninstallation,
disk analysis, system optimization, and live monitoring.`,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Assign Run in init() to break the initialization cycle between
	// rootCmd and runInteractiveMenu (which references rootCmd).
	rootCmd.Run = func(cmd *cobra.Command, args []string) {
		runInteractiveMenu()
	}

	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Show detailed operation logs")
	rootCmd.PersistentFlags().BoolVar(&runAdmin, "admin", false, "Re-launch PureWin with administrator privileges (UAC)")

	// PersistentPreRun: if --admin is set, re-launch elevated and exit.
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if !runAdmin {
			return
		}
		// Already elevated — nothing to do.
		if core.IsElevated() {
			return
		}
		// Build args without --admin to avoid infinite loop.
		var elevatedArgs []string
		for _, a := range os.Args[1:] {
			if a != "--admin" {
				elevatedArgs = append(elevatedArgs, a)
			}
		}
		if err := core.RunElevated(elevatedArgs); err != nil {
			fmt.Fprintf(os.Stderr, "%s %v\n", ui.IconError, err)
			os.Exit(1)
		}
	}

	// Register all subcommands
	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(optimizeCmd)
	rootCmd.AddCommand(analyzeCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(purgeCmd)
	rootCmd.AddCommand(installerCmd)
	rootCmd.AddCommand(completionCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(versionCmd)
}

// runInteractiveShell launches the persistent interactive shell with
// slash-command autocomplete. The shell runs in a loop: each iteration
// runs a bubbletea program; when the user invokes a command, the shell
// exits, the command runs with full terminal control, then the shell
// relaunches with preserved state (output history, command history).
func runInteractiveShell() {
	m := shell.NewShellModel(appVersion)

	// Add welcome output on first launch.
	m.AppendOutput("")

	for {
		p := tea.NewProgram(m)
		finalModel, err := p.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s Shell error: %v\n", ui.IconError, err)
			os.Exit(1)
		}

		result, ok := finalModel.(shell.ShellModel)
		if !ok {
			return
		}

		// User quit the shell entirely.
		if result.Quitting {
			return
		}

		// Command dispatch: run the cobra subcommand with full terminal control.
		if result.ExecCmd != "" {
			cmdArgs := append([]string{result.ExecCmd}, result.ExecArgs...)
			result.AppendOutput("")

			// Run the subcommand via cobra.
			rootCmd.SetArgs(cmdArgs)
			if err := rootCmd.Execute(); err != nil {
				result.AppendOutput("  Command failed: " + err.Error())
			}

			result.AppendOutput("")

			// Clear the exec signal and relaunch shell.
			result.ExecCmd = ""
			result.ExecArgs = nil
		}

		// Preserve state for next iteration.
		m = result
	}
}

// runInteractiveMenu is kept for backward compatibility but now
// launches the interactive shell instead of the old menu.
func runInteractiveMenu() {
	runInteractiveShell()
}
