package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// handleInitKeys handles keyboard input for the init view
func (m Model) handleInitKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.initState == nil {
		return m, nil
	}

	switch m.initState.currentStep {
	case InitStepCustomization:
		return m.handleCustomizationKeys(msg)
	}

	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Back):
		if m.initState.currentStep == InitStepWelcome {
			m.currentView = DashboardView
			return m, nil
		}
		// Go back one step
		if m.initState.currentStep > InitStepWelcome {
			m.initState.currentStep--
		}
		return m, nil
	case key.Matches(msg, m.keys.Up):
		if m.initState.currentStep == InitStepRecommendations ||
			m.initState.currentStep == InitStepPreview {
			if m.initState.selected > 0 {
				m.initState.selected--
			}
		}
		return m, nil
	case key.Matches(msg, m.keys.Down):
		var maxItems int
		switch m.initState.currentStep {
		case InitStepRecommendations:
			maxItems = len(m.initState.copyPatterns) + 1 // +1 for customization option
		case InitStepPreview:
			maxItems = 4 // Number of preview items
		}
		if m.initState.selected < maxItems-1 {
			m.initState.selected++
		}
		return m, nil
	case key.Matches(msg, m.keys.Enter):
		switch m.initState.currentStep {
		case InitStepWelcome:
			m.initState.currentStep = InitStepAnalysis
			return m, m.runProjectAnalysis()
		case InitStepRecommendations:
			if m.initState.selected == len(m.initState.copyPatterns) {
				// Selected "Customize" option
				m.initState.currentStep = InitStepCustomization
				m.initState.selected = 0
			} else {
				// Skip to preview
				m.initState.currentStep = InitStepPreview
				m.initState.selected = 0
			}
			return m, nil
		case InitStepPreview:
			// Execute initialization
			m.initState.currentStep = InitStepExecuting
			return m, m.runInitialization()
		case InitStepCreated:
			// Open script in editor
			m.initState.currentStep = InitStepExecuting
			return m, m.openPostCreateScript()
		case InitStepCommitConfirm:
			// Commit changes
			m.initState.currentStep = InitStepFinal
			return m, m.commitConfiguration()
		case InitStepComplete:
			// Go to final step
			m.initState.currentStep = InitStepFinal
			return m, nil
		case InitStepFinal:
			// Return to dashboard
			m.currentView = DashboardView
			return m, m.loadProjectInfo()
		}
	case msg.String() == "y" || msg.String() == "Y":
		if m.initState.currentStep == InitStepRecommendations {
			m.initState.currentStep = InitStepPreview
			m.initState.selected = 0
		} else if m.initState.currentStep == InitStepCreated {
			// Open script in external editor
			m.initState.currentStep = InitStepExecuting
			return m, m.openPostCreateScript()
		} else if m.initState.currentStep == InitStepCommitConfirm {
			// Commit the changes
			m.initState.currentStep = InitStepFinal
			return m, m.commitConfiguration()
		}
		return m, nil
	case msg.String() == "n" || msg.String() == "N":
		if m.initState.currentStep == InitStepRecommendations {
			m.initState.currentStep = InitStepWelcome
		} else if m.initState.currentStep == InitStepCreated {
			// Skip opening, go to complete
			m.initState.currentStep = InitStepComplete
		} else if m.initState.currentStep == InitStepCommitConfirm {
			// Skip commit, go to final
			m.initState.currentStep = InitStepFinal
			// Mark as initialized since we're skipping commit
			if m.repoInfo != nil {
				m.repoInfo.IsInitialized = true
			}
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
			// Toggle pattern selection and stay in patterns mode
			if m.initState.selected < len(m.initState.copyPatterns) {
				// This is a simplified toggle - in real implementation you'd toggle the pattern
			}
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
			maxItems = len(m.initState.copyPatterns)
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

	// If we're in branch name input mode, only allow specific keys
	if m.createState.currentStep == CreateStepBranchName {
		switch {
		case key.Matches(msg, m.keys.Back):
			m.currentView = DashboardView
			return m, m.loadProjectInfo()
		case key.Matches(msg, m.keys.Enter):
			if isValidBranchName(m.createState.branchName) {
				m.createState.currentStep = CreateStepBaseBranch
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

	// For other steps, handle normal shortcuts
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Back):
		// Go back one step (CreateStepBranchName is handled in text input block above)
		if m.createState.currentStep > CreateStepBranchName {
			m.createState.currentStep--
		}
		return m, nil
	case key.Matches(msg, m.keys.Up):
		if m.createState.currentStep == CreateStepBaseBranch {
			if m.createState.selectedBranch > 0 {
				m.createState.selectedBranch--
				m.createState.warningAccepted = false
			}
		} else if m.createState.currentStep == CreateStepComplete {
			// Navigate up in actions list
			if m.createState.selectedAction > 0 {
				m.createState.selectedAction--
			}
		}
		return m, nil
	case key.Matches(msg, m.keys.Down):
		if m.createState.currentStep == CreateStepBaseBranch {
			if m.createState.selectedBranch < len(m.createState.branchStatuses)-1 {
				m.createState.selectedBranch++
				m.createState.warningAccepted = false
			}
		} else if m.createState.currentStep == CreateStepComplete {
			// Navigate down in actions list
			actions := m.getAvailableActions()
			if m.createState.selectedAction < len(actions)-1 {
				m.createState.selectedAction++
			}
		}
		return m, nil
	case key.Matches(msg, m.keys.Enter):
		switch m.createState.currentStep {
		case CreateStepBaseBranch:
			if len(m.createState.branchStatuses) > 0 && m.createState.selectedBranch < len(m.createState.branchStatuses) {
				selectedStatus := m.createState.branchStatuses[m.createState.selectedBranch]
				if !selectedStatus.IsClean && !m.createState.warningAccepted {
					// Show warning, don't advance
					m.createState.showWarning = true
					return m, nil
				}
				m.createState.baseBranch = selectedStatus.Name
				m.createState.currentStep = CreateStepConfirm
			}
			return m, nil
		case CreateStepConfirm:
			m.createState.currentStep = CreateStepCreating
			return m, m.createWorktree()
		case CreateStepComplete:
			// Execute the selected action from simple list
			actions := m.getAvailableActions()
			if m.createState.selectedAction < len(actions) {
				action := actions[m.createState.selectedAction]
				// Execute the action if it has a command
				if action.Command != "" {
					if err := m.executeAction(action, fmt.Sprintf("../gren-worktrees/%s", m.createState.branchName)); err != nil {
						// Show error message briefly
						fmt.Printf("Failed to execute %s: %v\n", action.Name, err)
					}
				}
			}
			// Always return to dashboard after action
			m.refreshWorktrees() // Refresh worktrees list
			m.currentView = DashboardView
			// Refresh project info to check if .gren was deleted/created
			return m, m.loadProjectInfo()
		}
	case msg.String() == "y":
		// Handle 'y' as confirmation only for base branch step
		if m.createState.currentStep == CreateStepBaseBranch {
			// Check if current selected branch is dirty (needs warning acceptance)
			if len(m.createState.branchStatuses) > 0 && m.createState.selectedBranch < len(m.createState.branchStatuses) {
				selectedStatus := m.createState.branchStatuses[m.createState.selectedBranch]
				if !selectedStatus.IsClean {
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

	switch m.deleteState.currentStep {
	case DeleteStepSelection:
		return m.handleDeleteSelectionKeys(msg)
	case DeleteStepConfirm:
		return m.handleDeleteConfirmKeys(msg)
	case DeleteStepDeleting:
		// Only allow quit during deletion
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		}
	case DeleteStepComplete:
		// Allow quit/back or enter to return to dashboard
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Back), key.Matches(msg, m.keys.Enter):
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
		return m, tea.Quit
	case key.Matches(msg, m.keys.Back):
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
					m.deleteState.selectedWorktrees = append(
						m.deleteState.selectedWorktrees[:i],
						m.deleteState.selectedWorktrees[i+1:]...)
					alreadySelected = true
					break
				}
			}
			if !alreadySelected {
				// Add to selection
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
		return m, tea.Quit
	case key.Matches(msg, m.keys.Back):
		m.deleteState.currentStep = DeleteStepSelection
		return m, nil
	case msg.String() == "y" || msg.String() == "Y":
		// Proceed with deletion
		m.deleteState.currentStep = DeleteStepDeleting
		return m, m.deleteSelectedWorktrees()
	case msg.String() == "n" || msg.String() == "N":
		// Cancel deletion
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

	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Back):
		// Return to dashboard
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
			// Execute the action if it has a command
			if action.Command != "" {
				if err := m.executeAction(action, m.openInState.worktreePath); err != nil {
					// Show error message briefly
					fmt.Printf("Failed to execute %s: %v\n", action.Name, err)
				}
			}
		}
		// Return to dashboard after action
		m.currentView = DashboardView
		m.openInState = nil
		return m, nil
	}

	return m, nil
}