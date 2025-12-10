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

	// Title with warning color
	titleStyle := lipgloss.NewStyle().
		Foreground(ColorError).
		Bold(true)

	b.WriteString(titleStyle.Render("Delete Worktree"))
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
		b.WriteString(WizardDescStyle.Render("No worktree selected"))
		return b.String()
	}

	// Worktree info
	b.WriteString(WizardSubtitleStyle.Render(wt.Name))
	b.WriteString("\n")
	b.WriteString(WizardDescStyle.Render(shortenPath(wt.Path, 40)))
	b.WriteString("\n")
	b.WriteString(WorktreeBranchStyle.Render(wt.Branch))
	b.WriteString("\n\n")

	// Status warnings
	hasWarning := false
	if wt.StagedCount > 0 || wt.ModifiedCount > 0 {
		b.WriteString(ErrorStyle.Render("⚠ Has uncommitted changes"))
		b.WriteString("\n")
		hasWarning = true
	}
	if wt.UntrackedCount > 0 {
		b.WriteString(ErrorStyle.Render("⚠ Has untracked files"))
		b.WriteString("\n")
		hasWarning = true
	}
	if wt.UnpushedCount > 0 {
		b.WriteString(ErrorStyle.Render("⚠ Has unpushed commits"))
		b.WriteString("\n")
		hasWarning = true
	}

	if hasWarning {
		b.WriteString("\n")
		b.WriteString(WizardDescStyle.Render("Changes will be permanently lost!"))
		b.WriteString("\n\n")
	}

	// Confirmation prompt
	b.WriteString(WizardDescStyle.Render("This action cannot be undone."))
	b.WriteString("\n\n")

	// Help
	b.WriteString(HelpKeyStyle.Render("y"))
	b.WriteString(HelpTextStyle.Render(" delete"))
	b.WriteString(HelpSeparatorStyle.Render(" • "))
	b.WriteString(HelpKeyStyle.Render("n/esc"))
	b.WriteString(HelpTextStyle.Render(" cancel"))

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

	b.WriteString(titleStyle.Render("Worktree Deleted"))
	b.WriteString("\n\n")

	b.WriteString(WizardSuccessStyle.Render("✓ Successfully deleted"))
	b.WriteString("\n\n")

	if m.deleteState.targetWorktree != nil {
		b.WriteString(WizardDescStyle.Render(m.deleteState.targetWorktree.Name))
		b.WriteString("\n\n")
	}

	// Help
	b.WriteString(HelpKeyStyle.Render("enter"))
	b.WriteString(HelpTextStyle.Render(" continue"))
	b.WriteString(HelpSeparatorStyle.Render(" • "))
	b.WriteString(HelpKeyStyle.Render("q"))
	b.WriteString(HelpTextStyle.Render(" quit"))

	return b.String()
}
