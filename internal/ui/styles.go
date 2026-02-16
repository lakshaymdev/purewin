package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ─── Color Palette ───────────────────────────────────────────────────────────
// Charmtone-inspired vibrant palette (charmbracelet/x/exp/charmtone).
// Adaptive colors degrade gracefully in terminals without 256-color support.
// The Light variant targets light backgrounds; Dark targets dark backgrounds.

var (
	// Primary: Charple purple — selected items, focus states, active elements.
	ColorPrimary = lipgloss.AdaptiveColor{Light: "#5040CC", Dark: "#6B50FF"}

	// Secondary: Dolly pink — headers, highlights, interactive accents.
	ColorSecondary = lipgloss.AdaptiveColor{Light: "#CC4FCC", Dark: "#FF60FF"}

	// Success: Julep neon green — confirmations, check marks, completions.
	ColorSuccess = lipgloss.AdaptiveColor{Light: "#00A080", Dark: "#00FFB2"}

	// Warning: Tang bright orange — caution messages, non-destructive alerts.
	ColorWarning = lipgloss.AdaptiveColor{Light: "#CC7A48", Dark: "#FF985A"}

	// Error: Cherry hot pink — errors, danger zones, destructive operations.
	ColorError = lipgloss.AdaptiveColor{Light: "#CC2D70", Dark: "#FF388B"}

	// Info: Malibu blue — informational text, links, secondary actions.
	ColorInfo = lipgloss.AdaptiveColor{Light: "#0084CC", Dark: "#00A4FF"}

	// Muted: Squid — disabled items, hints, secondary text.
	ColorMuted = lipgloss.AdaptiveColor{Light: "#858392", Dark: "#605F6B"}

	// Surface: BBQ — subtle background tints for panels and cards.
	ColorSurface = lipgloss.AdaptiveColor{Light: "#F0EEF2", Dark: "#2D2C35"}

	// Text: Salt/Pepper — primary foreground text.
	ColorText = lipgloss.AdaptiveColor{Light: "#201F26", Dark: "#F1EFEF"}

	// TextDim: Smoke — dimmed foreground for secondary content.
	ColorTextDim = lipgloss.AdaptiveColor{Light: "#605F6B", Dark: "#BFBCC8"}

	// Accent: Bok mint — tags, pills, special highlights.
	ColorAccent = lipgloss.AdaptiveColor{Light: "#40CCB0", Dark: "#68FFD6"}

	// SurfaceDark: Pepper — deeper background for cards.
	ColorSurfaceDark = lipgloss.AdaptiveColor{Light: "#E8E6EC", Dark: "#201F26"}

	// Overlay: Iron — popup/modal backgrounds.
	ColorOverlay = lipgloss.AdaptiveColor{Light: "#E0DEE4", Dark: "#4D4C57"}

	// Border: Charcoal — panel borders.
	ColorBorder = lipgloss.AdaptiveColor{Light: "#BFBCC8", Dark: "#3A3943"}

	// BorderFocus: Charple — focused panel borders.
	ColorBorderFocus = lipgloss.AdaptiveColor{Light: "#5040CC", Dark: "#6B50FF"}

	// ── Per-Screen Accent Colors ──
	// Each major view gets its own accent for visual variety.

	// Teal: Turtle — status dashboard charts.
	ColorTeal = lipgloss.AdaptiveColor{Light: "#08A8A6", Dark: "#0ADCD9"}

	// Violet: for shell/prompt accents.
	ColorViolet = lipgloss.AdaptiveColor{Light: "#9A48CC", Dark: "#C259FF"}

	// Coral: for disk analyzer.
	ColorCoral = lipgloss.AdaptiveColor{Light: "#CC4664", Dark: "#FF577D"}

	// Blue: Sardine — for selector.
	ColorBlue = lipgloss.AdaptiveColor{Light: "#3F98CC", Dark: "#4FBEFE"}

	// Hazy: light purple — for menu.
	ColorHazy = lipgloss.AdaptiveColor{Light: "#6F5FCC", Dark: "#8B75FF"}
)

// ─── Icon Constants ──────────────────────────────────────────────────────────
// Unicode glyphs used throughout the UI for consistent visual language.
// Crush-inspired: refined, minimal, no emoji.

const (
	// Core icons
	IconCheck     = "✓"
	IconCross     = "×"
	IconWarning   = "!"
	IconArrow     = "→"
	IconDot       = "●"
	IconCircle    = "○"
	IconBullet    = "•"
	IconDash      = "─"
	IconCorner    = "└"
	IconPipe      = "│"
	IconFolder    = "◆"
	IconTrash     = "✕"
	IconPending   = "⋯"
	IconDiamond   = "◇"
	IconChevron   = "›"
	IconBlock     = "▌"
	IconRadioOn   = "◉"
	IconRadioOff  = IconCircle
	IconReload    = "⟳"
	IconHelp      = "?"
	IconPrompt    = "❯"
	IconDashLight = "╌"

	// Backward compatibility aliases
	IconSuccess    = IconCheck
	IconError      = IconCross
	IconSelected   = IconDot
	IconUnselected = IconCircle
)

