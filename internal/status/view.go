package status

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/lakshaymaurya-felt/purewin/internal/core"
	"github.com/lakshaymaurya-felt/purewin/internal/ui"
)

// ─── Reusable Styles ─────────────────────────────────────────────────────────
// Use ui package colors for consistency across all views.

// Module-level style vars (safe: lipgloss styles are immutable copies).
var (
	textStyle   = lipgloss.NewStyle().Foreground(ui.ColorText)
	dimStyle    = lipgloss.NewStyle().Foreground(ui.ColorMuted)
	subtleStyle = lipgloss.NewStyle().Foreground(ui.ColorTextDim)
	accentStyle = lipgloss.NewStyle().Foreground(ui.ColorPrimary)
	altStyle    = lipgloss.NewStyle().Foreground(ui.ColorTeal)
)

// ─── Top-level renderer ─────────────────────────────────────────────────────

func (m StatusModel) renderView() string {
	w := m.Width
	if w < 50 {
		w = 50
	}

	var s strings.Builder
	s.WriteString(m.renderTabs(w))
	s.WriteString("\n")

	if m.Metrics == nil {
		s.WriteString("\n")
		s.WriteString(dimStyle.Italic(true).Render("  Collecting metrics..."))
		s.WriteString("\n")
		return s.String()
	}

	switch m.Tab {
	case TabOverview:
		s.WriteString(m.renderOverview(w))
	case TabCPU:
		s.WriteString(m.renderCPU(w))
	case TabMemory:
		s.WriteString(m.renderMemory(w))
	case TabDisk:
		s.WriteString(m.renderDisk(w))
	case TabNetwork:
		s.WriteString(m.renderNetwork(w))
	case TabProcesses:
		s.WriteString(m.renderProcesses(w))
	}

	s.WriteString("\n")
	s.WriteString(m.renderStatusFooter())
	return s.String()
}

// ─── Tab bar ─────────────────────────────────────────────────────────────────

func (m StatusModel) renderTabs(w int) string {
	activeTab := lipgloss.NewStyle().
		Foreground(ui.ColorText).
		Bold(true).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(ui.ColorPrimary).
		Padding(0, 2)

	inactiveTab := lipgloss.NewStyle().
		Foreground(ui.ColorMuted).
		Padding(0, 2)

	dotStyle := lipgloss.NewStyle().Foreground(ui.ColorSecondary)

	var tabs []string
	for i, name := range TabNames {
		var label string
		if Tab(i) == m.Tab {
			label = fmt.Sprintf("%s %d·%s", dotStyle.Render(ui.IconDot), i+1, name)
			tabs = append(tabs, activeTab.Render(label))
		} else {
			label = fmt.Sprintf("  %d·%s", i+1, name)
			tabs = append(tabs, inactiveTab.Render(label))
		}
	}

	bar := lipgloss.JoinHorizontal(lipgloss.Bottom, tabs...)
	rule := ui.Divider(w)

	return bar + "\n" + rule
}

// ─── Overview tab ────────────────────────────────────────────────────────────

