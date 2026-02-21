package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"golang.org/x/sys/windows"
)

// ─── ASCII Mascot Art ──────────────────────────────────────────────────────────
// Matches the SVG logo (assets/logo.svg): a cute pet face with round ears,
// dot eyes, and a snout with a nose.

// mascotLines holds the raw ASCII mascot art, rendered line-by-line during intro.
var mascotLines = []string{
	`    ╭●╮       ╭●╮    `,
	`    ╰┬╯╭─────╮╰┬╯    `,
	`     ╰─│ ◉ ◉ │─╯     `,
	`       │ ╭─╮ │        `,
	`       │ ╰▽╯ │        `,
	`       ╰─────╯        `,
}

// groundLine is the terrain beneath the mascot.
var groundLine = `    ─────────────────  `

// moleLines is kept as an alias for backward compatibility.
var moleLines = mascotLines

// brandBanner is the large ASCII wordmark.
var brandLines = []string{
	"  ____                  __        ___       ",
	" |  _ \\ _   _ _ __ ___ \\ \\      / (_)_ __  ",
	" | |_) | | | | '__/ _ \\ \\ \\ /\\ / /| | '_ \\ ",
	" |  __/| |_| | | |  __/  \\ V  V / | | | | |",
	" |_|    \\__,_|_|  \\___|   \\_/\\_/  |_|_| |_|",
}

// tagline sits below the brand banner.
const tagline = "Deep clean and optimize your Windows."

// ─── Terminal Detection ──────────────────────────────────────────────────────

// isTerminal returns true if stdout is a terminal (not piped/redirected).
func isTerminal() bool {
	return isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
}

// EnableVTProcessing enables Virtual Terminal Processing on the Windows console
// so that ANSI escape codes work in cmd.exe and older PowerShell versions.
// Safe to call multiple times; idempotent.
func EnableVTProcessing() {
	stdout := windows.Handle(os.Stdout.Fd())
	var mode uint32
	_ = windows.GetConsoleMode(stdout, &mode)
	_ = windows.SetConsoleMode(stdout, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
}

// ─── Intro Animation ─────────────────────────────────────────────────────────

// ShowMoleIntro displays the animated mascot appearing line-by-line.
// Only runs in interactive terminals; silently returns otherwise.
// Dolly pink for the mascot, charple purple for the ground.
func ShowMoleIntro() {
	if !isTerminal() {
		return
	}

	// Ensure ANSI escape sequences work on Windows consoles.
	EnableVTProcessing()

	moleStyle := lipgloss.NewStyle().Foreground(ColorSecondary)
	groundStyle := lipgloss.NewStyle().Foreground(ColorPrimary)

	// Clear screen.
	fmt.Print("\033[2J\033[H")

	// Animate mascot line by line.
	for _, line := range mascotLines {
		fmt.Println(moleStyle.Render(line))
		time.Sleep(80 * time.Millisecond)
	}

	// Ground with a brief pause.
	fmt.Println(groundStyle.Render(groundLine))
	time.Sleep(80 * time.Millisecond)

	// Pause to admire the mascot.
	time.Sleep(500 * time.Millisecond)

	// Clear screen before continuing to main UI.
	fmt.Print("\033[2J\033[H")
}

// ─── Brand Banner ────────────────────────────────────────────────────────────

// ShowBrandBanner returns the full ASCII brand banner as a styled string,
// ready to be printed. Charple purple wordmark, muted tagline, info-styled URL.
func ShowBrandBanner() string {
	var b strings.Builder

	nameStyle := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)

	// ASCII wordmark.
	for _, line := range brandLines {
		b.WriteString(nameStyle.Render(line))
		b.WriteByte('\n')
	}
	b.WriteByte('\n')

	// Tagline.
	b.WriteString(MutedStyle().Italic(true).Render("  " + tagline))
	b.WriteByte('\n')

	// URL / attribution.
	b.WriteString(InfoStyle().Render("  https://github.com/lakshaymaurya-felt/purewin"))
	b.WriteByte('\n')

	return b.String()
}

// ─── Completion Banner ───────────────────────────────────────────────────────

// ShowCompletionBanner prints a post-operation summary with space freed,
// current free space, and a styled checkmark.
func ShowCompletionBanner(freed int64, freeSpace int64) {
	fmt.Println()

	// Build content
	var content strings.Builder
	content.WriteString(lipgloss.NewStyle().
		Foreground(ColorSuccess).
		Bold(true).
		Render(IconCheck + " Cleanup Complete!"))
	content.WriteString("\n\n")
	content.WriteString(fmt.Sprintf("%s  %s\n",
		lipgloss.NewStyle().Foreground(ColorText).Render("Space freed:"),
		FormatSize(freed)))
	content.WriteString(fmt.Sprintf("%s  %s",
		lipgloss.NewStyle().Foreground(ColorText).Render("Free space: "),
		FormatSize(freeSpace)))

	// Render in card
	fmt.Println(CardStyle().Width(50).Render(content.String()))
	fmt.Println()
}

// ─── Mascot Art (Static) ──────────────────────────────────────────────────────

// MoleArt returns the full mascot ASCII art as a single styled string.
// Useful for embedding in help screens or about dialogs.
func MoleArt() string {
	moleStyle := lipgloss.NewStyle().Foreground(ColorSecondary)
	groundStyle := lipgloss.NewStyle().Foreground(ColorPrimary)

	var b strings.Builder
	for _, line := range mascotLines {
		b.WriteString(moleStyle.Render(line))
		b.WriteByte('\n')
	}
	b.WriteString(groundStyle.Render(groundLine))
	b.WriteByte('\n')
	return b.String()
}
