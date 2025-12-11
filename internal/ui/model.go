package ui

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/langtind/gren/internal/logging"
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
		// Start async GitHub check if we have worktrees
		if len(m.worktrees) > 0 {
			m.githubLoading = true
			return m, tea.Batch(m.githubSpinner.Tick, m.startGitHubCheck())
		}
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
		if msg.selectedWorktree != nil {
			// Delete specific worktree
			m.setupDeleteStateForWorktree(*msg.selectedWorktree)
		} else {
			// Multi-select delete
			m.setupDeleteState()
		}
		return m, nil

	case projectAnalysisCompleteMsg:
		if m.initState != nil {
			m.initState.currentStep = InitStepRecommendations
			m.initState.detectedFiles = m.analyzeProject()
			m.initState.analysisComplete = true
			m.initState.packageManager = m.detectPackageManager()
			m.initState.postCreateCmd = m.detectPostCreateCommand()
		}
		return m, nil

	case initExecutionCompleteMsg:
		if m.initState != nil {
			if msg.err != nil {
				m.err = fmt.Errorf("initialization failed: %w", msg.err)
				m.initState.currentStep = InitStepComplete
			} else {
				m.initState.currentStep = InitStepCreated
				// Mark as initialized if successful
				if m.repoInfo != nil {
					m.repoInfo.IsInitialized = true
				}
			}
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

	case pruneCompleteMsg:
		// Prune operation complete
		if msg.err != nil {
			m.err = fmt.Errorf("prune failed: %w", msg.err)
		} else if msg.prunedCount > 0 {
			// Show success message briefly, then refresh worktrees
			m.err = nil
			// Refresh worktree list to reflect changes
			if err := m.refreshWorktrees(); err != nil {
				m.err = err
			}
		}
		return m, nil

	case staleCleanupCompleteMsg:
		// Stale cleanup operation complete
		logging.Info("Stale cleanup complete: cleaned %d, failed %d worktrees", msg.cleanedCount, msg.failedCount)

		// Build informative message
		if msg.failedCount > 0 && msg.cleanedCount > 0 {
			// Partial success - show both results
			var failedInfo string
			for i, name := range msg.failedNames {
				if i < len(msg.failedReasons) {
					failedInfo += fmt.Sprintf("\n  • %s (%s)", name, msg.failedReasons[i])
				}
			}
			m.err = fmt.Errorf("deleted %d worktree(s), %d failed:%s", msg.cleanedCount, msg.failedCount, failedInfo)
		} else if msg.failedCount > 0 {
			// All failed
			var failedInfo string
			for i, name := range msg.failedNames {
				if i < len(msg.failedReasons) {
					failedInfo += fmt.Sprintf("\n  • %s (%s)", name, msg.failedReasons[i])
				}
			}
			m.err = fmt.Errorf("cleanup failed - %d worktree(s) have uncommitted changes:%s", msg.failedCount, failedInfo)
		} else {
			m.err = nil
		}

		// Always refresh worktree list to reflect any successful deletions
		if err := m.refreshWorktrees(); err != nil && m.err == nil {
			m.err = err
		}

		// Clear cleanup state and return to dashboard
		m.cleanupState = nil
		m.currentView = DashboardView
		return m, nil

	case githubRefreshCompleteMsg:
		// GitHub refresh complete - update worktrees with PR info
		logging.Info("GitHub refresh complete: %d worktrees updated", len(msg.worktrees))
		m.worktrees = msg.worktrees
		m.githubLoading = false
		m.err = nil
		return m, nil

	case openPRCompleteMsg:
		// PR opened in browser
		if msg.err != nil {
			logging.Error("Failed to open PR: %v", msg.err)
			m.err = fmt.Errorf("failed to open PR: %w", msg.err)
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
				m.createState.createWarning = msg.warning // Store warning for display
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

	case availableBranchesLoadedMsg:
		if m.createState != nil {
			if msg.err != nil {
				m.err = fmt.Errorf("failed to load available branches: %w", msg.err)
				// Stay in create view but show error
				m.createState.currentStep = CreateStepBranchMode
			} else {
				m.createState.availableBranches = msg.branches
				m.createState.filteredAvailableBranches = msg.branches // Initialize filtered list
				m.createState.selectedBranch = 0
				m.createState.scrollOffset = 0
				m.createState.searchQuery = ""
				m.createState.isSearching = false
				// Debug: log how many branches we found
				if len(msg.branches) == 0 {
					m.err = fmt.Errorf("no available branches found for worktree creation")
					m.createState.currentStep = CreateStepBranchMode
				}
			}
		}
		return m, nil

	case configInitializedMsg:
		m.configState = &ConfigState{
			files:         msg.files,
			selectedIndex: 0,
		}
		return m, nil

	case configFileOpenedMsg:
		if msg.err != nil {
			m.err = msg.err
		}
		// Stay in config view after opening file
		return m, nil

	case aiScriptGeneratedMsg:
		if m.initState != nil {
			if msg.err != nil {
				m.initState.aiError = msg.err.Error()
			} else {
				m.initState.aiGeneratedScript = msg.script
				m.initState.aiError = ""
			}
			m.initState.currentStep = InitStepAIResult
			m.initState.selected = 0
		}
		return m, nil

	case spinner.TickMsg:
		var cmds []tea.Cmd

		// Handle GitHub loading spinner
		if m.githubLoading {
			var cmd tea.Cmd
			m.githubSpinner, cmd = m.githubSpinner.Update(msg)
			cmds = append(cmds, cmd)
		}

		// Handle spinner animation for CreateStepCreating
		if m.createState != nil && m.createState.currentStep == CreateStepCreating {
			var cmd tea.Cmd
			m.createState.spinner, cmd = m.createState.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

		// Handle spinner animation for DeleteStepDeleting
		if m.deleteState != nil && m.deleteState.currentStep == DeleteStepDeleting {
			var cmd tea.Cmd
			m.deleteSpinner, cmd = m.deleteSpinner.Update(msg)
			cmds = append(cmds, cmd)
		}

		if len(cmds) > 0 {
			return m, tea.Batch(cmds...)
		}
		return m, nil
	}

	// Handle keyboard input based on current view
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		// Handle help toggle globally on dashboard
		if m.currentView == DashboardView && key.Matches(keyMsg, m.keys.Help) {
			m.helpVisible = !m.helpVisible
			return m, nil
		}

		// Close help with esc
		if m.helpVisible && key.Matches(keyMsg, m.keys.Back) {
			m.helpVisible = false
			return m, nil
		}

		// If help is visible, ignore other keys
		if m.helpVisible {
			return m, nil
		}

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
		if m.currentView == ConfigView {
			return m.handleConfigKeys(keyMsg)
		}
		if m.currentView == ToolsView {
			return m.handleToolsKeys(keyMsg)
		}
		if m.currentView == CleanupView {
			return m.handleCleanupKeys(keyMsg)
		}

		// Dashboard keys
		logging.Debug("Dashboard key: %q", keyMsg.String())

		// Global keys
		switch {
		case key.Matches(keyMsg, m.keys.Quit):
			logging.Info("User quit from Dashboard")
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
			if selectedWorktree := m.getSelectedWorktree(); selectedWorktree != nil {
				logging.Info("Dashboard: opening 'Open in...' menu for worktree: %s", selectedWorktree.Name)
				m.currentView = OpenInView
				return m, m.initializeOpenInState(selectedWorktree.Path)
			}
			return m, nil

		case key.Matches(keyMsg, m.keys.New):
			// Only allow creating worktrees if initialized
			if m.repoInfo != nil && m.repoInfo.IsGitRepo && m.repoInfo.IsInitialized {
				logging.Info("Dashboard: entering CreateView (shortcut 'n')")
				m.currentView = CreateView
				// Use selected worktree's branch as suggested base
				var suggestedBase string
				if selectedWorktree := m.getSelectedWorktree(); selectedWorktree != nil {
					suggestedBase = selectedWorktree.Branch
				}
				return m, m.initializeCreateStateWithBase(suggestedBase)
			}
			return m, nil

		case key.Matches(keyMsg, m.keys.Delete):
			// Delete the currently selected worktree with confirmation
			if selectedWorktree := m.getSelectedWorktree(); selectedWorktree != nil && m.repoInfo != nil && m.repoInfo.IsInitialized {
				// Don't allow deleting the current worktree
				if selectedWorktree.IsCurrent {
					logging.Debug("Dashboard: cannot delete current worktree: %s", selectedWorktree.Name)
					return m, nil // Silently ignore - or could show error message
				}
				logging.Info("Dashboard: entering DeleteView for worktree: %s (shortcut 'd')", selectedWorktree.Name)
				m.currentView = DeleteView
				return m, m.initializeDeleteStateForWorktree(*selectedWorktree)
			}
			return m, nil

		case key.Matches(keyMsg, m.keys.Init):
			if m.repoInfo != nil && m.repoInfo.IsGitRepo && !m.repoInfo.IsInitialized {
				logging.Info("Dashboard: entering InitView (shortcut 'i')")
				m.currentView = InitView
				m.initState = &InitState{
					currentStep:       InitStepWelcome,
					detectedFiles:     []DetectedFile{},
					selected:          0,
					worktreeDir:       m.generateDefaultWorktreeDir(),
					customizationMode: "",
					editingText:       "",
					postCreateScript:  "",
					analysisComplete:  false,
					packageManager:    "",
					postCreateCmd:     "",
				}
			}
			return m, nil

		case key.Matches(keyMsg, m.keys.Config):
			if m.repoInfo != nil && m.repoInfo.IsGitRepo && m.repoInfo.IsInitialized {
				logging.Info("Dashboard: entering ConfigView (shortcut 'c')")
				m.currentView = ConfigView
				return m, m.initializeConfigState()
			}
			return m, nil
		case key.Matches(keyMsg, m.keys.Prune):
			// Only allow pruning if initialized and we have worktrees
			if m.repoInfo != nil && m.repoInfo.IsGitRepo && m.repoInfo.IsInitialized {
				logging.Info("Dashboard: running prune (shortcut 'p')")
				return m, m.pruneWorktrees()
			}
			return m, nil
		case key.Matches(keyMsg, m.keys.Navigate):
			// Navigate to selected worktree directory
			if selectedWorktree := m.getSelectedWorktree(); selectedWorktree != nil {
				logging.Info("Dashboard: navigating to worktree: %s (shortcut 'g')", selectedWorktree.Name)
				return m, m.navigateToWorktree(selectedWorktree.Path)
			}
			return m, nil
		case key.Matches(keyMsg, m.keys.Tools):
			// Open Tools menu
			if m.repoInfo != nil && m.repoInfo.IsGitRepo && m.repoInfo.IsInitialized {
				logging.Info("Dashboard: opening Tools menu (shortcut 't')")
				m.currentView = ToolsView
			}
			return m, nil
		}
	}

	return m, nil
}

