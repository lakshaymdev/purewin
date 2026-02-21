package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lakshaymaurya-felt/purewin/internal/analyze"
	"github.com/lakshaymaurya-felt/purewin/internal/ui"
	"github.com/spf13/cobra"
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze [path]",
	Short: "Explore disk usage",
	Long:  "Interactive disk space analyzer with visual tree view.",
	Args:  cobra.MaximumNArgs(1),
	Run:   runAnalyze,
}

func init() {
	analyzeCmd.Flags().Int("depth", 0, "Maximum directory depth to display")
	analyzeCmd.Flags().String("min-size", "", "Minimum size to display (e.g., 100MB)")
	analyzeCmd.Flags().StringSlice("exclude", nil, "Directories to exclude from scan")
}

func runAnalyze(cmd *cobra.Command, args []string) {
	// Determine target path (default: user home).
	target := ""
	if len(args) > 0 {
		target = args[0]
	}
	if target == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		target = home
	}

	// Validate the path exists.
	if _, err := os.Stat(target); err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot access %s: %v\n", target, err)
		os.Exit(1)
	}

	// Parse exclude list.
	exclude, _ := cmd.Flags().GetStringSlice("exclude")

	// Parse depth and min-size flags.
	depth, _ := cmd.Flags().GetInt("depth")
	minSizeStr, _ := cmd.Flags().GetString("min-size")
	minSize := parseMinSize(minSizeStr)

	// Try loading from cache first.
	root, err := analyze.LoadCache(target)
	if err != nil {
		// No valid cache — run a fresh scan with a progress spinner.
		scanner := analyze.NewScanner(8, exclude)

		done := make(chan struct{})
		go func() {
			frame := 0
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					frame = (frame + 1) % len(ui.SpinnerFrames)
					count := scanner.ScannedCount()
					fmt.Fprintf(os.Stderr, "\r  %s Scanning %s … %d entries",
						ui.SpinnerFrames[frame], target, count)
				}
			}
		}()

		root, err = scanner.Scan(target)
		close(done)
		fmt.Fprint(os.Stderr, "\r\033[K") // clear spinner line

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error scanning: %v\n", err)
			os.Exit(1)
		}

		// Persist results for next time.
		_ = analyze.SaveCache(root, target)
	}

	// Launch the TUI.
	model := analyze.NewAnalyzeModel(root, depth, minSize)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// parseMinSize parses a human-readable size string (e.g., "100MB", "1GB") into bytes.
// Returns 0 if the string is empty or invalid.
// parseMinSize parses a human-readable size string (e.g., "100MB", "1GB") into bytes.
// Returns 0 if the string is empty or invalid. Supported suffixes: B, KB, MB, GB, TB.
func parseMinSize(s string) int64 {
	if s == "" {
		return 0
	}
	s = strings.TrimSpace(strings.ToUpper(s))

	var multiplier int64 = 1
	if strings.HasSuffix(s, "TB") {
		multiplier = 1024 * 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "TB")
	} else if strings.HasSuffix(s, "GB") {
		multiplier = 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "GB")
	} else if strings.HasSuffix(s, "MB") {
		multiplier = 1024 * 1024
		s = strings.TrimSuffix(s, "MB")
	} else if strings.HasSuffix(s, "KB") {
		multiplier = 1024
		s = strings.TrimSuffix(s, "KB")
	} else if strings.HasSuffix(s, "B") {
		s = strings.TrimSuffix(s, "B")
	}

	s = strings.TrimSpace(s)

	// Use strconv.ParseFloat for strict parsing — rejects trailing chars like "100M".
	value, err := strconv.ParseFloat(s, 64)
	if err != nil || value < 0 {
		return 0
	}
	return int64(value * float64(multiplier))
}
