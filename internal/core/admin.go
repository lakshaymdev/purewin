package core

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

// IsElevated returns true if the current process is running with
// administrator privileges.
func IsElevated() bool {
	token := windows.GetCurrentProcessToken()
	return token.IsElevated()
}

// RequireAdmin returns an error if the current process is not elevated.
// The operation parameter is included in the error message for context.
func RequireAdmin(operation string) error {
	if IsElevated() {
		return nil
	}
	return fmt.Errorf(
		"operation %q requires administrator privileges\n"+
			"  → Re-run with: pw %s --admin\n"+
			"  → Or right-click Terminal → Run as Administrator",
		operation, operation,
	)
}

// RunElevated re-launches the current process with administrator privileges
// via the Windows ShellExecuteW "runas" verb. This triggers a UAC prompt.
// The current process exits after launching the elevated one.
// The args parameter should contain the command-line arguments to pass
// (excluding the --admin flag itself to avoid an infinite re-launch loop).
func RunElevated(args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine executable path: %w", err)
	}

	// Convert exe path and args to UTF16 for ShellExecuteW.
	exeUTF16, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return fmt.Errorf("invalid executable path: %w", err)
	}

	argStr := strings.Join(args, " ")
	argsUTF16, err := windows.UTF16PtrFromString(argStr)
	if err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}

	verbUTF16, _ := windows.UTF16PtrFromString("runas")

	// ShellExecuteW with "runas" triggers UAC. Returns error if ret <= 32.
	err = windows.ShellExecute(0, verbUTF16, exeUTF16, argsUTF16, nil, windows.SW_SHOWNORMAL)
	if err != nil {
		return fmt.Errorf("UAC elevation failed: %w", err)
	}

	// Elevated process launched successfully — exit the current one.
	os.Exit(0)
	return nil // unreachable
}