func (m StatusModel) renderOverview(w int) string {
	met := m.Metrics
	score := HealthScore(met)

	var s strings.Builder
	s.WriteString("\n")

	// ── Health Score (tag style) ──
	scoreLabel := "Excellent"
	scoreTag := ui.TagAccentStyle()
	switch {
	case score < 50:
		scoreLabel = "Critical"
		scoreTag = ui.TagErrorStyle()
	case score < 70:
		scoreLabel = "Fair"
		scoreTag = ui.TagWarningStyle()
	case score < 90:
		scoreLabel = "Good"
	}
	s.WriteString(fmt.Sprintf("  %s  %s\n",
		scoreTag.Render(fmt.Sprintf(" %d ", score)),
		dimStyle.Render(scoreLabel)))
	s.WriteString("\n")

	// ── System ──
	s.WriteString("  " + ui.SectionHeader("System", w-4) + "\n")
	hw := met.Hardware
	hwLine1 := fmt.Sprintf("  %s  %s  %s",
		textStyle.Render(hw.Hostname),
		dimStyle.Render("·"),
		subtleStyle.Render(fmt.Sprintf("%s %s", hw.OS, hw.OSVersion)))
	hwLine2Parts := []string{
		subtleStyle.Render(hw.CPUModel),
		subtleStyle.Render(fmt.Sprintf("%d cores", hw.CPUCores)),
		subtleStyle.Render(core.FormatSize(int64(hw.RAMTotal)) + " RAM"),
	}
	if met.GPU.Name != "" {
		hwLine2Parts = append(hwLine2Parts, subtleStyle.Render(met.GPU.Name))
	}
	hwLine2 := "  " + strings.Join(hwLine2Parts, dimStyle.Render("  ·  "))

	s.WriteString(hwLine1 + "\n")
	s.WriteString(hwLine2 + "\n")

	if met.Battery.HasBattery {
		batt := fmt.Sprintf("%d%%", met.Battery.Charge)
		if met.Battery.IsCharging {
			batt += " charging"
		}
		s.WriteString(fmt.Sprintf("  %s %s\n",
			dimStyle.Render("Battery"),
			subtleStyle.Render(batt)))
	}

	s.WriteString("\n")

	// ── Resources ──
	s.WriteString("  " + ui.SectionHeader("Resources", w-4) + "\n")
	barW := 20
	graphW := 30
	if w > 110 {
		barW = 28
		graphW = 40
	} else if w > 90 {
		barW = 24
		graphW = 35
	}

	// CPU with line graph
	s.WriteString(renderMetricRow("CPU", met.CPU.TotalPercent, barW, ""))
	if len(m.CPUHistory) > 1 {
		s.WriteString(renderLineGraph(m.CPUHistory, graphW, 6, ui.ColorPrimary, ""))
	}
	s.WriteString("\n")

	// Memory with line graph
	s.WriteString(renderMetricRow("MEM", met.Memory.UsedPercent, barW,
		fmt.Sprintf("%s / %s",
			core.FormatSize(int64(met.Memory.Used)),
			core.FormatSize(int64(met.Memory.Total)))))
	if len(m.MemHistory) > 1 {
		s.WriteString(renderLineGraph(m.MemHistory, graphW, 6, ui.ColorSecondary, ""))
	}
	s.WriteString("\n")

	// Disk
	if len(met.Disk.Partitions) > 0 {
		p := met.Disk.Partitions[0]
		s.WriteString(renderMetricRow("DSK", p.UsedPercent, barW,
			fmt.Sprintf("%s / %s  %s",
				core.FormatSize(int64(p.Used)),
				core.FormatSize(int64(p.Total)),
				dimStyle.Render(p.Path))))
		s.WriteString("\n")
	}

	// Network
	dlStyle := lipgloss.NewStyle().Foreground(ui.ColorTeal)
	ulStyle := lipgloss.NewStyle().Foreground(ui.ColorAccent)
	netDown := formatSpeed(met.Network.RecvSpeed)
	netUp := formatSpeed(met.Network.SendSpeed)
	s.WriteString(fmt.Sprintf("  %s  %s %s  %s %s\n",
		dimStyle.Render("NET    "),
		dlStyle.Render(ui.IconArrow),
		textStyle.Render(netDown),
		ulStyle.Render(ui.IconArrow),
		textStyle.Render(netUp)))

	if len(m.NetRecvHistory) > 1 {
		s.WriteString(fmt.Sprintf("  %s  %s  %s\n",
			dimStyle.Render("       "),
			renderSparklineU64(m.NetRecvHistory, graphW/2, ui.ColorTeal),
			renderSparklineU64(m.NetSendHistory, graphW/2, ui.ColorAccent)))
	}

	return s.String()
}

// renderMetricRow renders a single metric: label + bar + percent + optional detail.
func renderMetricRow(label string, pct float64, barW int, detail string) string {
	bar := ui.GradientBar(pct, barW)
	pctStr := textStyle.Render(fmt.Sprintf("%5.1f%%", pct))

	line := fmt.Sprintf("  %s  %s  %s",
		dimStyle.Render(fmt.Sprintf("%-7s", label)),
		bar,
		pctStr)

	if detail != "" {
		line += "  " + subtleStyle.Render(detail)
	}

	return line + "\n"
}

