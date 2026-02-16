package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── Selector Data ───────────────────────────────────────────────────────────

// SelectorItem represents a toggleable entry in a selection list.
type SelectorItem struct {
	// Label is the display name.
	Label string

	// Description is optional secondary text.
	Description string

	// Value is an opaque machine-use field for reverse-lookup.
	Value string

	// Size is a human-readable size string shown on the right.
	Size string

	// Selected indicates whether this item is currently checked.
	Selected bool

	// Disabled prevents toggling (item is shown but grayed out).
	Disabled bool

	// Category groups items under a heading. Items with the same Category
	// value appear under a shared header.
	Category string

	// sizeBytes is used internally for total-size calculation.
	sizeBytes int64
}

// ─── Selector Model ──────────────────────────────────────────────────────────

// SelectorModel is a Bubbletea model for multi-select checkbox lists with
// pagination, category headers, select-all/none, and live size totals.
type SelectorModel struct {
	items     []SelectorItem
	cursor    int
	page      int
	pageSize  int
	confirmed bool
	quitting  bool
	width     int
	height    int
	title     string
}

// NewSelectorModel creates a SelectorModel from the given items.
// Default page size is 15 items.
func NewSelectorModel(items []SelectorItem) SelectorModel {
	return SelectorModel{
		items:    items,
		cursor:   0,
		page:     0,
		pageSize: 15,
		width:    80,
		height:   24,
	}
}

// SetTitle sets an optional header displayed above the selector.
func (m SelectorModel) SetTitle(title string) SelectorModel {
	m.title = title
	return m
}

// SetPageSize overrides the default number of visible items per page.
func (m SelectorModel) SetPageSize(n int) SelectorModel {
	if n > 0 {
		m.pageSize = n
	}
	return m
}

// GetSelected returns all items currently marked as selected.
func (m SelectorModel) GetSelected() []SelectorItem {
	var result []SelectorItem
	for _, item := range m.items {
		if item.Selected {
			result = append(result, item)
		}
	}
	return result
}

// Confirmed returns true if the user pressed Enter to confirm.
func (m SelectorModel) Confirmed() bool {
	return m.confirmed
}

// Quitting returns true if the user exited without confirming.
func (m SelectorModel) Quitting() bool {
	return m.quitting
}

// ─── Pagination Helpers ──────────────────────────────────────────────────────

func (m SelectorModel) totalPages() int {
	n := len(m.items)
	if n == 0 {
		return 1
	}
	pages := n / m.pageSize
	if n%m.pageSize != 0 {
		pages++
	}
	return pages
}

func (m SelectorModel) pageStart() int {
	return m.page * m.pageSize
}

func (m SelectorModel) pageEnd() int {
	end := m.pageStart() + m.pageSize
	if end > len(m.items) {
		end = len(m.items)
	}
	return end
}

func (m SelectorModel) visibleItems() []SelectorItem {
	return m.items[m.pageStart():m.pageEnd()]
}

// ─── Size Calculation ────────────────────────────────────────────────────────

func (m SelectorModel) selectedCount() int {
	count := 0
	for _, item := range m.items {
		if item.Selected {
			count++
		}
	}
	return count
}

func (m SelectorModel) totalSelectedBytes() int64 {
	var total int64
	for _, item := range m.items {
		if item.Selected {
			total += item.sizeBytes
		}
	}
	return total
}

// ─── Bubbletea Interface ─────────────────────────────────────────────────────

// Init returns the initial command.
func (m SelectorModel) Init() tea.Cmd {
	return nil
}

// Update handles input messages.
func (m SelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Adjust page size to fit terminal, leaving room for header/footer.
		usable := msg.Height - 8
		if usable > 5 {
			m.pageSize = usable
		}
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
				// Page up if cursor moves above current page.
				if m.cursor < m.pageStart() {
					m.page--
				}
			} else {
				// Wrap to last item.
				m.cursor = len(m.items) - 1
				m.page = m.totalPages() - 1
			}

		// ── Navigate Down ──
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
				// Page down if cursor moves below current page.
				if m.cursor >= m.pageEnd() {
					m.page++
				}
			} else {
				// Wrap to first item.
				m.cursor = 0
				m.page = 0
			}

		// ── Page Up ──
		case "pgup", "ctrl+u":
			if m.page > 0 {
				m.page--
				m.cursor = m.pageStart()
			}

		// ── Page Down ──
		case "pgdown", "ctrl+d":
			if m.page < m.totalPages()-1 {
				m.page++
				m.cursor = m.pageStart()
			}

		// ── Toggle Selection ──
		case " ":
			if len(m.items) > 0 && !m.items[m.cursor].Disabled {
				m.items[m.cursor].Selected = !m.items[m.cursor].Selected
			}

		// ── Select All ──
		case "a":
			for i := range m.items {
				if !m.items[i].Disabled {
					m.items[i].Selected = true
				}
			}

		// ── Deselect All ──
		case "n":
			for i := range m.items {
				m.items[i].Selected = false
			}

		// ── Confirm Selection ──
		case "enter":
			m.confirmed = true
			return m, tea.Quit
		}
	}

	return m, nil
}

