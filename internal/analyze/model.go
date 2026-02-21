package analyze

import (
	"os/exec"
	"path/filepath"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lakshaymaurya-felt/purewin/internal/core"
)

// ─── Messages ────────────────────────────────────────────────────────────────

type deleteResultMsg struct {
	path  string
	freed int64
	err   error
}

func deleteEntry(entry *DirEntry) tea.Cmd {
	return func() tea.Msg {
		freed, err := core.SafeDelete(entry.Path, false)
		return deleteResultMsg{path: entry.Path, freed: freed, err: err}
	}
}

// ─── Model ───────────────────────────────────────────────────────────────────

// AnalyzeModel is the bubbletea Model for the disk analyzer TUI.
type AnalyzeModel struct {
	root          *DirEntry
	current       *DirEntry   // directory being displayed
	cursor        int         // selected item index
	breadcrumb    []*DirEntry // navigation history stack
	width         int
	height        int
	offset        int  // viewport scroll offset
	largeOnly     bool // filter: show only >100MB
	confirmDelete bool // two-key delete: Backspace then Enter
	quitting      bool
	err           error
	maxDepth      int   // 0 = unlimited
	minSize       int64 // 0 = show all
}

// NewAnalyzeModel creates an AnalyzeModel rooted at the given scan result.
func NewAnalyzeModel(root *DirEntry, maxDepth int, minSize int64) AnalyzeModel {
	return AnalyzeModel{
		root:     root,
		current:  root,
		width:    80,
		height:   24,
		maxDepth: maxDepth,
		minSize:  minSize,
	}
}

func (m AnalyzeModel) Init() tea.Cmd {
	return nil
}

func (m AnalyzeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// If awaiting delete confirmation, only Enter confirms.
		if m.confirmDelete {
			if msg.String() == "enter" {
				m.confirmDelete = false
				items := m.visibleItems()
				if m.cursor >= 0 && m.cursor < len(items) {
					return m, deleteEntry(items[m.cursor])
				}
			}
			m.confirmDelete = false
			return m, nil
		}

		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.ensureVisible()
			}

		case "down", "j":
			items := m.visibleItems()
			if m.cursor < len(items)-1 {
				m.cursor++
				m.ensureVisible()
			}

		case "right", "l":
			// Drill into a directory.
			items := m.visibleItems()
			if m.cursor >= 0 && m.cursor < len(items) {
				entry := items[m.cursor]
				if entry.IsDir && len(entry.Children) > 0 {
					m.breadcrumb = append(m.breadcrumb, m.current)
					m.current = entry
					m.cursor = 0
					m.offset = 0
				}
			}

		case "enter":
			// Open file/folder location in Explorer.
			items := m.visibleItems()
			if m.cursor >= 0 && m.cursor < len(items) {
				openInExplorer(items[m.cursor].Path)
			}

		case "left", "h":
			// Go up to parent directory.
			if len(m.breadcrumb) > 0 {
				m.current = m.breadcrumb[len(m.breadcrumb)-1]
				m.breadcrumb = m.breadcrumb[:len(m.breadcrumb)-1]
				m.cursor = 0
				m.offset = 0
			}

		case "backspace":
			// First key of two-key delete confirmation.
			items := m.visibleItems()
			if m.cursor >= 0 && m.cursor < len(items) {
				m.confirmDelete = true
			}

		case "L":
			m.largeOnly = !m.largeOnly
			m.cursor = 0
			m.offset = 0
		}

		return m, nil

	case deleteResultMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.removeEntry(msg.path)
		}
		return m, nil
	}

	return m, nil
}

// View delegates to view.go renderView.
func (m AnalyzeModel) View() string {
	return m.renderView()
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func (m *AnalyzeModel) ensureVisible() {
	vh := m.viewportHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+vh {
		m.offset = m.cursor - vh + 1
	}
}

func (m *AnalyzeModel) viewportHeight() int {
	h := m.height - 8 // header (4) + footer (3) + padding
	if h < 1 {
		h = 1
	}
	return h
}

// visibleItems returns the children of the current directory, optionally
// filtered to only entries ≥100 MiB.
func (m AnalyzeModel) visibleItems() []*DirEntry {
	if m.current == nil {
		return nil
	}

	// Calculate current depth from root.
	var currentDepth int
	if m.maxDepth > 0 {
		currentDepth = m.currentDepth()
	}

	var out []*DirEntry
	for _, c := range m.current.Children {
		// Filter by minimum size.
		if m.minSize > 0 && c.Size < m.minSize {
			continue
		}
		// Filter by size threshold (L key toggle).
		if m.largeOnly && c.Size < 100*1024*1024 {
			continue
		}
		// Filter by depth: hide directory children beyond maxDepth.
		if m.maxDepth > 0 && c.IsDir && currentDepth >= m.maxDepth {
			continue
		}
		out = append(out, c)
	}
	return out
}

// removeEntry deletes an entry from the current Children slice and
// recalculates the parent size.
func (m *AnalyzeModel) removeEntry(path string) {
	if m.current == nil {
		return
	}
	for i, c := range m.current.Children {
		if c.Path == path {
			m.current.Children = append(m.current.Children[:i], m.current.Children[i+1:]...)
			// Recalculate current directory size.
			var total int64
			for _, child := range m.current.Children {
				total += child.Size
			}
			m.current.Size = total
			if m.cursor >= len(m.current.Children) && m.cursor > 0 {
				m.cursor--
			}
			return
		}
	}
}

// currentDepth returns how many levels deep the current directory is from root.
func (m AnalyzeModel) currentDepth() int {
	return len(m.breadcrumb)
}

// openInExplorer opens the parent folder of a path with the item selected.
func openInExplorer(path string) {
	if runtime.GOOS == "windows" {
		dir := filepath.Dir(path)
		_ = exec.Command("explorer", "/select,", dir).Start()
	}
}
