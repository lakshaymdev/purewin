package shell

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/lakshaymaurya-felt/winmole/internal/ui"
)

// ─── Vercel-Inspired Cappuccino Shell Palette ────────────────────────────────
// Extends the global palette with shell-specific accent styles.
// Designed to be competitive with charmbracelet/crush aesthetics.

var (
	// Accent: dusty mauve — primary interactive elements.
	accent = lipgloss.AdaptiveColor{Light: "#8c6f7e", Dark: "#b89aab"}

	// Accent alt: deep periwinkle — secondary highlights (darkened for ≈4.5:1 contrast with cream).
	accentAlt = lipgloss.AdaptiveColor{Light: "#7a7899", Dark: "#6b6990"}

	// Dim: warm gray — chrome, borders, secondary text.
	dim = lipgloss.AdaptiveColor{Light: "#8a7e76", Dark: "#6b6360"}

	// Subtle: lighter warm gray — very faint elements.
	subtle = lipgloss.AdaptiveColor{Light: "#c4b8ac", Dark: "#4a4340"}

	// Cream: foreground text on dark backgrounds.
	cream = lipgloss.AdaptiveColor{Light: "#3b2618", Dark: "#f2e8dc"}

	// Surface reserved: {Light: "#f5ede4", Dark: "#2c2118"}

	// ── Prompt ──
	promptSymbol = lipgloss.NewStyle().Foreground(accent).Bold(true)
	promptLabel  = lipgloss.NewStyle().Foreground(ui.ColorText).Bold(true)

	// ── Banner ──
	bannerArt  = lipgloss.NewStyle().Foreground(accent)
	bannerName = lipgloss.NewStyle().Foreground(ui.ColorPrimary).Bold(true)
	bannerVer  = lipgloss.NewStyle().Foreground(ui.ColorMuted)
	bannerDesc = lipgloss.NewStyle().Foreground(dim)
	bannerHint = lipgloss.NewStyle().Foreground(dim).Italic(true)

	// ── Completions Popup ──
	compBorder       = lipgloss.NewStyle().Foreground(subtle)
	compActiveRow    = lipgloss.NewStyle().Background(accentAlt).Foreground(cream).Bold(true) // wraps full padded row
	compActiveName   = lipgloss.NewStyle().Background(accentAlt).Foreground(cream).Bold(true)
	compActiveDesc   = lipgloss.NewStyle().Background(accentAlt).Foreground(cream).Italic(true)
	compInactiveName = lipgloss.NewStyle().Foreground(ui.ColorText)
	compInactiveDesc = lipgloss.NewStyle().Foreground(dim).Italic(true)
	compAdminBadge   = lipgloss.NewStyle().Foreground(ui.ColorWarning)

	// ── Output ──
	outputText    = lipgloss.NewStyle().Foreground(ui.ColorText)
	outputEcho    = lipgloss.NewStyle().Foreground(accent).Bold(true)
	outputDimEcho = lipgloss.NewStyle().Foreground(dim)
	outputCmd     = lipgloss.NewStyle().Foreground(cream).Bold(true)

	// ── Scroll & Status ──
	scrollHint  = lipgloss.NewStyle().Foreground(dim).Italic(true)
	statusText  = lipgloss.NewStyle().Foreground(dim).Italic(true)
	statusKey   = lipgloss.NewStyle().Foreground(ui.ColorMuted)
	statusSep   = lipgloss.NewStyle().Foreground(subtle)
	statusAdmin = lipgloss.NewStyle().Foreground(ui.ColorWarning).Bold(true)
)

// cmdIcons maps command names to their emoji icons for the completions popup.
var cmdIcons = map[string]string{
	"clean":     "🧹",
	"uninstall": "📦",
	"optimize":  "⚡",
	"analyze":   "📊",
	"status":    "💓",
	"purge":     "🗑",
	"installer": "📥",
	"update":    "🔄",
	"version":   "🏷",
	"help":      "❓",
	"quit":      "👋",
}

// View renders the complete shell interface.
func (m ShellModel) View() string {
	if m.Quitting {
		return ""
	}

	w := m.Width
	if w < 40 {
		w = 40
	}

	var s strings.Builder

	// ── Welcome Banner (only on first launch, before any output) ──
	if len(m.OutputLines) <= 1 {
		s.WriteString(m.renderBanner(w))
	}

	// ── Output Viewport ──
	s.WriteString(m.renderOutput(w))

	// ── Completions Popup (overlays above prompt) ──
	if m.completions.IsOpen() {
		s.WriteString(m.renderCompletions(w))
	}

	// ── Input Separator ──
	sepLine := strings.Repeat("╌", w-4)
	s.WriteString("  " + compBorder.Render(sepLine) + "\n")

	// ── Prompt Line ──
	s.WriteString(m.renderPrompt(w))

	// ── Status Bar ──
	s.WriteString(m.renderStatusBar(w))

	return s.String()
}

