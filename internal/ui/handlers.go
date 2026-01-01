package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/langtind/gren/internal/logging"
)

// handleInitKeys handles keyboard input for the init view
func (m Model) handleInitKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.initState == nil {
		return m, nil
	}

	logging.Debug("InitView key: %q, step: %d", msg.String(), m.initState.currentStep)

	switch m.initState.currentStep {
	case InitStepCustomization:
		return m.handleCustomizationKeys(msg)
	case InitStepAIGenerating:
		// AI is generating, only allow quit
		if key.Matches(msg, m.keys.Quit) {
			return m, tea.Quit
		}
		return m, nil
	case InitStepAIResult:
		return m.handleAIResultKeys(msg)
	}

	switch {
	case key.Matches(msg, m.keys.Quit):
		logging.Info("User quit from InitView")
		return m, tea.Quit
	case key.Matches(msg, m.keys.Back):
		switch m.initState.currentStep {
		case InitStepWelcome:
			logging.Info("InitView: back to Dashboard from Welcome")
			m.currentView = DashboardView
			return m, nil
		case InitStepAnalysis, InitStepExecuting:
			// These are non-interactive steps, skip back further
			m.initState.currentStep = InitStepWelcome
		case InitStepCreated:
			// Go back to preview step
			m.initState.currentStep = InitStepPreview
			m.initState.selected = 0
		default:
			// Normal back navigation for interactive steps
			if m.initState.currentStep > InitStepWelcome {
				m.initState.currentStep--
			}
		}
		return m, nil
	case key.Matches(msg, m.keys.Up):
		if m.initState.currentStep == InitStepRecommendations ||
			m.initState.currentStep == InitStepGrenConfig ||
			m.initState.currentStep == InitStepPreview ||
			m.initState.currentStep == InitStepCreated {
			if m.initState.selected > 0 {
				m.initState.selected--
			}
		}
		return m, nil
	case key.Matches(msg, m.keys.Down):
		var maxItems int
		switch m.initState.currentStep {
		case InitStepRecommendations:
			maxItems = 3 // "Accept recommendations", "Customize configuration", "Generate with AI"
		case InitStepGrenConfig:
			maxItems = 2 // "Track in git", "Keep local"
		case InitStepPreview:
			maxItems = 3 // "Create configuration", "Back to customize", "Cancel"
		case InitStepCreated:
			maxItems = 2 // "Edit script" and "Skip and continue"
		}
		if m.initState.selected < maxItems-1 {
			m.initState.selected++
		}
		return m, nil
	case key.Matches(msg, m.keys.Enter):
		switch m.initState.currentStep {
		case InitStepWelcome:
			logging.Info("InitView: starting project analysis")
			m.initState.currentStep = InitStepAnalysis
			return m, m.runProjectAnalysis()
		case InitStepRecommendations:
			// Save the choice and go to .gren config step
			m.initState.recommendationMode = m.initState.selected
			m.initState.currentStep = InitStepGrenConfig
			m.initState.selected = 0 // Default to "Track in git"
			logging.Info("InitView: going to .gren configuration")
			return m, nil
		case InitStepGrenConfig:
			// Set trackGrenInGit based on selection (0=track, 1=gitignore)
			m.initState.trackGrenInGit = (m.initState.selected == 0)
			logging.Info("InitView: .gren config choice: track=%v", m.initState.trackGrenInGit)

			// Continue based on original recommendation choice
			switch m.initState.recommendationMode {
			case 0:
				// Accept recommendations -> go to preview
				logging.Info("InitView: going to preview")
				m.initState.currentStep = InitStepPreview
				m.initState.selected = 0
			case 1:
				// Customize -> go to customization
				logging.Info("InitView: entering customization")
				m.initState.currentStep = InitStepCustomization
				m.initState.selected = 0
			case 2:
				// AI generation -> generate script
				logging.Info("InitView: generating AI setup script")
				m.initState.currentStep = InitStepAIGenerating
				return m, m.generateAISetupScript()
			}
			return m, nil
		case InitStepPreview:
			switch m.initState.selected {
			case 0:
				// Create configuration
				logging.Info("InitView: executing initialization")
				m.initState.currentStep = InitStepExecuting
				return m, m.runInitialization()
			case 1:
				// Back to customize
				logging.Info("InitView: back to customization")
				m.initState.currentStep = InitStepCustomization
				m.initState.selected = 0
			case 2:
				// Cancel
				logging.Info("InitView: cancelled, back to Dashboard")
				m.currentView = DashboardView
				return m, m.loadProjectInfo()
			}
			return m, nil
		case InitStepCreated:
			if m.initState.selected == 0 {
				// Edit script option - open in editor and return to dashboard
				logging.Info("InitView: opening post-create script for editing")
				// Mark as initialized
				if m.repoInfo != nil {
					m.repoInfo.IsInitialized = true
				}
				m.currentView = DashboardView
				return m, tea.Batch(m.openPostCreateScript(), m.loadProjectInfo())
			} else {
				// Go to dashboard directly
				logging.Info("InitView: going to dashboard")
				// Mark as initialized
				if m.repoInfo != nil {
					m.repoInfo.IsInitialized = true
				}
				m.currentView = DashboardView
				return m, m.loadProjectInfo()
			}
		case InitStepCommitConfirm:
			// Commit changes
			logging.Info("InitView: committing configuration")
			m.initState.currentStep = InitStepFinal
			return m, m.commitConfiguration()
		case InitStepComplete:
			// Go to final step
			logging.Info("InitView: going to final step")
			m.initState.currentStep = InitStepFinal
			return m, nil
		case InitStepFinal:
			// Return to dashboard
			logging.Info("InitView: returning to dashboard")
			m.currentView = DashboardView
			return m, m.loadProjectInfo()
		}
	case msg.String() == "y" || msg.String() == "Y":
		if m.initState.currentStep == InitStepCommitConfirm {
			// Commit the changes
			m.initState.currentStep = InitStepFinal
			return m, m.commitConfiguration()
		}
		return m, nil
	case msg.String() == "n" || msg.String() == "N":
		if m.initState.currentStep == InitStepCommitConfirm {
			// Skip commit, go to final
			m.initState.currentStep = InitStepFinal
			// Mark as initialized since we're skipping commit
			if m.repoInfo != nil {
				m.repoInfo.IsInitialized = true
			}
		} else if m.initState.currentStep == InitStepFinal {
			// Go to dashboard and start create workflow
			m.currentView = CreateView
			m.createState = &CreateState{
				currentStep:  CreateStepBranchMode,
				selectedMode: 0,
				branchName:   "",
				baseBranch:   "",
				createMode:   CreateModeNewBranch,
			}
			return m, m.loadProjectInfo()
		}
		return m, nil
	}

	return m, nil
}

