package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderWelcomeStep shows the initial welcome
func (m Model) renderWelcomeStep() string {
	var b strings.Builder

	b.WriteString(WizardHeader("Initialize Gren"))
	b.WriteString("\n\n")

	// Brief intro
	intro := `Gren helps you manage git worktrees by:

• Creating separate directories for different branches
• Automatically setting up dependencies
• Symlinking environment files`

	b.WriteString(WizardDescStyle.Render(intro))
	b.WriteString("\n\n")

	// What we'll do
	b.WriteString(WizardSubtitleStyle.Render("This wizard will:"))
	b.WriteString("\n")
	b.WriteString(WizardDescStyle.Render("  1. Analyze your project"))
	b.WriteString("\n")
	b.WriteString(WizardDescStyle.Render("  2. Generate setup configuration"))
	b.WriteString("\n")
	b.WriteString(WizardDescStyle.Render("  3. Create .gren/ directory"))
	b.WriteString("\n\n")

	// Version info
	if m.version != "" {
		versionStyle := lipgloss.NewStyle().
			Foreground(ColorTextMuted).
			Align(lipgloss.Right)
		b.WriteString(versionStyle.Render(m.version))
		b.WriteString("\n\n")
	}

	b.WriteString(WizardHelpBar("enter start", "q quit"))

	return m.wrapWizardContent(b.String())
}

// renderAnalysisStep shows project analysis in progress
func (m Model) renderAnalysisStep() string {
	var b strings.Builder

	b.WriteString(WizardHeader("Analyzing Project"))
	b.WriteString("\n\n")

	if !m.initState.analysisComplete {
		spinnerStyle := lipgloss.NewStyle().Foreground(ColorAccent)
		b.WriteString(spinnerStyle.Render("◐ Scanning project structure..."))
		b.WriteString("\n")
		b.WriteString(WizardDescStyle.Render("○ Detecting configuration files..."))
		b.WriteString("\n")
		b.WriteString(WizardDescStyle.Render("○ Analyzing dependencies..."))
	} else {
		b.WriteString(WizardSuccessStyle.Render("✓ Analysis complete"))
	}

	return m.wrapWizardContent(b.String())
}
