package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lakshaymaurya-felt/purewin/internal/ui"
	"github.com/spf13/cobra"
)

const (
	completionMarkerStart = "# BEGIN PureWin completion"
	completionMarkerEnd   = "# END PureWin completion"
)

var completionCmd = &cobra.Command{
	Use:   "completion",
	Short: "Generate PowerShell tab completion",
	Long:  "Generate or install PowerShell tab completion for PureWin (wm).",
	RunE: func(cmd *cobra.Command, args []string) error {
		install, _ := cmd.Flags().GetBool("install")
		uninstall, _ := cmd.Flags().GetBool("uninstall")

		if uninstall {
			return uninstallCompletion()
		}

		if install {
			return installCompletion()
		}

		// Default: print to stdout
		return printCompletion()
	},
}

func init() {
	completionCmd.Flags().Bool("install", false, "Install completion to PowerShell profile")
	completionCmd.Flags().Bool("uninstall", false, "Remove completion from PowerShell profile")
}

// printCompletion outputs the completion script to stdout
func printCompletion() error {
	return rootCmd.GenPowerShellCompletion(os.Stdout)
}

// installCompletion generates and installs the completion script to the PowerShell profile
func installCompletion() error {
	// Generate completion script to a string
	var buf strings.Builder
	if err := rootCmd.GenPowerShellCompletion(&buf); err != nil {
		return fmt.Errorf("failed to generate completion script: %w", err)
	}

	completionScript := buf.String()

	// Find the PowerShell profile path
	profilePath, err := getPowerShellProfilePath()
	if err != nil {
		return err
	}

	// Ensure the directory exists
	profileDir := filepath.Dir(profilePath)
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		return fmt.Errorf("failed to create PowerShell profile directory: %w", err)
	}

	// Read existing profile content (if it exists)
	var existingContent string
	if data, err := os.ReadFile(profilePath); err == nil {
		existingContent = string(data)
	}

	// Remove any existing PureWin completion block
	existingContent = removeCompletionBlock(existingContent)

	// Append the new completion block
	newContent := existingContent
	if !strings.HasSuffix(newContent, "\n") && newContent != "" {
		newContent += "\n"
	}
	newContent += "\n" + completionMarkerStart + "\n"
	newContent += completionScript
	newContent += completionMarkerEnd + "\n"

	// Write the updated profile
	if err := os.WriteFile(profilePath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write PowerShell profile: %w", err)
	}

	// Success message
	fmt.Println(ui.SuccessStyle().Render(ui.IconSuccess + " PowerShell completion installed successfully!"))
	fmt.Printf("\nProfile location: %s\n", ui.MutedStyle().Render(profilePath))
	fmt.Println("\nTo activate the completion, restart PowerShell or run:")
	fmt.Println(ui.InfoStyle().Render(". $PROFILE"))

	return nil
}

// uninstallCompletion removes the PureWin completion block from the PowerShell profile
func uninstallCompletion() error {
	profilePath, err := getPowerShellProfilePath()
	if err != nil {
		return err
	}

	// Check if profile exists
	data, err := os.ReadFile(profilePath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println(ui.WarningStyle().Render(ui.IconWarning + " PowerShell profile not found. Nothing to uninstall."))
			return nil
		}
		return fmt.Errorf("failed to read PowerShell profile: %w", err)
	}

	existingContent := string(data)

	// Check if PureWin completion block exists
	if !strings.Contains(existingContent, completionMarkerStart) {
		fmt.Println(ui.WarningStyle().Render(ui.IconWarning + " PureWin completion not found in profile."))
		return nil
	}

	// Remove the completion block
	newContent := removeCompletionBlock(existingContent)

	// Write the updated profile
	if err := os.WriteFile(profilePath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("failed to write PowerShell profile: %w", err)
	}

	fmt.Println(ui.SuccessStyle().Render(ui.IconSuccess + " PowerShell completion removed successfully!"))
	fmt.Printf("\nProfile location: %s\n", ui.MutedStyle().Render(profilePath))

	return nil
}

// getPowerShellProfilePath returns the appropriate PowerShell profile path
// Prefers PS 7+ path if it exists, falls back to PS 5.1 path
func getPowerShellProfilePath() (string, error) {
	userProfile := os.Getenv("USERPROFILE")
	if userProfile == "" {
		return "", fmt.Errorf("USERPROFILE environment variable not set")
	}

	// PS 7+ path
	ps7Path := filepath.Join(userProfile, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1")

	// PS 5.1 path
	ps51Path := filepath.Join(userProfile, "Documents", "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1")

	// Check if PS 7+ path exists
	if _, err := os.Stat(ps7Path); err == nil {
		return ps7Path, nil
	}

	// Check if PS 5.1 path exists
	if _, err := os.Stat(ps51Path); err == nil {
		return ps51Path, nil
	}

	// Default to PS 7+ path if neither exists
	return ps7Path, nil
}

// removeCompletionBlock removes the PureWin completion block from content
func removeCompletionBlock(content string) string {
	startIdx := strings.Index(content, completionMarkerStart)
	if startIdx == -1 {
		return content
	}

	endIdx := strings.Index(content, completionMarkerEnd)
	if endIdx == -1 {
		return content
	}

	// Include the end marker in the removal
	endIdx += len(completionMarkerEnd)

	// Remove trailing newline if present
	if endIdx < len(content) && content[endIdx] == '\n' {
		endIdx++
	}

	return content[:startIdx] + content[endIdx:]
}