// handleCustomizationKeys handles keyboard input for the customization step
func (m Model) handleCustomizationKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.initState == nil {
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.Back):
		if m.initState.customizationMode != "" {
			// Go back to main customization menu
			m.initState.customizationMode = ""
			m.initState.editingText = ""
			m.initState.selected = 0
		} else {
			// Go back to recommendations
			m.initState.currentStep = InitStepRecommendations
		}
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		if m.initState.customizationMode == "" {
			// Main menu - enter sub-mode
			options := []string{"worktree", "patterns", "postcreate"}
			if m.initState.selected < len(options) {
				m.initState.customizationMode = options[m.initState.selected]
				m.initState.selected = 0

				// Initialize editing text for text fields
				if m.initState.customizationMode == "worktree" {
					m.initState.editingText = m.initState.worktreeDir
				} else if m.initState.customizationMode == "postcreate" {
					m.initState.editingText = m.initState.postCreateScript
				}
			}
		} else if m.initState.customizationMode == "worktree" {
			// Save worktree directory and go back to main menu
			m.initState.worktreeDir = m.initState.editingText
			m.initState.customizationMode = ""
			m.initState.editingText = ""
			m.initState.selected = 0
		} else if m.initState.customizationMode == "postcreate" {
			// Save post-create script and go back to main menu
			m.initState.postCreateScript = m.initState.editingText
			m.initState.customizationMode = ""
			m.initState.editingText = ""
			m.initState.selected = 0
		} else if m.initState.customizationMode == "patterns" {
			// Patterns mode is now read-only (edit post-create.sh to customize)
			// Just go back to main customization
			m.initState.customizationMode = ""
			m.initState.selected = 0
		}
		return m, nil

	case key.Matches(msg, m.keys.Up):
		if m.initState.selected > 0 {
			m.initState.selected--
		}
		return m, nil

	case key.Matches(msg, m.keys.Down):
		var maxItems int
		if m.initState.customizationMode == "" {
			maxItems = 3 // worktree, patterns, postcreate
		} else if m.initState.customizationMode == "patterns" {
			maxItems = len(m.initState.detectedFiles)
		}
		if m.initState.selected < maxItems-1 {
			m.initState.selected++
		}
		return m, nil

	default:
		// Handle text editing for text fields
		if m.initState.customizationMode == "worktree" || m.initState.customizationMode == "postcreate" {
			switch msg.Type {
			case tea.KeyBackspace:
				if len(m.initState.editingText) > 0 {
					m.initState.editingText = m.initState.editingText[:len(m.initState.editingText)-1]
				}
			case tea.KeyRunes:
				m.initState.editingText += string(msg.Runes)
			}
		}
		return m, nil
	}
}

