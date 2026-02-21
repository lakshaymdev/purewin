package clean

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lakshaymaurya-felt/purewin/internal/core"
	"github.com/lakshaymaurya-felt/purewin/internal/ui"
	"github.com/lakshaymaurya-felt/purewin/pkg/whitelist"
)

// ─── System Cache Scanning ───────────────────────────────────────────────────

// getWindowsDir returns the Windows directory from WINDIR or SYSTEMROOT
// environment variables, falling back to C:\Windows if not set.
func getWindowsDir() string {
	if windir := os.Getenv("WINDIR"); windir != "" {
		return windir
	}
	if sysroot := os.Getenv("SYSTEMROOT"); sysroot != "" {
		return sysroot
	}
	return `C:\Windows`
}

// ScanSystemCaches scans system-level caches that require admin privileges.
// Returns nil immediately if the process is not elevated.
func ScanSystemCaches(wl *whitelist.Whitelist) []CleanItem {
	if !core.IsElevated() {
		return nil
	}

	type systemTarget struct {
		name        string
		paths       []string
		description string
	}

	windir := getWindowsDir()

	targets := []systemTarget{
		{
			name:        "WindowsTemp",
			paths:       []string{filepath.Join(windir, "Temp")},
			description: "System temporary files",
		},
		{
			name:        "WUCache",
			paths:       []string{filepath.Join(windir, "SoftwareDistribution", "Download")},
			description: "Windows Update download cache",
		},
		{
			name:        "CBSLogs",
			paths:       []string{filepath.Join(windir, "Logs", "CBS")},
			description: "CBS servicing logs",
		},
		{
			name:        "DISMLogs",
			paths:       []string{filepath.Join(windir, "Logs", "DISM")},
			description: "DISM operation logs",
		},
		{
			name: "WERReports",
			paths: func() []string {
				pd := os.Getenv("ProgramData")
				if pd == "" {
					pd = `C:\ProgramData`
				}
				return []string{
					filepath.Join(pd, "Microsoft", "Windows", "WER", "ReportQueue"),
					filepath.Join(pd, "Microsoft", "Windows", "WER", "Temp"),
				}
			}(),
			description: "Windows Error Reporting",
		},
		{
			name:        "DeliveryOptimization",
			paths:       []string{filepath.Join(windir, "SoftwareDistribution", "DeliveryOptimization")},
			description: "Delivery Optimization cache",
		},
	}

	var items []CleanItem
	for _, t := range targets {
		for _, p := range t.paths {
			if _, err := os.Stat(p); err != nil {
				continue
			}
			if wl != nil && wl.IsWhitelisted(p) {
				continue
			}
			dirItems := scanDirectory(p, "system", t.description, wl)
			items = append(items, dirItems...)
		}
	}

	return items
}

// ─── Memory Dumps ────────────────────────────────────────────────────────────

// ScanMemoryDumps scans for kernel and minidump crash files.
// Returns nil if not elevated.
func ScanMemoryDumps() []CleanItem {
	if !core.IsElevated() {
		return nil
	}

	var items []CleanItem
	windir := getWindowsDir()

	// Full memory dump.
	memDump := filepath.Join(windir, "MEMORY.DMP")
	if info, err := os.Stat(memDump); err == nil {
		items = append(items, CleanItem{
			Path:        memDump,
			Size:        info.Size(),
			Category:    "system",
			Description: "Kernel memory dump",
		})
	}

	// Minidumps.
	minidumpDir := filepath.Join(windir, "Minidump")
	if _, err := os.Stat(minidumpDir); err == nil {
		dirItems := scanDirectory(minidumpDir, "system", "Minidump crash files", nil)
		items = append(items, dirItems...)
	}

	return items
}

// CleanMemoryDumps removes kernel and minidump crash files.
// Returns total bytes freed. Requires admin privileges.
func CleanMemoryDumps(dryRun bool) (int64, error) {
	if !core.IsElevated() {
		return 0, fmt.Errorf("cleaning memory dumps requires administrator privileges")
	}

	var totalFreed int64
	windir := getWindowsDir()

	// Full memory dump.
	memDump := filepath.Join(windir, "MEMORY.DMP")
	freed, err := core.SafeDelete(memDump, dryRun)
	if err == nil {
		totalFreed += freed
	}

	// Minidumps.
	minidumpDir := filepath.Join(windir, "Minidump")
	freed, _, err = core.SafeCleanDir(minidumpDir, "*", dryRun)
	if err == nil {
		totalFreed += freed
	}

	return totalFreed, nil
}

