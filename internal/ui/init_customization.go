package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderCustomizationStep shows the customization interface
func (m Model) renderCustomizationStep() string {
	if m.initState.customizationMode == "" {
		return m.renderCustomizationMenu()
	}

	switch m.initState.customizationMode {
	case "worktree":
		return m.renderWorktreeCustomization()
	case "patterns":
		return m.renderPatternsCustomization()
	case "postcreate":
		return m.renderPostCreateCustomization()
	default:
		return m.renderCustomizationMenu()
	}
}

// renderCustomizationMenu shows the main customization menu
func (m Model) renderCustomizationMenu() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("⚙️ Customize Configuration"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("Choose what to customize:"))
	content.WriteString("\n\n")

	options := []struct {
		name        string
		icon        string
		description string
	}{
		{"Worktree Location", "📂", fmt.Sprintf("Currently: %s", m.initState.worktreeDir)},
		{"File Patterns", "📋", fmt.Sprintf("%d patterns configured", len(m.initState.copyPatterns))},
		{"Post-Create Command", "⚡", fmt.Sprintf("Currently: %s", m.initState.postCreateCmd)},
	}

	for i, option := range options {
		var style lipgloss.Style
		if i == m.initState.selected {
			style = WorktreeSelectedStyle
		} else {
			style = WorktreeItemStyle
		}

		optionText := fmt.Sprintf("%s %s", option.icon, option.name)
		content.WriteString(style.Width(m.width-8).Render(optionText))
		content.WriteString("\n")
		content.WriteString(WorktreePathStyle.Render(fmt.Sprintf("   %s", option.description)))
		content.WriteString("\n\n")
	}

	content.WriteString(HelpStyle.Render("[enter] Customize  [↑↓] Navigate  [esc] Back to recommendations"))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}

// renderWorktreeCustomization shows worktree directory customization
func (m Model) renderWorktreeCustomization() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("📂 Worktree Location"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("Configure where worktrees will be created:"))
	content.WriteString("\n\n")

	// Input field
	inputStyle := WorktreeSelectedStyle
	displayText := m.initState.editingText
	if displayText == "" {
		displayText = m.initState.worktreeDir
	}

	content.WriteString(inputStyle.Width(m.width-8).Render(fmt.Sprintf("📁 %s▮", displayText)))
	content.WriteString("\n\n")

	// Help text
	content.WriteString(WorktreePathStyle.Render("Examples:"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   ../worktrees       (relative to current repo)"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   ~/dev/worktrees    (absolute path)"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   ./branches         (inside current repo)"))
	content.WriteString("\n\n")

	content.WriteString(HelpStyle.Render("[type] Edit path  [enter] Save  [esc] Back"))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}

// renderPatternsCustomization shows file pattern customization
func (m Model) renderPatternsCustomization() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("📋 File Patterns"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("Select which files to copy to new worktrees:"))
	content.WriteString("\n\n")

	if len(m.initState.copyPatterns) == 0 {
		content.WriteString(WorktreePathStyle.Render("No patterns configured yet."))
		content.WriteString("\n\n")
	} else {
		for i, pattern := range m.initState.copyPatterns {
			var style lipgloss.Style
			if i == m.initState.selected {
				style = WorktreeSelectedStyle
			} else {
				style = WorktreeItemStyle
			}

			// Status indicator
			status := "☐"
			if pattern.Enabled {
				status = "☑️"
			}

			// Detection indicator
			detection := ""
			if pattern.Detected {
				detection = " (auto-detected)"
			}

			patternText := fmt.Sprintf("%s %s%s", status, pattern.Pattern, detection)
			content.WriteString(style.Width(m.width-8).Render(patternText))
			content.WriteString("\n")
			content.WriteString(WorktreePathStyle.Render(fmt.Sprintf("   %s", pattern.Description)))
			content.WriteString("\n\n")
		}
	}

	content.WriteString(HelpStyle.Render("[space] Toggle  [↑↓] Navigate  [esc] Back"))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}

// renderPostCreateCustomization shows post-create command customization
func (m Model) renderPostCreateCustomization() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("⚡ Post-Create Command"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("Command to run after creating each worktree:"))
	content.WriteString("\n\n")

	// Input field
	inputStyle := WorktreeSelectedStyle
	displayText := m.initState.editingText
	if displayText == "" {
		displayText = m.initState.postCreateCmd
	}

	content.WriteString(inputStyle.Width(m.width-8).Render(fmt.Sprintf("⚡ %s▮", displayText)))
	content.WriteString("\n\n")

	// Help text
	content.WriteString(WorktreePathStyle.Render("Examples:"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   npm install           (install Node.js dependencies)"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   go mod download       (download Go modules)"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   pip install -r requirements.txt  (Python dependencies)"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   make setup            (custom setup script)"))
	content.WriteString("\n\n")

	content.WriteString(WorktreePathStyle.Render("Leave empty to create a custom script instead."))
	content.WriteString("\n\n")

	content.WriteString(HelpStyle.Render("[type] Edit command  [enter] Save  [esc] Back"))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}