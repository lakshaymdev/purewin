package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/lakshaymaurya-felt/purewin/internal/core"
	"github.com/lakshaymaurya-felt/purewin/internal/optimize"
	"github.com/lakshaymaurya-felt/purewin/internal/ui"
)

var optimizeCmd = &cobra.Command{
	Use:   "optimize",
	Short: "Check and maintain system",
	Long:  "Refresh caches, restart services, and optimize system performance.",
	Run:   runOptimize,
}

func init() {
	optimizeCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview optimization actions")
	optimizeCmd.Flags().Bool("whitelist", false, "Manage protected optimization rules")
	optimizeCmd.Flags().Bool("services", false, "Restart system services only")
	optimizeCmd.Flags().Bool("maintenance", false, "Run maintenance tasks only")
	optimizeCmd.Flags().Bool("startup", false, "Manage startup programs only")
}

// optimizeResult tracks the outcome of a single optimization operation.
type optimizeResult struct {
	Name    string
	Success bool
	Error   error
}

func runOptimize(cmd *cobra.Command, args []string) {
	servicesOnly, _ := cmd.Flags().GetBool("services")
	maintenanceOnly, _ := cmd.Flags().GetBool("maintenance")
	startupOnly, _ := cmd.Flags().GetBool("startup")

	// If --startup, show startup items and return.
	if startupOnly {
		optimize.ListStartupItems()
		return
	}

	// Warn about admin privileges for services and maintenance.
	if !core.IsElevated() && !dryRun {
		fmt.Println()
		fmt.Println(ui.WarningStyle().Render(
			fmt.Sprintf("  %s Most optimization tasks require administrator privileges.", ui.IconWarning)))
		fmt.Println(ui.MutedStyle().Render(
			"  → Re-run in an elevated terminal, or use --dry-run to preview."))
	}

	fmt.Println()
	fmt.Println(ui.SectionHeader("System Optimization", 50))
	fmt.Println()

	var results []optimizeResult
	runAll := !servicesOnly && !maintenanceOnly

	// ── Services ──
	if servicesOnly || runAll {
		results = append(results, runServiceOptimizations()...)
	}

	// ── Maintenance ──
	if maintenanceOnly || runAll {
		results = append(results, runMaintenanceOptimizations()...)
	}

	// ── Summary ──
	printOptimizeSummary(results)
}

// runServiceOptimizations executes service-related optimizations.
func runServiceOptimizations() []optimizeResult {
	fmt.Println(ui.SectionHeader("Services", 50))
	fmt.Println()

	var results []optimizeResult

	// DNS flush.
	results = append(results, runOptimizeTask("Flush DNS cache", func() error {
		return optimize.FlushDNS()
	}))

	// Restart managed services.
	for _, svc := range optimize.GetManagedServices() {
		svc := svc // capture for closure
		results = append(results, runOptimizeTask(
			fmt.Sprintf("Restart %s", svc.DisplayName),
			func() error {
				return optimize.RestartService(svc.Name)
			},
		))
	}

	fmt.Println()
	return results
}

// runMaintenanceOptimizations executes maintenance tasks.
func runMaintenanceOptimizations() []optimizeResult {
	fmt.Println(ui.SectionHeader("Maintenance", 50))
	fmt.Println()

	var results []optimizeResult

	results = append(results, runOptimizeTask("DISM component cleanup", func() error {
		return optimize.RunDISMCleanup()
	}))

	results = append(results, runOptimizeTask("System file integrity check", func() error {
		return optimize.RunSFCCheck()
	}))

	results = append(results, runOptimizeTask("Rebuild icon cache", func() error {
		return optimize.RebuildIconCache()
	}))

	results = append(results, runOptimizeTask("Rebuild search index", func() error {
		return optimize.RebuildSearchIndex()
	}))

	results = append(results, runOptimizeTask("Clear event logs", func() error {
		return optimize.ClearEventLogs()
	}))

	fmt.Println()
	return results
}

// runOptimizeTask runs a single optimization task with spinner feedback.
func runOptimizeTask(name string, fn func() error) optimizeResult {
	if dryRun {
		fmt.Printf("  %s %s\n",
			ui.WarningStyle().Render(ui.IconArrow),
			ui.MutedStyle().Render(fmt.Sprintf("[DRY RUN] %s", name)))
		return optimizeResult{Name: name, Success: true}
	}

	spin := ui.NewInlineSpinner()
	spin.Start(name + "...")

	err := fn()
	if err != nil {
		spin.StopWithError(fmt.Sprintf("%s: %s", name, err))
		return optimizeResult{Name: name, Success: false, Error: err}
	}

	spin.Stop(name)
	return optimizeResult{Name: name, Success: true}
}

// printOptimizeSummary displays the final results of all operations.
func printOptimizeSummary(results []optimizeResult) {
	if len(results) == 0 {
		return
	}

	fmt.Println(ui.Divider(40))
	fmt.Println()

	var successes, failures int
	for _, r := range results {
		if r.Success {
			successes++
		} else {
			failures++
		}
	}

	if successes > 0 {
		fmt.Println(ui.SuccessStyle().Render(
			fmt.Sprintf("  %s %d task(s) completed successfully", ui.IconSuccess, successes)))
	}
	if failures > 0 {
		fmt.Println(ui.ErrorStyle().Render(
			fmt.Sprintf("  %s %d task(s) failed", ui.IconError, failures)))
		for _, r := range results {
			if !r.Success {
				fmt.Printf("    %s %s\n",
					ui.ErrorStyle().Render(ui.IconBullet),
					ui.MutedStyle().Render(r.Name))
			}
		}
	}

	fmt.Println()
}