// ─── Windows Update Cache ────────────────────────────────────────────────────

// CleanWindowsUpdate stops the Windows Update service, cleans the download
// cache, and restarts the service. Requires admin privileges.
func CleanWindowsUpdate(dryRun bool) (int64, error) {
	if !core.IsElevated() {
		return 0, fmt.Errorf("cleaning Windows Update cache requires administrator privileges")
	}

	windir := getWindowsDir()
	downloadDir := filepath.Join(windir, "SoftwareDistribution", "Download")

	// Calculate size first.
	size, _ := core.GetDirSize(downloadDir)

	if dryRun {
		return size, nil
	}

	// Stop Windows Update service.
	if err := runServiceCommand("stop", "wuauserv"); err != nil {
		return 0, fmt.Errorf("failed to stop wuauserv: %w", err)
	}

	// Clean the download cache.
	freed, _, cleanErr := core.SafeCleanDir(downloadDir, "*", false)

	// Always restart the service, even if cleaning failed.
	if restartErr := runServiceCommand("start", "wuauserv"); restartErr != nil {
		if cleanErr != nil {
			return 0, fmt.Errorf("clean failed: %w; also failed to restart wuauserv: %v", cleanErr, restartErr)
		}
		return freed, fmt.Errorf("cleaned %s but failed to restart wuauserv: %w",
			core.FormatSize(freed), restartErr)
	}

	if cleanErr != nil {
		return 0, fmt.Errorf("failed to clean WU cache: %w", cleanErr)
	}

	return freed, nil
}

// runServiceCommand executes `net <action> <service>` and returns any error.
func runServiceCommand(action, service string) error {
	cmd := exec.Command("net", action, service)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("net %s %s: %w\n%s", action, service, err, strings.TrimSpace(string(output)))
	}
	return nil
}

// ─── Windows.old ─────────────────────────────────────────────────────────────

// windowsOldDir returns the path to the Windows.old directory.
func windowsOldDir() string {
	sysDrive := os.Getenv("SYSTEMDRIVE")
	if sysDrive == "" {
		sysDrive = "C:"
	}
	return filepath.Join(sysDrive, "Windows.old")
}

// WindowsOldSize returns the size of Windows.old if it exists.
// Returns 0 if not present or not elevated.
func WindowsOldSize() int64 {
	if !core.IsElevated() {
		return 0
	}

	dir := windowsOldDir()
	if _, err := os.Stat(dir); err != nil {
		return 0
	}

	size, err := core.GetDirSize(dir)
	if err != nil {
		return 0
	}
	return size
}

// CleanWindowsOld removes Windows.old after requiring a DangerConfirm
// from the user. This is irreversible. Requires admin privileges.
func CleanWindowsOld(dryRun bool) (int64, error) {
	if !core.IsElevated() {
		return 0, fmt.Errorf("removing Windows.old requires administrator privileges")
	}

	dir := windowsOldDir()
	if _, err := os.Stat(dir); err != nil {
		return 0, nil // Not present.
	}

	size, _ := core.GetDirSize(dir)

	if dryRun {
		return size, nil
	}

	// Require explicit dangerous confirmation.
	confirmed, err := ui.DangerConfirm(fmt.Sprintf(
		"Delete Windows.old (%s)? This is IRREVERSIBLE and removes your ability to roll back.",
		core.FormatSize(size),
	))
	if err != nil || !confirmed {
		return 0, nil // User declined.
	}

	freed, delErr := core.SafeDelete(dir, false)
	if delErr != nil {
		return 0, fmt.Errorf("failed to delete Windows.old: %w", delErr)
	}

	return freed, nil
}

// ─── WER User Reports ────────────────────────────────────────────────────────

// ScanWERUserReports scans Windows Error Reporting directories that are
// accessible without admin (user-level WER paths).
func ScanWERUserReports(wl *whitelist.Whitelist) []CleanItem {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return nil
	}

	werPaths := []string{
		filepath.Join(local, "Microsoft", "Windows", "WER", "ReportArchive"),
		filepath.Join(local, "Microsoft", "Windows", "WER", "ReportQueue"),
	}

	var items []CleanItem
	for _, p := range werPaths {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if wl != nil && wl.IsWhitelisted(p) {
			continue
		}
		dirItems := scanDirectory(p, "system", "Windows Error Reports (user)", wl)
		items = append(items, dirItems...)
	}

	return items
}
