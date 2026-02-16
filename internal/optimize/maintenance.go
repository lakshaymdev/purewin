package optimize

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/lakshaymaurya-felt/purewin/internal/core"
)

const (
	// maintenanceTimeout is the maximum time for long-running maintenance tasks.
	maintenanceTimeout = 10 * time.Minute
)

// ─── Public API ──────────────────────────────────────────────────────────────

// RunDISMCleanup runs the DISM component cleanup to free disk space.
func RunDISMCleanup() error {
	if err := core.RequireAdmin("DISM cleanup"); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), maintenanceTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "DISM.exe",
		"/Online", "/Cleanup-Image", "/StartComponentCleanup")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("DISM cleanup failed: %s: %w",
			truncateOutput(output, 300), err)
	}
	return nil
}

// RunSFCCheck runs the System File Checker in verify-only mode.
// It does NOT fix files — only reports integrity status.
func RunSFCCheck() error {
	if err := core.RequireAdmin("SFC check"); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), maintenanceTimeout)
	defer cancel()

	// /verifyonly checks integrity without repairing.
	// We never use /scannow to avoid modifying system files.
	cmd := exec.CommandContext(ctx, "sfc", "/verifyonly")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("SFC check reported issues: %s", truncateOutput(output, 300))
	}
	return nil
}

// RebuildIconCache kills Explorer, deletes the icon cache files, and
// restarts Explorer. This forces Windows to rebuild the icon cache.
func RebuildIconCache() error {
	if err := core.RequireAdmin("rebuild icon cache"); err != nil {
		return err
	}

	// Kill explorer.exe to release icon cache file handles.
	killCtx, killCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer killCancel()

	killCmd := exec.CommandContext(killCtx, "taskkill", "/F", "/IM", "explorer.exe")
	_, _ = killCmd.CombinedOutput() // Best effort.

	// Delete icon cache files.
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData != "" {
		// Modern icon cache: Explorer\iconcache_*.db
		cacheDir := filepath.Join(localAppData, "Microsoft", "Windows", "Explorer")
		pattern := filepath.Join(cacheDir, "iconcache*")
		matches, _ := filepath.Glob(pattern)
		for _, m := range matches {
			_ = os.Remove(m) // Best effort — some may still be locked.
		}

		// Legacy icon cache: IconCache.db
		legacyCache := filepath.Join(localAppData, "IconCache.db")
		_ = os.Remove(legacyCache)
	}

	// Restart explorer.exe.
	startCmd := exec.Command("cmd.exe", "/C", "start", "explorer.exe")
	_ = startCmd.Start() // Fire and forget.

	return nil
}

// RebuildSearchIndex restarts the Windows Search service to trigger a
// search index rebuild.
func RebuildSearchIndex() error {
	return RestartService("WSearch")
}

// ClearEventLogs clears the Application, System, and Security event logs.
func ClearEventLogs() error {
	if err := core.RequireAdmin("clear event logs"); err != nil {
		return err
	}

	logs := []string{"Application", "System", "Security"}
	var errs []string

	for _, logName := range logs {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		cmd := exec.CommandContext(ctx, "wevtutil", "cl", logName)
		if _, err := cmd.CombinedOutput(); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", logName, err))
		}
		cancel()
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to clear some logs: %s", strings.Join(errs, "; "))
	}
	return nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// truncateOutput trims and truncates command output for error messages.
func truncateOutput(output []byte, maxLen int) string {
	s := strings.TrimSpace(string(output))
	if len(s) > maxLen {
		s = s[:maxLen] + "..."
	}
	return s
}
