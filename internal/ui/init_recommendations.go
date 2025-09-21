package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderRecommendationsStep shows detected files and recommendations
func (m Model) renderRecommendationsStep() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("💡 Recommendations"))
	content.WriteString("\n\n")

	if len(m.initState.detectedFiles) == 0 {
		content.WriteString(WorktreeNameStyle.Render("No configuration files detected"))
		content.WriteString("\n")
		content.WriteString(WorktreePathStyle.Render("gren will create a basic setup for you."))
		content.WriteString("\n\n")
	} else {
		content.WriteString(WorktreeNameStyle.Render("Detected configuration files:"))
		content.WriteString("\n\n")

		// Group files by type
		filesByType := make(map[string][]DetectedFile)
		for _, file := range m.initState.detectedFiles {
			filesByType[file.Type] = append(filesByType[file.Type], file)
		}

		// Show each type
		typeOrder := []string{"env", "config", "tool"}
		typeEmojis := map[string]string{
			"env":    "🔐",
			"config": "⚙️",
			"tool":   "🛠️",
		}
		typeNames := map[string]string{
			"env":    "Environment Files",
			"config": "Configuration Files",
			"tool":   "Tool Configuration",
		}

		for _, fileType := range typeOrder {
			files := filesByType[fileType]
			if len(files) == 0 {
				continue
			}

			emoji := typeEmojis[fileType]
			name := typeNames[fileType]

			content.WriteString(WorktreeNameStyle.Render(fmt.Sprintf("%s %s", emoji, name)))
			content.WriteString("\n")

			for _, file := range files {
				indicator := "📄"
				if file.IsGitIgnored {
					indicator = "🙈"
				}
				content.WriteString(WorktreePathStyle.Render(fmt.Sprintf("   %s %s - %s", indicator, file.Path, file.Description)))
				content.WriteString("\n")
			}
			content.WriteString("\n")
		}
	}

	// Show detected package manager and command
	content.WriteString(WorktreeNameStyle.Render("Detected setup:"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render(fmt.Sprintf("   🌿 Worktree location: %s", m.initState.worktreeDir)))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render(fmt.Sprintf("   📦 Package manager: %s", m.initState.packageManager)))
	content.WriteString("\n")
	if m.initState.postCreateCmd != "" {
		content.WriteString(WorktreePathStyle.Render(fmt.Sprintf("   ⚡ Setup command: %s", m.initState.postCreateCmd)))
		content.WriteString("\n")
	}
	content.WriteString("\n")

	// Menu options
	content.WriteString(WorktreeNameStyle.Render("What would you like to do?"))
	content.WriteString("\n\n")

	options := []string{
		"Accept recommendations and continue",
		"Customize configuration",
	}

	for i, option := range options {
		var style lipgloss.Style
		if i == m.initState.selected {
			style = WorktreeSelectedStyle
		} else {
			style = WorktreeItemStyle
		}

		prefix := "   "
		if i == 0 {
			prefix = "✅ "
		} else {
			prefix = "⚙️ "
		}

		content.WriteString(style.Width(m.width-8).Render(fmt.Sprintf("%s%s", prefix, option)))
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(HelpStyle.Render("[enter] Select  [↑↓] Navigate  [esc] Back"))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}