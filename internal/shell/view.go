package shell

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/lakshaymaurya-felt/purewin/internal/ui"
)

// ─── Charmtone Shell Palette ─────────────────────────────────────────────────
// Extends the global palette with shell-specific violet accent styles.
// Each screen gets its own accent color for visual variety.

// Shell-specific accent colors — violet for shell's unique visual identity.
var (
	// Accent: Violet purple — primary interactive elements.
	accent = lipgloss.AdaptiveColor{Light: "#9A48CC", Dark: "#C259FF"}

	// Dim: Oyster gray — chrome, borders, secondary text.
	dim = lipgloss.AdaptiveColor{Light: "#858392", Dark: "#605F6B"}

	// ── Prompt ──
	promptSymbol = lipgloss.NewStyle().Foreground(accent).Bold(true)
	promptLabel  = lipgloss.NewStyle().Foreground(ui.ColorText).Bold(true)

	// ── Banner ──
	bannerName = lipgloss.NewStyle().Foreground(ui.ColorPrimary).Bold(true)
	bannerDesc = lipgloss.NewStyle().Foreground(ui.ColorTextDim).Italic(true)

	// ── Welcome Screen ──
	welcomeCardBorderCleanup = lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder()).
					BorderForeground(ui.ColorSuccess).
					Padding(1, 2)
	welcomeCardBorderSystem = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ui.ColorInfo).
				Padding(1, 2)
	welcomeCardBorderTools = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ui.ColorViolet).
				Padding(1, 2)
	welcomeTipsBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ui.ColorBorder).
			Padding(0, 2)
	welcomeCardTitle    = lipgloss.NewStyle().Bold(true)
	welcomeCmdName      = lipgloss.NewStyle().Foreground(ui.ColorText)
	welcomeCmdIcon      = lipgloss.NewStyle().Foreground(ui.ColorMuted)
	welcomeTipLabel     = lipgloss.NewStyle().Foreground(accent).Bold(true)
	welcomeTipCmd       = lipgloss.NewStyle().Foreground(ui.ColorPrimary)
	welcomeTipDesc      = lipgloss.NewStyle().Foreground(ui.ColorTextDim)
	welcomeHostname     = lipgloss.NewStyle().Foreground(ui.ColorText).Bold(true)
	welcomeAdminBadge   = lipgloss.NewStyle().Foreground(ui.ColorWarning).Bold(true)
	welcomeVersionBadge = lipgloss.NewStyle().Foreground(ui.ColorMuted)

	// ── Completions Popup ──
	compBorder       = lipgloss.NewStyle().Foreground(ui.ColorBorder)
	compActiveRow    = lipgloss.NewStyle().Background(ui.ColorOverlay).Foreground(ui.ColorText).Bold(true)
	compActiveName   = lipgloss.NewStyle().Background(ui.ColorOverlay).Foreground(ui.ColorText).Bold(true)
	compActiveDesc   = lipgloss.NewStyle().Background(ui.ColorOverlay).Foreground(ui.ColorTextDim).Italic(true)
	compInactiveName = lipgloss.NewStyle().Foreground(ui.ColorText)
	compInactiveDesc = lipgloss.NewStyle().Foreground(dim).Italic(true)
	compAdminBadge   = lipgloss.NewStyle().Foreground(ui.ColorWarning)

	// ── Output ──
	outputText    = lipgloss.NewStyle().Foreground(ui.ColorText)
	outputEcho    = lipgloss.NewStyle().Foreground(accent).Bold(true)
	outputDimEcho = lipgloss.NewStyle().Foreground(dim)
	outputCmd     = lipgloss.NewStyle().Foreground(ui.ColorText).Bold(true)

	// ── Scroll & Status ──
	scrollHint  = lipgloss.NewStyle().Foreground(dim).Italic(true)
	statusText  = lipgloss.NewStyle().Foreground(dim).Italic(true)
	statusKey   = lipgloss.NewStyle().Foreground(ui.ColorMuted)
	statusSep   = lipgloss.NewStyle().Foreground(ui.ColorBorder)
	statusAdmin = lipgloss.NewStyle().Foreground(ui.ColorWarning).Bold(true)
)

// ─── Welcome Mascot & Brand Art ──────────────────────────────────────────────
// ASCII mascot matching the SVG logo (assets/logo.svg) and the large wordmark.
// Kept local to the shell package so it's self-contained.

var welcomeMascotLines = []string{
	`    ╭●╮       ╭●╮    `,
	`    ╰┬╯╭─────╮╰┬╯    `,
	`     ╰─│ ◉ ◉ │─╯     `,
	`       │ ╭─╮ │        `,
	`       │ ╰▽╯ │        `,
	`       ╰─────╯        `,
}

var welcomeBrandLines = []string{
	`  ____                  __        ___       `,
	` |  _ \ _   _ _ __ ___ \ \      / (_)_ __  `,
	` | |_) | | | | '__/ _ \ \ \ /\ / /| | '_ \ `,
	` |  __/| |_| | | |  __/  \ V  V / | | | | |`,
	` |_|    \__,_|_|  \___|   \_/\_/  |_|_| |_|`,
}

