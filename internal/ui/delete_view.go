package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// deleteView renders the delete worktree wizard
func (m Model) deleteView() string {
	if m.deleteState == nil {
		return "Loading..."
	}

	switch m.deleteState.currentStep {
	case DeleteStepSelection:
		return m.renderDeleteSelectionStep()
	case DeleteStepConfirm:
		return m.renderDeleteConfirmStep()
	case DeleteStepDeleting:
		return m.renderDeletingStep()
	case DeleteStepComplete:
		return m.renderDeleteCompleteStep()
	default:
		return "Unknown step"
	}
}

// renderDeleteSelectionStep shows worktree selection
func (m Model) renderDeleteSelectionStep() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("🗑️ Delete Worktrees"))
	content.WriteString("\n\n")

	// Filter out current worktree
	var deletableWorktrees []Worktree
	for _, wt := range m.worktrees {
		if !wt.IsCurrent {
			deletableWorktrees = append(deletableWorktrees, wt)
		}
	}

	if len(deletableWorktrees) == 0 {
		content.WriteString(WorktreeNameStyle.Render("No worktrees available for deletion."))
		content.WriteString("\n")
		content.WriteString(WorktreePathStyle.Render("Create additional worktrees first."))
		content.WriteString("\n\n")
		content.WriteString(HelpStyle.Render("[esc] Back to dashboard"))
		return HeaderStyle.Width(m.width - 4).Render(content.String())
	}

	content.WriteString(WorktreeNameStyle.Render("Select worktrees to delete (use Space to select multiple):"))
	content.WriteString("\n\n")

	// Show warnings if any
	if len(m.deleteState.warnings) > 0 {
		for _, warning := range m.deleteState.warnings {
			content.WriteString(ErrorStyle.Render(warning))
			content.WriteString("\n")
		}
		content.WriteString("\n")
	}

	// List deletable worktrees
	for i, wt := range deletableWorktrees {
		var style lipgloss.Style
		if i == m.selected {
			style = WorktreeSelectedStyle
		} else {
			style = WorktreeItemStyle
		}

		// Check if this worktree is selected for deletion
		isSelected := false
		for _, deletableWt := range deletableWorktrees {
			for _, selectedIdx := range m.deleteState.selectedWorktrees {
				if selectedIdx < len(m.worktrees) && m.worktrees[selectedIdx].Name == deletableWt.Name && deletableWt.Name == wt.Name {
					isSelected = true
					break
				}
			}
			if isSelected {
				break
			}
		}

		checkbox := "☐"
		if isSelected {
			checkbox = "☑️"
		}

		// Status indicator
		var statusText string
		switch wt.Status {
		case "clean":
			statusText = "🟢 Clean"
		case "modified":
			statusText = "🟡 Modified"
		case "untracked":
			statusText = "🔴 Untracked files"
		case "mixed":
			statusText = "📝 Changes"
		case "unpushed":
			statusText = "⬆️ Unpushed"
		default:
			statusText = "🟢 Clean"
		}

		worktreeInfo := fmt.Sprintf("%s %s %s (%s)", checkbox, wt.Name, statusText, wt.Branch)
		content.WriteString(style.Width(m.width - 8).Render(worktreeInfo))
		content.WriteString("\n")
		content.WriteString(WorktreePathStyle.Render(fmt.Sprintf("   📍 %s", wt.Path)))
		content.WriteString("\n\n")
	}

	selectedCount := len(m.deleteState.selectedWorktrees)
	if selectedCount > 0 {
		content.WriteString(WorktreeNameStyle.Render(fmt.Sprintf("Selected: %d worktree(s)", selectedCount)))
		content.WriteString("\n\n")
		content.WriteString(HelpStyle.Render("[space] Toggle  [enter] Continue  [↑↓] Navigate  [esc] Cancel"))
	} else {
		content.WriteString(HelpStyle.Render("[space] Toggle selection  [↑↓] Navigate  [esc] Cancel"))
	}

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}