// handleCreateKeys handles keyboard input for the create view
func (m Model) handleCreateKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.createState == nil {
		return m, nil
	}

	logging.Debug("CreateView key: %q, step: %d, mode: %d", msg.String(), m.createState.currentStep, m.createState.createMode)

	// If we're in branch name input mode, only allow specific keys
	if m.createState.currentStep == CreateStepBranchName {
		switch {
		case key.Matches(msg, m.keys.Back):
			logging.Debug("CreateView: back from BranchName to BranchMode")
			m.createState.currentStep = CreateStepBranchMode
			return m, nil
		case key.Matches(msg, m.keys.Enter):
			if isValidBranchName(m.createState.branchName) {
				logging.Info("CreateView: branch name entered: %s, going to BaseBranch", m.createState.branchName)
				m.createState.currentStep = CreateStepBaseBranch
				// Reset search and center scroll when entering base branch step
				m.createState.searchQuery = ""
				m.createState.filteredBranches = m.createState.branchStatuses
				m.centerScrollOnSelectedBranch()
			}
			return m, nil
		case msg.Type == tea.KeyBackspace:
			if len(m.createState.branchName) > 0 {
				m.createState.branchName = m.createState.branchName[:len(m.createState.branchName)-1]
			}
			return m, nil
		case msg.Type == tea.KeyRunes:
			m.createState.branchName += string(msg.Runes)
			return m, nil
		default:
			// In text input mode, ignore all other shortcuts
			return m, nil
		}
	}

	// If we're in branch selection (base branch OR existing branch), handle search mode
	if m.createState.currentStep == CreateStepBaseBranch || m.createState.currentStep == CreateStepExistingBranch {
		// If in search mode, capture all typing
		if m.createState.isSearching {
			switch msg.Type {
			case tea.KeyBackspace:
				if len(m.createState.searchQuery) > 0 {
					m.createState.searchQuery = m.createState.searchQuery[:len(m.createState.searchQuery)-1]
					if m.createState.currentStep == CreateStepBaseBranch {
						m.filterBranches()
					} else {
						m.filterAvailableBranches()
					}
					logging.Debug("CreateView: search query updated: %q", m.createState.searchQuery)
				}
				return m, nil
			case tea.KeyRunes:
				m.createState.searchQuery += string(msg.Runes)
				if m.createState.currentStep == CreateStepBaseBranch {
					m.filterBranches()
				} else {
					m.filterAvailableBranches()
				}
				logging.Debug("CreateView: search query updated: %q", m.createState.searchQuery)
				return m, nil
			case tea.KeyEscape:
				// Exit search mode and clear search
				logging.Debug("CreateView: exited search mode (escape)")
				m.createState.isSearching = false
				m.createState.searchQuery = ""
				if m.createState.currentStep == CreateStepBaseBranch {
					m.createState.filteredBranches = m.createState.branchStatuses
				} else {
					m.createState.filteredAvailableBranches = m.createState.availableBranches
				}
				m.createState.selectedBranch = 0
				m.centerScrollOnSelectedBranch()
				return m, nil
			case tea.KeyEnter:
				// Exit search mode but keep filter, then let enter handler below select branch
				logging.Debug("CreateView: exited search mode (enter), filter: %q", m.createState.searchQuery)
				m.createState.isSearching = false
				// Don't return - let the enter key be handled below
			}
		} else {
			// Not in search mode - check for search trigger keys
			if msg.String() == "/" || msg.String() == "s" {
				logging.Debug("CreateView: entered search mode")
				m.createState.isSearching = true
				m.createState.searchQuery = ""
				return m, nil
			}
		}
	}

	// For other steps, handle normal shortcuts
	switch {
	case key.Matches(msg, m.keys.Quit):
		logging.Info("User quit from CreateView")
		return m, tea.Quit
	case key.Matches(msg, m.keys.Back):
		// Go back one step (CreateStepBranchName is handled in text input block above)
		switch m.createState.currentStep {
		case CreateStepBranchMode:
			logging.Info("CreateView: back to Dashboard from BranchMode")
			m.currentView = DashboardView
			return m, m.loadProjectInfo()
		case CreateStepBranchName:
			logging.Debug("CreateView: back to BranchMode from BranchName")
			m.createState.currentStep = CreateStepBranchMode
		case CreateStepExistingBranch:
			logging.Debug("CreateView: back to BranchMode from ExistingBranch")
			m.createState.currentStep = CreateStepBranchMode
		case CreateStepBaseBranch:
			if m.createState.createMode == CreateModeNewBranch {
				logging.Debug("CreateView: back to BranchName from BaseBranch")
				m.createState.currentStep = CreateStepBranchName
			} else {
				logging.Debug("CreateView: back to ExistingBranch from BaseBranch")
				m.createState.currentStep = CreateStepExistingBranch
			}
		case CreateStepConfirm:
			if m.createState.createMode == CreateModeNewBranch {
				logging.Debug("CreateView: back to BaseBranch from Confirm")
				m.createState.currentStep = CreateStepBaseBranch
			} else {
				logging.Debug("CreateView: back to ExistingBranch from Confirm")
				m.createState.currentStep = CreateStepExistingBranch
			}
		}
		return m, nil
	case key.Matches(msg, m.keys.Up):
		// Clear any errors when navigating
		m.err = nil
		switch m.createState.currentStep {
		case CreateStepBranchMode:
			if m.createState.selectedMode > 0 {
				m.createState.selectedMode--
			}
		case CreateStepExistingBranch:
			if m.createState.selectedBranch > 0 {
				m.createState.selectedBranch--
				// Adjust scroll offset to keep selection visible
				if m.createState.selectedBranch < m.createState.scrollOffset {
					m.createState.scrollOffset = m.createState.selectedBranch
				}
			}
		case CreateStepBaseBranch:
			if m.createState.selectedBranch > 0 {
				m.createState.selectedBranch--
				m.createState.warningAccepted = false
				// Adjust scroll offset to keep selection visible
				if m.createState.selectedBranch < m.createState.scrollOffset {
					m.createState.scrollOffset = m.createState.selectedBranch
				}
			}
		case CreateStepComplete:
			// Navigate up in actions list
			if m.createState.selectedAction > 0 {
				m.createState.selectedAction--
			}
		}
		return m, nil
	case key.Matches(msg, m.keys.Down):
		// Clear any errors when navigating
		m.err = nil
		switch m.createState.currentStep {
		case CreateStepBranchMode:
			if m.createState.selectedMode < 1 { // 0="Create new", 1="Use existing"
				m.createState.selectedMode++
			}
		case CreateStepExistingBranch:
			branches := m.createState.filteredAvailableBranches
			if len(branches) == 0 {
				branches = m.createState.availableBranches
			}
			if m.createState.selectedBranch < len(branches)-1 {
				m.createState.selectedBranch++
				// Adjust scroll offset to keep selection visible
				maxVisible := m.height - 15
				if maxVisible < 5 {
					maxVisible = 5
				}
				if maxVisible > 20 {
					maxVisible = 20
				}
				if m.createState.selectedBranch >= m.createState.scrollOffset+maxVisible {
					m.createState.scrollOffset = m.createState.selectedBranch - maxVisible + 1
				}
			}
		case CreateStepBaseBranch:
			branches := m.createState.filteredBranches
			if len(branches) == 0 {
				branches = m.createState.branchStatuses
			}
			if m.createState.selectedBranch < len(branches)-1 {
				m.createState.selectedBranch++
				m.createState.warningAccepted = false
				// Adjust scroll offset to keep selection visible
				maxVisible := m.height - 15
				if maxVisible < 5 {
					maxVisible = 5
				}
				if maxVisible > 20 {
					maxVisible = 20
				}
				if m.createState.selectedBranch >= m.createState.scrollOffset+maxVisible {
					m.createState.scrollOffset = m.createState.selectedBranch - maxVisible + 1
				}
			}
		case CreateStepComplete:
			// Navigate down in actions list
			actions := m.getAvailableActions()
			if m.createState.selectedAction < len(actions)-1 {
				m.createState.selectedAction++
			}
		}
		return m, nil
	case key.Matches(msg, m.keys.Enter):
		// Clear any errors when making selections
		m.err = nil
		switch m.createState.currentStep {
		case CreateStepBranchMode:
			// Set mode and advance to appropriate step
			if m.createState.selectedMode == 0 {
				logging.Info("CreateView: selected 'Create new branch' mode")
				m.createState.createMode = CreateModeNewBranch
				m.createState.currentStep = CreateStepBranchName
			} else {
				logging.Info("CreateView: selected 'Use existing branch' mode")
				m.createState.createMode = CreateModeExistingBranch
				m.createState.currentStep = CreateStepExistingBranch
				// Need to load available branches
				return m, m.loadAvailableBranches()
			}
			return m, nil
		case CreateStepExistingBranch:
			branches := m.createState.filteredAvailableBranches
			if len(branches) == 0 {
				branches = m.createState.availableBranches
			}
			if len(branches) > 0 && m.createState.selectedBranch < len(branches) {
				selectedBranch := branches[m.createState.selectedBranch]
				logging.Info("CreateView: selected existing branch: %s", selectedBranch.Name)
				m.createState.branchName = selectedBranch.Name
				m.createState.currentStep = CreateStepConfirm
			}
			return m, nil
		case CreateStepBaseBranch:
			branches := m.createState.filteredBranches
			if len(branches) == 0 {
				branches = m.createState.branchStatuses
			}
			if len(branches) > 0 && m.createState.selectedBranch < len(branches) {
				selectedStatus := branches[m.createState.selectedBranch]
				if !selectedStatus.IsClean && !m.createState.warningAccepted {
					// Show warning, don't advance
					logging.Debug("CreateView: showing dirty branch warning for: %s", selectedStatus.Name)
					m.createState.showWarning = true
					return m, nil
				}
				logging.Info("CreateView: selected base branch: %s (clean: %v)", selectedStatus.Name, selectedStatus.IsClean)
				m.createState.baseBranch = selectedStatus.Name
				m.createState.currentStep = CreateStepConfirm
			}
			return m, nil
		case CreateStepConfirm:
			logging.Info("CreateView: confirmed creation, branch: %s, base: %s", m.createState.branchName, m.createState.baseBranch)
			m.createState.currentStep = CreateStepCreating
			// Initialize spinner for creating step
			s := spinner.New()
			s.Spinner = spinner.Dot
			s.Style = SpinnerStyle
			m.createState.spinner = s
			return m, tea.Batch(m.createWorktree(), m.createState.spinner.Tick)
		case CreateStepComplete:
			// Execute the selected action from simple list
			actions := m.getAvailableActions()
			if m.createState.selectedAction < len(actions) {
				action := actions[m.createState.selectedAction]
				logging.Info("CreateView: executing action: %s", action.Name)
				// Handle navigate specially - it needs to quit TUI and write to temp file
				if action.Command == "navigate" {
					worktreePath := m.getWorktreePath(m.createState.branchName)
					logging.Info("CreateView: navigating to worktree: %s", worktreePath)
					return m, m.navigateToWorktree(worktreePath)
				}
				// Handle claude specially - quit TUI and launch claude in the worktree
				if action.Command == "claude" {
					worktreePath := m.getWorktreePath(m.createState.branchName)
					logging.Info("CreateView: launching Claude Code in: %s", worktreePath)
					return m, m.launchClaudeInWorktree(worktreePath)
				}
				// Execute other actions normally
				if action.Command != "" {
					if err := m.executeAction(action, m.getWorktreePath(m.createState.branchName)); err != nil {
						// Show error message briefly
						logging.Error("CreateView: action failed: %s - %v", action.Name, err)
						fmt.Printf("Failed to execute %s: %v\n", action.Name, err)
					}
				}
			}
			// Always return to dashboard after action
			logging.Debug("CreateView: returning to Dashboard")
			m.refreshWorktrees() // Refresh worktrees list
			m.currentView = DashboardView
			// Refresh project info to check if .gren was deleted/created
			return m, m.loadProjectInfo()
		}
	case msg.String() == "y":
		// Handle 'y' as confirmation only for base branch step
		if m.createState.currentStep == CreateStepBaseBranch {
			// Check if current selected branch is dirty (needs warning acceptance)
			branches := m.createState.filteredBranches
			if len(branches) == 0 {
				branches = m.createState.branchStatuses
			}
			if len(branches) > 0 && m.createState.selectedBranch < len(branches) {
				selectedStatus := branches[m.createState.selectedBranch]
				if !selectedStatus.IsClean {
					logging.Info("CreateView: user accepted dirty branch warning for: %s", selectedStatus.Name)
					m.createState.warningAccepted = true
					return m, nil
				}
			}
		}
		return m, nil
	}

	// List navigation for CreateStepComplete is now handled above

	return m, nil
}

