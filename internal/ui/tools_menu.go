package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
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

// getToolActions returns the list of available tool actions
func getToolActions(hasPR bool) []ToolAction {
	actions := []ToolAction{
		{Key: "r", Name: "Refresh status", Description: "Re-check stale status (git + GitHub)"},
		{Key: "c", Name: "Cleanup stale worktrees", Description: "Delete all stale worktrees"},
		{Key: "x", Name: "Prune missing worktrees", Description: "Remove references to deleted worktree directories"},
	}

	// Add GitHub section if selected worktree has a PR
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

	// Check if selected worktree has a PR
	hasPR := false
	if m.selected >= 0 && m.selected < len(m.worktrees) {
		hasPR = m.worktrees[m.selected].PRNumber > 0
	}

	// Actions
	actions := getToolActions(hasPR)
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
		m.cleanupState = &CleanupState{
			staleWorktrees: staleWorktrees,
			confirmed:      false,
		}
		m.currentView = CleanupView
		return m, nil

	case "x":
		// Prune missing worktrees
		logging.Info("Tools menu: running prune")
		m.currentView = DashboardView
		return m, m.pruneWorktrees()

	case "p":
		// Open PR in browser
		if m.selected >= 0 && m.selected < len(m.worktrees) {
			wt := m.worktrees[m.selected]
			if wt.PRNumber > 0 {
				logging.Info("Tools menu: opening PR #%d for %s", wt.PRNumber, wt.Branch)
				m.currentView = DashboardView
				return m, m.openPRInBrowser(wt.Branch)
			}
		}
		return m, nil
	}

	return m, nil
}

// renderCleanupConfirmation renders the cleanup confirmation dialog
func (m Model) renderCleanupConfirmation() string {
	if m.cleanupState == nil {
		return ""
	}

	// Always use fresh data from m.worktrees (may have been enriched since modal opened)
	var staleWorktrees []Worktree
	for _, wt := range m.worktrees {
		if wt.BranchStatus == "stale" && !wt.IsCurrent && !wt.IsMain {
			staleWorktrees = append(staleWorktrees, wt)
		}
	}

	if len(staleWorktrees) == 0 {
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

	// List worktrees to be deleted
	b.WriteString(lipgloss.NewStyle().Foreground(ColorTextSecondary).Render("The following worktrees will be deleted:"))
	b.WriteString("\n\n")

	for _, wt := range staleWorktrees {
		reason := wt.StaleReason
		if wt.PRNumber > 0 {
			prInfo := fmt.Sprintf(" (PR #%d %s)", wt.PRNumber, wt.PRState)
			reason = reason + lipgloss.NewStyle().Foreground(ColorSecondary).Render(prInfo)
		}
		b.WriteString("  • ")
		b.WriteString(lipgloss.NewStyle().Foreground(ColorTextPrimary).Render(wt.Branch))
		b.WriteString(" ")
		b.WriteString(lipgloss.NewStyle().Foreground(ColorTextMuted).Render("[" + reason + "]"))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Confirmation prompt (single line)
	promptStyle := lipgloss.NewStyle().Foreground(ColorTextSecondary)
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorSuccess)
	cancelStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorError)
	prompt := promptStyle.Render("Press ") +
		keyStyle.Render("y") +
		promptStyle.Render(" to confirm, ") +
		cancelStyle.Render("n") +
		promptStyle.Render(" or ") +
		cancelStyle.Render("esc") +
		promptStyle.Render(" to cancel")
	b.WriteString(prompt)

	// Return plain content - renderWithModalWidth handles the box styling
	return b.String()
}

// handleCleanupKeys handles key presses in cleanup confirmation view
func (m Model) handleCleanupKeys(msg tea.KeyMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		// User confirmed - execute cleanup
		logging.Info("Cleanup: user confirmed deletion of %d worktrees", len(m.cleanupState.staleWorktrees))
		m.currentView = DashboardView
		return m, m.cleanupStaleWorktrees()

	case "n", "N", "esc":
		// User cancelled
		logging.Info("Cleanup: user cancelled")
		m.cleanupState = nil
		m.currentView = DashboardView
		return m, nil
	}

	return m, nil
}
