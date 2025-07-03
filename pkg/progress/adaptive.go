package progress

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
)

// AdaptiveProgress provides beautiful progress indication that works in both TTY and non-TTY environments
type AdaptiveProgress struct {
	isTTY     bool
	renderer  *lipgloss.Renderer
	program   *tea.Program
	model     adaptiveModel
	output    io.Writer
	logOutput io.Writer
	startTime time.Time
}

// adaptiveModel represents the TUI model for progress indication
type adaptiveModel struct {
	spinner      spinner.Model
	progress     progress.Model
	message      string
	phase        string
	details      string
	percent      float64
	done         bool
	err          error
	showSpinner  bool
	showBar      bool
	phases       []PhaseInfo
	currentPhase int
	startTime    time.Time
}

// PhaseInfo represents a phase of the analysis process
type PhaseInfo struct {
	Name        string
	Description string
	Weight      float64 // Relative weight for progress calculation
}

// AnalysisStats contains statistics from the analysis
type AnalysisStats struct {
	Files         int
	Nodes         int
	Relationships int
	Interfaces    int
	CallChains    int
	ProjectID     string
}

// NewAdaptiveProgress creates a new adaptive progress indicator
func NewAdaptiveProgress(output io.Writer) *AdaptiveProgress {
	isTTY := isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())

	var logOutput io.Writer
	if isTTY {
		// In TTY mode, suppress all external logging to avoid conflicts with TUI
		logOutput = io.Discard
	} else {
		// In non-TTY mode, log to stderr
		logOutput = os.Stderr
	}

	ap := &AdaptiveProgress{
		isTTY:     isTTY,
		renderer:  lipgloss.NewRenderer(output),
		output:    output,
		logOutput: logOutput,
		startTime: time.Now(),
	}

	// Create the model
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = ap.getSpinnerStyle()

	progressBar := progress.New(progress.WithDefaultGradient())
	progressBar.ShowPercentage = true
	progressBar.Width = 40

	ap.model = adaptiveModel{
		spinner:     s,
		progress:    progressBar,
		showSpinner: true,
		showBar:     false,
		startTime:   time.Now(),
	}

	return ap
}

// SetPhases defines the phases of the operation for better progress tracking
func (ap *AdaptiveProgress) SetPhases(phases []PhaseInfo) {
	ap.model.phases = phases
	ap.model.currentPhase = 0
	if len(phases) > 0 {
		ap.model.phase = phases[0].Name
		ap.model.showBar = true
	}
}

// Start begins the progress indication
func (ap *AdaptiveProgress) Start(message string) {
	ap.model.message = message

	if ap.isTTY {
		// Start TUI mode
		ap.program = tea.NewProgram(&ap.model, tea.WithOutput(ap.output))
		go func() {
			if _, err := ap.program.Run(); err != nil {
				// Log error but don't crash
				fmt.Fprintf(ap.logOutput, "Progress UI error: %v\n", err)
			}
		}()
		// Give TUI time to initialize
		time.Sleep(50 * time.Millisecond)
	} else {
		// Simple text mode
		ap.logProgress("🚀 " + message)
	}
}

// UpdatePhase moves to the next phase
func (ap *AdaptiveProgress) UpdatePhase(phaseName string) {
	if ap.isTTY && ap.program != nil {
		ap.program.Send(phaseMsg{name: phaseName})
	} else {
		ap.logProgress("📋 " + phaseName)
	}
}

// UpdateProgress updates the progress percentage and details
func (ap *AdaptiveProgress) UpdateProgress(percent float64, details string) {
	if ap.isTTY && ap.program != nil {
		ap.program.Send(progressMsg{percent: percent, details: details})
	} else if details != "" {
		// Simple progress in non-TTY
		ap.logProgress(fmt.Sprintf("⚡ %.0f%% - %s", percent*100, details))
	}
}