// handleDeleteKeys handles keyboard input for the delete view
func (m Model) handleDeleteKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.deleteState == nil {
		return m, nil
	}

	logging.Debug("DeleteView key: %q, step: %d", msg.String(), m.deleteState.currentStep)

	switch m.deleteState.currentStep {
	case DeleteStepSelection:
		return m.handleDeleteSelectionKeys(msg)
	case DeleteStepConfirm:
		return m.handleDeleteConfirmKeys(msg)
	case DeleteStepDeleting:
		// Only allow quit during deletion
		switch {
		case key.Matches(msg, m.keys.Quit):
			logging.Info("User quit during deletion")
			return m, tea.Quit
		}
	case DeleteStepComplete:
		// Allow quit/back or enter to return to dashboard
		switch {
		case key.Matches(msg, m.keys.Quit):
			logging.Info("User quit from DeleteView complete")
			return m, tea.Quit
		case key.Matches(msg, m.keys.Back), key.Matches(msg, m.keys.Enter):
			logging.Info("DeleteView: returning to Dashboard after completion")
			m.currentView = DashboardView
			m.deleteState = nil
			return m, nil
		}
	}
	return m, nil
}

// handleDeleteSelectionKeys handles keyboard input for delete selection step
func (m Model) handleDeleteSelectionKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		logging.Info("User quit from DeleteView selection")
		return m, tea.Quit
	case key.Matches(msg, m.keys.Back):
		logging.Info("DeleteView: back to Dashboard from selection")
		m.currentView = DashboardView
		m.deleteState = nil
		return m, nil
	case key.Matches(msg, m.keys.Up):
		if m.selected > 0 {
			m.selected--
		}
		return m, nil
	case key.Matches(msg, m.keys.Down):
		// Count deletable worktrees (non-current ones)
		deletableCount := 0
		for _, wt := range m.worktrees {
			if !wt.IsCurrent {
				deletableCount++
			}
		}
		if m.selected < deletableCount-1 {
			m.selected++
		}
		return m, nil
	case key.Matches(msg, m.keys.Enter):
		if len(m.deleteState.selectedWorktrees) > 0 {
			logging.Info("DeleteView: proceeding to confirm with %d worktrees selected", len(m.deleteState.selectedWorktrees))
			m.deleteState.currentStep = DeleteStepConfirm
		}
		return m, nil
	case msg.String() == " ": // Space key for toggling selection
		// Find the index of the currently selected deletable worktree in the main worktrees list
		deletableIndex := 0
		actualIndex := -1
		for i, wt := range m.worktrees {
			if !wt.IsCurrent {
				if deletableIndex == m.selected {
					actualIndex = i
					break
				}
				deletableIndex++
			}
		}

		if actualIndex >= 0 {
			// Check if this worktree is already selected
			alreadySelected := false
			for i, selectedIdx := range m.deleteState.selectedWorktrees {
				if selectedIdx == actualIndex {
					// Remove from selection
					logging.Debug("DeleteView: deselected worktree: %s", m.worktrees[actualIndex].Name)
					m.deleteState.selectedWorktrees = append(
						m.deleteState.selectedWorktrees[:i],
						m.deleteState.selectedWorktrees[i+1:]...)
					alreadySelected = true
					break
				}
			}
			if !alreadySelected {
				// Add to selection
				logging.Debug("DeleteView: selected worktree: %s", m.worktrees[actualIndex].Name)
				m.deleteState.selectedWorktrees = append(m.deleteState.selectedWorktrees, actualIndex)
			}
		}
		return m, nil
	}
	return m, nil
}

