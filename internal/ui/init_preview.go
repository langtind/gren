package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderPreviewStep shows configuration preview before creation
func (m Model) renderPreviewStep() string {
	var b strings.Builder

	b.WriteString(WizardHeader("Review Configuration"))
	b.WriteString("\n\n")

	// Summary box
	summaryStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(0, 1)

	aiUsed := m.initState.postCreateScript != ""

	var summary strings.Builder
	summary.WriteString(WizardSubtitleStyle.Render("Location: ") + m.initState.worktreeDir + "\n")
	summary.WriteString(WizardSubtitleStyle.Render("Command:  ") + m.initState.postCreateCmd + "\n")
	summary.WriteString(WizardSubtitleStyle.Render("Files:    ") + fmt.Sprintf("%d to symlink", len(m.initState.detectedFiles)))
	if aiUsed {
		lineCount := len(strings.Split(m.initState.postCreateScript, "\n"))
		summary.WriteString("\n")
		summary.WriteString(WizardSubtitleStyle.Render("Script:   ") + fmt.Sprintf("AI-generated (%d lines)", lineCount))
	}

	b.WriteString(summaryStyle.Render(summary.String()))
	b.WriteString("\n\n")

	// Files to create
	b.WriteString(WizardSubtitleStyle.Render("Will create:"))
	b.WriteString("\n")
	b.WriteString(WizardDescStyle.Render("  .gren/config.toml"))
	b.WriteString("\n")
	if aiUsed {
		b.WriteString(WizardDescStyle.Render("  .gren/post-create.sh (AI-generated)"))
	} else {
		b.WriteString(WizardDescStyle.Render("  .gren/post-create.sh"))
	}
	b.WriteString("\n\n")

	// Options
	options := []string{
		"Create configuration",
		"Back to customize",
		"Cancel",
	}

	for i, opt := range options {
		b.WriteString(WizardOption(opt, i == m.initState.selected))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(WizardHelpBar("↑↓ select", "enter confirm", "esc back"))

	return m.wrapWizardContent(b.String())
}

// renderExecutingStep shows files being created
func (m Model) renderExecutingStep() string {
	var b strings.Builder

	b.WriteString(WizardHeader("Creating Configuration"))
	b.WriteString("\n\n")

	spinnerStyle := lipgloss.NewStyle().Foreground(ColorAccent)
	b.WriteString(spinnerStyle.Render("◐ Creating .gren directory..."))
	b.WriteString("\n")
	b.WriteString(WizardDescStyle.Render("○ Writing config.toml..."))
	b.WriteString("\n")
	b.WriteString(WizardDescStyle.Render("○ Generating post-create.sh..."))

	return m.wrapWizardContent(b.String())
}

// renderCreatedStep shows completion and options
func (m Model) renderCreatedStep() string {
	var b strings.Builder

	b.WriteString(WizardHeader("Setup Complete"))
	b.WriteString("\n\n")

	b.WriteString(WizardSuccessStyle.Render("✓ Gren is ready to use"))
	b.WriteString("\n\n")

	b.WriteString(WizardSubtitleStyle.Render("Created files:"))
	b.WriteString("\n")
	b.WriteString(WizardDescStyle.Render("  .gren/config.toml"))
	b.WriteString("\n")
	b.WriteString(WizardDescStyle.Render("  .gren/post-create.sh"))
	b.WriteString("\n\n")

	aiUsed := m.initState.postCreateScript != ""

	if aiUsed {
		b.WriteString(WizardSubtitleStyle.Render("Recommended:"))
		b.WriteString("\n")
		b.WriteString(WizardDescStyle.Render("  Review the AI-generated script before creating worktrees"))
		b.WriteString("\n\n")
	} else {
		b.WriteString(WizardSubtitleStyle.Render("Next steps:"))
		b.WriteString("\n")
		b.WriteString(WizardDescStyle.Render("  • Edit post-create.sh to customize setup"))
		b.WriteString("\n")
		b.WriteString(WizardDescStyle.Render("  • Press 'n' to create your first worktree"))
		b.WriteString("\n\n")
	}

	// Options
	var options []string
	if aiUsed {
		options = []string{
			"Review generated script",
			"Go to dashboard",
		}
	} else {
		options = []string{
			"Edit setup script",
			"Go to dashboard",
		}
	}

	for i, opt := range options {
		b.WriteString(WizardOption(opt, i == m.initState.selected))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(WizardHelpBar("↑↓ select", "enter confirm"))

	return m.wrapWizardContent(b.String())
}