// renderDeleteConfirmStep shows final confirmation
func (m Model) renderDeleteConfirmStep() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("⚠️ Confirm Deletion"))
	content.WriteString("\n\n")

	content.WriteString(ErrorStyle.Render("WARNING: This action cannot be undone!"))
	content.WriteString("\n\n")

	// Handle single worktree deletion
	if m.deleteState.targetWorktree != nil {
		wt := m.deleteState.targetWorktree
		content.WriteString(WorktreeNameStyle.Render(fmt.Sprintf("Delete worktree '%s'?", wt.Name)))
		content.WriteString("\n\n")

		content.WriteString(WorktreeItemStyle.Render(fmt.Sprintf("🗑️ %s", wt.Name)))
		content.WriteString("\n")
		content.WriteString(WorktreePathStyle.Render(fmt.Sprintf("   📍 %s", wt.Path)))
		content.WriteString("\n")
		content.WriteString(WorktreeBranchStyle.Render(fmt.Sprintf("   🌿 Branch: %s", wt.Branch)))

		if wt.Status != "clean" {
			var statusWarning string
			switch wt.Status {
			case "modified":
				statusWarning = "   ⚠️ Has uncommitted changes - will be lost!"
			case "untracked":
				statusWarning = "   ⚠️ Has untracked files - will be lost!"
			case "mixed":
				statusWarning = "   ⚠️ Has uncommitted and untracked changes - will be lost!"
			case "unpushed":
				statusWarning = "   ⚠️ Has unpushed commits - branch not on remote!"
			}
			content.WriteString("\n")
			content.WriteString(ErrorStyle.Render(statusWarning))
		}
		content.WriteString("\n\n")
	} else {
		// Handle multi-select deletion
		content.WriteString(WorktreeNameStyle.Render("The following worktrees will be permanently deleted:"))
		content.WriteString("\n\n")

		for _, selectedIdx := range m.deleteState.selectedWorktrees {
			if selectedIdx < len(m.worktrees) {
				wt := m.worktrees[selectedIdx]

				content.WriteString(WorktreeItemStyle.Render(fmt.Sprintf("🗑️ %s", wt.Name)))
				content.WriteString("\n")
				content.WriteString(WorktreePathStyle.Render(fmt.Sprintf("   📍 %s", wt.Path)))
				content.WriteString("\n")
				content.WriteString(WorktreeBranchStyle.Render(fmt.Sprintf("   🌿 Branch: %s", wt.Branch)))

				if wt.Status != "clean" {
					var statusWarning string
					switch wt.Status {
					case "modified":
						statusWarning = "   ⚠️ Has uncommitted changes - will be lost!"
					case "untracked":
						statusWarning = "   ⚠️ Has untracked files - will be lost!"
					case "mixed":
						statusWarning = "   ⚠️ Has uncommitted and untracked changes - will be lost!"
					case "unpushed":
						statusWarning = "   ⚠️ Has unpushed commits - branch not on remote!"
					}
					content.WriteString("\n")
					content.WriteString(ErrorStyle.Render(statusWarning))
				}
				content.WriteString("\n\n")
			}
		}
	}

	content.WriteString(HelpStyle.Render("[y] Delete worktrees  [n] Cancel  [esc] Back"))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}

// renderDeletingStep shows deletion progress
func (m Model) renderDeletingStep() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("🔄 Deleting Worktrees"))
	content.WriteString("\n\n")

	content.WriteString(SpinnerStyle.Render("⠋ Removing worktrees..."))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("⏳ Deleting git branches..."))
	content.WriteString("\n\n")

	content.WriteString(WorktreePathStyle.Render("Please wait while worktrees are being deleted..."))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}

// renderDeleteCompleteStep shows completion
func (m Model) renderDeleteCompleteStep() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("✅ Deletion Complete"))
	content.WriteString("\n\n")

	// Handle single worktree deletion
	if m.deleteState.targetWorktree != nil {
		content.WriteString(StatusCleanStyle.Render("Successfully deleted 1 worktree"))
		content.WriteString("\n\n")

		content.WriteString(WorktreeNameStyle.Render("Deleted worktree:"))
		content.WriteString("\n")
		content.WriteString(WorktreePathStyle.Render(fmt.Sprintf("✅ %s", m.deleteState.targetWorktree.Name)))
		content.WriteString("\n")
	} else {
		// Handle multi-select deletion
		deletedCount := len(m.deleteState.selectedWorktrees)
		content.WriteString(StatusCleanStyle.Render(fmt.Sprintf("Successfully deleted %d worktree(s)", deletedCount)))
		content.WriteString("\n\n")

		content.WriteString(WorktreeNameStyle.Render("Deleted worktrees:"))
		content.WriteString("\n")

		for _, selectedIdx := range m.deleteState.selectedWorktrees {
			if selectedIdx < len(m.worktrees) {
				wt := m.worktrees[selectedIdx]
				content.WriteString(WorktreePathStyle.Render(fmt.Sprintf("✅ %s", wt.Name)))
				content.WriteString("\n")
			}
		}
	}

	content.WriteString("\n")
	content.WriteString(HelpStyle.Render("[enter] Return to dashboard  [q] Quit"))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}
