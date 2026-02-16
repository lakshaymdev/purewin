package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── Menu Data ───────────────────────────────────────────────────────────────

// MenuItem represents a single entry in the interactive menu.
type MenuItem struct {
	// Title is the display label for this item.
	Title string

	// Description is the secondary help text below the title.
	Description string

	// Key is a unique identifier returned when this item is selected.
	Key string
}

// ─── Menu Model ──────────────────────────────────────────────────────────────

// MenuModel is a Bubbletea model for interactive, keyboard-driven menus.
// Supports arrow keys, Vim bindings (j/k), number keys (1–9), Enter to
// select, and Q/Esc to quit.
type MenuModel struct {
	items    []MenuItem
	cursor   int
	selected string
	quitting bool
	width    int
	height   int
	title    string
}

// NewMenuModel creates a MenuModel from the given items. The first item
// is highlighted by default.
func NewMenuModel(items []MenuItem) MenuModel {
	return MenuModel{
		items:  items,
		cursor: 0,
		width:  80,
		height: 24,
	}
}

// SetTitle sets an optional header displayed above the menu items.
func (m MenuModel) SetTitle(title string) MenuModel {
	m.title = title
	return m
}

// Selected returns the Key of the item the user chose, or "" if they quit.
func (m MenuModel) Selected() string {
	return m.selected
}

// Quitting returns true if the user exited without selecting.
func (m MenuModel) Quitting() bool {
	return m.quitting
}

// ─── Bubbletea Interface ─────────────────────────────────────────────────────

// Init returns the initial command. No startup side-effects needed.
func (m MenuModel) Init() tea.Cmd {
	return nil
}

// Update handles incoming messages and updates the model state.
func (m MenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
				// Wrap to bottom.
				m.cursor = len(m.items) - 1
			}

		// ── Navigate Down ──
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			} else {
				// Wrap to top.
				m.cursor = 0
			}

		// ── Select ──
		case "enter":
			if len(m.items) > 0 {
				m.selected = m.items[m.cursor].Key
				return m, tea.Quit
			}

		// ── Number keys 1–9 for quick select ──
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			idx := int(msg.String()[0]-'0') - 1
			if idx >= 0 && idx < len(m.items) {
				m.cursor = idx
				m.selected = m.items[idx].Key
				return m, tea.Quit
			}
		}
	}

	return m, nil
}

// View renders the menu UI as a string.
func (m MenuModel) View() string {
	if m.quitting && m.selected == "" {
		return ""
	}

	var b strings.Builder

	// ── Title ──
	if m.title != "" {
		titleStyle := lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true).
			MarginBottom(1)
		b.WriteString(titleStyle.Render(m.title))
		b.WriteString("\n\n")
	}

	// ── Items ──
	for i, item := range m.items {
		isActive := i == m.cursor
		number := fmt.Sprintf("%d", i+1)

		if isActive {
			// Active row: block cursor + number + bold title.
			arrow := lipgloss.NewStyle().
				Foreground(ColorHazy).
				Bold(true).
				Render(IconBlock)

			num := lipgloss.NewStyle().
				Foreground(ColorHazy).
				Bold(true).
				Render(number)

			title := lipgloss.NewStyle().
				Foreground(ColorHazy).
				Bold(true).
				Render(item.Title)

			b.WriteString(fmt.Sprintf(" %s %s. %s\n", arrow, num, title))

			// Description on the next line.
			if item.Description != "" {
				desc := MenuDescriptionStyle().Render(item.Description)
				b.WriteString(desc)
				b.WriteByte('\n')
			}
		} else {
			// Inactive row: just number + title in muted tone.
			num := MutedStyle().Render(number)
			title := lipgloss.NewStyle().
				Foreground(ColorText).
				Render(item.Title)

			b.WriteString(fmt.Sprintf("   %s. %s\n", num, title))
		}
	}

	// ── Hint Bar ──
	b.WriteByte('\n')
	hints := HintBarStyle().Render("  ↑↓ Navigate │ Enter Select │ 1-9 Quick Select │ Q Quit")
	b.WriteString(hints)
	b.WriteByte('\n')

	return b.String()
}

// ─── Runner ──────────────────────────────────────────────────────────────────

// RunMenu creates a Bubbletea program, runs the menu, and returns the
// selected MenuItem key. Returns ("", nil) if the user quit without selecting.
func RunMenu(items []MenuItem, title string) (string, error) {
	m := NewMenuModel(items).SetTitle(title)
	p := tea.NewProgram(m, tea.WithAltScreen())

	final, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("menu error: %w", err)
	}

	result := final.(MenuModel)
	return result.Selected(), nil
}
