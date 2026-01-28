package ui

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/guptarohit/asciigraph"

	"goburn/hardware"
	"goburn/worker"
)

// Model represents the TUI application state.
type Model struct {
	workerPool   *worker.Pool
	lastCounter  uint64
	startTime    time.Time
	duration     time.Duration
	opsHistory   []float64
	cpuHistory   []float64
	tempHistory  []float64
	fanHistory   []float64
	maxPoints    int
	currentStats hardware.Stats
	currentOps   uint64
	maxOps       uint64
	maxFanRPM    int
	width        int
	height       int
}

type tickMsg time.Time

// tickCmd returns a command that sends a tick message every second.
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Init initializes the TUI model and starts the tick loop.
func (m Model) Init() tea.Cmd {
	return tickCmd()
}

// Update handles incoming messages and updates the model state.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tickMsg:
		return m.handleTick()
	}

	return m, nil
}

// handleKeyPress processes keyboard input.
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "+", "=":
		// Increase workers
		m.workerPool.SetWorkers(m.workerPool.GetActiveCount() + 1)

	case "-", "_":
		// Decrease workers (minimum 1)
		newCount := m.workerPool.GetActiveCount() - 1
		if newCount >= 1 {
			m.workerPool.SetWorkers(newCount)
		}
	}

	return m, nil
}

// handleTick updates metrics and checks duration.
func (m Model) handleTick() (tea.Model, tea.Cmd) {
	// Update operation counter
	c := atomic.LoadUint64(m.workerPool.GetCounter())
	m.currentOps = (c - m.lastCounter) / 1_000_000
	m.lastCounter = c

	// Track maximum ops for Y-axis scaling
	if m.currentOps > m.maxOps {
		m.maxOps = m.currentOps
	}

	// Update hardware stats
	m.currentStats = hardware.Get()

	// Track maximum fan RPM for Y-axis scaling
	for _, rpm := range m.currentStats.FanRPMs {
		if rpm > m.maxFanRPM {
			m.maxFanRPM = rpm
		}
	}

	// Update history buffers
	m.updateHistory()

	// Check if duration exceeded
	if time.Since(m.startTime) >= m.duration {
		return m, tea.Quit
	}

	return m, tickCmd()
}

// updateHistory adds current metrics to history buffers.
func (m *Model) updateHistory() {
	// Operations history
	m.opsHistory = append(m.opsHistory, float64(m.currentOps))
	if len(m.opsHistory) > m.maxPoints {
		m.opsHistory = m.opsHistory[1:]
	}

	// CPU frequency history
	if m.currentStats.CPUFreqPct > 0 {
		m.cpuHistory = append(m.cpuHistory, m.currentStats.CPUFreqPct)
		if len(m.cpuHistory) > m.maxPoints {
			m.cpuHistory = m.cpuHistory[1:]
		}
	}

	// Temperature history
	if m.currentStats.Temperature > 0 {
		m.tempHistory = append(m.tempHistory, m.currentStats.Temperature)
		if len(m.tempHistory) > m.maxPoints {
			m.tempHistory = m.tempHistory[1:]
		}
	}

	// Fan speed history (averaged across all fans)
	if len(m.currentStats.FanRPMs) > 0 {
		avgFan := 0.0
		for _, rpm := range m.currentStats.FanRPMs {
			avgFan += float64(rpm)
		}
		avgFan /= float64(len(m.currentStats.FanRPMs))

		m.fanHistory = append(m.fanHistory, avgFan)
		if len(m.fanHistory) > m.maxPoints {
			m.fanHistory = m.fanHistory[1:]
		}
	}
}

// View renders the TUI.
func (m Model) View() string {
	elapsed := time.Since(m.startTime).Round(time.Second)

	// Build components
	header := m.renderHeader(elapsed)
	stats := m.renderStats()
	graphs := m.renderGraphs()
	help := m.renderHelp()

	// Combine all components with minimal spacing
	content := lipgloss.JoinVertical(lipgloss.Left,
		header,
		"\n",
		stats,
		"\n",
		graphs,
		"\n",
		help,
	)

	return content
}

// renderHeader creates the title bar.
func (m Model) renderHeader(elapsed time.Duration) string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF6B35")).
		Background(lipgloss.Color("#1a1a1a")).
		Padding(0, 1)

	// Progress bar
	progress := float64(elapsed) / float64(m.duration)
	progressBar := m.renderProgressBar(progress)

	title := titleStyle.Render(fmt.Sprintf("🔥 GOBURN"))

	timeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7EC8E3")).
		Background(lipgloss.Color("#1a1a1a")).
		Padding(0, 2)

	workerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#98D8C8")).
		Background(lipgloss.Color("#1a1a1a")).
		Padding(0, 2)

	timeInfo := timeStyle.Render(fmt.Sprintf("⏱  %s / %s", elapsed, m.duration.Round(time.Second)))
	workerInfo := workerStyle.Render(fmt.Sprintf("⚙  %d workers", m.workerPool.GetActiveCount()))

	topLine := lipgloss.JoinHorizontal(lipgloss.Center, title, timeInfo, workerInfo)

	headerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#FF6B35")).
		Padding(0, 1)

	return headerStyle.Render(lipgloss.JoinVertical(lipgloss.Left, topLine, progressBar))
}

