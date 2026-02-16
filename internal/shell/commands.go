package shell

// ─── Command Definitions ─────────────────────────────────────────────────────
// Each CmdDef maps a slash command to its display metadata and execution mode.
// The actual execution is handled by the shell runner loop in cmd/root.go.

// ExecMode describes how a command should be executed from the shell.
type ExecMode int

const (
	// ExecCobra exits the shell and runs the command via cobra (default).
	// The command gets full terminal control (its own spinners, selectors, etc.).
	ExecCobra ExecMode = iota

	// ExecInline handles the command inside the shell without exiting.
	ExecInline

	// ExecQuit exits the shell entirely.
	ExecQuit
)

// CmdDef defines a slash command available in the shell.
type CmdDef struct {
	Name        string   // e.g., "clean" (without leading /)
	Description string   // shown in completions popup
	Usage       string   // e.g., "/clean [--dry-run] [--all|--user|--browser|--dev|--system]"
	Mode        ExecMode // how to execute
	AdminHint   bool     // true if the command may need admin privileges
}

// AllCommands returns the full list of available slash commands.
func AllCommands() []CmdDef {
	return []CmdDef{
		{
			Name:        "clean",
			Description: "Deep clean system caches and temp files",
			Usage:       "/clean [--dry-run] [--all|--user|--browser|--dev|--system]",
			Mode:        ExecCobra,
			AdminHint:   true,
		},
		{
			Name:        "uninstall",
			Description: "Remove installed applications",
			Usage:       "/uninstall [--search name] [--quiet]",
			Mode:        ExecCobra,
			AdminHint:   true,
		},
		{
			Name:        "optimize",
			Description: "Speed up Windows with service tuning",
			Usage:       "/optimize [--dry-run] [--services|--maintenance|--startup]",
			Mode:        ExecCobra,
			AdminHint:   true,
		},
		{
			Name:        "analyze",
			Description: "Explore disk space usage",
			Usage:       "/analyze [path]",
			Mode:        ExecCobra,
		},
		{
			Name:        "status",
			Description: "Live system health monitor",
			Usage:       "/status [--json]",
			Mode:        ExecCobra,
		},
		{
			Name:        "purge",
			Description: "Clean project build artifacts",
			Usage:       "/purge [--dry-run] [--min-age days] [--min-size bytes]",
			Mode:        ExecCobra,
		},
		{
			Name:        "installer",
			Description: "Find and remove old installer files",
			Usage:       "/installer [--dry-run] [--min-age days]",
			Mode:        ExecCobra,
		},
		{
			Name:        "update",
			Description: "Check for PureWin updates",
			Usage:       "/update [--force]",
			Mode:        ExecCobra,
		},
		{
			Name:        "version",
			Description: "Show version info",
			Usage:       "/version",
			Mode:        ExecInline,
		},
		{
			Name:        "help",
			Description: "Show available commands",
			Usage:       "/help [command]",
			Mode:        ExecInline,
		},
		{
			Name:        "quit",
			Description: "Exit PureWin",
			Usage:       "/quit",
			Mode:        ExecQuit,
		},
	}
}
