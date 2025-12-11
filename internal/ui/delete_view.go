package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// deleteView renders the delete worktree flow
// Note: Delete confirmation and completion are now shown as modal overlays on the dashboard
func (m Model) deleteView() string {
	if m.deleteState == nil {
		return "Loading..."
	}

	// All delete steps now use modal overlays on dashboard
	return m.dashboardView()
}

// renderDeleteConfirmModal renders the delete confirmation as a modal
func (m Model) renderDeleteConfirmModal() string {
	if m.deleteState == nil {
		return ""
	}

	var b strings.Builder

	// Header with warning emoji (consistent with cleanup modal)
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorWarning)
	b.WriteString(titleStyle.Render("⚠ Delete Worktree"))
	b.WriteString("\n\n")

	// Get the worktree to delete
	var wt *Worktree
	if m.deleteState.targetWorktree != nil {
		wt = m.deleteState.targetWorktree
	} else if len(m.deleteState.selectedWorktrees) > 0 && m.deleteState.selectedWorktrees[0] < len(m.worktrees) {
		w := m.worktrees[m.deleteState.selectedWorktrees[0]]
		wt = &w
	}

	if wt == nil {
		b.WriteString(lipgloss.NewStyle().Foreground(ColorTextSecondary).Render("No worktree selected"))
		return b.String()
	}

	// Description
	b.WriteString(lipgloss.NewStyle().Foreground(ColorTextSecondary).Render("The following worktree will be deleted:"))
	b.WriteString("\n\n")

	// Worktree info (styled like cleanup list item)
	b.WriteString("  • ")
	b.WriteString(lipgloss.NewStyle().Foreground(ColorTextPrimary).Bold(true).Render(wt.Branch))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(ColorTextMuted).Render("    " + shortenPath(wt.Path, 50)))
	b.WriteString("\n")

	// Status warnings
	hasWarning := false
	var warnings []string
	if wt.StagedCount > 0 || wt.ModifiedCount > 0 {
		warnings = append(warnings, "uncommitted changes")
		hasWarning = true
	}
	if wt.UntrackedCount > 0 {
		warnings = append(warnings, "untracked files")
		hasWarning = true
	}
	if wt.UnpushedCount > 0 {
		warnings = append(warnings, "unpushed commits")
		hasWarning = true
	}

	if hasWarning {
		b.WriteString("\n")
		warningStyle := lipgloss.NewStyle().Foreground(ColorError).Bold(true)
		b.WriteString(warningStyle.Render("⚠ Has " + strings.Join(warnings, ", ")))
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(ColorTextSecondary).Render("  Changes will be permanently lost!"))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Confirmation prompt (consistent with cleanup)
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

	return b.String()
}

// renderDeleteDeletingModal renders the deletion in progress message as a modal
func (m Model) renderDeleteDeletingModal() string {
	if m.deleteState == nil {
		return ""
	}

	var b strings.Builder

	// Header with warning color (consistent)
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorWarning)
	b.WriteString(titleStyle.Render("⚠ Delete Worktree"))
	b.WriteString("\n\n")

	// Get the worktree being deleted
	var wt *Worktree
	if m.deleteState.targetWorktree != nil {
		wt = m.deleteState.targetWorktree
	} else if len(m.deleteState.selectedWorktrees) > 0 && m.deleteState.selectedWorktrees[0] < len(m.worktrees) {
		w := m.worktrees[m.deleteState.selectedWorktrees[0]]
		wt = &w
	}

	// Spinner with "Deleting..." message
	spinnerText := m.deleteSpinner.View() + " Deleting..."
	b.WriteString(lipgloss.NewStyle().Foreground(ColorTextSecondary).Render(spinnerText))
	b.WriteString("\n\n")

	// Show which worktree is being deleted
	if wt != nil {
		b.WriteString("  • ")
		b.WriteString(lipgloss.NewStyle().Foreground(ColorTextPrimary).Bold(true).Render(wt.Branch))
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(ColorTextMuted).Render("    " + shortenPath(wt.Path, 50)))
	}

	return b.String()
}

// renderDeleteCompleteModal renders the deletion complete message as a modal
func (m Model) renderDeleteCompleteModal() string {
	if m.deleteState == nil {
		return ""
	}

	var b strings.Builder

	// Title with success color
	titleStyle := lipgloss.NewStyle().
		Foreground(ColorSuccess).
		Bold(true)

	b.WriteString(titleStyle.Render("✓ Worktree Deleted"))
	b.WriteString("\n\n")

	// Description
	b.WriteString(lipgloss.NewStyle().Foreground(ColorTextSecondary).Render("Successfully deleted:"))
	b.WriteString("\n\n")

	// Show deleted worktree
	if m.deleteState.targetWorktree != nil {
		b.WriteString("  • ")
		b.WriteString(lipgloss.NewStyle().Foreground(ColorTextPrimary).Bold(true).Render(m.deleteState.targetWorktree.Branch))
		b.WriteString("\n\n")
	}

	// Help
	promptStyle := lipgloss.NewStyle().Foreground(ColorTextSecondary)
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorSuccess)

	b.WriteString(promptStyle.Render("Press "))
	b.WriteString(keyStyle.Render("enter"))
	b.WriteString(promptStyle.Render(" to continue"))

	return b.String()
}