// renderProgressBar creates a gradient progress bar.
func (m Model) renderProgressBar(progress float64) string {
	if progress > 1.0 {
		progress = 1.0
	}

	barWidth := 60
	if m.width > 80 {
		barWidth = m.width - 30
	}

	filled := int(float64(barWidth) * progress)
	empty := barWidth - filled

	filledStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF87"))
	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#444444"))

	bar := filledStyle.Render(strings.Repeat("█", filled)) +
		  emptyStyle.Render(strings.Repeat("░", empty))

	percentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true)

	return bar + " " + percentStyle.Render(fmt.Sprintf("%.0f%%", progress*100))
}

// renderStats creates the current statistics line.
func (m Model) renderStats() string {
	// Create stat cards with color-coded values
	opsCard := m.createStatCard("⚡", "Operations", fmt.Sprintf("%d M/s", m.currentOps), "#FFD700")

	var cpuCard, tempCard, fanCard string

	if m.currentStats.CPUFreqMax > 0 {
		cpuColor := getPercentageColor(m.currentStats.CPUFreqPct)
		cpuCard = m.createStatCard("🖥", "CPU Freq", fmt.Sprintf("%d MHz", m.currentStats.CPUFreqCur), cpuColor)
	}

	if m.currentStats.Temperature > 0 {
		tempColor := getTempColor(m.currentStats.Temperature)
		tempCard = m.createStatCard("🌡", "Temp", fmt.Sprintf("%.1f°C", m.currentStats.Temperature), tempColor)
	}

	if len(m.currentStats.FanRPMs) > 0 {
		avgRPM := 0
		for _, rpm := range m.currentStats.FanRPMs {
			avgRPM += rpm
		}
		avgRPM /= len(m.currentStats.FanRPMs)
		fanCard = m.createStatCard("🌀", "Fan Avg", fmt.Sprintf("%d RPM", avgRPM), "#00CED1")
	}

	cards := []string{opsCard}
	if cpuCard != "" {
		cards = append(cards, cpuCard)
	}
	if tempCard != "" {
		cards = append(cards, tempCard)
	}
	if fanCard != "" {
		cards = append(cards, fanCard)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, cards...)
}

// createStatCard creates a styled stat card.
func (m Model) createStatCard(icon, label, value, color string) string {
	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(color)).
		Padding(0, 1).
		Margin(0, 0, 0, 1)

	iconStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(color)).
		Bold(true)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888"))

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(color)).
		Bold(true)

	content := fmt.Sprintf("%s %s\n%s",
		iconStyle.Render(icon),
		labelStyle.Render(label),
		valueStyle.Render(value))

	return cardStyle.Render(content)
}

// getPercentageColor returns color based on percentage value.
func getPercentageColor(pct float64) string {
	if pct < 50 {
		return "#00FF87"
	} else if pct < 75 {
		return "#FFD700"
	}
	return "#FF6B35"
}

// getTempColor returns color based on temperature.
func getTempColor(temp float64) string {
	if temp < 50 {
		return "#00FF87"
	} else if temp < 70 {
		return "#FFD700"
	} else if temp < 85 {
		return "#FF8C00"
	}
	return "#FF0000"
}

// renderGraphs creates the 2x2 graph panel layout.
func (m Model) renderGraphs() string {
	graphHeight, graphWidth := m.calculateGraphDimensions()

	// Calculate Y-axis bounds for each graph
	maxOpsY := float64(m.maxOps) * 1.2
	if maxOpsY < 100 {
		maxOpsY = 100
	}

	maxFanY := float64(m.maxFanRPM) * 1.2
	if maxFanY < 6000 {
		maxFanY = 6000
	}

	// Create individual graphs
	graph1 := m.renderGraph("Operations (M/s)", m.opsHistory, 0, maxOpsY, graphHeight, graphWidth)
	graph2 := m.renderGraph("CPU Frequency (%)", m.cpuHistory, 0, 100.0, graphHeight, graphWidth)
	graph3 := m.renderGraph("Temperature (°C)", m.tempHistory, 0, 100.0, graphHeight, graphWidth)
	graph4 := m.renderGraph("Fan Speed (RPM avg)", m.fanHistory, 0, maxFanY, graphHeight, graphWidth)

	// Layout in 2x2 grid
	row1 := lipgloss.JoinHorizontal(lipgloss.Top, graph1, graph2)
	row2 := lipgloss.JoinHorizontal(lipgloss.Top, graph3, graph4)

	return lipgloss.JoinVertical(lipgloss.Left, row1, row2)
}

