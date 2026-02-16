package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/lakshaymaurya-felt/purewin/internal/clean"
	"github.com/lakshaymaurya-felt/purewin/internal/config"
	"github.com/lakshaymaurya-felt/purewin/internal/core"
	"github.com/lakshaymaurya-felt/purewin/internal/ui"
	"github.com/lakshaymaurya-felt/purewin/pkg/whitelist"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Free up disk space",
	Long:  "Deep cleanup of caches, logs, temp files, and browser leftovers to reclaim disk space.",
	Run:   runClean,
}

func init() {
	cleanCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview the cleanup plan without deleting")
	cleanCmd.Flags().Bool("whitelist", false, "Manage protected caches")
	cleanCmd.Flags().Bool("all", false, "Clean all categories")
	cleanCmd.Flags().Bool("user", false, "Clean user caches only")
	cleanCmd.Flags().Bool("system", false, "Clean system caches only (requires admin)")
	cleanCmd.Flags().Bool("browser", false, "Clean browser caches only")
	cleanCmd.Flags().Bool("dev", false, "Clean developer tool caches only")
}

// ─── Main Entry Point ────────────────────────────────────────────────────────

func runClean(cmd *cobra.Command, args []string) {
	// Load configuration.
	cfg, err := config.Load()
	if err != nil {
		fmt.Println(ui.ErrorStyle().Render(
			fmt.Sprintf("  %s Failed to load config: %v", ui.IconError, err)))
		os.Exit(1)
	}

	// Override dry-run from config if flag not explicitly set.
	if !cmd.Flags().Changed("dry-run") && cfg.DryRunMode {
		dryRun = true
	}

	// Debug mode.
	debugMode := debug || cfg.DebugMode

	// Load whitelist.
	wlPath := filepath.Join(cfg.ConfigDir, "whitelist.txt")
	wl, wlErr := whitelist.Load(wlPath)
	if wlErr != nil {
		fmt.Println(ui.WarningStyle().Render(
			fmt.Sprintf("  %s Could not load whitelist: %v", ui.IconWarning, wlErr)))
		wl = nil
	}

	// Parse category flags.
	allFlag, _ := cmd.Flags().GetBool("all")
	userFlag, _ := cmd.Flags().GetBool("user")
	systemFlag, _ := cmd.Flags().GetBool("system")
	browserFlag, _ := cmd.Flags().GetBool("browser")
	devFlag, _ := cmd.Flags().GetBool("dev")

	// Default to all if no category specified.
	if !allFlag && !userFlag && !systemFlag && !browserFlag && !devFlag {
		allFlag = true
	}

	isAdmin := core.IsElevated()

	// ── Header ───────────────────────────────────────────────────────────
	fmt.Println()
	fmt.Println(ui.SectionHeader("Deep Clean", 55))

	if dryRun {
		fmt.Println(ui.WarningStyle().Render(
			fmt.Sprintf("  %s  DRY RUN MODE — no files will be deleted", ui.IconWarning)))
	}
	if !isAdmin && (allFlag || systemFlag) {
		fmt.Println(ui.WarningStyle().Render(
			fmt.Sprintf("  %s  Not running as admin — system items will be skipped", ui.IconWarning)))
	}
	fmt.Println()

	// ── Scan Phase ───────────────────────────────────────────────────────
	spinner := ui.NewInlineSpinner()
	spinner.Start("Scanning for cleanable files...")

	var allResults []clean.ScanResult

	// User caches: use config targets via ScanAll.
	if allFlag || userFlag {
		userTargets := config.GetTargetsByCategory("user")
		userResults := clean.ScanAll(userTargets, wl, isAdmin)
		allResults = append(allResults, userResults...)
	}

	// Browser caches: use specialized multi-profile scanner.
	if allFlag || browserFlag {
		browserItems := clean.ScanBrowserCaches(wl)
		if len(browserItems) > 0 {
			browserGroups := groupItemsByDescription(browserItems)
			for name, items := range browserGroups {
				allResults = append(allResults, clean.ItemsToResult(name, items))
			}
		}
	}

	// Developer caches: use specialized scanner for safety.
	if allFlag || devFlag {
		devItems := clean.ScanDevCaches(wl)
		if len(devItems) > 0 {
			devGroups := groupItemsByDescription(devItems)
			for name, items := range devGroups {
				allResults = append(allResults, clean.ItemsToResult(name, items))
			}
		}
	}

	// System caches: use config targets via ScanAll (admin-gated).
	if allFlag || systemFlag {
		systemTargets := config.GetTargetsByCategory("system")
		systemResults := clean.ScanAll(systemTargets, wl, isAdmin)
		allResults = append(allResults, systemResults...)

		// Memory dumps (separate scan).
		dumpItems := clean.ScanMemoryDumps()
		if len(dumpItems) > 0 {
			allResults = append(allResults, clean.ItemsToResult("MemoryDumps", dumpItems))
		}

		// WER user-level reports (no admin needed).
		werItems := clean.ScanWERUserReports(wl)
		if len(werItems) > 0 {
			allResults = append(allResults, clean.ItemsToResult("WER User Reports", werItems))
		}
	}

	// Recycle Bin (user category, via Shell API).
	var recycleBinSize int64
	if allFlag || userFlag {
		recycleBinSize, _ = clean.ScanRecycleBin()
	}

	// Go module cache size.
	var goModSize int64
	if allFlag || devFlag {
		goModSize = clean.GoModCacheSize()
	}

	// Windows.old size.
	var windowsOldSize int64
	if (allFlag || systemFlag) && isAdmin {
		windowsOldSize = clean.WindowsOldSize()
	}

	spinner.Stop("Scan complete")

	// ── Calculate Totals ─────────────────────────────────────────────────
	totalSize := clean.TotalSizeAll(allResults) + recycleBinSize + goModSize + windowsOldSize
	totalItems := clean.TotalItemCount(allResults)

	if totalSize == 0 {
		fmt.Println()
		fmt.Println(ui.SuccessStyle().Render(
			fmt.Sprintf("  %s  System is clean! Nothing to remove.", ui.IconSuccess)))
		fmt.Println()
		return
	}

	// ── Display Results ──────────────────────────────────────────────────
	displayCleanResults(allResults, recycleBinSize, goModSize, windowsOldSize)

	fmt.Println(ui.Divider(55))
	fmt.Printf("  %-35s %s  %s\n",
		ui.BoldStyle().Render("Total"),
		ui.FormatSize(totalSize),
		ui.MutedStyle().Render(fmt.Sprintf("(%d items)", totalItems)),
	)
	fmt.Println()

	// ── Dry Run: Export and Exit ─────────────────────────────────────────
	if dryRun {
		drc := core.NewDryRunContext()
		for _, r := range allResults {
			for _, item := range r.Items {
				drc.Add(item.Path, item.Size, item.Category)
			}
		}
		if recycleBinSize > 0 {
			drc.Add("Recycle Bin (Shell API)", recycleBinSize, "user")
		}
		if goModSize > 0 {
			drc.Add("Go module cache", goModSize, "dev")
		}
		if windowsOldSize > 0 {
			drc.Add(`C:\Windows.old`, windowsOldSize, "system")
		}

		drc.PrintSummary()

		exportPath := filepath.Join(cfg.ConfigDir, "clean-list.txt")
		if exportErr := drc.ExportToFile(exportPath); exportErr != nil {
			fmt.Println(ui.WarningStyle().Render(
				fmt.Sprintf("  %s  Could not export: %v", ui.IconWarning, exportErr)))
		} else {
			fmt.Println(ui.MutedStyle().Render(
				fmt.Sprintf("  Report saved to %s", exportPath)))
		}
		fmt.Println()
		return
	}

	// ── Confirm ──────────────────────────────────────────────────────────
	confirmed, confirmErr := ui.Confirm(
		fmt.Sprintf("  Proceed to free %s?", core.FormatSize(totalSize)))
	if confirmErr != nil || !confirmed {
		fmt.Println(ui.MutedStyle().Render("  Cleanup cancelled."))
		fmt.Println()
		return
	}

	// ── Initialize Logger ────────────────────────────────────────────────
	logger, logErr := core.NewLogger(cfg.LogFile)
	if logErr != nil {
		if debugMode {
			fmt.Println(ui.WarningStyle().Render(
				fmt.Sprintf("  %s  Logging unavailable: %v", ui.IconWarning, logErr)))
		}
		logger = nil
	} else {
		defer logger.Close()
		logger.LogSession("clean")
	}

	// ── Execute Cleanup ──────────────────────────────────────────────────
	cleanSpinner := ui.NewInlineSpinner()
	cleanSpinner.Start("Cleaning...")

	var totalFreed int64
	var totalCleaned int
	var errCount int

	// Delete all scanned items via SafeDelete.
	for _, r := range allResults {
		for _, item := range r.Items {
			cleanSpinner.UpdateMessage(
				fmt.Sprintf("Cleaning %s...", filepath.Base(item.Path)))

			freed, delErr := core.SafeDelete(item.Path, false)
			if delErr != nil {
				errCount++
				if debugMode {
					fmt.Printf("\n  %s %v\n", ui.IconError, delErr)
				}
				if logger != nil {
					logger.Log("DELETE", item.Path, 0, delErr)
				}
				continue
			}

			totalFreed += freed
			totalCleaned++
			if logger != nil {
				logger.Log("DELETE", item.Path, freed, nil)
			}
		}
	}

	// Empty Recycle Bin.
	if recycleBinSize > 0 {
		cleanSpinner.UpdateMessage("Emptying Recycle Bin...")
		if rbErr := clean.EmptyRecycleBin(false); rbErr != nil {
			errCount++
			if logger != nil {
				logger.Log("EMPTY_RECYCLE_BIN", "RecycleBin", 0, rbErr)
			}
		} else {
			totalFreed += recycleBinSize
			totalCleaned++
			if logger != nil {
				logger.Log("EMPTY_RECYCLE_BIN", "RecycleBin", recycleBinSize, nil)
			}
		}
	}

	// Go module cache.
	if goModSize > 0 {
		cleanSpinner.UpdateMessage("Cleaning Go module cache...")
		freed, goErr := clean.CleanGoModCache(false)
		if goErr != nil {
			errCount++
			if logger != nil {
				logger.Log("GO_CLEAN_MODCACHE", "go mod cache", 0, goErr)
			}
		} else {
			totalFreed += freed
			totalCleaned++
			if logger != nil {
				logger.Log("GO_CLEAN_MODCACHE", "go mod cache", freed, nil)
			}
		}
	}

	// Windows.old (requires DangerConfirm inside CleanWindowsOld).
	if windowsOldSize > 0 {
		cleanSpinner.Stop("Pausing for confirmation...")

		freed, woErr := clean.CleanWindowsOld(false)
		if woErr != nil {
			errCount++
			if logger != nil {
				logger.Log("DELETE_WINDOWS_OLD", `C:\Windows.old`, 0, woErr)
			}
		} else if freed > 0 {
			totalFreed += freed
			totalCleaned++
			if logger != nil {
				logger.Log("DELETE_WINDOWS_OLD", `C:\Windows.old`, freed, nil)
			}
		}

		// Restart spinner for remaining work.
		cleanSpinner = ui.NewInlineSpinner()
		cleanSpinner.Start("Finishing cleanup...")
	}

	cleanSpinner.Stop("Cleanup complete")

	// Log session summary.
	if logger != nil {
		logger.LogSummary(totalFreed, totalCleaned, errCount)
	}

	// ── Completion Banner ────────────────────────────────────────────────
	fmt.Println()
	fmt.Println(ui.Divider(55))
	fmt.Println()

	successBanner := lipgloss.NewStyle().
		Foreground(ui.ColorSuccess).
		Bold(true)

	fmt.Println(successBanner.Render(
		fmt.Sprintf("  %s  Freed %s across %d items",
			ui.IconSuccess, core.FormatSize(totalFreed), totalCleaned)))

	if errCount > 0 {
		fmt.Println(ui.WarningStyle().Render(
			fmt.Sprintf("  %s  %d items skipped (locked or access denied)",
				ui.IconWarning, errCount)))
	}
	fmt.Println()
}

