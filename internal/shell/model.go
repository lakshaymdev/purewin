package shell

import (
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/lakshaymaurya-felt/purewin/internal/core"
)

// ─── Shell Model ─────────────────────────────────────────────────────────────
// The interactive shell is the primary TUI for PureWin. It provides a REPL
// with slash-command autocomplete, command history, and a scrollable output
// area. Commands execute by exiting the shell (tea.Quit), letting the runner
// loop dispatch the command, then relaunching the shell with preserved state.

// ShellModel is the bubbletea Model for the interactive shell.
type ShellModel struct {
	// Input
	textInput textinput.Model

	// Completions (dumb component — methods only, no Update)
	completions *Completions

	// Output history (preserved across shell relaunches)
	OutputLines []string

	// Command history (up/down to recall)
	CmdHistory []string
	historyIdx int    // -1 = not browsing history
	savedInput string // saved input while browsing history

	// Execution signal: set before tea.Quit to tell the runner what to do
	ExecCmd  string   // cobra command name (e.g., "clean")
	ExecArgs []string // additional args (e.g., ["--dry-run"])

	// State
	Quitting  bool
	Width     int
	Height    int
	IsAdmin   bool
	Version   string
	Hostname  string
	scrollPos int // viewport scroll offset (0 = bottom)
}

// NewShellModel creates a fresh shell model.
func NewShellModel(version string) ShellModel {
	ti := textinput.New()
	ti.Placeholder = "Type / for commands..."
	ti.Prompt = "" // We render the prompt ourselves for styling
	ti.CharLimit = 256
	ti.Focus()

	cmds := AllCommands()

	hostname, _ := os.Hostname()

	return ShellModel{
		textInput:   ti,
		completions: NewCompletions(cmds),
		historyIdx:  -1,
		Width:       80,
		Height:      24,
		IsAdmin:     core.IsElevated(),
		Version:     version,
		Hostname:    hostname,
	}
}