// ─── CPU tab ─────────────────────────────────────────────────────────────────

func (m StatusModel) renderCPU(w int) string {
	met := m.Metrics
	barW := 40
	if w > 110 {
		barW = 56
	}

	var lines []string
	lines = append(lines, "")

	// ── Total ──
	lines = append(lines, "  "+ui.SectionHeader("Total", barW+20))
	totalLabel := accentStyle.Bold(true).Render("CPU")
	totalPct := textStyle.Render(fmt.Sprintf("%5.1f%%", met.CPU.TotalPercent))
	lines = append(lines,
		fmt.Sprintf("  %s  %s  %s", totalLabel, ui.GradientBar(met.CPU.TotalPercent, barW), totalPct))
	lines = append(lines, "")

	// Line graph history.
	if len(m.CPUHistory) > 1 {
		lines = append(lines, renderLineGraph(m.CPUHistory, 40, 8, ui.ColorPrimary, "CPU History"))
	}

	// ── Per Core ──
	lines = append(lines, "  "+ui.SectionHeader("Per Core", barW+20))
	for i, pct := range met.CPU.PerCore {
		coreBar := ui.GradientBar(pct, barW-10)
		lines = append(lines,
			fmt.Sprintf("  %s  %s  %s",
				dimStyle.Render(fmt.Sprintf("Core %-2d", i)),
				coreBar,
				textStyle.Render(fmt.Sprintf("%5.1f%%", pct))))
	}

	return strings.Join(lines, "\n")
}

// ─── Memory tab ──────────────────────────────────────────────────────────────

func (m StatusModel) renderMemory(w int) string {
	met := m.Metrics
	barW := 40
	if w > 110 {
		barW = 56
	}

	ml := dimStyle    // label
	mv := accentStyle // value
	mp := textStyle   // percent

	var lines []string
	lines = append(lines, "")

	// ── Physical ──
	lines = append(lines, "  "+ui.SectionHeader("Physical", barW+20))
	lines = append(lines,
		fmt.Sprintf("  %s  %s  %s",
			ml.Bold(true).Render("Used      "),
			ui.GradientBar(met.Memory.UsedPercent, barW),
			mp.Render(fmt.Sprintf("%5.1f%%", met.Memory.UsedPercent))))
	lines = append(lines, "")

	// Line graph history.
	if len(m.MemHistory) > 1 {
		lines = append(lines, renderLineGraph(m.MemHistory, 40, 8, ui.ColorSecondary, "Memory History"))
	}
	lines = append(lines,
		fmt.Sprintf("  %s  %s", ml.Render("Total     "), mv.Render(core.FormatSize(int64(met.Memory.Total)))))
	lines = append(lines,
		fmt.Sprintf("  %s  %s", ml.Render("Used      "), mv.Render(core.FormatSize(int64(met.Memory.Used)))))
	lines = append(lines,
		fmt.Sprintf("  %s  %s", ml.Render("Available "), mv.Render(core.FormatSize(int64(met.Memory.Available)))))
	lines = append(lines,
		fmt.Sprintf("  %s  %s", ml.Render("Free      "), mv.Render(core.FormatSize(int64(met.Memory.Free)))))

	if met.Memory.SwapTotal > 0 {
		lines = append(lines, "")
		// ── Swap ──
		lines = append(lines, "  "+ui.SectionHeader("Swap", barW+20))
		lines = append(lines,
			fmt.Sprintf("  %s  %s  %s",
				ml.Bold(true).Render("Used      "),
				ui.GradientBar(met.Memory.SwapPercent, barW),
				mp.Render(fmt.Sprintf("%5.1f%%", met.Memory.SwapPercent))))
		lines = append(lines,
			fmt.Sprintf("  %s  %s / %s",
				ml.Render("Swap Used "),
				mv.Render(core.FormatSize(int64(met.Memory.SwapUsed))),
				mv.Render(core.FormatSize(int64(met.Memory.SwapTotal)))))
	}

	return strings.Join(lines, "\n")
}

// ─── Disk tab ────────────────────────────────────────────────────────────────