// handleDeleteConfirmKeys handles keyboard input for delete confirmation step
func (m Model) handleDeleteConfirmKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		logging.Info("User quit from DeleteView confirm")
		return m, tea.Quit
	case key.Matches(msg, m.keys.Back):
		// For single worktree deletion, go back to dashboard
		// For multi-select deletion, go back to selection step
		if m.deleteState.targetWorktree != nil {
			logging.Info("DeleteView: cancelled single worktree deletion, back to Dashboard")
			m.currentView = DashboardView
			m.deleteState = nil
		} else {
			logging.Debug("DeleteView: back to selection from confirm")
			m.deleteState.currentStep = DeleteStepSelection
		}
		return m, nil
	case msg.String() == "y" || msg.String() == "Y":
		// Proceed with deletion
		logging.Info("DeleteView: user confirmed deletion")
		m.deleteState.currentStep = DeleteStepDeleting
		// Check if worktree has uncommitted changes - if so, we need --force
		if m.deleteState.targetWorktree != nil {
			wt := m.deleteState.targetWorktree
			if wt.StagedCount > 0 || wt.ModifiedCount > 0 || wt.UntrackedCount > 0 {
				m.deleteState.forceDelete = true
				logging.Info("DeleteView: worktree has uncommitted changes, will use --force")
			}
		}
		// Start spinner and deletion command
		return m, tea.Batch(m.deleteSpinner.Tick, m.deleteSelectedWorktrees())
	case msg.String() == "n" || msg.String() == "N":
		// Cancel deletion
		logging.Info("DeleteView: user cancelled deletion")
		m.currentView = DashboardView
		m.deleteState = nil
		return m, nil
	}
	return m, nil
}

