package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// createView renders the create worktree view
func (m Model) createView() string {
	title := TitleStyle.Render("🌱 Create New Worktree")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		"Branch name: [coming soon]",
		"",
		"Base branch: [coming soon]",
		"",
		"📋 Post-create setup:",
		"✅ Copy .env files",
		"✅ Install dependencies (bun)",
		"✅ Run custom hook",
		"",
		HelpStyle.Render("[esc] Back to dashboard"),
	)

	return HeaderStyle.Width(m.width - 4).Render(content)
}

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