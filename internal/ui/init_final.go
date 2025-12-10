package ui

import (
	"strings"
)

// renderCompleteStep shows completion
func (m Model) renderCompleteStep() string {
	var b strings.Builder

	b.WriteString(WizardHeader("Setup Complete"))
	b.WriteString("\n\n")

	b.WriteString(WizardSuccessStyle.Render("✓ Gren is ready to use"))
	b.WriteString("\n\n")

	b.WriteString(WizardSubtitleStyle.Render("Quick start:"))
	b.WriteString("\n")
	b.WriteString(WizardDescStyle.Render("  Press 'n' to create your first worktree"))
	b.WriteString("\n\n")

	b.WriteString(WizardSubtitleStyle.Render("Each worktree will:"))
	b.WriteString("\n")
	b.WriteString(WizardDescStyle.Render("  • Copy environment files"))
	b.WriteString("\n")
	b.WriteString(WizardDescStyle.Render("  • Install dependencies"))
	b.WriteString("\n")
	b.WriteString(WizardDescStyle.Render("  • Run your setup script"))
	b.WriteString("\n\n")

	b.WriteString(WizardHelpBar("enter dashboard", "q quit"))

	return m.wrapWizardContent(b.String())
}

// renderCommitConfirmStep shows commit confirmation
func (m Model) renderCommitConfirmStep() string {
	var b strings.Builder

	b.WriteString(WizardHeader("Commit Configuration"))
	b.WriteString("\n\n")

	b.WriteString(WizardSubtitleStyle.Render("Files ready to commit:"))
	b.WriteString("\n")
	b.WriteString(WizardDescStyle.Render("  .gren/config.json"))
	b.WriteString("\n")
	b.WriteString(WizardDescStyle.Render("  .gren/post-create.sh"))
	b.WriteString("\n")
	if needsGitignoreUpdate() {
		b.WriteString(WizardDescStyle.Render("  .gitignore"))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	b.WriteString(WizardSubtitleStyle.Render("Benefits:"))
	b.WriteString("\n")
	b.WriteString(WizardDescStyle.Render("  • Share with team"))
	b.WriteString("\n")
	b.WriteString(WizardDescStyle.Render("  • Consistent setup"))
	b.WriteString("\n")
	b.WriteString(WizardDescStyle.Render("  • Track changes"))
	b.WriteString("\n\n")

	b.WriteString(WizardHelpBar("y commit", "n skip", "esc back"))

	return m.wrapWizardContent(b.String())
}

// renderFinalStep shows the final completion
func (m Model) renderFinalStep() string {
	var b strings.Builder

	b.WriteString(WizardHeader("All Done"))
	b.WriteString("\n\n")

	b.WriteString(WizardSuccessStyle.Render("✓ Gren is configured and ready"))
	b.WriteString("\n\n")

	b.WriteString(WizardSubtitleStyle.Render("Create your first worktree:"))
	b.WriteString("\n")
	b.WriteString(WizardDescStyle.Render("  Press 'n' from the dashboard"))
	b.WriteString("\n\n")

	b.WriteString(WizardSubtitleStyle.Render("Configuration files:"))
	b.WriteString("\n")
	b.WriteString(WizardDescStyle.Render("  .gren/config.json      - settings"))
	b.WriteString("\n")
	b.WriteString(WizardDescStyle.Render("  .gren/post-create.sh   - setup script"))
	b.WriteString("\n\n")

	b.WriteString(WizardHelpBar("enter dashboard", "q quit"))

	return m.wrapWizardContent(b.String())
}
