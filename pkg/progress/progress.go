package progress

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/compozy/gograph/pkg/logger"
)

// -----
// Models
// -----

// Model represents the progress indicator model
type Model struct {
	spinner  spinner.Model
	message  string
	done     bool
	err      error
	progress float64
	total    int
	current  int
}

// -----
// Messages
// -----

// UpdateMsg updates the progress message
type UpdateMsg struct {
	Message string
}

// Msg updates the progress percentage
type Msg struct {
	Current int
	Total   int
}

// DoneMsg signals completion
type DoneMsg struct {
	Error error
}

// -----
// Styles
// -----

var (
	spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))
	textStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("82"))
)

// -----
// Constructor
// -----

// New creates a new progress indicator
func New(message string) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle
	return Model{
		spinner: s,
		message: message,
	}
}

// -----
// Bubbletea Interface
// -----

// Init implements tea.Model
func (m *Model) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update implements tea.Model
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			m.done = true
			return m, tea.Quit
		}
		return m, nil

	case UpdateMsg:
		m.message = msg.Message
		return m, nil

	case Msg:
		m.current = msg.Current
		m.total = msg.Total
		if m.total > 0 {
			m.progress = float64(m.current) / float64(m.total)
		}
		return m, nil

	case DoneMsg:
		m.done = true
		m.err = msg.Error
		return m, tea.Quit

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	default:
		return m, nil
	}
}

// View implements tea.Model
func (m *Model) View() string {
	if m.done {
		if m.err != nil {
			return errorStyle.Render("✗ ") + textStyle.Render(m.message) +
				errorStyle.Render(fmt.Sprintf(" - Error: %v", m.err))
		}
		return successStyle.Render("✓ ") + textStyle.Render(m.message)
	}

	str := m.spinner.View() + " " + textStyle.Render(m.message)

	// Add progress if available
	if m.total > 0 {
		percentage := int(m.progress * 100)
		str += textStyle.Render(fmt.Sprintf(" [%d/%d] %d%%", m.current, m.total, percentage))
	}

	return str
}

// -----
// Runner
// -----

// Runner provides a simple interface to run operations with progress
type Runner struct {
	program *tea.Program
}

// NewRunner creates a new progress runner
func NewRunner(message string) *Runner {
	model := New(message)
	return &Runner{
		program: tea.NewProgram(&model),
	}
}

// Start starts the progress indicator
func (r *Runner) Start() {
	go func() {
		if _, err := r.program.Run(); err != nil {
			// Log error but don't panic
			logger.Error("Error running progress", "error", err)
		}
	}()
	// Give the UI time to start
	time.Sleep(50 * time.Millisecond)
}

// Update updates the progress message
func (r *Runner) Update(message string) {
	r.program.Send(UpdateMsg{Message: message})
}

// SetProgress updates the progress counter
func (r *Runner) SetProgress(current, total int) {
	r.program.Send(Msg{Current: current, Total: total})
}

// Done signals completion
func (r *Runner) Done(err error) {
	r.program.Send(DoneMsg{Error: err})
	// Give the UI time to render final state
	time.Sleep(50 * time.Millisecond)
}

// Success signals successful completion
func (r *Runner) Success() {
	r.Done(nil)
}

// -----
// Simple Progress Functions
// -----

// WithProgress runs a function with a progress indicator
func WithProgress(message string, fn func() error) error {
	runner := NewRunner(message)
	runner.Start()
	err := fn()
	runner.Done(err)
	return err
}

// WithProgressSteps runs a function with progress tracking
func WithProgressSteps(message string, fn func(update func(string), progress func(int, int)) error) error {
	runner := NewRunner(message)
	runner.Start()

	updateFn := func(msg string) {
		runner.Update(msg)
	}

	progressFn := func(current, total int) {
		runner.SetProgress(current, total)
	}

	err := fn(updateFn, progressFn)
	runner.Done(err)
	return err
}