// renderGraph creates a single graph panel.
func (m Model) renderGraph(title string, data []float64, minY, maxY float64, height, width int) string {
	// Determine color based on graph type
	var borderColor, graphColor string
	switch {
	case strings.Contains(title, "Operations"):
		borderColor = "#FFD700"
		graphColor = "#FFD700"
	case strings.Contains(title, "CPU"):
		borderColor = "#7EC8E3"
		graphColor = "#00CED1"
	case strings.Contains(title, "Temperature"):
		// Dynamic color based on current temp
		if len(data) > 0 {
			currentTemp := data[len(data)-1]
			borderColor = getTempColor(currentTemp)
			graphColor = getTempColor(currentTemp)
		} else {
			borderColor = "#00FF87"
			graphColor = "#00FF87"
		}
	case strings.Contains(title, "Fan"):
		borderColor = "#98D8C8"
		graphColor = "#5FD7FF"
	default:
		borderColor = "#888888"
		graphColor = "#FFFFFF"
	}

	// Fixed width for consistent panel sizing
	panelWidth := width + 10
	panelStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color(borderColor)).
		Padding(1, 1).
		Width(panelWidth).
		Height(height + 8) // Fixed height for alignment

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(borderColor)).
		Background(lipgloss.Color("#1a1a1a")).
		Padding(0, 1)

	graphStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(graphColor))

	var g strings.Builder
	g.WriteString(labelStyle.Render(title))
	g.WriteString("\n\n")

	if len(data) > 1 {
		graph := asciigraph.Plot(data,
			asciigraph.Height(height),
			asciigraph.Width(width),
			asciigraph.LowerBound(minY),
			asciigraph.UpperBound(maxY))
		g.WriteString(graphStyle.Render(graph))

		// Add current value indicator
		currentVal := data[len(data)-1]
		currentStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(graphColor)).
			Bold(true).
			MarginTop(1)
		g.WriteString("\n")
		g.WriteString(currentStyle.Render(fmt.Sprintf("▶ %.1f", currentVal)))
	} else {
		waitStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666666")).
			Italic(true)
		g.WriteString(waitStyle.Render("⏳ Collecting data..."))
	}

	return panelStyle.Render(g.String())
}

// renderHelp creates the help text.
func (m Model) renderHelp() string {
	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF6B35")).
		Bold(true).
		Padding(0, 1)

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888"))

	dividerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#444444"))

	help := lipgloss.JoinHorizontal(lipgloss.Center,
		keyStyle.Render("+"),
		descStyle.Render("increase"),
		dividerStyle.Render(" • "),
		keyStyle.Render("-"),
		descStyle.Render("decrease"),
		dividerStyle.Render(" • "),
		keyStyle.Render("q"),
		descStyle.Render("quit"),
	)

	containerStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#444444")).
		Padding(0, 1)

	return containerStyle.Render(help)
}

// calculateGraphDimensions determines optimal graph size based on terminal dimensions.
func (m Model) calculateGraphDimensions() (height, width int) {
	height = 15
	width = 50

	if m.width > 0 && m.height > 0 {
		// Account for UI overhead (header + stats + help + spacing)
		uiOverhead := 14      // header, stats, help lines + spacing
		panelBorderHeight := 8 // borders and padding per panel
		availableHeight := m.height - uiOverhead

		// Divide by 2 for two rows of graphs
		height = (availableHeight / 2) - panelBorderHeight
		if height < 6 {
			height = 6
		}
		if height > 20 {
			height = 20
		}

		// Calculate width for two columns
		panelBorderWidth := 8  // borders and padding per panel
		availableWidth := m.width
		width = (availableWidth / 2) - panelBorderWidth
		if width < 30 {
			width = 30
		}
		if width > 70 {
			width = 70
		}
	}

	return height, width
}

// RunGraphMode starts the interactive TUI.
func RunGraphMode(counter *uint64, duration time.Duration, startTime time.Time, initialWorkers int) {
	wp := worker.New(counter, initialWorkers)

	m := Model{
		workerPool:  wp,
		lastCounter: 0,
		startTime:   startTime,
		duration:    duration,
		maxPoints:   60,
		maxOps:      10,
		maxFanRPM:   1000,
		width:       120,
		height:      30,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