// Success completes with success
func (ap *AdaptiveProgress) Success(message string) {
	duration := time.Since(ap.startTime)
	successMsg := fmt.Sprintf("✅ %s (%.2fs)", message, duration.Seconds())

	if ap.isTTY && ap.program != nil {
		ap.program.Send(doneMsg{success: true, message: successMsg})
		ap.program.Quit()
		time.Sleep(100 * time.Millisecond) // Let TUI finish

		// Add helpful next steps message
		fmt.Fprintf(ap.output, "\n💡 Next steps:\n")
		fmt.Fprintf(ap.output, "   • Use Claude Code MCP tools to query your graph\n")
		fmt.Fprintf(ap.output, "   • Run 'gograph --help' to see available commands\n")
		fmt.Fprintf(ap.output, "   • View your graph at http://localhost:7474 (Neo4j Browser)\n")
	} else {
		ap.logProgress(successMsg)
		// Add next steps for non-TTY mode too
		fmt.Fprintf(ap.output, "💡 Next steps:\n")
		fmt.Fprintf(ap.output, "   • Use Claude Code MCP tools to query your graph\n")
		fmt.Fprintf(ap.output, "   • Run 'gograph --help' to see available commands\n")
		fmt.Fprintf(ap.output, "   • View your graph at http://localhost:7474 (Neo4j Browser)\n")
	}
}

// SuccessWithStats completes with success and shows detailed statistics
func (ap *AdaptiveProgress) SuccessWithStats(message string, stats AnalysisStats) {
	duration := time.Since(ap.startTime)
	successMsg := fmt.Sprintf("✅ %s (%.2fs)", message, duration.Seconds())

	if ap.isTTY && ap.program != nil {
		ap.program.Send(doneMsg{success: true, message: successMsg})
		ap.program.Quit()
		time.Sleep(100 * time.Millisecond) // Let TUI finish

		// Show beautiful statistics
		ap.displayStats(stats, duration)
	} else {
		ap.logProgress(successMsg)
		// Show statistics in non-TTY mode too
		ap.displayStats(stats, duration)
	}
}

// displayStats shows beautiful analysis statistics
func (ap *AdaptiveProgress) displayStats(stats AnalysisStats, duration time.Duration) {
	// Create styled statistics display
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.AdaptiveColor{Light: "#1F2937", Dark: "#F9FAFB"}).
		MarginTop(1).
		MarginBottom(1)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}).
		Width(16)

	valueStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.AdaptiveColor{Light: "#059669", Dark: "#10B981"})

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#D1D5DB", Dark: "#374151"}).
		Padding(1, 2).
		MarginTop(1)

	// Build statistics content
	var content strings.Builder
	content.WriteString(titleStyle.Render("📊 Analysis Results"))
	content.WriteString("\n\n")

	// Project info
	content.WriteString(labelStyle.Render("Project ID:"))
	content.WriteString(" " + valueStyle.Render(stats.ProjectID) + "\n")

	content.WriteString(labelStyle.Render("Duration:"))
	content.WriteString(" " + valueStyle.Render(fmt.Sprintf("%.2fs", duration.Seconds())) + "\n\n")

	// File statistics
	content.WriteString(labelStyle.Render("Go Files:"))
	content.WriteString(" " + valueStyle.Render(fmt.Sprintf("%d", stats.Files)) + "\n")

	// Graph statistics
	content.WriteString(labelStyle.Render("Graph Nodes:"))
	content.WriteString(" " + valueStyle.Render(fmt.Sprintf("%d", stats.Nodes)) + "\n")

	content.WriteString(labelStyle.Render("Relationships:"))
	content.WriteString(" " + valueStyle.Render(fmt.Sprintf("%d", stats.Relationships)) + "\n")

	// Analysis statistics
	content.WriteString(labelStyle.Render("Interfaces:"))
	content.WriteString(" " + valueStyle.Render(fmt.Sprintf("%d", stats.Interfaces)) + "\n")

	content.WriteString(labelStyle.Render("Call Chains:"))
	content.WriteString(" " + valueStyle.Render(fmt.Sprintf("%d", stats.CallChains)) + "\n")

	// Performance metrics
	if stats.Files > 0 {
		nodesPerFile := float64(stats.Nodes) / float64(stats.Files)
		content.WriteString("\n")
		content.WriteString(labelStyle.Render("Nodes per File:"))
		content.WriteString(" " + valueStyle.Render(fmt.Sprintf("%.1f", nodesPerFile)) + "\n")
	}

	// Display in a beautiful box
	fmt.Fprintf(ap.output, "%s\n", boxStyle.Render(content.String()))

	// Add helpful next steps
	nextStepsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#6366F1", Dark: "#8B5CF6"}).
		MarginTop(1)

	fmt.Fprintf(ap.output, "%s\n", nextStepsStyle.Render("💡 Next steps:"))
	fmt.Fprintf(ap.output, "   • Use Claude Code MCP tools to query your graph\n")
	fmt.Fprintf(ap.output, "   • Run 'gograph --help' to see available commands\n")
	fmt.Fprintf(ap.output, "   • View your graph at http://localhost:7474 (Neo4j Browser)\n")
}