func (m StatusModel) renderDisk(w int) string {
	met := m.Metrics
	barW := 36
	if w > 110 {
		barW = 48
	}

	dl := accentStyle.Bold(true) // drive label
	dp := textStyle              // percent
	dv := subtleStyle            // size values

	var lines []string
	lines = append(lines, "")

	for _, p := range met.Disk.Partitions {
		lines = append(lines,
			fmt.Sprintf("  %s %s  %s  %s / %s",
				dl.Render(fmt.Sprintf("%-4s", p.Path)),
				ui.GradientBar(p.UsedPercent, barW),
				dp.Render(fmt.Sprintf("%5.1f%%", p.UsedPercent)),
				dv.Render(core.FormatSize(int64(p.Used))),
				dv.Render(core.FormatSize(int64(p.Total)))))
	}

	lines = append(lines, "")
	rdLabel := lipgloss.NewStyle().Foreground(ui.ColorTeal).Render(ui.IconArrow + " Read")
	wrLabel := lipgloss.NewStyle().Foreground(ui.ColorWarning).Render(ui.IconArrow + " Write")
	lines = append(lines,
		fmt.Sprintf("  %s   %s   %s  %s",
			rdLabel, dv.Render(core.FormatSize(int64(met.Disk.ReadBytes))),
			wrLabel, dv.Render(core.FormatSize(int64(met.Disk.WriteBytes)))))

	return strings.Join(lines, "\n")
}

// ─── Network tab ─────────────────────────────────────────────────────────────

func (m StatusModel) renderNetwork(w int) string {
	met := m.Metrics
	dlStyle := lipgloss.NewStyle().Foreground(ui.ColorTeal)
	ulStyle := lipgloss.NewStyle().Foreground(ui.ColorAccent)

	var lines []string
	lines = append(lines, "")

	lines = append(lines,
		fmt.Sprintf("  %s %s  %s",
			dlStyle.Render(ui.IconArrow), dlStyle.Render("Download"),
			textStyle.Render(formatSpeed(met.Network.RecvSpeed))))
	lines = append(lines,
		fmt.Sprintf("  %s %s    %s",
			ulStyle.Render(ui.IconArrow), ulStyle.Render("Upload"),
			textStyle.Render(formatSpeed(met.Network.SendSpeed))))

	lines = append(lines, "")
	lines = append(lines,
		fmt.Sprintf("  %s  %s", dimStyle.Render("Total Recv"), subtleStyle.Render(core.FormatSize(int64(met.Network.BytesRecv)))))
	lines = append(lines,
		fmt.Sprintf("  %s  %s", dimStyle.Render("Total Sent"), subtleStyle.Render(core.FormatSize(int64(met.Network.BytesSent)))))

	// Sparklines.
	if len(m.NetRecvHistory) > 1 {
		lines = append(lines, "")
		lines = append(lines,
			dlStyle.Render("  "+ui.IconArrow+" ")+renderSparklineU64(m.NetRecvHistory, 30, ui.ColorTeal))
		lines = append(lines,
			ulStyle.Render("  "+ui.IconArrow+" ")+renderSparklineU64(m.NetSendHistory, 30, ui.ColorAccent))
	}

	return strings.Join(lines, "\n")
}

// ─── Processes tab ───────────────────────────────────────────────────────────

func (m StatusModel) renderProcesses(w int) string {
	met := m.Metrics
	barW := 24
	if w > 100 {
		barW = 32
	}

	var lines []string
	lines = append(lines, "")
	lines = append(lines, "  "+ui.SectionHeader("Top Processes", w-4))
	lines = append(lines, "")

	nameW := 22
	if w > 100 {
		nameW = 30
	}

	header := fmt.Sprintf("  %-6s %-*s %s  %6s  %6s", "PID", nameW, "Name", strings.Repeat(" ", barW), "CPU%", "Mem%")
	lines = append(lines, dimStyle.Render(header))
	lines = append(lines, "  "+ui.Divider(w-4))

	for _, p := range met.TopProcs {
		name := p.Name
		if len(name) > nameW {
			name = name[:nameW-1] + "…"
		}
		cpuClamp := p.CPUPct
		if cpuClamp > 100 {
			cpuClamp = 100
		}
		bar := ui.GradientBar(cpuClamp, barW)
		lines = append(lines,
			fmt.Sprintf("  %s %s %s  %s  %s",
				subtleStyle.Render(fmt.Sprintf("%-6d", p.PID)),
				textStyle.Render(fmt.Sprintf("%-*s", nameW, name)),
				bar,
				textStyle.Render(fmt.Sprintf("%5.1f%%", p.CPUPct)),
				subtleStyle.Render(fmt.Sprintf("%5.1f%%", p.MemPct))))
	}

	if len(met.TopProcs) == 0 {
		lines = append(lines,
			dimStyle.Italic(true).Render("  (no process data yet)"))
	}

	return strings.Join(lines, "\n")
}