// View renders the selector UI.
func (m SelectorModel) View() string {
	if m.quitting && !m.confirmed {
		return ""
	}

	var b strings.Builder

	// ── Title ──
	if m.title != "" {
		b.WriteString(HeaderStyle().Render(m.title))
		b.WriteString(Divider(50))
		b.WriteString("\n\n")
	}

	// ── Selection summary (tag-style) ──
	selCount := m.selectedCount()
	totalCount := len(m.items)
	countTag := TagStyle().Render(fmt.Sprintf(" %d/%d ", selCount, totalCount))

	totalBytes := m.totalSelectedBytes()
	var summaryLine string
	if totalBytes > 0 {
		sizeTag := TagAccentStyle().Render(" " + FormatSizePlain(totalBytes) + " ")
		summaryLine = countTag + "  " + sizeTag
	} else {
		summaryLine = countTag
	}

	b.WriteString("  " + summaryLine)
	b.WriteString("\n\n")

	// ── Items ──
	visible := m.visibleItems()
	pageStart := m.pageStart()
	lastCategory := ""

	for i, item := range visible {
		globalIdx := pageStart + i
		isActive := globalIdx == m.cursor

		// Category header (only when category changes).
		if item.Category != "" && item.Category != lastCategory {
			lastCategory = item.Category
			b.WriteString(SectionHeader(item.Category, 50))
			b.WriteByte('\n')
		}

		// Build the line.
		var line strings.Builder

		// Cursor indicator (crush-style thick bar focus).
		if isActive {
			line.WriteString(lipgloss.NewStyle().
				Foreground(ColorBlue).
				Bold(true).
				Render(IconBlock + " "))
		} else {
			line.WriteString("  ")
		}

		// Checkbox.
		if item.Disabled {
			line.WriteString(MutedStyle().Render(IconDash + " "))
		} else if item.Selected {
			line.WriteString(lipgloss.NewStyle().
				Foreground(ColorBlue).
				Bold(true).
				Render(IconRadioOn + " "))
		} else {
			line.WriteString(MutedStyle().Render(IconRadioOff + " "))
		}

		// Label.
		if item.Disabled {
			line.WriteString(MutedStyle().Render(item.Label))
		} else if isActive {
			line.WriteString(lipgloss.NewStyle().
				Foreground(ColorBlue).
				Bold(true).
				Render(item.Label))
		} else if item.Selected {
			line.WriteString(lipgloss.NewStyle().
				Foreground(ColorBlue).
				Render(item.Label))
		} else {
			line.WriteString(lipgloss.NewStyle().
				Foreground(ColorText).
				Render(item.Label))
		}

		// Size on the right.
		if item.Size != "" {
			line.WriteString("  ")
			sizeStyle := MutedStyle()
			if item.Selected && !item.Disabled {
				sizeStyle = lipgloss.NewStyle().Foreground(ColorBlue)
			}
			line.WriteString(sizeStyle.Render(item.Size))
		}

		b.WriteString(line.String())
		b.WriteByte('\n')

		// Description for active item.
		if isActive && item.Description != "" {
			desc := MutedStyle().Italic(true).Render(item.Description)
			b.WriteString("      " + desc)
			b.WriteByte('\n')
		}
	}

	// ── Pagination indicator ──
	totalPages := m.totalPages()
	if totalPages > 1 {
		b.WriteByte('\n')
		pageInfo := fmt.Sprintf("  Page %d/%d", m.page+1, totalPages)
		b.WriteString(MutedStyle().Render(pageInfo))
		b.WriteByte('\n')
	}

	// ── Hint Bar ──
	b.WriteByte('\n')
	var hints []string
	hints = append(hints, "↑↓ nav")
	hints = append(hints, "space toggle")
	hints = append(hints, "a all")
	hints = append(hints, "n none")
	if totalPages > 1 {
		hints = append(hints, "pgup/pgdn pages")
	}
	hints = append(hints, "enter ok")
	hints = append(hints, "q quit")

	hintText := "  " + strings.Join(hints, " "+IconPipe+" ")
	b.WriteString(HintBarStyle().Render(hintText))
	b.WriteByte('\n')

	return b.String()
}

// ─── Runner ──────────────────────────────────────────────────────────────────

// RunSelector creates a Bubbletea program, runs the selector, and returns
// the selected items. Returns (nil, nil) if the user quit without confirming.
func RunSelector(items []SelectorItem, title string) ([]SelectorItem, error) {
	m := NewSelectorModel(items).SetTitle(title)
	p := tea.NewProgram(m, tea.WithAltScreen())

	final, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("selector error: %w", err)
	}

	result := final.(SelectorModel)
	if !result.Confirmed() {
		return nil, nil
	}

	return result.GetSelected(), nil
}