// View renders the current view
func (m Model) View() string {
	var baseView string

	switch m.currentView {
	case DashboardView:
		baseView = m.dashboardView()
		// Show help overlay if visible
		if m.helpVisible {
			return m.renderHelpOverlay(baseView)
		}
	case CreateView:
		baseView = m.createView()
	case DeleteView:
		// Delete steps are shown as modal overlays on dashboard
		baseView = m.dashboardView()
		if m.deleteState != nil {
			switch m.deleteState.currentStep {
			case DeleteStepConfirm:
				return m.renderDeleteModal(baseView)
			case DeleteStepDeleting:
				return m.renderWithModalWidth(baseView, m.renderDeleteDeletingModal(), 70, ColorWarning)
			case DeleteStepComplete:
				return m.renderWithModalWidth(baseView, m.renderDeleteCompleteModal(), 70, ColorSuccess)
			}
		}
		return baseView
	case InitView:
		baseView = m.initView()
	case SettingsView:
		baseView = m.settingsView()
	case OpenInView:
		// Render dashboard with modal overlay
		baseView = m.dashboardView()
		return m.renderWithModalWidth(baseView, m.renderOpenInModal(), 70, ColorPrimary)
	case ConfigView:
		baseView = m.configView()
	case ToolsView:
		// Render dashboard with Tools menu modal overlay
		baseView = m.dashboardView()
		return m.renderToolsModal(baseView)
	case CleanupView:
		// Render dashboard with cleanup confirmation modal overlay (wider modal, warning color)
		baseView = m.dashboardView()
		return m.renderWithModalWidth(baseView, m.renderCleanupConfirmation(), 70, ColorWarning)
	default:
		baseView = m.dashboardView()
	}

	return baseView
}

// generateDefaultWorktreeDir creates a default worktree directory name based on current working directory
func (m Model) generateDefaultWorktreeDir() string {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return "../gren-worktrees" // fallback
	}

	// Get the directory name
	dirName := filepath.Base(cwd)
	if dirName == "" || dirName == "." || dirName == "/" {
		return "../gren-worktrees" // fallback
	}

	// Create worktree directory name based on current directory
	return fmt.Sprintf("../%s-worktrees", dirName)
}

// getWorktreeDir returns the configured worktree directory or a default
func (m Model) getWorktreeDir() string {
	// Check config first
	if m.config != nil && m.config.WorktreeDir != "" {
		return m.config.WorktreeDir
	}
	// Fall back to default
	return m.generateDefaultWorktreeDir()
}

// getWorktreePath returns the full path for a worktree given a branch name
func (m Model) getWorktreePath(branchName string) string {
	return fmt.Sprintf("%s/%s", m.getWorktreeDir(), sanitizeBranchForPath(branchName))
}