// ─── Footer ──────────────────────────────────────────────────────────────────

func (m StatusModel) renderStatusFooter() string {
	hints := "  Tab/Shift-Tab switch  " + ui.IconPipe + "  1-6 jump  " + ui.IconPipe + "  q quit"
	footer := ui.HintBarStyle().Render(hints)

	if m.Err != nil {
		errStr := lipgloss.NewStyle().
			Foreground(ui.ColorError).
			Render("  " + ui.IconError + " " + m.Err.Error())
		return errStr + "\n" + footer
	}
	return footer
}

// ─── Drawing primitives ─────────────────────────────────────────────────────

// renderSparkline renders a mini chart from float64 data using block chars.
func renderSparkline(data []float64, width int, color lipgloss.AdaptiveColor) string {
	blocks := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	var maxVal float64
	for _, v := range data {
		if v > maxVal {
			maxVal = v
		}
	}
	if maxVal == 0 {
		maxVal = 1
	}
	d := data
	if len(d) > width {
		d = d[len(d)-width:]
	}
	var b strings.Builder
	for _, v := range d {
		idx := int(v / maxVal * 7)
		if idx > 7 {
			idx = 7
		}
		if idx < 0 {
			idx = 0
		}
		b.WriteRune(blocks[idx])
	}
	for i := len(d); i < width; i++ {
		b.WriteRune(blocks[0])
	}
	return lipgloss.NewStyle().Foreground(color).Render(b.String())
}

// renderSparklineU64 renders a mini chart from uint64 data.
func renderSparklineU64(data []uint64, width int, color lipgloss.AdaptiveColor) string {
	blocks := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	var maxVal uint64
	for _, v := range data {
		if v > maxVal {
			maxVal = v
		}
	}
	if maxVal == 0 {
		maxVal = 1
	}
	d := data
	if len(d) > width {
		d = d[len(d)-width:]
	}
	var b strings.Builder
	for _, v := range d {
		idx := int(float64(v) / float64(maxVal) * 7)
		if idx > 7 {
			idx = 7
		}
		if idx < 0 {
			idx = 0
		}
		b.WriteRune(blocks[idx])
	}
	for i := len(d); i < width; i++ {
		b.WriteRune(blocks[0])
	}
	return lipgloss.NewStyle().Foreground(color).Render(b.String())
}

// formatSpeed returns a human-readable bytes/sec string.
func formatSpeed(bps uint64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case bps >= gb:
		return fmt.Sprintf("%.1f GB/s", float64(bps)/float64(gb))
	case bps >= mb:
		return fmt.Sprintf("%.1f MB/s", float64(bps)/float64(mb))
	case bps >= kb:
		return fmt.Sprintf("%.1f KB/s", float64(bps)/float64(kb))
	default:
		return fmt.Sprintf("%d B/s", bps)
	}
}

// ─── Line Graph ──────────────────────────────────────────────────────────────

