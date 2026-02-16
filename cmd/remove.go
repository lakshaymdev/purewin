package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lakshaymaurya-felt/purewin/internal/config"
	"github.com/lakshaymaurya-felt/purewin/internal/ui"
	"github.com/lakshaymaurya-felt/purewin/internal/update"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove PureWin from system",
	Long:  "Uninstall PureWin, remove configuration files and cached data.",
	Run:   runRemove,
}

func runRemove(cmd *cobra.Command, args []string) {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("%s Failed to load config: %v\n", ui.ErrorStyle().Render(ui.IconError), err)
		os.Exit(1)
	}

	// Get binary path
	exePath, err := os.Executable()
	if err != nil {
		fmt.Printf("%s Failed to get executable path: %v\n", ui.ErrorStyle().Render(ui.IconError), err)
		os.Exit(1)
	}
	exePath, _ = filepath.EvalSymlinks(exePath)

	// Show removal plan
	fmt.Println()
	fmt.Println(ui.SectionHeader("Remove PureWin", 50))
	fmt.Println()
	fmt.Println(ui.WarningStyle().Render("  The following will be removed:"))
	fmt.Println()
	fmt.Printf("    %s Binary: %s\n", ui.IconBullet, ui.MutedStyle().Render(exePath))
	fmt.Printf("    %s Config: %s\n", ui.IconBullet, ui.MutedStyle().Render(cfg.ConfigDir))
	if cfg.CacheDir != cfg.ConfigDir {
		fmt.Printf("    %s Cache:  %s\n", ui.IconBullet, ui.MutedStyle().Render(cfg.CacheDir))
	}
	fmt.Println()

	// Danger confirmation
	confirmed, err := ui.DangerConfirm("This will permanently delete PureWin and all its data")
	if err != nil {
		fmt.Printf("%s Error: %v\n", ui.ErrorStyle().Render(ui.IconError), err)
		os.Exit(1)
	}

	if !confirmed {
		fmt.Println()
		fmt.Println(ui.MutedStyle().Render("  Removal cancelled."))
		fmt.Println()
		return
	}

	// Perform removal
	fmt.Println()
	fmt.Println(ui.MutedStyle().Render("  Removing PureWin..."))
	fmt.Println()

	if err := update.SelfRemove(cfg.ConfigDir, cfg.CacheDir); err != nil {
		fmt.Printf("%s Removal failed: %v\n", ui.ErrorStyle().Render(ui.IconError), err)
		os.Exit(1)
	}

	// Success message (this may not be seen if the process exits quickly)
	fmt.Printf("  %s PureWin has been removed from your system.\n",
		ui.SuccessStyle().Render(ui.IconSuccess))
	fmt.Println()
	fmt.Println(ui.MutedStyle().Render("  Goodbye!"))
	fmt.Println()
}
