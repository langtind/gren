package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/langtind/gren/internal/logging"
)

// ToolAction represents an action in the Tools menu
type ToolAction struct {
	Key         string
	Name        string
	Description string
	IsSection   bool // If true, this is a section header, not an action
}

func getToolActions(hasPR bool, hasSelectedWorktree bool) []ToolAction {
	actions := []ToolAction{
		{Key: "r", Name: "Refresh status", Description: "Re-check stale status (git + GitHub)"},
		{Key: "c", Name: "Cleanup stale worktrees", Description: "Delete all stale worktrees"},
		{Key: "x", Name: "Prune missing worktrees", Description: "Remove references to deleted worktree directories"},
	}

	actions = append(actions,
		ToolAction{IsSection: true, Name: "Workflow"},
		ToolAction{Key: "s", Name: "Commit changes", Description: "Stage and commit with optional LLM message"},
		ToolAction{Key: "f", Name: "Run in all worktrees", Description: "Execute command in every worktree"},
	)

	if hasSelectedWorktree {
		actions = append(actions,
			ToolAction{Key: "M", Name: "Merge to main", Description: "Squash, merge, and cleanup worktree"},
		)
	}

	if hasPR {
		actions = append(actions,
			ToolAction{IsSection: true, Name: "GitHub"},
			ToolAction{Key: "p", Name: "Open PR in browser", Description: "Open the pull request in your browser"},
		)
	}

	return actions
}

// renderToolsMenu renders the Tools menu modal content
func (m Model) renderToolsMenu() string {
	logging.Debug("Rendering Tools menu")

	var content strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true)
	content.WriteString(titleStyle.Render("Tools"))
	content.WriteString("\n\n")

	hasPR := false
	hasSelectedWorktree := false
	if m.selected >= 0 && m.selected < len(m.worktrees) {
		hasPR = m.worktrees[m.selected].PRNumber > 0
		hasSelectedWorktree = !m.worktrees[m.selected].IsCurrent && !m.worktrees[m.selected].IsMain
	}

	actions := getToolActions(hasPR, hasSelectedWorktree)
	keyStyle := lipgloss.NewStyle().Foreground(ColorSecondary).Bold(true)
	nameStyle := lipgloss.NewStyle().Foreground(ColorText)
	sectionStyle := lipgloss.NewStyle().Foreground(ColorTextMuted).Bold(true)

	for _, action := range actions {
		if action.IsSection {
			content.WriteString("\n")
			content.WriteString("  ")
			content.WriteString(sectionStyle.Render("─── " + action.Name + " ───"))
			content.WriteString("\n")
		} else {
			content.WriteString("  ")
			content.WriteString(keyStyle.Render(action.Key))
			content.WriteString(" · ")
			content.WriteString(nameStyle.Render(action.Name))
			content.WriteString("\n")
		}
	}

	// Help text
	content.WriteString("\n")
	helpStyle := lipgloss.NewStyle().Foreground(ColorTextMuted)
	content.WriteString(helpStyle.Render("Press key to execute • ESC to close"))

	return content.String()
}

// renderToolsModal renders the Tools menu as a modal overlay on the dashboard
func (m Model) renderToolsModal(baseView string) string {
	logging.Debug("Rendering Tools modal overlay")
	return m.renderWithModalWidth(baseView, m.renderToolsMenu(), 70, ColorPrimary)
}