// renderLineGraph renders a proper ASCII line graph with Y-axis labels, graph
// area using block characters, and time-based X-axis markers.
func renderLineGraph(data []float64, width, height int, color lipgloss.AdaptiveColor, label string) string {
	if len(data) == 0 || width < 10 || height < 3 {
		return ""
	}

	blocks := []rune{' ', '▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	yAxisW := 6 // "100% │" = 6 chars
	graphW := width - yAxisW
	if graphW < 5 {
		graphW = 5
	}

	// Resample data to fit graphW columns.
	sampled := resampleData(data, graphW)

	// Determine value range — for percentage data, cap at 100.
	maxVal := 0.0
	for _, v := range sampled {
		if v > maxVal {
			maxVal = v
		}
	}
	if maxVal <= 0 {
		maxVal = 1
	}
	if maxVal <= 100 {
		maxVal = 100 // Use 100% scale for percentage data
	}

	graphStyle := lipgloss.NewStyle().Foreground(color)
	axisStyle := dimStyle
	borderStyle := lipgloss.NewStyle().Foreground(ui.ColorBorder)

	var lines []string

	// Optional label above graph.
	if label != "" {
		lines = append(lines, "  "+graphStyle.Bold(true).Render("  "+label))
	}

	// Render rows top-to-bottom (row 0 = top = highest value).
	for row := 0; row < height; row++ {
		// Y-axis label: show at top, middle, bottom.
		var yLabel string
		switch {
		case row == 0:
			yLabel = fmt.Sprintf("%3.0f%%", maxVal)
		case row == height/2:
			yLabel = fmt.Sprintf("%3.0f%%", maxVal/2)
		case row == height-1:
			yLabel = "  0%"
		default:
			yLabel = "    "
		}

		// Row threshold: what value range does this row represent?
		rowTop := maxVal * float64(height-row) / float64(height)
		rowBot := maxVal * float64(height-row-1) / float64(height)

		var rowBuf strings.Builder
		for _, v := range sampled {
			if v >= rowTop {
				// Full block.
				rowBuf.WriteRune(blocks[8])
			} else if v > rowBot {
				// Partial block: map the fractional part into block indices.
				frac := (v - rowBot) / (rowTop - rowBot)
				idx := int(math.Round(frac * 8))
				if idx < 0 {
					idx = 0
				}
				if idx > 8 {
					idx = 8
				}
				rowBuf.WriteRune(blocks[idx])
			} else {
				rowBuf.WriteRune(' ')
			}
		}

		// Pad to graphW.
		rowStr := rowBuf.String()
		for len([]rune(rowStr)) < graphW {
			rowStr += " "
		}

		lines = append(lines, fmt.Sprintf("  %s %s%s",
			axisStyle.Render(yLabel),
			borderStyle.Render("│"),
			graphStyle.Render(rowStr)))
	}

	// X-axis bottom border.
	xBorder := "  " + axisStyle.Render("     ") + borderStyle.Render("└"+strings.Repeat("─", graphW))
	lines = append(lines, xBorder)

	// X-axis time labels.
	xLabels := make([]byte, graphW)
	for i := range xLabels {
		xLabels[i] = ' '
	}
	// Place "now" at the right edge.
	nowLabel := "now"
	if graphW >= len(nowLabel)+1 {
		pos := graphW - len(nowLabel)
		copy(xLabels[pos:], nowLabel)
	}
	// Place time markers at intervals.
	if len(data) > 1 && graphW > 15 {
		midIdx := graphW / 2
		secPerSample := 1 // Typically 1 second per sample
		midSec := (graphW - midIdx) * secPerSample
		midLabel := fmt.Sprintf("-%ds", midSec)
		if midIdx >= len(midLabel) {
			start := midIdx - len(midLabel)/2
			if start >= 0 && start+len(midLabel) < graphW-4 {
				copy(xLabels[start:], midLabel)
			}
		}
	}
	lines = append(lines, "  "+axisStyle.Render("      ")+axisStyle.Render(string(xLabels)))

	return strings.Join(lines, "\n") + "\n"
}

// resampleData reduces or pads data to exactly targetLen points.
func resampleData(data []float64, targetLen int) []float64 {
	n := len(data)
	if n == 0 {
		return make([]float64, targetLen)
	}
	if n <= targetLen {
		// Pad with zeros on the left (older data = empty).
		result := make([]float64, targetLen)
		offset := targetLen - n
		copy(result[offset:], data)
		return result
	}
	// Downsample: take the last targetLen points.
	return data[n-targetLen:]
}