// SpinnerFrames contains the braille-dot animation sequence for spinners.
var SpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// ─── Core Styles ─────────────────────────────────────────────────────────────
// Reusable lipgloss styles for the entire application. Each is a function
// returning a fresh copy so callers can extend without mutating shared state.

// SuccessStyle renders text in julep green.
func SuccessStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ColorSuccess)
}

// ErrorStyle renders text in cherry hot pink.
func ErrorStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ColorError)
}

// WarningStyle renders text in tang orange.
func WarningStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ColorWarning)
}

// InfoStyle renders text in malibu blue.
func InfoStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ColorInfo)
}

// MutedStyle renders text in squid gray.
func MutedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ColorMuted)
}

// HeaderStyle renders bold, dolly pink header text with a bottom margin.
func HeaderStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true).
		MarginBottom(1)
}

// BoldStyle renders bold text in the primary foreground color.
func BoldStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(ColorText).
		Bold(true)
}

// ─── Composite Styles ────────────────────────────────────────────────────────

// MenuItemStyle is the base style for unselected menu items.
func MenuItemStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		PaddingLeft(2)
}

// MenuItemActiveStyle is the highlighted style for the selected menu item.
func MenuItemActiveStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true).
		PaddingLeft(1)
}

// MenuDescriptionStyle renders item descriptions in muted text.
func MenuDescriptionStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(ColorTextDim).
		PaddingLeft(4)
}

// HintBarStyle renders the bottom key-hint bar.
func HintBarStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(ColorMuted).
		MarginTop(1).
		Italic(true)
}

// DangerBoxStyle renders a bordered danger zone panel.
func DangerBoxStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(ColorError).
		Bold(true).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorError).
		Padding(0, 1)
}

// CategoryHeaderStyle renders category divider labels.
func CategoryHeaderStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true).
		MarginTop(1).
		PaddingLeft(1)
}

// ─── Premium Styles ──────────────────────────────────────────────────────────
// Crush-inspired panel, card, tag, and gradient helpers for the TUI overhaul.

// PanelStyle renders a rounded-border panel with subtle border color.
func PanelStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(1, 2)
}

// PanelFocusedStyle renders a panel with the focus border color.
func PanelFocusedStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorderFocus).
		Padding(1, 2)
}

// CardStyle renders a card with rounded border and minimal padding.
func CardStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(0, 2)
}

// TagStyle renders a small tag/pill with background color and padding.
func TagStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(ColorText).
		Background(ColorSurface).
		Padding(0, 1)
}

// TagAccentStyle renders an accent-colored tag.
func TagAccentStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(ColorText).
		Background(ColorAccent).
		Padding(0, 1).
		Bold(true)
}

// TagErrorStyle renders an error tag with error background.
func TagErrorStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(ColorText).
		Background(ColorError).
		Padding(0, 1).
		Bold(true)
}

// TagWarningStyle renders a warning tag.
func TagWarningStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(ColorSurfaceDark).
		Background(ColorWarning).
		Padding(0, 1).
		Bold(true)
}

// SectionHeader renders: "── Label ──────────" at the given width.
func SectionHeader(label string, width int) string {
	styled := lipgloss.NewStyle().Foreground(ColorSecondary).Bold(true).Render(label)
	labelW := lipgloss.Width(styled)
	pre := "── "
	remaining := width - labelW - len(pre) - 1
	if remaining < 0 {
		remaining = 0
	}
	suf := " " + strings.Repeat("─", remaining)
	return MutedStyle().Render(pre) + styled + MutedStyle().Render(suf)
}

// GradientBar renders a filled/empty bar with color that shifts based on percentage.
func GradientBar(pct float64, width int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := int(pct / 100 * float64(width))
	if filled > width {
		filled = width
	}

	barColor := ColorPrimary
	if pct >= 90 {
		barColor = ColorError
	} else if pct >= 70 {
		barColor = ColorWarning
	}

	fStr := lipgloss.NewStyle().Foreground(barColor).Render(strings.Repeat("█", filled))
	eStr := MutedStyle().Render(strings.Repeat("░", width-filled))
	return fStr + eStr
}

