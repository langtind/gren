package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderPreviewStep shows configuration preview before creation
func (m Model) renderPreviewStep() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("👀 Configuration Preview"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("Review your configuration:"))
	content.WriteString("\n\n")

	// Worktree configuration
	content.WriteString(WorktreeNameStyle.Render("📂 Worktree Settings"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render(fmt.Sprintf("   • Location: %s", m.initState.worktreeDir)))
	content.WriteString("\n")
	if m.initState.postCreateCmd != "" {
		content.WriteString(WorktreePathStyle.Render(fmt.Sprintf("   • Setup command: %s", m.initState.postCreateCmd)))
		content.WriteString("\n")
	}
	content.WriteString("\n")

	// File patterns
	content.WriteString(WorktreeNameStyle.Render("📋 File Patterns"))
	content.WriteString("\n")
	if len(m.initState.copyPatterns) == 0 {
		content.WriteString(WorktreePathStyle.Render("   • No file patterns configured"))
	} else {
		enabledCount := 0
		for _, pattern := range m.initState.copyPatterns {
			if pattern.Enabled {
				enabledCount++
				content.WriteString(WorktreePathStyle.Render(fmt.Sprintf("   • %s (%s)", pattern.Pattern, pattern.Type)))
				content.WriteString("\n")
			}
		}
		if enabledCount == 0 {
			content.WriteString(WorktreePathStyle.Render("   • No patterns enabled"))
		}
	}
	content.WriteString("\n")

	// Generated files preview
	content.WriteString(WorktreeNameStyle.Render("📄 Files to be created"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   • .gren/config.yml        (main configuration)"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   • .gren/post-create.sh    (setup script)"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   • .gitignore              (updated if needed)"))
	content.WriteString("\n\n")

	// Options menu
	content.WriteString(WorktreeNameStyle.Render("Next steps:"))
	content.WriteString("\n\n")

	options := []string{
		"Create configuration files",
		"Back to customization",
		"Back to recommendations",
		"Cancel setup",
	}

	for i, option := range options {
		var style lipgloss.Style
		if i == m.initState.selected {
			style = WorktreeSelectedStyle
		} else {
			style = WorktreeItemStyle
		}

		prefix := "   "
		switch i {
		case 0:
			prefix = "✅ "
		case 1:
			prefix = "⚙️ "
		case 2:
			prefix = "💡 "
		case 3:
			prefix = "❌ "
		}

		content.WriteString(style.Width(m.width-8).Render(fmt.Sprintf("%s%s", prefix, option)))
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(HelpStyle.Render("[enter] Select  [↑↓] Navigate  [esc] Back"))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}

// renderExecutingStep shows files being created
func (m Model) renderExecutingStep() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("⚙️ Creating Configuration"))
	content.WriteString("\n\n")

	content.WriteString(SpinnerStyle.Render("⠋ Creating .gren directory..."))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("⏳ Writing configuration files..."))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("⏸️ Generating setup script..."))
	content.WriteString("\n\n")

	content.WriteString(WorktreePathStyle.Render("This will only take a moment..."))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}

// renderCreatedStep shows completion and options
func (m Model) renderCreatedStep() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("🎉 Configuration Created!"))
	content.WriteString("\n\n")

	content.WriteString(StatusCleanStyle.Render("✅ Configuration files have been created"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("Files created:"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   📄 .gren/config.yml"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   📄 .gren/post-create.sh"))
	content.WriteString("\n")
	if needsGitignoreUpdate() {
		content.WriteString(WorktreePathStyle.Render("   📄 .gitignore (updated)"))
		content.WriteString("\n")
	}
	content.WriteString("\n")

	content.WriteString(WorktreeNameStyle.Render("Current configuration:"))
	content.WriteString("\n")
	if m.initState.postCreateCmd != "" {
		content.WriteString(WorktreePathStyle.Render(fmt.Sprintf("⚡ Command: %s", m.initState.postCreateCmd)))
	} else {
		content.WriteString(WorktreePathStyle.Render("📝 Custom script (will be created)"))
	}
	content.WriteString("\n\n")

	// Next steps
	content.WriteString(WorktreeNameStyle.Render("Would you like to:"))
	content.WriteString("\n\n")

	options := []string{
		"Edit the post-create script (recommended)",
		"Skip and continue",
	}

	for i, option := range options {
		var style lipgloss.Style
		if i == m.initState.selected {
			style = WorktreeSelectedStyle
		} else {
			style = WorktreeItemStyle
		}

		prefix := "📝 "
		description := "Customize the setup commands for your project"
		if i == 1 {
			prefix = "⏭️ "
			description = "Use the generated script as-is"
		}

		content.WriteString(style.Width(m.width-8).Render(fmt.Sprintf("%s%s", prefix, option)))
		content.WriteString("\n")
		content.WriteString(WorktreePathStyle.Render(fmt.Sprintf("   %s", description)))
		content.WriteString("\n")
		if i < len(options)-1 {
			content.WriteString("\n")
		}
	}

	content.WriteString("\n\n")
	content.WriteString(HelpStyle.Render("[enter] Select  [↑↓] Navigate  [esc] Back"))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}