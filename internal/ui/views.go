package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// The createView has been moved to create_view.go for better organization

// The deleteView has been moved to delete_view.go for better organization

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