// cmdIcons maps command names to their crush-style Unicode glyphs for the completions popup.
var cmdIcons = map[string]string{
	"clean":     ui.IconTrash,
	"uninstall": ui.IconFolder,
	"optimize":  ui.IconArrow,
	"analyze":   ui.IconDiamond,
	"status":    ui.IconDot,
	"purge":     ui.IconTrash,
	"installer": ui.IconFolder,
	"update":    ui.IconReload,
	"version":   ui.IconDiamond,
	"help":      ui.IconHelp,
	"quit":      ui.IconCross,
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

	showBanner := len(m.OutputLines) <= 1

	// ── Welcome Banner (only on first launch, before any output) ──
	if showBanner {
		s.WriteString(m.renderBanner(w))
	}

	// ── Output Viewport (skip when banner owns the screen) ──
	if !showBanner {
		s.WriteString(m.renderOutput(w))
	}

	// ── Completions Popup (overlays above prompt) ──
	if m.completions.IsOpen() {
		s.WriteString(m.renderCompletions(w))
	}

	// ── Input Separator ──
	sepLine := strings.Repeat(ui.IconDashLight, w-4)
	s.WriteString("  " + compBorder.Render(sepLine) + "\n")

	// ── Prompt Line ──
	s.WriteString(m.renderPrompt(w))

	// ── Status Bar ──
	s.WriteString(m.renderStatusBar(w))

	return s.String()
}

// ─── Banner ──────────────────────────────────────────────────────────────────
// Full-screen welcome experience. Vertically centered, fills the terminal with
// brand art, command cards, system info, and quick-start tips.

func (m ShellModel) renderBanner(w int) string {
	// Reserve lines for the chrome below the banner:
	// separator (1) + prompt (1) + status bar newline+content+newline (3)
	const chromeLines = 5
	availH := m.Height - chromeLines
	if availH < 10 {
		availH = 10
	}

	// ── Compact mode for tiny terminals ──
	if m.Height < 20 || w < 55 {
		return m.renderBannerCompact(w, availH)
	}

	// ── Build content blocks ──
	brandBlock := m.renderWelcomeBrand()
	infoBar := m.renderWelcomeInfoBar()
	cardsBlock := m.renderWelcomeCards(w)
	tipsBlock := m.renderWelcomeTips(w)

	// Stack vertically, center-aligned.
	content := lipgloss.JoinVertical(lipgloss.Center,
		brandBlock,
		"",
		infoBar,
		"",
		cardsBlock,
		"",
		tipsBlock,
	)

	// Center the whole block in the available space.
	return lipgloss.Place(w, availH, lipgloss.Center, lipgloss.Center, content)
}

// renderBannerCompact renders a minimal banner for small terminals.
func (m ShellModel) renderBannerCompact(_ int, availH int) string {
	if availH < 6 {
		availH = 6
	}

	title := bannerName.Render("PureWin") + "  " + welcomeVersionBadge.Render(m.Version)
	desc := bannerDesc.Render("Deep clean and optimize your Windows.")
	hint := lipgloss.NewStyle().Foreground(dim).Italic(true).
		Render("Type / for commands " + ui.IconBullet + " /help for details")

	content := lipgloss.JoinVertical(lipgloss.Center, title, desc, "", hint)
	return lipgloss.Place(m.Width, availH, lipgloss.Center, lipgloss.Center, content)
}

// ─── Welcome Sub-Renderers ───────────────────────────────────────────────────

// renderWelcomeBrand renders the mascot + large ASCII wordmark + tagline.
func (m ShellModel) renderWelcomeBrand() string {
	mascotStyle := lipgloss.NewStyle().Foreground(ui.ColorSecondary)
	artStyle := lipgloss.NewStyle().Foreground(ui.ColorPrimary).Bold(true)

	// Mascot (matches assets/logo.svg).
	var mascotBlock strings.Builder
	for _, line := range welcomeMascotLines {
		mascotBlock.WriteString(mascotStyle.Render(line))
		mascotBlock.WriteByte('\n')
	}

	// Wordmark.
	var artBlock strings.Builder
	for _, line := range welcomeBrandLines {
		artBlock.WriteString(artStyle.Render(line))
		artBlock.WriteByte('\n')
	}

	tagline := bannerDesc.Render("Deep clean and optimize your Windows.")

	return lipgloss.JoinVertical(lipgloss.Center,
		strings.TrimRight(mascotBlock.String(), "\n"),
		"",
		strings.TrimRight(artBlock.String(), "\n"),
		"",
		tagline,
	)
}

// renderWelcomeInfoBar renders the hostname · admin · version status line.
func (m ShellModel) renderWelcomeInfoBar() string {
	sep := lipgloss.NewStyle().Foreground(ui.ColorBorder).Render(" " + ui.IconBullet + " ")

	var parts []string

	if m.Hostname != "" {
		parts = append(parts, welcomeHostname.Render(m.Hostname))
	}

	if m.IsAdmin {
		parts = append(parts, welcomeAdminBadge.Render(ui.IconDot+" admin"))
	}

	parts = append(parts, welcomeVersionBadge.Render("v"+m.Version))

	return strings.Join(parts, sep)
}