// ─── Banner ──────────────────────────────────────────────────────────────────

func (m ShellModel) renderBanner(w int) string {
	var s strings.Builder

	s.WriteString("\n")

	// 5-line mole ASCII art in accent color.
	art := []string{
		`    ╱▔▔▔╲    `,
		`   ╱  ◉ ◉ ╲  `,
		`  ▕   ▽   ▏  `,
		`   ╲ ═══ ╱   `,
		`    ╲▁▁▁╱    `,
	}
	for _, line := range art {
		s.WriteString("  " + bannerArt.Render(line) + "\n")
	}

	s.WriteString("\n")
	s.WriteString("  " + bannerName.Render("WinMole") +
		"  " + bannerVer.Render(m.Version) + "\n")
	s.WriteString("  " + bannerDesc.Render("Deep clean and optimize your Windows.") + "\n")
	s.WriteString("\n")
	s.WriteString("  " + bannerHint.Render("Type / for commands · /help for details") + "\n")
	s.WriteString("\n")

	// Thin separator.
	sep := strings.Repeat("─", w-4)
	s.WriteString("  " + compBorder.Render(sep) + "\n")
	s.WriteString("\n")

	return s.String()
}

// ─── Output Viewport ─────────────────────────────────────────────────────────

func (m ShellModel) renderOutput(w int) string {
	if len(m.OutputLines) == 0 {
		return ""
	}

	vpHeight := m.viewportHeight()
	lines := m.OutputLines

	// Calculate visible window.
	totalLines := len(lines)
	startIdx := totalLines - vpHeight - m.scrollPos
	if startIdx < 0 {
		startIdx = 0
	}
	endIdx := startIdx + vpHeight
	if endIdx > totalLines {
		endIdx = totalLines
	}

	var s strings.Builder

	// Scroll-up indicator (above output).
	if startIdx > 0 {
		indicator := fmt.Sprintf("  ↑ %d more above", startIdx)
		s.WriteString(scrollHint.Render(indicator) + "\n")
	}

	for i := startIdx; i < endIdx; i++ {
		line := lines[i]

		// Truncate long lines (rune-aware, O(log n) binary search).
		if lipgloss.Width(line) > w-2 {
			runes := []rune(line)
			lo, hi := 0, len(runes)
			for lo < hi {
				mid := (lo + hi + 1) / 2
				if lipgloss.Width(string(runes[:mid])) <= w-5 {
					lo = mid
				} else {
					hi = mid - 1
				}
			}
			line = string(runes[:lo]) + "..."
		}

		// Style echo lines (lines starting with "wm ❯") differently.
		if strings.HasPrefix(line, "wm ❯ ") {
			cmd := strings.TrimPrefix(line, "wm ❯ ")
			echoLine := outputDimEcho.Render("wm") + " " +
				outputEcho.Render("❯") + " " +
				outputCmd.Render(cmd)
			s.WriteString(echoLine + "\n")
		} else if line == "" {
			// Preserve empty lines.
			s.WriteString("\n")
		} else {
			s.WriteString(outputText.Render(line) + "\n")
		}
	}

	// Scroll-down indicator (below output).
	if m.scrollPos > 0 {
		hiddenBelow := m.scrollPos
		indicator := fmt.Sprintf("  ↓ %d more below", hiddenBelow)
		s.WriteString(scrollHint.Render(indicator) + "\n")
	}

	return s.String()
}

// ─── Completions Popup ───────────────────────────────────────────────────────

