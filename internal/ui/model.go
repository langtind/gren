package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Update handles all incoming messages and updates the model state
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case projectInfoMsg:
		m = m.updateProjectInfo(msg.info, msg.err)
		return m, nil

	case initializeMsg:
		// Handle initialization for create/delete views
		return m, nil

	case createInitMsg:
		if msg.err != nil {
			m.err = fmt.Errorf("failed to initialize create state: %w", msg.err)
			m.currentView = DashboardView
			return m, nil
		}
		m.setupCreateState(msg)
		return m, nil

	case deleteInitMsg:
		if msg.err != nil {
			m.err = fmt.Errorf("failed to initialize delete state: %w", msg.err)
			m.currentView = DashboardView
			return m, nil
		}
		m.setupDeleteState()
		return m, nil

	case projectAnalysisCompleteMsg:
		if m.initState != nil {
			m.initState.currentStep = InitStepRecommendations
			m.initState.detectedFiles = m.analyzeProject()
			m.initState.copyPatterns = m.generateCopyPatterns(m.initState.detectedFiles)
			m.initState.analysisComplete = true
			m.initState.packageManager = m.detectPackageManager()
			m.initState.postCreateCmd = m.detectPostCreateCommand()
		}
		return m, nil

	case initExecutionCompleteMsg:
		if m.initState != nil {
			m.initState.currentStep = InitStepCreated
		}
		return m, nil

	case scriptCreateCompleteMsg:
		// Script files created, show confirmation
		if m.initState != nil {
			if msg.err != nil {
				m.err = fmt.Errorf("failed to create script files: %w", msg.err)
				m.initState.currentStep = InitStepComplete
			} else {
				m.initState.currentStep = InitStepCreated
			}
		}
		return m, nil

	case scriptEditCompleteMsg:
		// Script editing complete
		if m.initState != nil {
			if msg.err != nil {
				m.err = fmt.Errorf("script editing failed: %w", msg.err)
			}
			m.initState.currentStep = InitStepCommitConfirm
		}
		return m, nil

	case commitCompleteMsg:
		// Commit complete
		if m.initState != nil {
			if msg.err != nil {
				m.err = fmt.Errorf("commit failed: %w", msg.err)
			}
			m.initState.currentStep = InitStepFinal
			// Mark as initialized
			if m.repoInfo != nil {
				m.repoInfo.IsInitialized = true
			}
		}
		return m, nil

	case worktreeCreatedMsg:
		if m.createState != nil {
			if msg.err != nil {
				m.err = fmt.Errorf("worktree creation failed: %w", msg.err)
				m.currentView = DashboardView
				// Refresh project info to check if .gren was deleted/created
				return m, m.loadProjectInfo()
			} else {
				// Refresh worktrees list after successful creation
				m.refreshWorktrees()
				m.createState.currentStep = CreateStepComplete
				m.initializeActionsList()
			}
		}
		return m, nil

	case worktreeDeletedMsg:
		if m.deleteState != nil {
			if msg.err != nil {
				m.err = fmt.Errorf("worktree deletion failed: %w", msg.err)
				m.currentView = DashboardView
				// Refresh project info to check if .gren was deleted/created
				return m, m.loadProjectInfo()
			} else {
				// Refresh worktrees list after successful deletion
				m.refreshWorktrees()
				m.deleteState.currentStep = DeleteStepComplete
			}
		}
		return m, nil

	case openInInitializedMsg:
		m.initializeOpenInStateFromMsg(msg)
		return m, nil
	}

	// Handle keyboard input based on current view
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if m.currentView == InitView {
			return m.handleInitKeys(keyMsg)
		}
		if m.currentView == CreateView {
			return m.handleCreateKeys(keyMsg)
		}
		if m.currentView == DeleteView {
			return m.handleDeleteKeys(keyMsg)
		}
		if m.currentView == OpenInView {
			return m.handleOpenInKeys(keyMsg)
		}

		// Global keys
		switch {
		case key.Matches(keyMsg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(keyMsg, m.keys.Up):
			if m.selected > 0 {
				m.selected--
			}
		case key.Matches(keyMsg, m.keys.Down):
			if m.selected < len(m.worktrees)-1 {
				m.selected++
			}

		case key.Matches(keyMsg, m.keys.Enter):
			// Show "Open in..." menu for selected worktree
			if len(m.worktrees) > 0 && m.selected < len(m.worktrees) {
				selectedWorktree := m.worktrees[m.selected]
				m.currentView = OpenInView
				return m, m.initializeOpenInState(selectedWorktree.Path)
			}
			return m, nil

		case key.Matches(keyMsg, m.keys.New):
			// Only allow creating worktrees if initialized
			if m.repoInfo != nil && m.repoInfo.IsGitRepo && m.repoInfo.IsInitialized {
				m.currentView = CreateView
				return m, m.initializeCreateState()
			}
			return m, nil

		case key.Matches(keyMsg, m.keys.Delete):
			// Only allow deleting worktrees if we have some and are initialized
			if len(m.worktrees) > 0 && m.repoInfo != nil && m.repoInfo.IsInitialized {
				m.currentView = DeleteView
				return m, m.initializeDeleteState()
			}
			return m, nil

		case key.Matches(keyMsg, m.keys.Init):
			if m.repoInfo != nil && m.repoInfo.IsGitRepo && !m.repoInfo.IsInitialized {
				m.currentView = InitView
				m.initState = &InitState{
					currentStep:       InitStepWelcome,
					detectedFiles:     []DetectedFile{},
					copyPatterns:      []CopyPattern{},
					selected:          0,
					worktreeDir:       "../gren-worktrees",
					customizationMode: "",
					editingText:       "",
					postCreateScript:  "",
					analysisComplete:  false,
					packageManager:    "",
					postCreateCmd:     "",
				}
			}
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
	case InitView:
		return m.initView()
	case SettingsView:
		return m.settingsView()
	case OpenInView:
		return m.openInView()
	default:
		return m.dashboardView()
	}
}

// openInView renders the "Open in..." view
func (m Model) openInView() string {
	if m.openInState == nil {
		return "Loading..."
	}

	if len(m.openInState.actions) == 0 {
		return HeaderStyle.Width(m.width - 4).Render("No actions available")
	}

	var content strings.Builder

	content.WriteString(TitleStyle.Render("Open in..."))
	content.WriteString("\n\n")

	// Render each action as a simple list item
	for i, action := range m.openInState.actions {
		prefix := "  "
		if i == m.openInState.selectedIndex {
			prefix = "▶ "
			// Just change text color, no border/box
			content.WriteString(WorktreeNameStyle.Foreground(PrimaryColor).Render(fmt.Sprintf("%s%s %s", prefix, action.Icon, action.Name)))
		} else {
			content.WriteString(fmt.Sprintf("%s%s %s", prefix, action.Icon, action.Name))
		}
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(HelpStyle.Render("↑↓ Navigate • Enter Select • Esc Back"))

	return content.String()
}