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

	// Combine all components
	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		stats,
		"\n",
		graphs,
		help,
	)
}

// renderHeader creates the title bar.
func (m Model) renderHeader(elapsed time.Duration) string {
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#00ff00")).
		Padding(0, 1)

	return style.Render(fmt.Sprintf("🔥 GOBURN [%s/%s] Workers: %d",
		elapsed, m.duration.Round(time.Second), m.workerPool.GetActiveCount()))
}

// renderStats creates the current statistics line.
func (m Model) renderStats() string {
	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#ffffff"))

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00ffff"))

	var s strings.Builder

	// Operations
	s.WriteString(labelStyle.Render("Ops: "))
	s.WriteString(valueStyle.Render(fmt.Sprintf("%d M/s", m.currentOps)))

	// CPU Frequency
	if m.currentStats.CPUFreqMax > 0 {
		s.WriteString(" │ ")
		s.WriteString(labelStyle.Render("CPU: "))
		s.WriteString(valueStyle.Render(fmt.Sprintf("%d MHz (%.0f%%)",
			m.currentStats.CPUFreqCur, m.currentStats.CPUFreqPct)))
	}

	// Temperature
	if m.currentStats.Temperature > 0 {
		s.WriteString(" │ ")
		s.WriteString(labelStyle.Render("Temp: "))
		s.WriteString(valueStyle.Render(fmt.Sprintf("%.1f°C", m.currentStats.Temperature)))
	}

	// Fan speeds
	if len(m.currentStats.FanRPMs) > 0 {
		s.WriteString(" │ ")
		s.WriteString(labelStyle.Render("Fans: "))
		fanStrs := make([]string, len(m.currentStats.FanRPMs))
		for i, rpm := range m.currentStats.FanRPMs {
			fanStrs[i] = fmt.Sprintf("%d", rpm)
		}
		s.WriteString(valueStyle.Render(joinStrings(fanStrs, ",")))
	}

	return s.String()
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
	panelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#444444")).
		Padding(1, 2)

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#ffffff"))

	graphStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ffff00"))

	var g strings.Builder
	g.WriteString(labelStyle.Render(title))
	g.WriteString("\n")

	if len(data) > 1 {
		graph := asciigraph.Plot(data,
			asciigraph.Height(height),
			asciigraph.Width(width),
			asciigraph.LowerBound(minY),
			asciigraph.UpperBound(maxY))
		g.WriteString(graphStyle.Render(graph))
	} else {
		g.WriteString("Waiting for data...")
	}

	return panelStyle.Render(g.String())
}

// renderHelp creates the help text.
func (m Model) renderHelp() string {
	style := lipgloss.NewStyle().
		Faint(true).
		Foreground(lipgloss.Color("#888888")).
		Padding(1, 0, 0, 0)

	return style.Render("Controls: [+] increase workers │ [-] decrease workers │ [q] quit")
}

// calculateGraphDimensions determines optimal graph size based on terminal dimensions.
func (m Model) calculateGraphDimensions() (height, width int) {
	height = 15
	width = 50

	if m.width > 0 && m.height > 0 {
		// Account for UI overhead
		headerLines := 6      // title, stats, blank, help
		panelOverhead := 6    // borders and padding per row
		availableHeight := m.height - headerLines - panelOverhead

		// Divide by 2 for two rows of graphs
		height = (availableHeight / 2) - 3
		if height < 8 {
			height = 8
		}
		if height > 30 {
			height = 30
		}

		// Calculate width for two columns
		panelWidthOverhead := 12
		availableWidth := m.width - panelWidthOverhead
		width = (availableWidth / 2) - 6
		if width < 30 {
			width = 30
		}
		if width > 80 {
			width = 80
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
