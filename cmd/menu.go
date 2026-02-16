package cmd

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/lakshaymaurya-felt/winmole/internal/core"
	"github.com/lakshaymaurya-felt/winmole/internal/ui"
)

// ─── Menu Item Definitions ───────────────────────────────────────────────────

// mainMenuItem holds the display metadata and command key for each menu entry.
type mainMenuItem struct {
	icon        string
	label       string
	description string
	command     string
}

// mainMenuItems defines the exact order and content of the interactive menu.
var mainMenuItems = []mainMenuItem{
	{icon: ui.IconTrash, label: "Clean", description: "Deep clean system caches and temp files", command: "clean"},
	{icon: ui.IconFolder, label: "Uninstall", description: "Remove installed applications", command: "uninstall"},
	{icon: ui.IconArrow, label: "Optimize", description: "Speed up Windows with service tuning", command: "optimize"},
	{icon: ui.IconDiamond, label: "Analyze", description: "Explore disk space usage", command: "analyze"},
	{icon: ui.IconDot, label: "Status", description: "Live system health monitor", command: "status"},
	{icon: ui.IconTrash, label: "Purge", description: "Clean project build artifacts", command: "purge"},
	{icon: ui.IconFolder, label: "Installers", description: "Find and remove old installers", command: "installer"},
	{icon: ui.IconPending, label: "Update", description: "Check for WinMole updates", command: "update"},
	{icon: ui.IconCross, label: "Remove", description: "Uninstall WinMole from this system", command: "remove"},
}

// ─── Main Menu Model ─────────────────────────────────────────────────────────

// mainMenuModel is the bubbletea model for the full-screen interactive menu.
type mainMenuModel struct {
	items    []mainMenuItem
	cursor   int
	selected string
	quitting bool
	width    int
	height   int
	isAdmin  bool
}

// newMainMenuModel creates a new main menu model with admin detection.
func newMainMenuModel() mainMenuModel {
	return mainMenuModel{
		items:   mainMenuItems,
		cursor:  0,
		width:   80,
		height:  24,
		isAdmin: core.IsElevated(),
	}
}

// Init returns the initial command (window size request).
func (m mainMenuModel) Init() tea.Cmd {
	return nil
}

// Update handles key presses and window resize events.
func (m mainMenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {

		// ── Quit ──
		case "q", "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		// ── Navigate Up ──
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			} else {
				m.cursor = len(m.items) - 1
			}

		// ── Navigate Down ──
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			} else {
				m.cursor = 0
			}

		// ── Select ──
		case "enter":
			if len(m.items) > 0 {
				m.selected = m.items[m.cursor].command
				return m, tea.Quit
			}

		// ── Number keys 1-9 for quick select ──
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			idx := int(msg.String()[0]-'0') - 1
			if idx >= 0 && idx < len(m.items) {
				m.cursor = idx
				m.selected = m.items[idx].command
				return m, tea.Quit
			}
		}
	}

	return m, nil
}

// View renders the full-screen menu layout.
func (m mainMenuModel) View() string {
	if m.quitting && m.selected == "" {
		return ""
	}

	var b strings.Builder

	// ── Brand Banner ──
	b.WriteString(ui.ShowBrandBanner())
	b.WriteByte('\n')

	// ── Title ──
	b.WriteString(ui.SectionHeader("Choose an action", 50))
	b.WriteString("\n\n")

	// ── Menu Items ──
	for i, item := range m.items {
		isActive := i == m.cursor
		number := fmt.Sprintf("%d", i+1)

		if isActive {
			// Active: ▌ 1. ◆ Clean  Deep clean system caches...
			cursor := lipgloss.NewStyle().Foreground(ui.ColorHazy).Bold(true).Render(ui.IconBlock)
			num := lipgloss.NewStyle().Foreground(ui.ColorHazy).Bold(true).Render(number)
			icon := lipgloss.NewStyle().Foreground(ui.ColorHazy).Render(item.icon)
			label := lipgloss.NewStyle().Foreground(ui.ColorHazy).Bold(true).Render(item.label)
			desc := lipgloss.NewStyle().Foreground(ui.ColorTextDim).Render(item.description)
			b.WriteString(fmt.Sprintf(" %s %s. %s %s  %s\n", cursor, num, icon, label, desc))
		} else {
			// Inactive:   1. ◆ Clean  Deep clean system caches...
			num := ui.MutedStyle().Render(number)
			icon := ui.MutedStyle().Render(item.icon)
			label := lipgloss.NewStyle().Foreground(ui.ColorText).Render(item.label)
			desc := lipgloss.NewStyle().Foreground(ui.ColorMuted).Render(item.description)
			b.WriteString(fmt.Sprintf("   %s. %s %s  %s\n", num, icon, label, desc))
		}
	}

	// ── Hint Bar ──
	b.WriteByte('\n')
	hints := ui.HintBarStyle().Render("↑↓ nav │ enter select │ 1-9 quick │ q quit")
	b.WriteString(hints)
	b.WriteByte('\n')

	// ── Footer: Admin status + Version ──
	b.WriteByte('\n')
	var footerParts []string

	if m.isAdmin {
		adminStyle := lipgloss.NewStyle().Foreground(ui.ColorWarning)
		footerParts = append(footerParts, adminStyle.Render(ui.IconDot+" admin"))
	}

	footerParts = append(footerParts, ui.MutedStyle().Render(fmt.Sprintf("v%s", appVersion)))

	b.WriteString(strings.Join(footerParts, " "+ui.IconPipe+" "))
	b.WriteByte('\n')

	return b.String()
}

// ─── Runner ──────────────────────────────────────────────────────────────────

// runMainMenu launches the bubbletea program in alt-screen mode and returns
// the selected command name. Returns "" if the user quit without selecting.
func runMainMenu() (string, error) {
	m := newMainMenuModel()
	p := tea.NewProgram(m, tea.WithAltScreen())

	final, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("menu error: %w", err)
	}

	result := final.(mainMenuModel)
	if result.quitting && result.selected == "" {
		return "", nil
	}
	return result.selected, nil
}
