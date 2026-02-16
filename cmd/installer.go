package cmd

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lakshaymaurya-felt/purewin/internal/core"
	"github.com/lakshaymaurya-felt/purewin/internal/installer"
	"github.com/lakshaymaurya-felt/purewin/internal/ui"
	"github.com/spf13/cobra"
)

var installerCmd = &cobra.Command{
	Use:   "installer",
	Short: "Find and remove installer files",
	Long:  "Scan Downloads, Desktop, and package manager caches for installer files (.exe, .msi, .msix).",
	Run:   runInstaller,
}

func init() {
	installerCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview without deleting")
	installerCmd.Flags().Int("min-age", 0, "Minimum file age in days")
	installerCmd.Flags().String("min-size", "", "Minimum file size (e.g., 10MB)")
}

func runInstaller(cmd *cobra.Command, args []string) {
	// Parse flags
	minAge, _ := cmd.Flags().GetInt("min-age")
	minSizeStr, _ := cmd.Flags().GetString("min-size")

	var minSize int64
	if minSizeStr != "" {
		size, err := parseSize(minSizeStr)
		if err != nil {
			fmt.Printf("%s Invalid size format: %v\n", ui.ErrorStyle().Render(ui.IconError), err)
			fmt.Println(ui.MutedStyle().Render("  Examples: 10MB, 1GB, 500KB"))
			os.Exit(1)
		}
		minSize = size
	}

	// Start scanning
	fmt.Println()
	fmt.Println(ui.SectionHeader("Installer Cleanup", 50))
	fmt.Println()

	spinner := ui.NewInlineSpinner()
	spinner.Start("Scanning for installer files...")

	// Scan for installers
	files, err := installer.ScanInstallers(minAge, minSize)
	if err != nil {
		spinner.StopWithError(fmt.Sprintf("Scan failed: %v", err))
		os.Exit(1)
	}

	spinner.Stop(fmt.Sprintf("Found %d installer files", len(files)))

	if len(files) == 0 {
		fmt.Println()
		fmt.Println(ui.SuccessStyle().Render(fmt.Sprintf("  %s No installer files found!", ui.IconCheck)))
		fmt.Println()
		return
	}

	// Convert to selector items
	items := installerFilesToSelectorItems(files)

	// Show selector
	selected, err := ui.RunSelector(items, "Select installer files to delete:")
	if err != nil {
		fmt.Printf("%s Selector error: %v\n", ui.ErrorStyle().Render(ui.IconError), err)
		os.Exit(1)
	}

	if selected == nil || len(selected) == 0 {
		fmt.Println()
		fmt.Println(ui.MutedStyle().Render("  No files selected. Exiting."))
		fmt.Println()
		return
	}

	// Convert back to installer files
	selectedFiles := make([]installer.InstallerFile, 0, len(selected))
	for _, item := range selected {
		// Find the file by path
		for _, file := range files {
			if file.Path == item.Value {
				selectedFiles = append(selectedFiles, file)
				break
			}
		}
	}

	// Show summary
	fmt.Println()
	totalSize := installer.GetTotalSize(selectedFiles)
	fmt.Printf("  %s\n", ui.BoldStyle().Render(fmt.Sprintf("Will delete %d files (%s)",
		len(selectedFiles), core.FormatSize(totalSize))))
	fmt.Println()

	// Confirm
	if !dryRun {
		confirmed, err := ui.Confirm("Proceed with deletion?")
		if err != nil {
			fmt.Printf("%s Error: %v\n", ui.ErrorStyle().Render(ui.IconError), err)
			os.Exit(1)
		}
		if !confirmed {
			fmt.Println()
			fmt.Println(ui.MutedStyle().Render("  Cancelled."))
			fmt.Println()
			return
		}
	}

	// Delete
	fmt.Println()
	freed, count, cleanErr := installer.CleanInstallers(selectedFiles, dryRun)

	if dryRun {
		fmt.Println()
		fmt.Println(ui.InfoStyle().Render("  [DRY RUN] No files were deleted"))
		fmt.Printf("  Would free: %s from %d files\n", core.FormatSize(freed), count)
		fmt.Println()
	} else {
		fmt.Println()
		if cleanErr != nil {
			fmt.Printf("%s Completed with errors: %v\n", ui.WarningStyle().Render(ui.IconWarning), cleanErr)
		} else {
			fmt.Printf("%s Success!\n", ui.SuccessStyle().Render(ui.IconSuccess))
		}
		fmt.Printf("  Freed: %s from %d files\n", ui.SuccessStyle().Render(core.FormatSize(freed)), count)
		fmt.Println()
	}
}

// installerFilesToSelectorItems converts installer files to selector items.
func installerFilesToSelectorItems(files []installer.InstallerFile) []ui.SelectorItem {
	// Group by source
	sourceGroups := installer.GroupBySource(files)

	// Sort sources
	sources := make([]string, 0, len(sourceGroups))
	for s := range sourceGroups {
		sources = append(sources, s)
	}
	sort.Strings(sources)

	// Build items
	items := make([]ui.SelectorItem, 0, len(files))
	for _, source := range sources {
		group := sourceGroups[source]
		// Sort by size descending
		sort.Slice(group, func(i, j int) bool {
			return group[i].Size > group[j].Size
		})

		for _, file := range group {
			// Age
			age := time.Since(file.ModTime)
			ageStr := formatInstallerAge(age)

			item := ui.SelectorItem{
				Label:       file.Name,
				Description: fmt.Sprintf("%s • %s old", file.Path, ageStr),
				Value:       file.Path,
				Size:        core.FormatSize(file.Size),
				Selected:    true,
				Disabled:    false,
				Category:    source,
			}

			items = append(items, item)
		}
	}

	return items
}

// formatInstallerAge formats age in human-readable format.
func formatInstallerAge(d time.Duration) string {
	if d < 24*time.Hour {
		hours := int(d.Hours())
		if hours == 0 {
			return "less than 1 hour"
		}
		if hours == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", hours)
	}

	days := int(d.Hours() / 24)
	if days == 1 {
		return "1 day"
	}
	if days < 30 {
		return fmt.Sprintf("%d days", days)
	}

	months := days / 30
	if months == 1 {
		return "1 month"
	}
	return fmt.Sprintf("%d months", months)
}

// parseSize parses a size string like "10MB", "1.5GB" to bytes.
func parseSize(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToUpper(s))

	// Extract number and unit
	var numStr string
	var unit string
	for i, r := range s {
		if r >= '0' && r <= '9' || r == '.' {
			numStr += string(r)
		} else {
			unit = s[i:]
			break
		}
	}

	if numStr == "" {
		return 0, fmt.Errorf("no number found in size string")
	}

	num, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number: %w", err)
	}

	multiplier := int64(1)
	switch unit {
	case "B", "":
		multiplier = 1
	case "KB", "K":
		multiplier = 1024
	case "MB", "M":
		multiplier = 1024 * 1024
	case "GB", "G":
		multiplier = 1024 * 1024 * 1024
	case "TB", "T":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unknown unit: %s", unit)
	}

	return int64(num * float64(multiplier)), nil
}