// Error completes with error
func (ap *AdaptiveProgress) Error(err error) {
	duration := time.Since(ap.startTime)
	errorMsg := fmt.Sprintf("❌ Failed after %.2fs: %v", duration.Seconds(), err)

	if ap.isTTY && ap.program != nil {
		ap.program.Send(doneMsg{success: false, message: errorMsg, err: err})
		ap.program.Quit()
		time.Sleep(100 * time.Millisecond) // Let TUI finish
	} else {
		ap.logProgress(errorMsg)
	}
}

// logProgress outputs progress to non-TTY terminals
func (ap *AdaptiveProgress) logProgress(message string) {
	timestamp := time.Now().Format("15:04:05")
	fmt.Fprintf(ap.output, "[%s] %s\n", timestamp, message)
}

// Styles for different components
func (ap *AdaptiveProgress) getSpinnerStyle() lipgloss.Style {
	return ap.renderer.NewStyle().Foreground(lipgloss.AdaptiveColor{
		Light: "#8B5CF6", // Purple
		Dark:  "#A78BFA", // Light purple
	})
}

// Messages for the TUI
type phaseMsg struct {
	name string
}

type progressMsg struct {
	percent float64
	details string
}

type doneMsg struct {
	success bool
	message string
	err     error
}

// Bubbletea Model Implementation
func (m *adaptiveModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m *adaptiveModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case phaseMsg:
		m.phase = msg.name
		// Find phase index and update current phase
		for i, phase := range m.phases {
			if phase.Name == msg.name {
				m.currentPhase = i
				break
			}
		}
		return m, nil

	case progressMsg:
		m.percent = msg.percent
		m.details = msg.details
		if m.showBar {
			progressCmd := m.progress.SetPercent(msg.percent)
			return m, progressCmd
		}
		return m, nil

	case doneMsg:
		m.done = true
		m.message = msg.message
		m.err = msg.err
		return m, tea.Quit

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	default:
		// Update progress bar if it's shown
		if m.showBar {
			progressModel, progressCmd := m.progress.Update(msg)
			if progressBar, ok := progressModel.(progress.Model); ok {
				m.progress = progressBar
			}
			return m, progressCmd
		}
	}

	return m, nil
}

func (m *adaptiveModel) View() string {
	// Get the parent AdaptiveProgress reference through a different approach
	// For now, we'll implement the view logic directly in the model
	if m.done {
		var style lipgloss.Style
		if m.err != nil {
			style = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{
				Light: "#DC2626", // Red
				Dark:  "#F87171", // Light red
			})
		} else {
			style = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{
				Light: "#059669", // Green
				Dark:  "#10B981", // Light green
			})
		}
		return style.Render(m.message)
	}

	var parts []string

	// Title style
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{
		Light: "#1F2937", // Dark gray
		Dark:  "#F9FAFB", // Light gray
	})

	// Phase style
	phaseStyle := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
		Light: "#6B7280", // Medium gray
		Dark:  "#9CA3AF", // Light gray
	})

	// Details style
	detailsStyle := lipgloss.NewStyle().Faint(true).Foreground(lipgloss.AdaptiveColor{
		Light: "#9CA3AF", // Light gray
		Dark:  "#6B7280", // Medium gray
	})

	// Title with spinner
	if m.showSpinner {
		title := lipgloss.JoinHorizontal(lipgloss.Left,
			m.spinner.View(),
			" ",
			titleStyle.Render(m.message),
		)
		parts = append(parts, title)
	} else {
		parts = append(parts, titleStyle.Render(m.message))
	}

	// Current phase
	if m.phase != "" {
		parts = append(parts, phaseStyle.Render("→ "+m.phase))
	}

	// Progress bar
	if m.showBar && len(m.phases) > 0 {
		progressView := m.progress.View()
		phaseInfo := fmt.Sprintf("Phase %d/%d", m.currentPhase+1, len(m.phases))

		progressLine := lipgloss.JoinHorizontal(lipgloss.Left,
			progressView,
			" ",
			phaseStyle.Render(phaseInfo),
		)
		parts = append(parts, progressLine)
	}

	// Details
	if m.details != "" {
		parts = append(parts, detailsStyle.Render("  "+m.details))
	}

	// Elapsed time
	elapsed := time.Since(m.startTime)
	timeInfo := detailsStyle.Render(fmt.Sprintf("  Elapsed: %.1fs", elapsed.Seconds()))
	parts = append(parts, timeInfo)

	return strings.Join(parts, "\n")
}
