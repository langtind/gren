package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// The createView has been moved to create_view.go for better organization

// deleteView renders the delete worktree view
func (m Model) deleteView() string {
	title := TitleStyle.Render("🗑️ Delete Worktree")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		"Select worktrees to delete: [coming soon]",
		"",
		HelpStyle.Render("[esc] Back to dashboard"),
	)

	return HeaderStyle.Width(m.width - 4).Render(content)
}

// settingsView renders the settings view
func (m Model) settingsView() string {
	title := TitleStyle.Render("⚙️ Settings")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		"Configuration: [coming soon]",
		"",
		HelpStyle.Render("[esc] Back to dashboard"),
	)

	return HeaderStyle.Width(m.width - 4).Render(content)
}