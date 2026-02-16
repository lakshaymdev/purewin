package cmd

import (
	"fmt"
	"os"

	"github.com/lakshaymaurya-felt/purewin/internal/config"
	"github.com/lakshaymaurya-felt/purewin/internal/ui"
	"github.com/lakshaymaurya-felt/purewin/internal/update"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update PureWin",
	Long:  "Check for and install the latest version of PureWin from GitHub releases.",
	Run:   runUpdate,
}

func init() {
	updateCmd.Flags().Bool("force", false, "Force reinstall latest version")
}

func runUpdate(cmd *cobra.Command, args []string) {
	force, _ := cmd.Flags().GetBool("force")

	// Load config
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("%s Failed to load config: %v\n", ui.ErrorStyle().Render(ui.IconError), err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println(ui.SectionHeader("Update", 50))
	fmt.Println()

	// Show current version
	fmt.Printf("  Current version: %s\n", ui.InfoStyle().Render(appVersion))
	fmt.Println()

	// Check for updates
	spinner := ui.NewInlineSpinner()
	spinner.Start("Checking for updates...")

	latestVersion, downloadURL, err := update.CheckForUpdate(appVersion)
	if err != nil {
		spinner.StopWithError(fmt.Sprintf("Update check failed: %v", err))
		os.Exit(1)
	}

	spinner.Stop("Update check complete")

	// Compare versions
	if !force && !update.IsNewerVersion(appVersion, latestVersion) {
		fmt.Println()
		fmt.Printf("  %s You're already running the latest version!\n",
			ui.SuccessStyle().Render(ui.IconSuccess))
		fmt.Println()
		return
	}

	// Show version info
	fmt.Println()
	fmt.Printf("  Latest version: %s\n", ui.SuccessStyle().Render(latestVersion))
	if force && latestVersion == appVersion {
		fmt.Printf("  %s Force reinstalling current version\n",
			ui.WarningStyle().Render(ui.IconWarning))
	}
	fmt.Println()

	// Confirm update
	confirmed, err := ui.Confirm("Download and install update?")
	if err != nil {
		fmt.Printf("%s Error: %v\n", ui.ErrorStyle().Render(ui.IconError), err)
		os.Exit(1)
	}
	if !confirmed {
		fmt.Println()
		fmt.Println(ui.MutedStyle().Render("  Update cancelled."))
		fmt.Println()
		return
	}

	// Download update
	fmt.Println()
	spinner = ui.NewInlineSpinner()
	spinner.Start("Downloading update...")

	tempPath, err := update.DownloadUpdate(downloadURL)
	if err != nil {
		spinner.StopWithError(fmt.Sprintf("Download failed: %v", err))
		os.Exit(1)
	}

	spinner.Stop("Download complete")

	// Apply update
	spinner = ui.NewInlineSpinner()
	spinner.Start("Installing update...")

	if err := update.ApplyUpdate(tempPath); err != nil {
		spinner.StopWithError(fmt.Sprintf("Installation failed: %v", err))
		// Clean up temp file
		_ = os.Remove(tempPath)
		os.Exit(1)
	}

	// Clean up temp file
	_ = os.Remove(tempPath)

	spinner.Stop("Update installed successfully")

	// Success message
	fmt.Println()
	fmt.Printf("  %s PureWin has been updated to version %s\n",
		ui.SuccessStyle().Render(ui.IconSuccess),
		ui.SuccessStyle().Render(latestVersion))
	fmt.Println()
	fmt.Println(ui.MutedStyle().Render("  Restart PureWin to use the new version."))
	fmt.Println()

	// Update the background check cache
	update.CheckForUpdateBackground(latestVersion, cfg.CacheDir)
}
