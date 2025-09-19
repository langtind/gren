package ui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// ViewState represents the current view/screen
type ViewState int

const (
	DashboardView ViewState = iota
	CreateView
	DeleteView
	SettingsView
)

// Worktree represents a git worktree
type Worktree struct {
	Name   string
	Path   string
	Branch string
	Status string // "clean", "modified", "building", etc.
}

// Model is the main TUI model
type Model struct {
	// Current view state
	currentView ViewState

	// Dashboard data
	projectName string
	worktrees   []Worktree
	selected    int

	// Screen dimensions
	width  int
	height int

	// Key bindings
	keys KeyMap
}

// KeyMap defines key bindings
type KeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Enter  key.Binding
	Back   key.Binding
	New    key.Binding
	Delete key.Binding
	Init   key.Binding
	Help   key.Binding
	Quit   key.Binding
}

// DefaultKeyMap returns default key bindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter", " "),
			key.WithHelp("enter", "select"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		New: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "new worktree"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete"),
		),
		Init: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "initialize"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

// NewModel creates a new model
func NewModel() Model {
	return Model{
		currentView: DashboardView,
		projectName: "", // Will be detected from git
		worktrees:   []Worktree{}, // Will be loaded from git
		selected:    0,
		keys:        DefaultKeyMap(),
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.loadProjectInfo(),
	)
}

// loadProjectInfo loads git repository information
func (m Model) loadProjectInfo() tea.Cmd {
	return func() tea.Msg {
		// This will be a message to update the model with git info
		return projectInfoMsg{}
	}
}

// Messages
type projectInfoMsg struct{}

// Message to update project info
func (m Model) updateProjectInfo() Model {
	// Import git package functions here
	// For now, just use current directory name as project name
	m.projectName = "gren" // This will be replaced with actual git detection
	return m
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case projectInfoMsg:
		m = m.updateProjectInfo()
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keys.Up):
			if m.selected > 0 {
				m.selected--
			}

		case key.Matches(msg, m.keys.Down):
			if m.selected < len(m.worktrees)-1 {
				m.selected++
			}

		case key.Matches(msg, m.keys.Enter):
			// TODO: Navigate to selected worktree
			return m, nil

		case key.Matches(msg, m.keys.New):
			m.currentView = CreateView
			return m, nil

		case key.Matches(msg, m.keys.Delete):
			m.currentView = DeleteView
			return m, nil

		case key.Matches(msg, m.keys.Init):
			// TODO: Handle initialization
			return m, nil

		case key.Matches(msg, m.keys.Back):
			m.currentView = DashboardView
			return m, nil
		}
	}

	return m, nil
}

// View renders the current view
func (m Model) View() string {
	switch m.currentView {
	case DashboardView:
		return m.dashboardView()
	case CreateView:
		return m.createView()
	case DeleteView:
		return m.deleteView()
	case SettingsView:
		return m.settingsView()
	default:
		return m.dashboardView()
	}
}