// handleToolsKeys handles keyboard input for the Tools menu
func (m Model) handleToolsKeys(keyMsg tea.KeyMsg) (Model, tea.Cmd) {
	logging.Debug("Tools menu key: %q", keyMsg.String())

	switch {
	case key.Matches(keyMsg, m.keys.Back), key.Matches(keyMsg, m.keys.Tools):
		// ESC or 't' again closes the menu
		logging.Info("Tools menu: closing (ESC or 't')")
		m.currentView = DashboardView
		return m, nil

	case key.Matches(keyMsg, m.keys.Quit):
		// 'q' quits from Tools menu too
		logging.Info("Tools menu: user quit")
		return m, tea.Quit
	}

	// Handle tool-specific keys
	keyStr := keyMsg.String()
	switch keyStr {
	case "r":
		// Refresh status (git + GitHub)
		logging.Info("Tools menu: refreshing status")
		m.currentView = DashboardView
		m.githubLoading = true
		return m, tea.Batch(m.githubSpinner.Tick, m.refreshAllStatus())

	case "c":
		// Cleanup stale worktrees - show confirmation first
		logging.Info("Tools menu: showing cleanup confirmation")

		// Find stale worktrees
		var staleWorktrees []Worktree
		for _, wt := range m.worktrees {
			if wt.BranchStatus == "stale" && !wt.IsCurrent && !wt.IsMain {
				staleWorktrees = append(staleWorktrees, wt)
			}
		}

		if len(staleWorktrees) == 0 {
			logging.Info("Tools menu: no stale worktrees to cleanup")
			m.currentView = DashboardView
			return m, nil
		}

		// Create spinner for cleanup progress
		s := spinner.New()
		s.Spinner = spinner.Dot
		s.Style = lipgloss.NewStyle().Foreground(ColorWarning)

		// Select all worktrees by default
		selectedIndices := make(map[int]bool)
		for i := range staleWorktrees {
			selectedIndices[i] = true
		}

		m.cleanupState = &CleanupState{
			staleWorktrees:  staleWorktrees,
			confirmed:       false,
			selectedIndices: selectedIndices,
			cursorIndex:     0,
			inProgress:      false,
			currentIndex:    -1,
			deletedIndices:  make(map[int]bool),
			failedWorktrees: make(map[int]string),
			totalCleaned:    0,
			totalFailed:     0,
			cleanupSpinner:  s,
		}
		m.currentView = CleanupView
		return m, nil

	case "x":
		// Prune missing worktrees
		logging.Info("Tools menu: running prune")
		m.currentView = DashboardView
		return m, m.pruneWorktrees()

	case "p":
		if m.selected >= 0 && m.selected < len(m.worktrees) {
			wt := m.worktrees[m.selected]
			if wt.PRNumber > 0 {
				logging.Info("Tools menu: opening PR #%d for %s", wt.PRNumber, wt.Branch)
				m.currentView = DashboardView
				return m, m.openPRInBrowser(wt.Branch)
			}
		}
		return m, nil

	case "s":
		logging.Info("Tools menu: opening step commit")
		m.stepCommitState = &StepCommitState{
			currentStep: StepCommitStepOptions,
			useLLM:      false,
			message:     "",
		}
		m.currentView = StepCommitView
		return m, nil

	case "f":
		logging.Info("Tools menu: opening for-each")
		m.forEachState = &ForEachState{
			command:     "",
			skipCurrent: false,
			skipMain:    true,
			inProgress:  false,
			inputMode:   true,
			results:     []ForEachResult{},
		}
		m.currentView = ForEachView
		return m, nil

	case "M":
		if m.selected >= 0 && m.selected < len(m.worktrees) {
			wt := m.worktrees[m.selected]
			if !wt.IsCurrent && !wt.IsMain {
				logging.Info("Tools menu: opening merge for %s", wt.Branch)
				m.mergeState = &MergeState{
					currentStep:    MergeStepConfirm,
					sourceWorktree: &wt,
					targetBranch:   m.getDefaultBranch(),
					squash:         true,
					remove:         true,
					rebase:         true,
				}
				m.currentView = MergeView
				return m, nil
			}
		}
		return m, nil
	}

	return m, nil
}

func (m Model) getDefaultBranch() string {
	for _, wt := range m.worktrees {
		if wt.Branch == "main" || wt.Branch == "master" {
			return wt.Branch
		}
	}
	return "main"
}

// renderCleanupConfirmation renders appropriate view based on cleanup state
func (m Model) renderCleanupConfirmation() string {
	if m.cleanupState == nil {
		return ""
	}

	// Route to appropriate sub-view
	if m.cleanupState.inProgress {
		return m.renderCleanupProgress()
	}

	if !m.cleanupState.inProgress && len(m.cleanupState.failedWorktrees) > 0 {
		return m.renderCleanupFailureSummary()
	}

	// Default: show initial confirmation
	return m.renderCleanupConfirmationInitial()
}

