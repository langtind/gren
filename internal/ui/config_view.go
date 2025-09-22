package ui

import (
	"fmt"
	"strings"
)

// configView renders the config file selector
func (m Model) configView() string {
	if m.configState == nil {
		return "Loading..."
	}

	var content strings.Builder

	content.WriteString(TitleStyle.Render("🔧 Configuration Files"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("Select file to edit:"))
	content.WriteString("\n\n")

	if len(m.configState.files) == 0 {
		content.WriteString(WorktreePathStyle.Render("No configuration files found."))
		content.WriteString("\n")
		content.WriteString(WorktreePathStyle.Render("Initialize gren first to create config files."))
		content.WriteString("\n\n")
		content.WriteString(HelpStyle.Render("[esc] Back"))
		return HeaderStyle.Width(m.width - 4).Render(content.String())
	}

	// File list with simple styling
	for i, file := range m.configState.files {
		prefix := "  "
		if i == m.configState.selectedIndex {
			prefix = "▶ "
		}

		fileLine := fmt.Sprintf("%s%s %s", prefix, file.Icon, file.Name)

		// Apply color styling for selected item
		if i == m.configState.selectedIndex {
			content.WriteString(WorktreeNameStyle.Foreground(PrimaryColor).Render(fileLine))
		} else {
			content.WriteString(fileLine)
		}
		content.WriteString("\n")

		// Show description for selected file
		if i == m.configState.selectedIndex {
			content.WriteString(WorktreePathStyle.Render(fmt.Sprintf("   %s", file.Description)))
			content.WriteString("\n")
		}
	}

	content.WriteString("\n")
	content.WriteString(HelpStyle.Render("↑↓ Navigate • Enter Open • Esc Back"))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}