// FocusBorder returns a left-border style for focused items (crush-style thick bar).
func FocusBorder() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.Border{Left: IconBlock}, false, false, false, true).
		BorderForeground(ColorPrimary).
		PaddingLeft(1)
}

// ─── Formatting Helpers ──────────────────────────────────────────────────────

// FormatSize returns a human-readable, styled file-size string.
// Uses binary units (KiB, MiB, GiB, TiB) for precision.
func FormatSize(bytes int64) string {
	const (
		_         = iota
		kib int64 = 1 << (10 * iota)
		mib
		gib
		tib
	)

	var size string
	switch {
	case bytes >= tib:
		size = fmt.Sprintf("%.1f TiB", float64(bytes)/float64(tib))
	case bytes >= gib:
		size = fmt.Sprintf("%.1f GiB", float64(bytes)/float64(gib))
	case bytes >= mib:
		size = fmt.Sprintf("%.1f MiB", float64(bytes)/float64(mib))
	case bytes >= kib:
		size = fmt.Sprintf("%.1f KiB", float64(bytes)/float64(kib))
	default:
		size = fmt.Sprintf("%d B", bytes)
	}

	// Color-code by magnitude: huge = cherry, large = orange, medium = blue, small = muted.
	style := MutedStyle()
	switch {
	case bytes >= gib:
		style = ErrorStyle().Bold(true)
	case bytes >= 100*mib:
		style = WarningStyle()
	case bytes >= mib:
		style = InfoStyle()
	}

	return style.Render(size)
}

// FormatSizePlain returns a human-readable file-size string without any styling.
func FormatSizePlain(bytes int64) string {
	const (
		_         = iota
		kib int64 = 1 << (10 * iota)
		mib
		gib
		tib
	)
	switch {
	case bytes >= tib:
		return fmt.Sprintf("%.1f TiB", float64(bytes)/float64(tib))
	case bytes >= gib:
		return fmt.Sprintf("%.1f GiB", float64(bytes)/float64(gib))
	case bytes >= mib:
		return fmt.Sprintf("%.1f MiB", float64(bytes)/float64(mib))
	case bytes >= kib:
		return fmt.Sprintf("%.1f KiB", float64(bytes)/float64(kib))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// FormatPath truncates and styles a filesystem path to fit within maxWidth.
// It preserves the drive letter (or root) and the final path component,
// replacing the middle with an ellipsis when needed.
func FormatPath(path string) string {
	return FormatPathWidth(path, 50)
}

// FormatPathWidth truncates a path to the given width, preserving meaningful
// components on both ends.
func FormatPathWidth(path string, maxWidth int) string {
	// Normalize separators for display.
	display := filepath.ToSlash(path)

	if maxWidth <= 0 {
		return ""
	}
	if maxWidth <= 3 {
		return MutedStyle().Render("…")
	}

	if len(display) <= maxWidth {
		return MutedStyle().Render(display)
	}

	parts := strings.Split(display, "/")
	if len(parts) <= 2 {
		// Can't meaningfully truncate — just clip.
		return MutedStyle().Render(display[:maxWidth-1] + "…")
	}

	// Keep first component (drive/root) and last component (filename).
	head := parts[0]
	tail := parts[len(parts)-1]

	// Build from the end until we run out of budget.
	ellipsis := "/…/"
	budget := maxWidth - len(head) - len(ellipsis) - len(tail)
	if budget <= 0 {
		// Even head + tail overflow; just clip.
		clipped := head + ellipsis + tail
		if len(clipped) > maxWidth {
			clipped = clipped[:maxWidth-1] + "…"
		}
		return MutedStyle().Render(clipped)
	}

	// Accumulate path segments from the end.
	var middle []string
	remaining := budget
	for i := len(parts) - 2; i >= 1; i-- {
		seg := parts[i]
		needed := len(seg) + 1 // +1 for the "/"
		if remaining-needed < 0 {
			break
		}
		middle = append([]string{seg}, middle...)
		remaining -= needed
	}

	if len(middle) == len(parts)-2 {
		// Everything fits after all.
		return MutedStyle().Render(display)
	}

	result := head + ellipsis + strings.Join(middle, "/")
	if len(middle) > 0 {
		result += "/"
	}
	result += tail

	return MutedStyle().Render(result)
}

// FormatCount renders a number with the given label, styled by magnitude.
func FormatCount(n int, label string) string {
	s := fmt.Sprintf("%d %s", n, label)
	if n == 0 {
		return MutedStyle().Render(s)
	}
	return InfoStyle().Render(s)
}

// Divider returns a horizontal rule string of the given width.
func Divider(width int) string {
	if width <= 0 {
		width = 40
	}
	return MutedStyle().Render(strings.Repeat("─", width))
}
