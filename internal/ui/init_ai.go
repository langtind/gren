package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderAIGeneratingStep shows the AI script generation in progress
func (m Model) renderAIGeneratingStep() string {
	var b strings.Builder

	b.WriteString(WizardHeader("Generating Setup Script"))
	b.WriteString("\n\n")

	spinnerStyle := lipgloss.NewStyle().Foreground(ColorAccent)
	b.WriteString(spinnerStyle.Render("◐ Analyzing project with Claude Code..."))
	b.WriteString("\n\n")

	b.WriteString(WizardDescStyle.Render("Using Claude Code CLI to generate a setup script"))
	b.WriteString("\n")
	b.WriteString(WizardDescStyle.Render("This may take a few seconds"))
	b.WriteString("\n")

	return m.wrapWizardContent(b.String())
}

// renderAIResultStep shows the generated AI script for review
func (m Model) renderAIResultStep() string {
	var b strings.Builder

	if m.initState.aiError != "" {
		b.WriteString(WizardHeader("Generation Failed"))
		b.WriteString("\n\n")

		errorStyle := lipgloss.NewStyle().Foreground(ColorError)
		b.WriteString(errorStyle.Render(m.initState.aiError))
		b.WriteString("\n\n")

		b.WriteString(WizardHelpBar("enter back", "q quit"))
		return m.wrapWizardContent(b.String())
	}

	b.WriteString(WizardHeader("Claude Code Generated Script"))
	b.WriteString("\n\n")

	b.WriteString(WizardSuccessStyle.Render("Script generated successfully"))
	b.WriteString("\n\n")

	// Show a preview of the script (truncated if too long)
	b.WriteString(WizardSubtitleStyle.Render("Preview:"))
	b.WriteString("\n")

	scriptLines := strings.Split(m.initState.aiGeneratedScript, "\n")
	maxLines := 12
	if len(scriptLines) > maxLines {
		for i := 0; i < maxLines; i++ {
			b.WriteString(WizardDescStyle.Render("  " + scriptLines[i]))
			b.WriteString("\n")
		}
		b.WriteString(WizardDescStyle.Render("  ..."))
		b.WriteString("\n")
	} else {
		for _, line := range scriptLines {
			b.WriteString(WizardDescStyle.Render("  " + line))
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")

	// Options
	options := []string{
		"Use this script",
		"Regenerate",
		"Edit manually instead",
	}

	for i, opt := range options {
		b.WriteString(WizardOption(opt, i == m.initState.selected))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(WizardHelpBar("↑↓ select", "enter confirm", "esc back"))

	return m.wrapWizardContent(b.String())
}
