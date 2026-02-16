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
func RestartService(name string) error {
	if err := core.RequireAdmin("restart service"); err != nil {
		return err
	}

	// Stop the service (ignore error — service may not be running).
	stopCtx, stopCancel := context.WithTimeout(context.Background(), serviceTimeout)
	defer stopCancel()

	stopCmd := exec.CommandContext(stopCtx, "net", "stop", name)
	_, _ = stopCmd.CombinedOutput()

	// Start the service.
	startCtx, startCancel := context.WithTimeout(context.Background(), serviceTimeout)
	defer startCancel()

	startCmd := exec.CommandContext(startCtx, "net", "start", name)
	output, err := startCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start service %s: %s: %w",
			name, strings.TrimSpace(string(output)), err)
	}
	return nil
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