func (m ShellModel) renderCompletions(w int) string {
	filtered := m.completions.Filtered()
	if len(filtered) == 0 {
		return ""
	}

	cursor := m.completions.Cursor()

	// Box dimensions.
	boxWidth := 54
	if w < 60 {
		boxWidth = w - 6
	}
	if boxWidth < 30 {
		boxWidth = 30
	}
	innerWidth := boxWidth - 2 // account for │ borders

	// Max visible items with scroll support.
	maxVisible := 8
	if len(filtered) < maxVisible {
		maxVisible = len(filtered)
	}

	// Scroll the list if cursor is beyond visible range.
	startIdx := 0
	if cursor >= maxVisible {
		startIdx = cursor - maxVisible + 1
	}
	endIdx := startIdx + maxVisible
	if endIdx > len(filtered) {
		endIdx = len(filtered)
	}

	var s strings.Builder
	s.WriteString("\n")

	// ╭─ Top border ─╮
	topBorder := "╭" + strings.Repeat("─", boxWidth-2) + "╮"
	s.WriteString("  " + compBorder.Render(topBorder) + "\n")

	// Scroll-up indicator.
	if startIdx > 0 {
		above := fmt.Sprintf("  ↑ %d more", startIdx)
		s.WriteString("  " + compBorder.Render("│") +
			scrollHint.Render(padToWidth(above, innerWidth)) +
			compBorder.Render("│") + "\n")
	}

	// Render each completion item.
	for i := startIdx; i < endIdx; i++ {
		cmd := filtered[i]

		// Icon.
		icon := cmdIcons[cmd.Name]
		if icon == "" {
			icon = " ·" // 2-cell placeholder matching emoji width
		}

		// Admin badge.
		adminMark := ""
		if cmd.AdminHint {
			adminMark = " ⚑"
		}

		// Name and description.
		name := "/" + cmd.Name
		desc := cmd.Description

		// Calculate available space for description.
		nameField := fmt.Sprintf("%-12s", name)
		fixedLen := 1 + 2 + 1 + 12 // " "(1) + icon(~2) + " "(1) + name(12)
		if adminMark != "" {
			fixedLen += 3
		}
		maxDesc := innerWidth - fixedLen - 1
		if maxDesc < 4 {
			desc = ""
		} else if len(desc) > maxDesc {
			desc = desc[:maxDesc-3] + "..."
		}

		// Build the content line.
		var contentLine string
		adminStr := ""
		if adminMark != "" {
			adminStr = " " + compAdminBadge.Render("⚑")
		}

		if i == cursor {
			// Active: highlighted background row.
			content := " " + icon + " " + compActiveName.Render(nameField) +
				adminStr + " " + compActiveDesc.Render(desc)
			contentLine = padToWidth(content, innerWidth)
			contentLine = compActiveRow.Render(contentLine)
		} else {
			// Inactive: normal row.
			content := " " + icon + " " + compInactiveName.Render(nameField) +
				adminStr + " " + compInactiveDesc.Render(desc)
			contentLine = padToWidth(content, innerWidth)
		}

		s.WriteString("  " + compBorder.Render("│") +
			contentLine +
			compBorder.Render("│") + "\n")
	}

	// Scroll-down indicator.
	if endIdx < len(filtered) {
		below := fmt.Sprintf("  ↓ %d more", len(filtered)-endIdx)
		s.WriteString("  " + compBorder.Render("│") +
			scrollHint.Render(padToWidth(below, innerWidth)) +
			compBorder.Render("│") + "\n")
	}

	// ╰─ Bottom border ─╯
	bottomBorder := "╰" + strings.Repeat("─", boxWidth-2) + "╯"
	s.WriteString("  " + compBorder.Render(bottomBorder) + "\n")

	return s.String()
}

// ─── Prompt ──────────────────────────────────────────────────────────────────

func (m ShellModel) renderPrompt(_ int) string {
	label := promptLabel.Render("wm")
	symbol := promptSymbol.Render(" ❯ ")
	input := m.textInput.View()
	return label + symbol + input + "\n"
}

// ─── Status Bar ──────────────────────────────────────────────────────────────

func (m ShellModel) renderStatusBar(_ int) string {
	sep := statusSep.Render(" │ ")

	var parts []string

	// Admin badge.
	if m.IsAdmin {
		parts = append(parts, statusAdmin.Render("⚑ admin"))
	}

	// Key hints.
	hints := []struct{ key, desc string }{
		{"/", "commands"},
		{"↑↓", "history"},
		{"pgup/dn", "scroll"},
		{"ctrl+c", "quit"},
	}
	for _, h := range hints {
		parts = append(parts, statusKey.Render(h.key)+" "+statusText.Render(h.desc))
	}

	return "\n" + "  " + strings.Join(parts, sep) + "\n"
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// padToWidth pads a string with spaces to fill the given width.
// Uses lipgloss.Width for accurate ANSI-aware measurement.
func padToWidth(s string, width int) string {
	currentWidth := lipgloss.Width(s)
	if currentWidth >= width {
		return s
	}
	return s + strings.Repeat(" ", width-currentWidth)
}