// Init returns the initial command.
func (m ShellModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles all messages.
func (m ShellModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Pass to text input for cursor blink etc.
	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

// handleKey processes keyboard input with priority: completions > history > input.
func (m ShellModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// ── Global quit ──
	if key == "ctrl+c" {
		m.Quitting = true
		return m, tea.Quit
	}

	// ── Completions open: route keys there first ──
	if m.completions.IsOpen() {
		switch key {
		case "up":
			m.completions.MoveUp()
			return m, nil
		case "down":
			m.completions.MoveDown()
			return m, nil
		case "tab":
			// Tab accepts the selected completion.
			if sel := m.completions.Selected(); sel != nil {
				m.textInput.SetValue("/" + sel.Name + " ")
				m.textInput.SetCursor(len(m.textInput.Value()))
				m.completions.Close()
			}
			return m, nil
		case "enter":
			// Enter accepts the selected completion and executes.
			if sel := m.completions.Selected(); sel != nil {
				m.textInput.SetValue("/" + sel.Name)
				m.completions.Close()
				return m.executeInput()
			}
			return m, nil
		case "esc":
			m.completions.Close()
			return m, nil
		}

		// Any other key: pass to text input, then re-filter.
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		m.updateCompletions()
		return m, cmd
	}

	// ── Command history navigation ──
	switch key {
	case "up":
		if len(m.CmdHistory) > 0 {
			if m.historyIdx == -1 {
				m.savedInput = m.textInput.Value()
				m.historyIdx = len(m.CmdHistory) - 1
			} else if m.historyIdx > 0 {
				m.historyIdx--
			}
			m.textInput.SetValue(m.CmdHistory[m.historyIdx])
			m.textInput.SetCursor(len(m.textInput.Value()))
		}
		return m, nil

	case "down":
		if m.historyIdx >= 0 {
			if m.historyIdx < len(m.CmdHistory)-1 {
				m.historyIdx++
				m.textInput.SetValue(m.CmdHistory[m.historyIdx])
			} else {
				m.historyIdx = -1
				m.textInput.SetValue(m.savedInput)
			}
			m.textInput.SetCursor(len(m.textInput.Value()))
		}
		return m, nil
	}

	// ── Scroll ──
	switch key {
	case "pgup", "ctrl+u":
		m.scrollUp(10)
		return m, nil
	case "pgdown", "ctrl+d":
		m.scrollDown(10)
		return m, nil
	}

	// ── Submit ──
	if key == "enter" {
		return m.executeInput()
	}

	// ── Esc clears input ──
	if key == "esc" {
		m.textInput.SetValue("")
		m.historyIdx = -1
		return m, nil
	}

	// ── Default: pass to text input ──
	// Reset history browsing when user types a character.
	if m.historyIdx >= 0 {
		m.historyIdx = -1
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)

	// Check if we should open/update completions.
	m.updateCompletions()

	return m, cmd
}

// updateCompletions opens or filters completions based on current input.
func (m *ShellModel) updateCompletions() {
	val := m.textInput.Value()

	if strings.HasPrefix(val, "/") && !strings.Contains(val, " ") {
		// Input starts with / and has no spaces → show completions.
		query := val[1:] // strip leading /
		if !m.completions.IsOpen() {
			m.completions.Open()
		}
		m.completions.Filter(query)
	} else {
		// Not a slash prefix or has spaces (args) → close.
		if m.completions.IsOpen() {
			m.completions.Close()
		}
	}
}

// executeInput parses the current input and dispatches the command.
func (m ShellModel) executeInput() (tea.Model, tea.Cmd) {
	raw := strings.TrimSpace(m.textInput.Value())
	if raw == "" {
		return m, nil
	}

	// Add to history (dedup consecutive, cap at 500).
	if len(m.CmdHistory) == 0 || m.CmdHistory[len(m.CmdHistory)-1] != raw {
		m.CmdHistory = append(m.CmdHistory, raw)
		if len(m.CmdHistory) > 500 {
			m.CmdHistory = m.CmdHistory[1:]
		}
	}
	m.historyIdx = -1

	// Record in output.
	m.AppendOutput("pw \u276f " + raw)

	// Parse slash command.
	if !strings.HasPrefix(raw, "/") {
		m.AppendOutput("  Unknown input. Type / for available commands.")
		m.textInput.SetValue("")
		return m, nil
	}

	parts := strings.Fields(raw[1:]) // strip leading /
	if len(parts) == 0 {
		m.textInput.SetValue("")
		return m, nil
	}

	cmdName := strings.ToLower(parts[0])
	args := parts[1:]

	// Find the command definition.
	var found *CmdDef
	for _, c := range AllCommands() {
		if c.Name == cmdName {
			found = &c
			break
		}
	}

	if found == nil {
		m.AppendOutput("  Unknown command: /" + cmdName + ". Type /help for available commands.")
		m.textInput.SetValue("")
		return m, nil
	}

	// Handle by execution mode.
	switch found.Mode {
	case ExecQuit:
		m.Quitting = true
		return m, tea.Quit

	case ExecInline:
		m.handleInline(cmdName, args)
		m.textInput.SetValue("")
		return m, nil

	case ExecCobra:
		// Signal the runner loop to execute this command.
		m.ExecCmd = cmdName
		m.ExecArgs = args
		m.textInput.SetValue("")
		return m, tea.Quit
	}

	m.textInput.SetValue("")
	return m, nil
}

// handleInline executes commands that don't need to exit the shell.
func (m *ShellModel) handleInline(name string, args []string) {
	switch name {
	case "help":
		if len(args) > 0 {
			m.showCommandHelp(args[0])
		} else {
			m.showHelp()
		}
	case "version":
		m.AppendOutput("  PureWin " + m.Version)
	}
}

// showHelp renders the help listing into output.
func (m *ShellModel) showHelp() {
	m.AppendOutput("")
	m.AppendOutput("  Available commands:")
	m.AppendOutput("")
	for _, cmd := range AllCommands() {
		admin := ""
		if cmd.AdminHint {
			admin = " (admin)"
		}
		m.AppendOutput("    /" + padRight(cmd.Name, 12) + cmd.Description + admin)
	}
	m.AppendOutput("")
	m.AppendOutput("  Type / to see autocomplete suggestions.")
	m.AppendOutput("")
}

// showCommandHelp renders help for a specific command.
func (m *ShellModel) showCommandHelp(name string) {
	for _, cmd := range AllCommands() {
		if cmd.Name == name {
			m.AppendOutput("")
			m.AppendOutput("  /" + cmd.Name + " \u2014 " + cmd.Description)
			m.AppendOutput("  Usage: " + cmd.Usage)
			if cmd.AdminHint {
				m.AppendOutput("  Note: May require administrator privileges.")
			}
			m.AppendOutput("")
			return
		}
	}
	m.AppendOutput("  Unknown command: /" + name)
}

// AppendOutput adds a line to the output history and auto-scrolls to bottom.
// Output is capped at 5000 lines to prevent unbounded memory growth.
func (m *ShellModel) AppendOutput(line string) {
	m.OutputLines = append(m.OutputLines, line)
	if len(m.OutputLines) > 5000 {
		m.OutputLines = m.OutputLines[len(m.OutputLines)-5000:]
		maxScroll := len(m.OutputLines) - m.viewportHeight()
		if maxScroll < 0 {
			maxScroll = 0
		}
		if m.scrollPos > maxScroll {
			m.scrollPos = maxScroll
		}
	}
	m.scrollPos = 0
}

// scrollUp scrolls the viewport up by n lines.
func (m *ShellModel) scrollUp(n int) {
	m.scrollPos += n
	maxScroll := len(m.OutputLines) - m.viewportHeight()
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.scrollPos > maxScroll {
		m.scrollPos = maxScroll
	}
}

// scrollDown scrolls the viewport down by n lines.
func (m *ShellModel) scrollDown(n int) {
	m.scrollPos -= n
	if m.scrollPos < 0 {
		m.scrollPos = 0
	}
}

// viewportHeight returns the number of visible output lines.
func (m *ShellModel) viewportHeight() int {
	// Total height minus: welcome banner (5) + prompt (2) + status bar (1) + padding (2)
	h := m.Height - 10
	if h < 5 {
		h = 5
	}
	return h
}

// padRight pads a string to the given width with spaces.
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