// renderCleanupConfirmationInitial renders the interactive selection dialog
func (m Model) renderCleanupConfirmationInitial() string {
	if m.cleanupState == nil || len(m.cleanupState.staleWorktrees) == 0 {
		return ""
	}

	var b strings.Builder

	// Header
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorWarning).
		MarginBottom(1)
	b.WriteString(titleStyle.Render("⚠ Cleanup Stale Worktrees"))
	b.WriteString("\n\n")

	// Check if any selected worktree has submodules
	hasSelectedSubmodules := false
	for i, wt := range m.cleanupState.staleWorktrees {
		if m.cleanupState.selectedIndices[i] && wt.HasSubmodules {
			hasSelectedSubmodules = true
			break
		}
	}

	// Description
	selectedCount := len(m.cleanupState.selectedIndices)
	totalCount := len(m.cleanupState.staleWorktrees)
	descStyle := lipgloss.NewStyle().Foreground(ColorTextSecondary)
	b.WriteString(descStyle.Render(fmt.Sprintf("Select worktrees to delete (%d/%d selected):", selectedCount, totalCount)))
	b.WriteString("\n\n")

	// Force delete option (cursor index -1)
	forceCursor := "  "
	if m.cleanupState.cursorIndex == -1 {
		forceCursor = "> "
	}
	forceCheckbox := "[ ]"
	// Auto-check if submodules are selected
	forceEnabled := m.cleanupState.forceDelete || hasSelectedSubmodules
	if forceEnabled {
		forceCheckbox = "[✓]"
	}
	var forceStyle lipgloss.Style
	if m.cleanupState.cursorIndex == -1 {
		forceStyle = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
	} else {
		forceStyle = lipgloss.NewStyle().Foreground(ColorWarning)
	}
	b.WriteString(forceCursor)
	b.WriteString(forceStyle.Render(forceCheckbox))
	b.WriteString(" ")
	forceLabel := "Force delete (ignore uncommitted changes)"
	if hasSelectedSubmodules {
		forceLabel = "Force delete (required for submodules)"
	}
	b.WriteString(forceStyle.Render(forceLabel))
	b.WriteString("\n\n")

	// Interactive list with checkboxes
	for i, wt := range m.cleanupState.staleWorktrees {
		// Cursor indicator
		cursor := "  "
		if i == m.cleanupState.cursorIndex {
			cursor = "> "
		}

		// Checkbox
		checkbox := "[ ]"
		if m.cleanupState.selectedIndices[i] {
			checkbox = "[✓]"
		}

		// Highlight current line
		var lineStyle lipgloss.Style
		if i == m.cleanupState.cursorIndex {
			lineStyle = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
		} else {
			lineStyle = lipgloss.NewStyle().Foreground(ColorTextPrimary)
		}

		// Build reason text with submodule indicator
		reason := wt.StaleReason
		if wt.PRNumber > 0 {
			reason = fmt.Sprintf("%s (PR #%d %s)", reason, wt.PRNumber, wt.PRState)
		}
		if wt.HasSubmodules {
			reason += " 📦"
		}

		// Render line
		b.WriteString(cursor)
		b.WriteString(lineStyle.Render(checkbox))
		b.WriteString(" ")
		b.WriteString(lineStyle.Render(wt.Branch))
		b.WriteString(" ")
		b.WriteString(lipgloss.NewStyle().Foreground(ColorTextMuted).Render("[" + reason + "]"))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Help text
	promptStyle := lipgloss.NewStyle().Foreground(ColorTextSecondary)
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorSuccess)
	cancelStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorError)

	help := promptStyle.Render("↑/↓ or j/k: navigate • ") +
		keyStyle.Render("space") +
		promptStyle.Render(": toggle • ") +
		keyStyle.Render("enter") +
		promptStyle.Render(": confirm • ") +
		cancelStyle.Render("esc") +
		promptStyle.Render(": cancel")
	b.WriteString(help)

	// Submodule legend if any worktree has submodules
	hasAnySubmodules := false
	for _, wt := range m.cleanupState.staleWorktrees {
		if wt.HasSubmodules {
			hasAnySubmodules = true
			break
		}
	}
	if hasAnySubmodules {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(ColorTextMuted).Render("📦 = has submodules (requires force delete)"))
	}

	return b.String()
}

