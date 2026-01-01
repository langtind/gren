package ui

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

	case cleanupStartedMsg:
		// Initialize cleanup state and start first deletion
		if m.cleanupState != nil {
			m.cleanupState.inProgress = true
			m.cleanupState.currentIndex = -1
			m.cleanupState.deletedIndices = make(map[int]bool)
			m.cleanupState.failedWorktrees = make(map[int]string)
			m.cleanupState.totalCleaned = 0
			m.cleanupState.totalFailed = 0

			// Build sorted list of selected indices
			var selectedList []int
			for idx := range m.cleanupState.selectedIndices {
				selectedList = append(selectedList, idx)
			}
			// Sort the list
			for i := 0; i < len(selectedList); i++ {
				for j := i + 1; j < len(selectedList); j++ {
					if selectedList[i] > selectedList[j] {
						selectedList[i], selectedList[j] = selectedList[j], selectedList[i]
					}
				}
			}
			m.cleanupState.selectedIndicesList = selectedList

			logging.Info("Cleanup started: %d worktrees to process", len(selectedList))

			// Start spinner and first deletion (using first selected index)
			if len(selectedList) > 0 {
				firstIdx := selectedList[0]
				return m, tea.Batch(
					m.cleanupState.cleanupSpinner.Tick,
					func() tea.Msg {
						return cleanupItemStartMsg{worktreeIndex: firstIdx, worktreeName: m.cleanupState.staleWorktrees[firstIdx].Branch}
					},
					m.deleteNextWorktree(firstIdx),
				)
			}
		}
		return m, nil

	case cleanupItemStartMsg:
		// Mark worktree as currently being deleted (triggers spinner display)
		if m.cleanupState != nil {
			m.cleanupState.currentIndex = msg.worktreeIndex
			logging.Debug("Cleanup: starting deletion of index %d: %s", msg.worktreeIndex, msg.worktreeName)
		}
		return m, nil

	case cleanupItemCompleteMsg:
		// Handle individual deletion completion
		if m.cleanupState != nil {
			if msg.success {
				// Mark as successfully deleted (will be removed from UI)
				m.cleanupState.deletedIndices[msg.worktreeIndex] = true
				m.cleanupState.totalCleaned++
				logging.Info("Cleanup: deleted %s (%d/%d)",
					msg.worktreeName,
					m.cleanupState.totalCleaned+m.cleanupState.totalFailed,
					len(m.cleanupState.staleWorktrees))
			} else {
				// Mark as failed with error (will show red ✗)
				m.cleanupState.failedWorktrees[msg.worktreeIndex] = msg.errorMsg
				m.cleanupState.totalFailed++
				logging.Error("Cleanup: failed %s: %s", msg.worktreeName, msg.errorMsg)
			}

			// Clear current index (no longer showing spinner)
			m.cleanupState.currentIndex = -1

			// Find next selected index to process
			var nextIdx int = -1
			foundCurrent := false
			for _, idx := range m.cleanupState.selectedIndicesList {
				if foundCurrent {
					nextIdx = idx
					break
				}
				if idx == msg.worktreeIndex {
					foundCurrent = true
				}
			}

			// Determine next action
			if nextIdx != -1 {
				// More worktrees to process
				return m, tea.Batch(
					func() tea.Msg {
						return cleanupItemStartMsg{
							worktreeIndex: nextIdx,
							worktreeName:  m.cleanupState.staleWorktrees[nextIdx].Branch,
						}
					},
					m.deleteNextWorktree(nextIdx),
				)
			}
			// All done - send finished message
			return m, func() tea.Msg {
				return cleanupFinishedMsg{
					totalCleaned: m.cleanupState.totalCleaned,
					totalFailed:  m.cleanupState.totalFailed,
				}
			}
		}
		return m, nil

	case cleanupFinishedMsg:
		// Cleanup complete - update state and refresh worktree list
		logging.Info("Cleanup finished: %d cleaned, %d failed", msg.totalCleaned, msg.totalFailed)

		if m.cleanupState != nil {
			m.cleanupState.inProgress = false
			m.cleanupState.currentIndex = -1
		}

		// Note: Cleanup errors are stored in cleanupState.failedWorktrees and displayed
		// in the CleanupView's failure summary modal. We intentionally don't set m.err
		// here to avoid duplicate error display in the global dashboard view.

		// Refresh worktree list to reflect deletions
		if err := m.refreshWorktrees(); err != nil {
			m.err = err
		}

		// Return to dashboard if all succeeded, stay in CleanupView if failures
		if msg.totalFailed == 0 {
			m.cleanupState = nil
			m.currentView = DashboardView
		}
		// else: stay in CleanupView to show failure summary

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

	case compareInitMsg:
		if msg.err != nil {
			m.err = fmt.Errorf("compare failed: %w", msg.err)
			m.currentView = DashboardView
			return m, nil
		}
		m.compareState = &CompareState{
			sourceWorktree: msg.sourceWorktree,
			sourcePath:     msg.sourcePath,
			files:          msg.files,
			selectedIndex:  0,
			scrollOffset:   0,
			selectAll:      true, // All selected by default
			diffContent:    "",   // Will be loaded below
		}
		// Load diff for first file
		if len(msg.files) > 0 {
			return m, m.loadCompareDiff(msg.sourcePath, msg.files[0].Path)
		}
		return m, nil

	case compareApplyCompleteMsg:
		if m.compareState != nil {
			if msg.err != nil {
				m.compareState.applyError = msg.err.Error()
			} else {
				m.compareState.appliedCount = msg.appliedCount
			}
			m.compareState.applyComplete = true
			m.compareState.applyInProgress = false
		}
		return m, nil

	case compareDiffViewedMsg:
		// Diff viewing completed (returned from external pager)
		if msg.err != nil {
			m.err = msg.err
		}
		return m, nil

	case compareDiffLoadedMsg:
		if m.compareState != nil {
			if msg.err != nil {
				m.compareState.diffContent = fmt.Sprintf("Error loading diff: %v", msg.err)
			} else {
				m.compareState.diffContent = msg.content
				m.compareState.diffScrollOffset = 0
			}
		}
		return m, nil

	case mergeProgressMsg:
		if m.mergeState != nil {
			m.mergeState.progressMsg = msg.message
		}
		return m, nil

	case mergeCompleteMsg:
		if m.mergeState != nil {
			m.mergeState.currentStep = MergeStepComplete
			m.mergeState.result = msg.result
			m.mergeState.err = msg.err
			if msg.err == nil {
				m.refreshWorktrees()
			}
		}
		return m, nil

	case forEachItemCompleteMsg:
		if m.forEachState != nil {
			m.forEachState.results = append(m.forEachState.results, ForEachResult{
				Worktree: msg.worktree,
				Output:   msg.output,
				Success:  msg.success,
			})
			m.forEachState.currentIndex++
		}
		return m, nil

	case forEachCompleteMsg:
		if m.forEachState != nil {
			m.forEachState.inProgress = false
		}
		return m, nil

	case llmMessageGeneratedMsg:
		if m.stepCommitState != nil {
			if msg.err != nil {
				// Show error and go back to options
				m.stepCommitState.currentStep = StepCommitStepComplete
				m.stepCommitState.err = msg.err
			} else {
				// Show generated message for review/edit
				m.stepCommitState.message = msg.message
				m.stepCommitState.currentStep = StepCommitStepMessage
			}
		}
		return m, nil

	case stepCommitCompleteMsg:
		if m.stepCommitState != nil {
			m.stepCommitState.currentStep = StepCommitStepComplete
			m.stepCommitState.result = msg.result
			m.stepCommitState.err = msg.err
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

		// Handle spinner animation for cleanup in progress
		if m.cleanupState != nil && m.cleanupState.inProgress {
			var cmd tea.Cmd
			m.cleanupState.cleanupSpinner, cmd = m.cleanupState.cleanupSpinner.Update(msg)
			cmds = append(cmds, cmd)
		}

		// Handle spinner animation for compare loading
		if m.currentView == CompareView && m.compareState == nil {
			var cmd tea.Cmd
			m.compareSpinner, cmd = m.compareSpinner.Update(msg)
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
		if m.currentView == CompareView {
			return m.handleCompareKeys(keyMsg)
		}
		if m.currentView == MergeView {
			return m.handleMergeKeys(keyMsg)
		}
		if m.currentView == ForEachView {
			return m.handleForEachKeys(keyMsg)
		}
		if m.currentView == StepCommitView {
			return m.handleStepCommitKeys(keyMsg)
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

		case key.Matches(keyMsg, m.keys.Compare):
			// Compare selected worktree to current
			if selectedWorktree := m.getSelectedWorktree(); selectedWorktree != nil {
				// Can't compare current worktree to itself
				if selectedWorktree.IsCurrent {
					logging.Debug("Dashboard: cannot compare current worktree to itself")
					return m, nil
				}
				if m.repoInfo != nil && m.repoInfo.IsGitRepo && m.repoInfo.IsInitialized {
					logging.Info("Dashboard: entering CompareView for worktree: %s (shortcut 'm')", selectedWorktree.Name)
					m.currentView = CompareView
					// Initialize spinner for loading state
					s := spinner.New()
					s.Spinner = spinner.Dot
					s.Style = lipgloss.NewStyle().Foreground(ColorPrimary)
					m.compareSpinner = s
					return m, tea.Batch(m.initializeCompareState(selectedWorktree.Name), m.compareSpinner.Tick)
				}
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
	case CompareView:
		baseView = m.renderCompareView()
		if m.helpVisible {
			return m.renderCompareHelpOverlay(baseView)
		}
	case MergeView:
		baseView = m.dashboardView()
		return m.renderWithModalWidth(baseView, m.renderMergeView(), 60, ColorPrimary)
	case ForEachView:
		baseView = m.dashboardView()
		return m.renderWithModalWidth(baseView, m.renderForEachView(), 60, ColorPrimary)
	case StepCommitView:
		baseView = m.dashboardView()
		return m.renderWithModalWidth(baseView, m.renderStepCommitView(), 60, ColorPrimary)
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
