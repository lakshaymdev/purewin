package optimize

import (
	"fmt"

	"golang.org/x/sys/windows/registry"

	"github.com/lakshaymaurya-felt/purewin/internal/ui"
)

// StartupItem represents an application configured to run at startup.
type StartupItem struct {
	Name     string
	Command  string
	Location string
	Enabled  bool
	Source   string // "Registry" or "TaskScheduler"
}

// ─── Registry Sources ────────────────────────────────────────────────────────

// startupRegistrySource describes a registry path containing Run entries.
type startupRegistrySource struct {
	root         registry.Key
	path         string
	approvedPath string
	label        string
}

// startupSources defines the registry Run keys to scan.
var startupSources = []startupRegistrySource{
	{
		root:         registry.CURRENT_USER,
		path:         `Software\Microsoft\Windows\CurrentVersion\Run`,
		approvedPath: `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run`,
		label:        `HKCU\...\Run`,
	},
	{
		root:         registry.LOCAL_MACHINE,
		path:         `Software\Microsoft\Windows\CurrentVersion\Run`,
		approvedPath: `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run`,
		label:        `HKLM\...\Run`,
	},
}

// ─── Public API ──────────────────────────────────────────────────────────────

// GetStartupItems reads startup entries from registry Run keys.
func GetStartupItems() ([]StartupItem, error) {
	var items []StartupItem

	for _, src := range startupSources {
		found, err := readStartupFromRegistry(src)
		if err != nil {
			// Key may not exist or access denied; skip silently.
			continue
		}
		items = append(items, found...)
	}

	return items, nil
}

// ToggleStartupItem enables or disables a startup entry by modifying
// the StartupApproved registry key. Only works for registry-based items.
func ToggleStartupItem(item StartupItem, enable bool) error {
	if item.Source != "Registry" {
		return fmt.Errorf("toggle is only supported for registry-based startup items")
	}

	// Find the matching source to locate the approved path.
	for _, src := range startupSources {
		if src.label != item.Location {
			continue
		}

		key, err := registry.OpenKey(src.root, src.approvedPath,
			registry.QUERY_VALUE|registry.SET_VALUE)
		if err != nil {
			return fmt.Errorf("cannot open StartupApproved key: %w", err)
		}
		defer key.Close()

		// Read existing value or create a new 12-byte blob.
		data, _, dataErr := key.GetBinaryValue(item.Name)
		if dataErr != nil || len(data) < 12 {
			data = make([]byte, 12)
		}

		// Byte[0]: 0x02 = enabled, 0x03 = disabled.
		if enable {
			data[0] = 0x02
		} else {
			data[0] = 0x03
		}

		if err := key.SetBinaryValue(item.Name, data); err != nil {
			return fmt.Errorf("cannot update StartupApproved for %s: %w", item.Name, err)
		}
		return nil
	}

	return fmt.Errorf("startup item location %q not recognized", item.Location)
}

// ListStartupItems displays a formatted list of all startup items.
func ListStartupItems() {
	items, err := GetStartupItems()
	if err != nil {
		fmt.Println(ui.ErrorStyle().Render(
			fmt.Sprintf("  %s Failed to read startup items: %s", ui.IconError, err)))
		return
	}

	if len(items) == 0 {
		fmt.Println(ui.MutedStyle().Render("  No startup items found."))
		return
	}

	fmt.Println()
	fmt.Println(ui.HeaderStyle().Render("  Startup Programs"))
	fmt.Println()

	for _, item := range items {
		var status string
		if item.Enabled {
			status = ui.SuccessStyle().Bold(true).Render(ui.IconSelected + " Enabled ")
		} else {
			status = ui.MutedStyle().Render(ui.IconUnselected + " Disabled")
		}

		name := ui.BoldStyle().Render(item.Name)
		loc := ui.MutedStyle().Render(item.Location)

		fmt.Printf("  %s  %-30s  %s\n", status, name, loc)

		// Show command on the next line, truncated for readability.
		cmd := item.Command
		if len(cmd) > 70 {
			cmd = cmd[:67] + "..."
		}
		fmt.Printf("         %s\n", ui.MutedStyle().Render(cmd))
	}

	fmt.Println()
	enabled := countEnabled(items)
	fmt.Printf("  %s\n", ui.MutedStyle().Render(
		fmt.Sprintf("%d startup items (%d enabled)", len(items), enabled)))
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// readStartupFromRegistry reads Run key values and checks StartupApproved
// status for each entry.
func readStartupFromRegistry(src startupRegistrySource) ([]StartupItem, error) {
	key, err := registry.OpenKey(src.root, src.path, registry.QUERY_VALUE)
	if err != nil {
		return nil, err
	}
	defer key.Close()

	names, err := key.ReadValueNames(-1)
	if err != nil {
		return nil, err
	}

	// Read the StartupApproved key for enabled/disabled status.
	approvedStatus := readApprovedStatus(src.root, src.approvedPath)

	var items []StartupItem
	for _, name := range names {
		val, _, valErr := key.GetStringValue(name)
		if valErr != nil {
			continue
		}

		enabled := true
		if status, ok := approvedStatus[name]; ok {
			enabled = status
		}

		items = append(items, StartupItem{
			Name:     name,
			Command:  val,
			Location: src.label,
			Enabled:  enabled,
			Source:   "Registry",
		})
	}

	return items, nil
}

// readApprovedStatus reads the StartupApproved registry key to determine
// which startup entries are enabled or disabled.
// Returns a map of value name → enabled status.
func readApprovedStatus(root registry.Key, path string) map[string]bool {
	result := make(map[string]bool)

	key, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
	if err != nil {
		return result
	}
	defer key.Close()

	names, err := key.ReadValueNames(-1)
	if err != nil {
		return result
	}

	for _, name := range names {
		data, _, dataErr := key.GetBinaryValue(name)
		if dataErr != nil || len(data) < 1 {
			continue
		}
		// Byte[0]: 0x02 or 0x06 = enabled, 0x03 = disabled.
		result[name] = data[0] == 0x02 || data[0] == 0x06
	}

	return result
}

// countEnabled returns the number of enabled startup items.
func countEnabled(items []StartupItem) int {
	count := 0
	for _, item := range items {
		if item.Enabled {
			count++
		}
	}
	return count
}
