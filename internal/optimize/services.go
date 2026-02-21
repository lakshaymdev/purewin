package optimize

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/lakshaymaurya-felt/purewin/internal/core"
)

const (
	// serviceTimeout is the maximum time to wait for a service operation.
	serviceTimeout = 30 * time.Second
)

// ManagedService describes a Windows service that PureWin can manage.
type ManagedService struct {
	Name        string
	DisplayName string
}

// GetManagedServices returns the list of services that PureWin can restart.
func GetManagedServices() []ManagedService {
	return []ManagedService{
		{Name: "Dnscache", DisplayName: "DNS Client"},
		{Name: "Dhcp", DisplayName: "DHCP Client"},
		{Name: "WSearch", DisplayName: "Windows Search"},
		{Name: "wuauserv", DisplayName: "Windows Update"},
	}
}

// ─── Public API ──────────────────────────────────────────────────────────────

// FlushDNS clears the DNS resolver cache.
func FlushDNS() error {
	if err := core.RequireAdmin("flush DNS"); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), serviceTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ipconfig", "/flushdns")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to flush DNS: %s: %w",
			strings.TrimSpace(string(output)), err)
	}
	return nil
}

// RestartService stops and then starts a Windows service by name.
// It checks whether the service is stoppable before attempting a restart,
// and uses "net stop /Y" to auto-confirm dependent service stops.
func RestartService(name string) error {
	if err := core.RequireAdmin("restart service"); err != nil {
		return err
	}

	// Check if the service is stoppable via "sc queryex".
	if !isServiceStoppable(name) {
		// Service can't be stopped (e.g., Dnscache is NOT_STOPPABLE).
		// Verify it's running and treat as success — no action needed.
		status, _ := GetServiceStatus(name)
		if strings.Contains(strings.ToUpper(status), "RUNNING") {
			return nil
		}
		return fmt.Errorf("service %s is not stoppable and not running", name)
	}

	// Stop the service with /Y to auto-confirm dependent service shutdowns.
	stopCtx, stopCancel := context.WithTimeout(context.Background(), serviceTimeout)
	defer stopCancel()

	stopCmd := exec.CommandContext(stopCtx, "net", "stop", name, "/Y")
	_, _ = stopCmd.CombinedOutput() // Ignore error — service may not be running.

	// Brief pause to let the service fully stop.
	time.Sleep(1 * time.Second)

	// Start the service.
	startCtx, startCancel := context.WithTimeout(context.Background(), serviceTimeout)
	defer startCancel()

	startCmd := exec.CommandContext(startCtx, "net", "start", name)
	output, err := startCmd.CombinedOutput()
	if err != nil {
		outStr := strings.TrimSpace(string(output))
		// "The requested service has already been started" is not an error.
		if strings.Contains(outStr, "2182") || strings.Contains(strings.ToLower(outStr), "already been started") {
			return nil
		}
		return fmt.Errorf("failed to start service %s: %s: %w", name, outStr, err)
	}
	return nil
}

// isServiceStoppable queries "sc queryex" to check if a service accepts stop commands.
// Returns false for services marked NOT_STOPPABLE (e.g., Dnscache).
func isServiceStoppable(name string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sc", "queryex", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false // Can't query — assume not stoppable (fail safe).
	}

	outStr := strings.ToUpper(string(output))
	// If the output contains NOT_STOPPABLE, the service can't be stopped.
	if strings.Contains(outStr, "NOT_STOPPABLE") {
		return false
	}
	// If it contains STOPPABLE, it can be stopped.
	if strings.Contains(outStr, "STOPPABLE") {
		return true
	}
	// Default: assume stoppable.
	return true
}

// GetServiceStatus queries the current status of a Windows service.
func GetServiceStatus(name string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), serviceTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sc", "query", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to query service %s: %w", name, err)
	}

	// Parse STATE line from sc query output.
	// Format: "        STATE              : 4  RUNNING"
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "STATE") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1]), nil
			}
		}
	}

	return "UNKNOWN", nil
}
