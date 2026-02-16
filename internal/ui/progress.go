package ui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── Spinner Model (Bubbletea) ───────────────────────────────────────────────

// SpinnerModel is a Bubbletea model wrapping the bubbles spinner with a
// customizable message. Ticks at 100ms for smooth animation.
type SpinnerModel struct {
	spinner  spinner.Model
	message  string
	done     bool
	quitting bool
}

// NewSpinner creates a SpinnerModel with the given initial message.
// Uses the braille-dot spinner style in charple purple.
func NewSpinner(message string) SpinnerModel {
	s := spinner.New()
	s.Spinner = spinner.Spinner{
		Frames: SpinnerFrames,
		FPS:    100 * time.Millisecond,
	}
	s.Style = lipgloss.NewStyle().Foreground(ColorPrimary)

	return SpinnerModel{
		spinner: s,
		message: message,
	}
}

// SetMessage updates the spinner's displayed text in-flight.
func (m *SpinnerModel) SetMessage(msg string) {
	m.message = msg
}

// Done marks the spinner as finished so the View clears.
func (m *SpinnerModel) Done() {
	m.done = true
}

// Init starts the spinner tick loop.
func (m SpinnerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update handles spinner tick messages and quit keys.
func (m SpinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case spinnerDoneMsg:
		m.done = true
		return m, tea.Quit
	}

	return m, nil
}

// View renders the spinner + message.
func (m SpinnerModel) View() string {
	if m.done || m.quitting {
		return ""
	}
	msgStyle := lipgloss.NewStyle().Foreground(ColorText)
	return fmt.Sprintf("  %s %s", m.spinner.View(), msgStyle.Render(m.message))
}

// spinnerDoneMsg signals the spinner to stop.
type spinnerDoneMsg struct{}

// SpinnerDone returns a Cmd that stops the spinner.
func SpinnerDone() tea.Msg {
	return spinnerDoneMsg{}
}

// ─── Progress Bar Model (Bubbletea) ──────────────────────────────────────────

// progressTickMsg triggers periodic redraws for the progress bar.
type progressTickMsg time.Time

// ProgressBarModel is a Bubbletea model for a full-featured progress bar with
// percentage, byte counts, and a descriptive label. Width adapts to terminal.
type ProgressBarModel struct {
	bar     progress.Model
	total   int64
	current int64
	label   string
	done    bool
	width   int
}

// NewProgressBar creates a ProgressBarModel for the given total byte count.
// The bar uses a charple→dolly gradient matching the charmtone palette.
func NewProgressBar(total int64, label string) ProgressBarModel {
	p := progress.New(
		progress.WithScaledGradient(string(ColorPrimary.Dark), string(ColorSecondary.Dark)),
		progress.WithWidth(40),
		progress.WithoutPercentage(),
	)

	return ProgressBarModel{
		bar:   p,
		total: total,
		label: label,
		width: 80,
	}
}

// SetCurrent updates the current byte count. Call this before or during
// the Bubbletea event loop.
func (m *ProgressBarModel) SetCurrent(n int64) {
	if n > m.total {
		n = m.total
	}
	m.current = n
}

// SetLabel updates the descriptive text shown alongside the bar.
func (m *ProgressBarModel) SetLabel(label string) {
	m.label = label
}

// percent returns the completion ratio [0.0, 1.0].
func (m ProgressBarModel) percent() float64 {
	if m.total <= 0 {
		return 0
	}
	p := float64(m.current) / float64(m.total)
	if p > 1.0 {
		p = 1.0
	}
	return p
}

// Init starts the periodic tick for redraws.
func (m ProgressBarModel) Init() tea.Cmd {
	return tickProgress()
}

func tickProgress() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return progressTickMsg(t)
	})
}

// Update handles resize, tick, and progress frame messages.
func (m ProgressBarModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		// Reserve space for: 2 padding + percentage + byte counts + label.
		barWidth := msg.Width - 40
		if barWidth < 20 {
			barWidth = 20
		}
		if barWidth > 60 {
			barWidth = 60
		}
		m.bar.Width = barWidth
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.done = true
			return m, tea.Quit
		}

	case progressTickMsg:
		if m.current >= m.total && m.total > 0 {
			m.done = true
			return m, tea.Quit
		}
		return m, tickProgress()

	case progress.FrameMsg:
		model, cmd := m.bar.Update(msg)
		m.bar = model.(progress.Model)
		return m, cmd
	}

	return m, nil
}