// renderCleanupProgress renders live progress during cleanup
func (m Model) renderCleanupProgress() string {
	if m.cleanupState == nil {
		return ""
	}

	var b strings.Builder

	// Header
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorWarning)
	b.WriteString(titleStyle.Render("⚠ Cleaning Up Stale Worktrees"))
	b.WriteString("\n\n")

	// Progress counter - only count SELECTED worktrees, not all stale ones
	progressStyle := lipgloss.NewStyle().Foreground(ColorTextSecondary)
	total := len(m.cleanupState.selectedIndices)
	completed := m.cleanupState.totalCleaned + m.cleanupState.totalFailed
	b.WriteString(progressStyle.Render(fmt.Sprintf("Progress: %d/%d", completed, total)))
	b.WriteString("\n\n")

	// List only SELECTED worktrees with their current status
	for i, wt := range m.cleanupState.staleWorktrees {
		// Skip worktrees that were not selected for deletion
		if !m.cleanupState.selectedIndices[i] {
			continue
		}

		// Check status of this worktree
		if m.cleanupState.deletedIndices[i] {
			// Successfully deleted - skip (removed from view)
			continue
		}

		if errMsg, failed := m.cleanupState.failedWorktrees[i]; failed {
			// Failed deletion - show with red ✗ and error
			b.WriteString("  ")
			b.WriteString(lipgloss.NewStyle().Foreground(ColorError).Render("✗"))
			b.WriteString(" ")
			b.WriteString(lipgloss.NewStyle().Foreground(ColorTextPrimary).Render(wt.Branch))
			b.WriteString(" ")
			b.WriteString(lipgloss.NewStyle().Foreground(ColorError).Render("[" + errMsg + "]"))
			b.WriteString("\n")
		} else if i == m.cleanupState.currentIndex {
			// Currently deleting - show spinner with dimmed text
			spinner := m.cleanupState.cleanupSpinner.View()
			b.WriteString("  ")
			b.WriteString(spinner)
			b.WriteString(" ")
			b.WriteString(lipgloss.NewStyle().Foreground(ColorTextMuted).Render(wt.Branch))
			b.WriteString("\n")
		} else {
			// Pending deletion - show normal
			b.WriteString("  • ")
			b.WriteString(lipgloss.NewStyle().Foreground(ColorTextSecondary).Render(wt.Branch))
			b.WriteString("\n")
		}
	}

	return b.String()
}