// cmdGroup holds metadata for a category card on the welcome screen.
type cmdGroup struct {
	title string
	color lipgloss.AdaptiveColor
	style lipgloss.Style
	cmds  []struct{ icon, name string }
}

// renderWelcomeCards renders the command category cards in a responsive grid.
func (m ShellModel) renderWelcomeCards(w int) string {
	groups := []cmdGroup{
		{
			title: "Cleanup",
			color: ui.ColorSuccess,
			style: welcomeCardBorderCleanup,
			cmds: []struct{ icon, name string }{
				{ui.IconTrash, "/clean"},
				{ui.IconTrash, "/purge"},
				{ui.IconFolder, "/installer"},
			},
		},
		{
			title: "System",
			color: ui.ColorInfo,
			style: welcomeCardBorderSystem,
			cmds: []struct{ icon, name string }{
				{ui.IconArrow, "/optimize"},
				{ui.IconDot, "/status"},
				{ui.IconFolder, "/uninstall"},
			},
		},
		{
			title: "Tools",
			color: ui.ColorViolet,
			style: welcomeCardBorderTools,
			cmds: []struct{ icon, name string }{
				{ui.IconDiamond, "/analyze"},
				{ui.IconReload, "/update"},
				{ui.IconHelp, "/help"},
			},
		},
	}

	// Determine card width based on terminal width.
	cardInner := 16
	if w >= 80 {
		cardInner = 18
	}

	var cards []string
	for _, g := range groups {
		titleStyled := welcomeCardTitle.Foreground(g.color).Render(g.title)
		var lines []string
		lines = append(lines, titleStyled)
		lines = append(lines, "")
		for _, c := range g.cmds {
			line := welcomeCmdIcon.Render(c.icon) + " " + welcomeCmdName.Render(c.name)
			lines = append(lines, line)
		}
		body := lipgloss.JoinVertical(lipgloss.Left, lines...)
		card := g.style.Width(cardInner).Render(body)
		cards = append(cards, card)
	}

	// Layout: 3-col if wide enough, 2-col + 1 stacked, or single column.
	if w >= 76 {
		return lipgloss.JoinHorizontal(lipgloss.Top, cards[0], "  ", cards[1], "  ", cards[2])
	}
	if w >= 52 {
		topRow := lipgloss.JoinHorizontal(lipgloss.Top, cards[0], "  ", cards[1])
		return lipgloss.JoinVertical(lipgloss.Center, topRow, "", cards[2])
	}
	return lipgloss.JoinVertical(lipgloss.Center, cards[0], "", cards[1], "", cards[2])
}

// renderWelcomeTips renders the quick-start tips panel.
func (m ShellModel) renderWelcomeTips(w int) string {
	label := welcomeTipLabel.Render("Quick Start")

	tips := []struct{ cmd, desc string }{
		{"/", "see all commands"},
		{"/clean --dry-run", "preview cleanup"},
		{"/status", "live system monitor"},
	}

	var lines []string
	lines = append(lines, label)
	lines = append(lines, "")
	for _, t := range tips {
		line := ui.IconChevron + " " + welcomeTipCmd.Render(t.cmd) + "  " + welcomeTipDesc.Render(t.desc)
		lines = append(lines, line)
	}

	body := lipgloss.JoinVertical(lipgloss.Left, lines...)

	boxWidth := 42
	if w < 50 {
		boxWidth = w - 8
	}
	if boxWidth < 30 {
		boxWidth = 30
	}

	return welcomeTipsBox.Width(boxWidth).Render(body)
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

		// Style echo lines (lines starting with "pw ❯") differently.
		if strings.HasPrefix(line, "pw "+ui.IconPrompt+" ") {
			cmd := strings.TrimPrefix(line, "pw "+ui.IconPrompt+" ")
			echoLine := outputDimEcho.Render("pw") + " " +
				outputEcho.Render(ui.IconPrompt) + " " +
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
			icon = " " + ui.IconBullet
		}

		// Admin badge.
		adminMark := ""
		if cmd.AdminHint {
			adminMark = " " + ui.IconDot
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
			adminStr = " " + compAdminBadge.Render(ui.IconDot)
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
	label := promptLabel.Render("pw")
	symbol := promptSymbol.Render(" " + ui.IconPrompt + " ")
	input := m.textInput.View()
	return label + symbol + input + "\n"
}

// ─── Status Bar ──────────────────────────────────────────────────────────────

func (m ShellModel) renderStatusBar(_ int) string {
	sep := statusSep.Render(" " + ui.IconPipe + " ")

	var parts []string

	// Admin badge with IconDot.
	if m.IsAdmin {
		parts = append(parts, statusAdmin.Render(ui.IconDot+" admin"))
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