// handleOpenInKeys handles keyboard input for the "Open in..." view
func (m Model) handleOpenInKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.openInState == nil {
		return m, nil
	}

	logging.Debug("OpenInView key: %q", msg.String())

	switch {
	case key.Matches(msg, m.keys.Quit):
		logging.Info("User quit from OpenInView")
		return m, tea.Quit
	case key.Matches(msg, m.keys.Back):
		// Return to dashboard
		logging.Debug("OpenInView: back to Dashboard")
		m.currentView = DashboardView
		m.openInState = nil
		return m, nil
	case key.Matches(msg, m.keys.Up):
		// Navigate up in the list
		if m.openInState.selectedIndex > 0 {
			m.openInState.selectedIndex--
		}
		return m, nil
	case key.Matches(msg, m.keys.Down):
		// Navigate down in the list
		if m.openInState.selectedIndex < len(m.openInState.actions)-1 {
			m.openInState.selectedIndex++
		}
		return m, nil
	case key.Matches(msg, m.keys.Enter):
		// Execute the selected action
		if m.openInState.selectedIndex < len(m.openInState.actions) {
			action := m.openInState.actions[m.openInState.selectedIndex]
			logging.Info("OpenInView: executing action: %s on %s", action.Name, m.openInState.worktreePath)
			// Handle navigate specially - it needs to quit TUI and write to temp file
			if action.Command == "navigate" {
				logging.Info("OpenInView: navigating to worktree: %s", m.openInState.worktreePath)
				return m, m.navigateToWorktree(m.openInState.worktreePath)
			}
			// Handle claude specially - quit TUI and launch claude in the worktree
			if action.Command == "claude" {
				logging.Info("OpenInView: launching Claude Code in: %s", m.openInState.worktreePath)
				return m, m.launchClaudeInWorktree(m.openInState.worktreePath)
			}
			// Execute other actions normally
			if action.Command != "" {
				if err := m.executeAction(action, m.openInState.worktreePath); err != nil {
					// Show error message briefly
					logging.Error("OpenInView: action failed: %s - %v", action.Name, err)
					fmt.Printf("Failed to execute %s: %v\n", action.Name, err)
				}
			}
		}
		// Return to dashboard after action
		logging.Debug("OpenInView: returning to Dashboard")
		m.currentView = DashboardView
		m.openInState = nil
		return m, nil
	}

	return m, nil
}

// handleAIResultKeys handles keyboard input for the AI result step
func (m Model) handleAIResultKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.initState == nil {
		return m, nil
	}

	// If there was an error, any key goes back to recommendations
	if m.initState.aiError != "" {
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Enter), key.Matches(msg, m.keys.Back):
			m.initState.currentStep = InitStepRecommendations
			m.initState.selected = 2 // Keep AI option selected
			return m, nil
		}
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Back):
		m.initState.currentStep = InitStepRecommendations
		m.initState.selected = 2
		return m, nil
	case key.Matches(msg, m.keys.Up):
		if m.initState.selected > 0 {
			m.initState.selected--
		}
		return m, nil
	case key.Matches(msg, m.keys.Down):
		if m.initState.selected < 2 { // 3 options
			m.initState.selected++
		}
		return m, nil
	case key.Matches(msg, m.keys.Enter):
		switch m.initState.selected {
		case 0:
			// Use this script - save it and go to preview
			logging.Info("InitView: using AI-generated script")
			m.initState.postCreateScript = m.initState.aiGeneratedScript
			m.initState.currentStep = InitStepPreview
			m.initState.selected = 0
		case 1:
			// Regenerate
			logging.Info("InitView: regenerating AI script")
			m.initState.currentStep = InitStepAIGenerating
			return m, m.generateAISetupScript()
		case 2:
			// Edit manually instead - go to customization
			logging.Info("InitView: editing manually instead")
			m.initState.currentStep = InitStepCustomization
			m.initState.customizationMode = "postcreate"
			m.initState.editingText = m.initState.aiGeneratedScript
			m.initState.selected = 0
		}
		return m, nil
	}

	return m, nil
}

// handleConfigKeys handles keyboard input for the config view
func (m Model) handleConfigKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.configState == nil {
		return m, nil
	}

	logging.Debug("ConfigView key: %q", msg.String())

	switch {
	case key.Matches(msg, m.keys.Quit):
		logging.Info("User quit from ConfigView")
		return m, tea.Quit
	case key.Matches(msg, m.keys.Back):
		// Return to dashboard
		logging.Debug("ConfigView: back to Dashboard")
		m.currentView = DashboardView
		m.configState = nil
		return m, nil
	case key.Matches(msg, m.keys.Up):
		// Navigate up in the list
		if m.configState.selectedIndex > 0 {
			m.configState.selectedIndex--
		}
		return m, nil
	case key.Matches(msg, m.keys.Down):
		// Navigate down in the list
		if m.configState.selectedIndex < len(m.configState.files)-1 {
			m.configState.selectedIndex++
		}
		return m, nil
	case key.Matches(msg, m.keys.Enter):
		// Open the selected config file
		if m.configState.selectedIndex < len(m.configState.files) {
			selectedFile := m.configState.files[m.configState.selectedIndex]
			logging.Info("ConfigView: opening config file: %s", selectedFile.Path)
			return m, m.openConfigFile(selectedFile.Path)
		}
		return m, nil
	}

	return m, nil
}