// ─── Display Helpers ─────────────────────────────────────────────────────────

// displayCleanResults prints scan results grouped by high-level category.
func displayCleanResults(
	results []clean.ScanResult,
	recycleBinSize, goModSize, windowsOldSize int64,
) {
	groups := clean.GroupByCategory(results)

	type categoryDef struct {
		key   string
		label string
	}

	categories := []categoryDef{
		{"user", "User Caches"},
		{"browser", "Browser Caches"},
		{"dev", "Developer Tools"},
		{"system", "System"},
	}

	fmt.Println()

	for _, cat := range categories {
		groupResults, hasGroup := groups[cat.key]

		// Check if this category has extra line items to show.
		hasExtra := false
		switch cat.key {
		case "user":
			hasExtra = recycleBinSize > 0
		case "dev":
			hasExtra = goModSize > 0 || clean.IsDockerAvailable()
		case "system":
			hasExtra = windowsOldSize > 0
		}

		if !hasGroup && !hasExtra {
			continue
		}

		// Category header.
		fmt.Println(ui.SectionHeader(cat.label, 55))

		// Sort results within category for stable output.
		if hasGroup {
			sort.Slice(groupResults, func(i, j int) bool {
				return groupResults[i].Category < groupResults[j].Category
			})

			for _, r := range groupResults {
				fmt.Printf("    %-31s  %10s  %s\n",
					r.Category,
					ui.FormatSize(r.TotalSize),
					ui.MutedStyle().Render(fmt.Sprintf("(%d items)", r.ItemCount)),
				)
			}
		}

		// Extra line items per category.
		switch cat.key {
		case "user":
			if recycleBinSize > 0 {
				fmt.Printf("    %-31s  %10s\n",
					"Recycle Bin",
					ui.FormatSize(recycleBinSize),
				)
			}
		case "dev":
			if goModSize > 0 {
				fmt.Printf("    %-31s  %10s  %s\n",
					"Go module cache",
					ui.FormatSize(goModSize),
					ui.MutedStyle().Render("(go clean -modcache)"),
				)
			}
			if clean.IsDockerAvailable() {
				fmt.Printf("    %-31s  %10s  %s\n",
					"Docker build cache",
					ui.MutedStyle().Render("   ?"),
					ui.MutedStyle().Render("(docker builder prune)"),
				)
			}
		case "system":
			if windowsOldSize > 0 {
				fmt.Printf("    %-31s  %10s  %s\n",
					"Windows.old",
					ui.FormatSize(windowsOldSize),
					ui.WarningStyle().Render("(requires confirmation)"),
				)
			}
		}

		fmt.Println()
	}
}

// groupItemsByDescription groups CleanItems by their Description field.
func groupItemsByDescription(items []clean.CleanItem) map[string][]clean.CleanItem {
	groups := make(map[string][]clean.CleanItem)
	for _, item := range items {
		groups[item.Description] = append(groups[item.Description], item)
	}
	return groups
}
