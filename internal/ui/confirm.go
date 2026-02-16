package ui

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ─── Simple Confirm ──────────────────────────────────────────────────────────

// Confirm presents a Y/N prompt and returns true if the user types y or Y.
// Default is No (pressing Enter without input returns false).
//
//	"Proceed with cleanup? [y/N]: "
func Confirm(message string) (bool, error) {
	promptStyle := BoldStyle()
	hintStyle := MutedStyle()

	fmt.Printf("%s %s ",
		promptStyle.Render(message),
		hintStyle.Render("[y/N]:"),
	)

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read input: %w", err)
	}

	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes", nil
}

// ─── Danger Confirm ──────────────────────────────────────────────────────────

// DangerConfirm presents a dangerous-operation confirmation that requires
// the user to type the word "yes" (not just "y"). Used for irreversible
// actions like deleting Windows.old.
//
// The message is rendered in red with a warning icon and a bordered panel.
func DangerConfirm(message string) (bool, error) {
	warnTag := TagErrorStyle().Render(" " + IconWarning + " WARNING ")

	dangerMsg := lipgloss.NewStyle().
		Foreground(ColorError).
		Bold(true).
		Render(message)

	box := DangerBoxStyle()

	// Print the danger panel.
	fmt.Println()
	fmt.Println(box.Render(fmt.Sprintf("%s  %s", warnTag, dangerMsg)))
	fmt.Println()

	// Instruction line.
	instructStyle := lipgloss.NewStyle().Foreground(ColorText)
	yesPrompt := TagErrorStyle().Render(` "yes" `)

	fmt.Printf("%s %s %s ",
		instructStyle.Render("  Type"),
		yesPrompt,
		instructStyle.Render("to confirm:"),
	)

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read input: %w", err)
	}

	input = strings.TrimSpace(strings.ToLower(input))
	return input == "yes", nil
}

// ─── Press Enter ─────────────────────────────────────────────────────────────

// PressEnterToContinue pauses execution until the user presses Enter.
// Displays the provided message (or a default) in muted style.
func PressEnterToContinue(message string) {
	if message == "" {
		message = "Press Enter to continue..."
	}

	hintStyle := MutedStyle().Italic(true)
	fmt.Printf("\n  %s ", hintStyle.Render(message))

	reader := bufio.NewReader(os.Stdin)
	_, _ = reader.ReadString('\n')
}

// ─── Choose Option ───────────────────────────────────────────────────────────

// ChooseOption presents a numbered list of options and asks the user to pick
// one by entering its number. Returns the zero-based index of the chosen
// option. Returns (-1, nil) if the user enters nothing or an invalid number.
func ChooseOption(message string, options []string) (int, error) {
	if len(options) == 0 {
		return -1, fmt.Errorf("no options provided")
	}

	// Header.
	headerStyle := HeaderStyle()
	fmt.Printf("\n%s\n\n", headerStyle.Render(message))

	// Numbered list.
	numStyle := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	optStyle := lipgloss.NewStyle().Foreground(ColorText)

	for i, opt := range options {
		fmt.Printf("  %s %s\n",
			numStyle.Render(fmt.Sprintf("%d.", i+1)),
			optStyle.Render(opt),
		)
	}

	// Prompt.
	promptStyle := lipgloss.NewStyle().Foreground(ColorMuted)
	rangeHint := fmt.Sprintf("[1-%d]", len(options))
	fmt.Printf("\n%s %s ",
		promptStyle.Render("  Enter choice"),
		promptStyle.Render(rangeHint+":"),
	)

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return -1, fmt.Errorf("failed to read input: %w", err)
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return -1, nil
	}

	num, err := strconv.Atoi(input)
	if err != nil || num < 1 || num > len(options) {
		errMsg := lipgloss.NewStyle().Foreground(ColorError).
			Render(fmt.Sprintf("  %s Invalid choice: %s", IconError, input))
		fmt.Println(errMsg)
		return -1, nil
	}

	return num - 1, nil
}