// handleMergeKeys handles keyboard input for the merge view
func (m Model) handleMergeKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.mergeState == nil {
		return m, nil
	}

	logging.Debug("MergeView key: %q, step: %d", msg.String(), m.mergeState.currentStep)

	switch m.mergeState.currentStep {
	case MergeStepConfirm:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Back):
			m.currentView = DashboardView
			m.mergeState = nil
			return m, nil
		case key.Matches(msg, m.keys.Up), key.Matches(msg, m.keys.Down):
			// Toggle options
			switch msg.String() {
			case "s", "S":
				m.mergeState.squash = !m.mergeState.squash
			case "r", "R":
				m.mergeState.rebase = !m.mergeState.rebase
			case "d", "D":
				m.mergeState.remove = !m.mergeState.remove
			}
			return m, nil
		case msg.String() == "s" || msg.String() == "S":
			m.mergeState.squash = !m.mergeState.squash
			return m, nil
		case msg.String() == "r" || msg.String() == "R":
			m.mergeState.rebase = !m.mergeState.rebase
			return m, nil
		case msg.String() == "d" || msg.String() == "D":
			m.mergeState.remove = !m.mergeState.remove
			return m, nil
		case key.Matches(msg, m.keys.Enter):
			m.mergeState.currentStep = MergeStepInProgress
			return m, m.executeMerge()
		}
	case MergeStepInProgress:
		// Only allow quit during merge
		if key.Matches(msg, m.keys.Quit) {
			return m, tea.Quit
		}
		return m, nil
	case MergeStepComplete:
		switch {
		case key.Matches(msg, m.keys.Back), key.Matches(msg, m.keys.Enter):
			m.currentView = DashboardView
			m.mergeState = nil
			return m, nil
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		}
	}

	return m, nil
}

// handleForEachKeys handles keyboard input for the for-each view
func (m Model) handleForEachKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.forEachState == nil {
		return m, nil
	}

	logging.Debug("ForEachView key: %q, inputMode: %v", msg.String(), m.forEachState.inputMode)

	// If showing results (not in progress and has results), allow closing
	if !m.forEachState.inProgress && len(m.forEachState.results) > 0 {
		switch {
		case key.Matches(msg, m.keys.Back), key.Matches(msg, m.keys.Enter):
			m.currentView = DashboardView
			m.forEachState = nil
			return m, nil
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		}
		return m, nil
	}

	// Command input mode
	if m.forEachState.inputMode {
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Back):
			m.currentView = DashboardView
			m.forEachState = nil
			return m, nil
		case key.Matches(msg, m.keys.Enter):
			if strings.TrimSpace(m.forEachState.command) != "" {
				m.forEachState.inputMode = false
				m.forEachState.inProgress = true
				return m, m.executeForEach()
			}
			return m, nil
		case msg.Type == tea.KeyBackspace:
			if len(m.forEachState.command) > 0 {
				m.forEachState.command = m.forEachState.command[:len(m.forEachState.command)-1]
			}
			return m, nil
		case msg.Type == tea.KeyRunes:
			m.forEachState.command += string(msg.Runes)
			return m, nil
		case msg.String() == "tab":
			// Toggle options with tab
			m.forEachState.skipMain = !m.forEachState.skipMain
			return m, nil
		}
	}

	return m, nil
}

// handleStepCommitKeys handles keyboard input for the step commit view
func (m Model) handleStepCommitKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.stepCommitState == nil {
		return m, nil
	}

	logging.Debug("StepCommitView key: %q, step: %d", msg.String(), m.stepCommitState.currentStep)

	switch m.stepCommitState.currentStep {
	case StepCommitStepOptions:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Back):
			m.currentView = DashboardView
			m.stepCommitState = nil
			return m, nil
		case msg.String() == "tab":
			// Toggle LLM option with tab (consistent with for-each view)
			m.stepCommitState.useLLM = !m.stepCommitState.useLLM
			return m, nil
		case key.Matches(msg, m.keys.Enter):
			if m.stepCommitState.useLLM {
				// Go directly to execution with LLM
				m.stepCommitState.currentStep = StepCommitStepInProgress
				return m, m.executeStepCommit()
			}
			// Need to enter commit message
			m.stepCommitState.currentStep = StepCommitStepMessage
			return m, nil
		}
	case StepCommitStepMessage:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Back):
			m.stepCommitState.currentStep = StepCommitStepOptions
			return m, nil
		case key.Matches(msg, m.keys.Enter):
			if strings.TrimSpace(m.stepCommitState.message) != "" {
				m.stepCommitState.currentStep = StepCommitStepInProgress
				return m, m.executeStepCommit()
			}
			return m, nil
		case msg.Type == tea.KeyBackspace:
			if len(m.stepCommitState.message) > 0 {
				m.stepCommitState.message = m.stepCommitState.message[:len(m.stepCommitState.message)-1]
			}
			return m, nil
		case msg.Type == tea.KeyRunes:
			m.stepCommitState.message += string(msg.Runes)
			return m, nil
		}
	case StepCommitStepInProgress:
		if key.Matches(msg, m.keys.Quit) {
			return m, tea.Quit
		}
		return m, nil
	case StepCommitStepComplete:
		switch {
		case key.Matches(msg, m.keys.Back), key.Matches(msg, m.keys.Enter):
			m.currentView = DashboardView
			m.stepCommitState = nil
			return m, nil
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		}
	}

	return m, nil
}

