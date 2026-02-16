package uninstall

import (
	"fmt"

	"github.com/lakshaymaurya-felt/purewin/internal/core"
	"github.com/lakshaymaurya-felt/purewin/internal/ui"
)

// RunBatchUninstall presents a multi-select UI for the given applications,
// confirms the selection, and executes uninstalls with progress feedback.
// In dryRun mode, operations are listed but not executed.
func RunBatchUninstall(apps []InstalledApp, dryRun bool) error {
	if len(apps) == 0 {
		fmt.Println(ui.MutedStyle().Render("  No applications found."))
		return nil
	}

	// 1. Convert to selector items.
	items := make([]ui.SelectorItem, len(apps))
	for i, app := range apps {
		desc := app.Publisher
		if app.Version != "" {
			if desc != "" {
				desc += " • "
			}
			desc += "v" + app.Version
		}

		items[i] = ui.SelectorItem{
			Label:       app.Name,
			Description: desc,
			Size:        formatAppSize(app.EstimatedSize),
		}
	}

	// 2. Run the selector.
	selected, err := ui.RunSelector(items, "Select applications to uninstall")
	if err != nil {
		return fmt.Errorf("selector error: %w", err)
	}
	if len(selected) == 0 {
		fmt.Println(ui.MutedStyle().Render("  No applications selected."))
		return nil
	}

	// 3. Map selected items back to apps.
	selectedApps := mapSelectedApps(apps, selected)

	// 4. Show what was selected.
	fmt.Println()
	fmt.Println(ui.HeaderStyle().Render(
		fmt.Sprintf("  %d application(s) selected for removal:", len(selectedApps))))
	for _, app := range selectedApps {
		sizeStr := ""
		if app.EstimatedSize > 0 {
			sizeStr = " (" + core.FormatSize(app.EstimatedSize) + ")"
		}
		fmt.Printf("  %s %s%s\n", ui.IconBullet, app.Name, sizeStr)
	}
	fmt.Println()

	// 5. Dry-run: report only.
	if dryRun {
		fmt.Println(ui.WarningStyle().Render(
			"  DRY RUN — no applications will be uninstalled."))
		return nil
	}

	// 6. Confirm before executing.
	confirmed, err := ui.DangerConfirm("This will uninstall the selected applications")
	if err != nil {
		return fmt.Errorf("confirmation error: %w", err)
	}
	if !confirmed {
		fmt.Println(ui.MutedStyle().Render("  Cancelled."))
		return nil
	}

	// 7. Execute uninstalls with progress.
	fmt.Println()
	var successes, failures int

	for _, app := range selectedApps {
		spin := ui.NewInlineSpinner()
		spin.Start(fmt.Sprintf("Uninstalling %s...", app.Name))

		uninstErr := UninstallApp(app, false)
		if uninstErr != nil {
			spin.StopWithError(fmt.Sprintf("Failed to uninstall %s: %s", app.Name, uninstErr))
			failures++
		} else {
			spin.Stop(fmt.Sprintf("Uninstalled %s", app.Name))
			successes++
		}
	}

	// 8. Summary.
	fmt.Println()
	fmt.Println(ui.Divider(40))
	if successes > 0 {
		fmt.Println(ui.SuccessStyle().Render(
			fmt.Sprintf("  %s %d application(s) uninstalled successfully", ui.IconSuccess, successes)))
	}
	if failures > 0 {
		fmt.Println(ui.ErrorStyle().Render(
			fmt.Sprintf("  %s %d application(s) failed to uninstall", ui.IconError, failures)))
	}

	return nil
}

// mapSelectedApps maps selected SelectorItems back to InstalledApp entries
// by matching on the Label field.
func mapSelectedApps(apps []InstalledApp, selected []ui.SelectorItem) []InstalledApp {
	selectedSet := make(map[string]bool)
	for _, s := range selected {
		selectedSet[s.Label] = true
	}

	var result []InstalledApp
	for _, app := range apps {
		if selectedSet[app.Name] {
			result = append(result, app)
		}
	}
	return result
}

// formatAppSize returns a human-readable size string for display.
func formatAppSize(bytes int64) string {
	if bytes <= 0 {
		return ""
	}
	return core.FormatSize(bytes)
}