// View renders the progress bar with stats.
// Format: [████████░░░░░░░░] 45% │ 2.3 GiB / 5.1 GiB │ Cleaning Chrome cache...
func (m ProgressBarModel) View() string {
	if m.done {
		return ""
	}

	pct := m.percent()

	// For extremely narrow terminals, show only the percentage.
	if m.width < 40 {
		return fmt.Sprintf("  %3d%%", int(pct*100))
	}

	pctStr := fmt.Sprintf("%3d%%", int(pct*100))

	// Byte counts.
	currentSize := FormatSize(m.current)
	totalSize := FormatSize(m.total)

	// Label (truncate if too long).
	label := m.label
	maxLabelWidth := m.width - m.bar.Width - 30
	if maxLabelWidth < 10 {
		maxLabelWidth = 10
	}
	if len(label) > maxLabelWidth {
		if maxLabelWidth <= 1 {
			label = "…"
		} else {
			label = label[:maxLabelWidth-1] + "…"
		}
	}

	pctStyle := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	sepStyle := MutedStyle()
	labelStyle := lipgloss.NewStyle().Foreground(ColorTextDim)

	var b strings.Builder
	b.WriteString("  ")
	b.WriteString(m.bar.ViewAs(pct))
	b.WriteString(" ")
	b.WriteString(pctStyle.Render(pctStr))
	b.WriteString(sepStyle.Render(" │ "))
	b.WriteString(currentSize)
	b.WriteString(sepStyle.Render(" / "))
	b.WriteString(totalSize)

	if label != "" {
		b.WriteString(sepStyle.Render(" │ "))
		b.WriteString(labelStyle.Render(label))
	}

	return b.String()
}

// ─── Inline Spinner (non-Bubbletea) ──────────────────────────────────────────
// For use outside of full TUI mode — updates a single line with \r.

// InlineSpinner is a simple, goroutine-driven spinner that overwrites the
// current terminal line. Safe for use in non-Bubbletea contexts (e.g., during
// sequential CLI operations).
type InlineSpinner struct {
	message  string
	stop     chan struct{}
	done     chan struct{}
	mu       sync.Mutex
	stopOnce sync.Once
}

// NewInlineSpinner creates an InlineSpinner (does not start automatically).
func NewInlineSpinner() *InlineSpinner {
	return &InlineSpinner{
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
}

// Start begins the spinner animation in a background goroutine, displaying
// the given message alongside the rotating frames.
func (s *InlineSpinner) Start(message string) {
	s.mu.Lock()
	s.message = message
	s.mu.Unlock()

	go func() {
		defer close(s.done)
		frameIdx := 0
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-s.stop:
				return
			case <-ticker.C:
				s.mu.Lock()
				msg := s.message
				s.mu.Unlock()

				frame := SpinnerFrames[frameIdx%len(SpinnerFrames)]
				// Use green for the spinner frame.
				coloredFrame := lipgloss.NewStyle().
					Foreground(ColorPrimary).
					Render(frame)

				// \r overwrites the current line; spaces clear any residual text.
				fmt.Printf("\r  %s %s    ", coloredFrame, msg)
				frameIdx++
			}
		}
	}()
}

// UpdateMessage changes the displayed message while the spinner is running.
func (s *InlineSpinner) UpdateMessage(message string) {
	s.mu.Lock()
	s.message = message
	s.mu.Unlock()
}

// Stop halts the spinner and prints a final message on the same line.
func (s *InlineSpinner) Stop(finalMessage string) {
	s.stopOnce.Do(func() { close(s.stop) })
	<-s.done

	// Clear the spinner line and write the final message.
	check := SuccessStyle().Bold(true).Render(IconCheck)

	fmt.Printf("\r  %s %s    \n", check, finalMessage)
}

// StopWithError halts the spinner and prints an error message.
func (s *InlineSpinner) StopWithError(errMessage string) {
	s.stopOnce.Do(func() { close(s.stop) })
	<-s.done

	cross := ErrorStyle().Bold(true).Render(IconCross)

	fmt.Printf("\r  %s %s    \n", cross, errMessage)
}