// handleCompareKeys handles keyboard input for the compare view
func (m Model) handleCompareKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.compareState == nil {
		return m, nil
	}

	logging.Debug("CompareView key: %q, diffFocused: %v", msg.String(), m.compareState.diffFocused)

	// If apply is complete, any key returns to dashboard
	if m.compareState.applyComplete {
		logging.Debug("CompareView: returning to Dashboard after apply")
		m.currentView = DashboardView
		m.compareState = nil
		return m, nil
	}

	// Handle diff focused mode (scrolling diff)
	if m.compareState.diffFocused {
		switch {
		case key.Matches(msg, m.keys.Back), key.Matches(msg, m.keys.Left), msg.String() == "h" || msg.String() == "H":
			// Exit diff focus mode
			m.compareState.diffFocused = false
			return m, nil
		case key.Matches(msg, m.keys.Up), msg.String() == "k" || msg.String() == "K":
			// Scroll diff up
			if m.compareState.diffScrollOffset > 0 {
				m.compareState.diffScrollOffset--
			}
			return m, nil
		case key.Matches(msg, m.keys.Down), msg.String() == "j" || msg.String() == "J":
			// Scroll diff down
			diffLines := len(strings.Split(m.compareState.diffContent, "\n"))
			visibleLines := m.height - 10
			if m.compareState.diffScrollOffset < diffLines-visibleLines {
				m.compareState.diffScrollOffset++
			}
			return m, nil
		}
		return m, nil
	}

	// Normal mode (file list focused)
	switch {
	case msg.String() == "?":
		// Toggle help overlay
		m.helpVisible = !m.helpVisible
		return m, nil
	case key.Matches(msg, m.keys.Back):
		// Esc - close help if visible, otherwise go back
		if m.helpVisible {
			m.helpVisible = false
			return m, nil
		}
		logging.Debug("CompareView: back to Dashboard (esc)")
		m.currentView = DashboardView
		m.compareState = nil
		return m, nil
	case key.Matches(msg, m.keys.Up), msg.String() == "k" || msg.String() == "K":
		// Navigate up in the file list
		if m.compareState.selectedIndex > 0 {
			m.compareState.selectedIndex--
			m.compareState.diffScrollOffset = 0 // Reset diff scroll
			// Adjust scroll offset if needed
			if m.compareState.selectedIndex < m.compareState.scrollOffset {
				m.compareState.scrollOffset = m.compareState.selectedIndex
			}
			// Load diff for new selection
			file := m.compareState.files[m.compareState.selectedIndex]
			return m, m.loadCompareDiff(m.compareState.sourcePath, file.Path)
		}
		return m, nil
	case key.Matches(msg, m.keys.Down), msg.String() == "j" || msg.String() == "J":
		// Navigate down in the file list
		if m.compareState.selectedIndex < len(m.compareState.files)-1 {
			m.compareState.selectedIndex++
			m.compareState.diffScrollOffset = 0 // Reset diff scroll
			// Adjust scroll offset if needed
			visibleLines := m.height - 10
			if visibleLines < 5 {
				visibleLines = 5
			}
			if m.compareState.selectedIndex >= m.compareState.scrollOffset+visibleLines {
				m.compareState.scrollOffset = m.compareState.selectedIndex - visibleLines + 1
			}
			// Load diff for new selection
			file := m.compareState.files[m.compareState.selectedIndex]
			return m, m.loadCompareDiff(m.compareState.sourcePath, file.Path)
		}
		return m, nil
	case key.Matches(msg, m.keys.Enter), key.Matches(msg, m.keys.Right), msg.String() == "l" || msg.String() == "L":
		// Enter diff focus mode (for scrolling)
		m.compareState.diffFocused = true
		return m, nil
	case msg.String() == " ": // Space key
		// Toggle selection for current file
		if m.compareState.selectedIndex < len(m.compareState.files) {
			m.compareState.files[m.compareState.selectedIndex].Selected = !m.compareState.files[m.compareState.selectedIndex].Selected
			logging.Debug("CompareView: toggled selection for %s: %v",
				m.compareState.files[m.compareState.selectedIndex].Path,
				m.compareState.files[m.compareState.selectedIndex].Selected)
		}
		return m, nil
	case msg.String() == "a" || msg.String() == "A":
		// Toggle all files
		allSelected := true
		for _, f := range m.compareState.files {
			if !f.Selected {
				allSelected = false
				break
			}
		}
		// Toggle: if all selected, deselect all; otherwise select all
		newState := !allSelected
		for i := range m.compareState.files {
			m.compareState.files[i].Selected = newState
		}
		m.compareState.selectAll = newState
		logging.Debug("CompareView: toggled all files to: %v", newState)
		return m, nil
	case msg.String() == "y" || msg.String() == "Y":
		// Apply selected files
		selectedCount := 0
		for _, f := range m.compareState.files {
			if f.Selected {
				selectedCount++
			}
		}
		if selectedCount == 0 {
			logging.Debug("CompareView: no files selected to apply")
			return m, nil
		}
		logging.Info("CompareView: applying %d selected files", selectedCount)
		m.compareState.applyInProgress = true
		return m, m.applyCompareChanges()
	}

	return m, nil
}
