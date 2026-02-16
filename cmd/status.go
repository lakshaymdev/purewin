package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lakshaymaurya-felt/purewin/internal/status"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Monitor system health",
	Long:  "Real-time dashboard with CPU, memory, disk, network, GPU, and battery metrics.",
	Run:   runStatus,
}

func init() {
	statusCmd.Flags().Int("refresh", 1, "Refresh interval in seconds")
	statusCmd.Flags().Bool("json", false, "Output metrics as JSON")
}

func runStatus(cmd *cobra.Command, args []string) {
	jsonMode, _ := cmd.Flags().GetBool("json")
	refreshSecs, _ := cmd.Flags().GetInt("refresh")

	if jsonMode {
		// Single-shot: collect once, print JSON, exit.
		metrics, err := status.CollectMetrics(nil, 0)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		data, _ := json.MarshalIndent(metrics, "", "  ")
		fmt.Println(string(data))
		return
	}

	// Interactive dashboard.
	interval := time.Duration(refreshSecs) * time.Second
	model := status.NewStatusModel(interval)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