// renderCleanupFailureSummary renders summary after cleanup with failures
func (m Model) renderCleanupFailureSummary() string {
	if m.cleanupState == nil {
		return ""
	}

	var b strings.Builder

	// Header - show success or failure
	titleStyle := lipgloss.NewStyle().Bold(true)
	if m.cleanupState.totalCleaned > 0 {
		// Partial success
		b.WriteString(titleStyle.Foreground(ColorWarning).Render("⚠ Cleanup Partially Complete"))
	} else {
		// Total failure
		b.WriteString(titleStyle.Foreground(ColorError).Render("✗ Cleanup Failed"))
	}
	b.WriteString("\n\n")

	// Summary text
	summaryStyle := lipgloss.NewStyle().Foreground(ColorTextSecondary)
	if m.cleanupState.totalCleaned > 0 {
		b.WriteString(summaryStyle.Render(fmt.Sprintf("Deleted %d worktree(s), %d failed:",
			m.cleanupState.totalCleaned, m.cleanupState.totalFailed)))
	} else {
		b.WriteString(summaryStyle.Render(fmt.Sprintf("Failed to delete %d worktree(s):",
			m.cleanupState.totalFailed)))
	}
	b.WriteString("\n\n")

	// List only failed worktrees
	for idx, errMsg := range m.cleanupState.failedWorktrees {
		wt := m.cleanupState.staleWorktrees[idx]
		b.WriteString("  ")
		b.WriteString(lipgloss.NewStyle().Foreground(ColorError).Render("✗"))
		b.WriteString(" ")
		b.WriteString(lipgloss.NewStyle().Foreground(ColorTextPrimary).Render(wt.Branch))
		b.WriteString(" ")
		b.WriteString(lipgloss.NewStyle().Foreground(ColorError).Render("[" + errMsg + "]"))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Help text
	promptStyle := lipgloss.NewStyle().Foreground(ColorTextSecondary)
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorSuccess)
	b.WriteString(promptStyle.Render("Press "))
	b.WriteString(keyStyle.Render("enter"))
	b.WriteString(promptStyle.Render(" or "))
	b.WriteString(keyStyle.Render("esc"))
	b.WriteString(promptStyle.Render(" to close"))

	return b.String()
}

// handleCleanupKeys handles key presses in cleanup views
func (m Model) handleCleanupKeys(msg tea.KeyMsg) (Model, tea.Cmd) {
	if m.cleanupState == nil {
		m.currentView = DashboardView
		return m, nil
	}

	// Block all input during cleanup (user must wait for completion)
	if m.cleanupState.inProgress {
		return m, nil
	}

	// If showing failure summary, allow enter/esc to close
	if !m.cleanupState.confirmed {
		// Selection mode - handle navigation and toggle
		switch msg.String() {
		case "up", "k":
			// Move cursor up (can go to -1 for force delete option)
			if m.cleanupState.cursorIndex > -1 {
				m.cleanupState.cursorIndex--
			}
			return m, nil

		case "down", "j":
			// Move cursor down
			if m.cleanupState.cursorIndex < len(m.cleanupState.staleWorktrees)-1 {
				m.cleanupState.cursorIndex++
			}
			return m, nil

		case " ":
			// Toggle selection at current cursor
			if m.cleanupState.cursorIndex == -1 {
				// Toggle force delete option
				m.cleanupState.forceDelete = !m.cleanupState.forceDelete
				logging.Info("Cleanup: force delete toggled to %v", m.cleanupState.forceDelete)
			} else {
				// Toggle worktree selection
				idx := m.cleanupState.cursorIndex
				if m.cleanupState.selectedIndices[idx] {
					delete(m.cleanupState.selectedIndices, idx)
				} else {
					m.cleanupState.selectedIndices[idx] = true
				}
			}
			return m, nil

		case "enter":
			// Confirm and start cleanup (only if at least one is selected)
			if len(m.cleanupState.selectedIndices) == 0 {
				logging.Info("Cleanup: no worktrees selected, ignoring enter")
				return m, nil
			}

			// Auto-enable force delete if any selected worktree has submodules
			for i, wt := range m.cleanupState.staleWorktrees {
				if m.cleanupState.selectedIndices[i] && wt.HasSubmodules {
					m.cleanupState.forceDelete = true
					logging.Info("Cleanup: auto-enabled force delete due to submodules")
					break
				}
			}

			logging.Info("Cleanup: user confirmed deletion of %d worktrees (force=%v)",
				len(m.cleanupState.selectedIndices), m.cleanupState.forceDelete)
			m.cleanupState.confirmed = true
			return m, m.cleanupStaleWorktrees()

		case "esc":
			// Cancel
			logging.Info("Cleanup: user cancelled")
			m.cleanupState = nil
			m.currentView = DashboardView
			return m, nil
		}
	} else {
		// Failure summary state - allow closing with enter/esc
		switch msg.String() {
		case "enter", "esc":
			logging.Info("Cleanup: closing failure summary")
			m.cleanupState = nil
			m.currentView = DashboardView
			return m, nil
		}
	}

	return m, nil
}
