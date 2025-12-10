package ui

import (
	"fmt"
	"strings"
)

// renderRecommendationsStep shows detected files and recommendations
func (m Model) renderRecommendationsStep() string {
	var b strings.Builder

	b.WriteString(WizardHeader("Configuration"))
	b.WriteString("\n\n")

	// Detected setup
	b.WriteString(WizardSubtitleStyle.Render("Detected"))
	b.WriteString("\n")

	// Package manager
	if m.initState.packageManager != "" {
		b.WriteString(WizardDescStyle.Render(fmt.Sprintf("  Package manager: %s", m.initState.packageManager)))
	} else {
		b.WriteString(WizardDescStyle.Render("  Package manager: none detected"))
	}
	b.WriteString("\n")

	// Worktree location
	b.WriteString(WizardDescStyle.Render(fmt.Sprintf("  Worktree location: %s", m.initState.worktreeDir)))
	b.WriteString("\n\n")

	// Detected files
	if len(m.initState.detectedFiles) > 0 {
		b.WriteString(WizardSubtitleStyle.Render("Files to copy"))
		b.WriteString("\n")

		for _, file := range m.initState.detectedFiles {
			icon := "📄"
			if file.IsGitIgnored {
				icon = "🔒"
			}
			b.WriteString(WizardDescStyle.Render(fmt.Sprintf("  %s %s", icon, file.Path)))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Setup command
	b.WriteString(WizardSubtitleStyle.Render("Setup command"))
	b.WriteString("\n")
	if m.initState.postCreateCmd != "" {
		b.WriteString(WizardDescStyle.Render(fmt.Sprintf("  %s", m.initState.postCreateCmd)))
	} else {
		b.WriteString(WizardDescStyle.Render("  (none - will create empty script)"))
	}
	b.WriteString("\n\n")

	// Options
	options := []string{
		"Accept and create configuration",
		"Customize settings",
		"Generate setup script with AI",
	}

	for i, option := range options {
		b.WriteString(WizardOption(option, i == m.initState.selected))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(WizardHelpBar("↑↓ select", "enter confirm", "esc back"))

	return m.wrapWizardContent(b.String())
}
