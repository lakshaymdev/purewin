package optimize

import (
	"context"
	"errors"
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

	// Query service state to determine what action to take.
	stoppable, stopped, queryErr := queryServiceState(name)
	if queryErr != nil {
		return fmt.Errorf("cannot determine service state for %s: %w", name, queryErr)
	}

	// If service is already stopped, just start it.
	if stopped {
		return startService(name)
	}

	// If service is running but NOT_STOPPABLE (e.g., Dnscache), treat as success.
	if !stoppable {
		return nil
	}

	// Enumerate ACTIVE dependent services BEFORE stopping so we can restart them after.
	dependents := getRunningDependentServices(name)

	// Stop dependent services FIRST — sc stop on the parent will fail with
	// ERROR_DEPENDENT_SERVICES_RUNNING (1051) if dependents are still running.
	for _, dep := range dependents {
		depCtx, depCancel := context.WithTimeout(context.Background(), serviceTimeout)
		depCmd := exec.CommandContext(depCtx, "sc", "stop", dep)
		_, _ = depCmd.CombinedOutput() // Best effort — they may already be stopping.
		depCancel()
	}

	// Wait for ALL dependents to actually reach STOPPED state before touching
	// the parent. sc stop returns immediately (just sends the signal), so
	// dependents are typically still in STOP_PENDING at this point.
	if len(dependents) > 0 {
		waitForServicesStopped(dependents, 15*time.Second)
	}

	// Now stop the parent service.
	if err := stopServiceWithRetry(name); err != nil {
		return err
	}

	// Brief pause to let the service fully stop.
	time.Sleep(1 * time.Second)

	// Start the main service first.
	if err := startService(name); err != nil {
		return err
	}

	// Restart only the dependent services that were running before we stopped them.
	for _, dep := range dependents {
		_ = startService(dep) // Best effort — don't fail the whole operation for a dependent.
	}

	return nil
}

// stopServiceWithRetry stops a service via sc stop, handling common error codes:
//   - 1062 (ERROR_SERVICE_NOT_ACTIVE): already stopped, treat as success
//   - 1051 (ERROR_DEPENDENT_SERVICES_RUNNING): dependents still stopping, retry with backoff
func stopServiceWithRetry(name string) error {
	const maxRetries = 3
	delays := []time.Duration{2 * time.Second, 3 * time.Second, 5 * time.Second}

	var lastOutput string
	for attempt := 0; attempt <= maxRetries; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), serviceTimeout)
		cmd := exec.CommandContext(ctx, "sc", "stop", name)
		out, err := cmd.CombinedOutput()
		cancel()

		if err == nil {
			return nil
		}

		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			return fmt.Errorf("failed to stop service %s: %w", name, err)
		}

		code := exitErr.ExitCode()
		switch code {
		case 1062:
			// Already stopped — success.
			return nil
		case 1051:
			// Dependents still stopping — retry after delay if attempts remain.
			lastOutput = strings.TrimSpace(string(out))
			if attempt < maxRetries {
				time.Sleep(delays[attempt])
				continue
			}
			return fmt.Errorf("failed to stop service %s (dependents still running after %d retries): %s", name, maxRetries, lastOutput)
		default:
			return fmt.Errorf("failed to stop service %s: %s", name, strings.TrimSpace(string(out)))
		}
	}

	return fmt.Errorf("failed to stop service %s: %s", name, lastOutput)
}

// waitForServicesStopped polls until all named services reach STOPPED state or timeout.
// Best effort — returns silently if services don't stop in time (caller will
// handle the 1051 retry). Polling interval: 500ms.
func waitForServicesStopped(names []string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	remaining := make(map[string]bool, len(names))
	for _, n := range names {
		remaining[n] = true
	}

	for time.Now().Before(deadline) && len(remaining) > 0 {
		for name := range remaining {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			cmd := exec.CommandContext(ctx, "sc", "query", name)
			out, err := cmd.CombinedOutput()
			cancel()
			if err != nil {
				// Query failed — service may not exist or already stopped.
				delete(remaining, name)
				continue
			}
			if strings.Contains(strings.ToUpper(string(out)), "STOPPED") {
				delete(remaining, name)
			}
		}
		if len(remaining) > 0 {
			time.Sleep(500 * time.Millisecond)
		}
	}
}

// startService starts a single service and handles "already started" as success.
// Uses sc start instead of net start for locale-independent error handling.
func startService(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), serviceTimeout)
	defer cancel()

	// Use "sc start" — its output is always English and exit codes are well-defined.
	cmd := exec.CommandContext(ctx, "sc", "start", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// sc start returns 1056 (ERROR_SERVICE_ALREADY_RUNNING) when already started.
			// This is not an error — the service is in the desired state.
			if exitErr.ExitCode() == 1056 {
				return nil
			}
		}
		return fmt.Errorf("failed to start service %s: %s: %w", name, strings.TrimSpace(string(output)), err)
	}
	return nil
}

// getActiveDependentServices returns the names of ACTIVE services that depend on the given service.
// Active = RUNNING, START_PENDING, or CONTINUE_PENDING (any state that net stop /Y would interrupt).
// Excludes STOPPED services to avoid restarting ones that were intentionally stopped.
func getRunningDependentServices(name string) []string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sc", "enumdepend", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil
	}

	// Parse SERVICE_NAME + STATE pairs from sc enumdepend output.
	// Format:
	//   SERVICE_NAME: SomeDependentService
	//   ...
	//   STATE              : 4  RUNNING
	//
	// Active states to capture: RUNNING (4), START_PENDING (2), CONTINUE_PENDING (5).
	// Skip: STOPPED (1), STOP_PENDING (3), PAUSE_PENDING (6), PAUSED (7).
	var deps []string
	var currentDep string
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "SERVICE_NAME:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				currentDep = strings.TrimSpace(parts[1])
			}
		}
		if strings.HasPrefix(line, "STATE") && currentDep != "" {
			upper := strings.ToUpper(line)
			if strings.Contains(upper, "RUNNING") ||
				strings.Contains(upper, "START_PENDING") ||
				strings.Contains(upper, "CONTINUE_PENDING") {
				deps = append(deps, currentDep)
			}
			currentDep = "" // Reset after processing STATE for this service.
		}
	}
	return deps
}

// queryServiceState queries "sc queryex" to determine:
//   - stoppable: whether the service accepts stop commands
//   - stopped: whether the service is currently stopped
//
// Returns error when the query itself fails.
func queryServiceState(name string) (stoppable bool, stopped bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sc", "queryex", name)
	output, queryErr := cmd.CombinedOutput()
	if queryErr != nil {
		return false, false, fmt.Errorf("failed to query service %s: %w", name, queryErr)
	}

	outStr := strings.ToUpper(string(output))

	// Check if service is stopped.
	if strings.Contains(outStr, "STOPPED") {
		return false, true, nil
	}

	// Check for PENDING states — unsafe to stop/start during transitions.
	if strings.Contains(outStr, "PENDING") {
		return false, false, fmt.Errorf("service %s is in a transitional state, try again later", name)
	}

	// Check stop capability.
	if strings.Contains(outStr, "NOT_STOPPABLE") {
		return false, false, nil
	}
	if strings.Contains(outStr, "STOPPABLE") {
		return true, false, nil
	}

	// Unrecognized state — don't assume stoppable.
	return false, false, fmt.Errorf("unable to determine state for service %s", name)
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
