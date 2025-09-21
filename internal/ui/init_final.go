package ui

import (
	"strings"
)

// renderCompleteStep shows completion
func (m Model) renderCompleteStep() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("✅ Setup Complete"))
	content.WriteString("\n\n")

	content.WriteString(StatusCleanStyle.Render("🎉 gren has been successfully configured!"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("What's next:"))
	content.WriteString("\n\n")

	content.WriteString(WorktreePathStyle.Render("1. Create your first worktree with 'n'"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("2. Switch between worktrees with ↑↓ and Enter"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("3. Customize .gren/post-create.sh as needed"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("Pro tips:"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("• Each worktree is a separate working directory"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("• Environment files are automatically copied"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("• Dependencies are installed automatically"))
	content.WriteString("\n\n")

	content.WriteString(HelpStyle.Render("[enter] Go to dashboard  [q] Quit"))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}

// renderCommitConfirmStep shows commit confirmation
func (m Model) renderCommitConfirmStep() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("📤 Commit Configuration"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("Your gren configuration is ready!"))
	content.WriteString("\n\n")

	content.WriteString(WorktreePathStyle.Render("The configuration files have been created:"))
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

	content.WriteString(WorktreeNameStyle.Render("Would you like to commit these files to git?"))
	content.WriteString("\n\n")

	// Benefits of committing
	content.WriteString(WorktreePathStyle.Render("✅ Benefits of committing:"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   • Share configuration with team members"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   • Keep setup consistent across machines"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   • Track changes to configuration"))
	content.WriteString("\n\n")

	content.WriteString(WorktreePathStyle.Render("⚠️ You can always commit manually later with:"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   git add .gren/ && git commit -m 'Add gren configuration'"))
	content.WriteString("\n\n")

	content.WriteString(HelpStyle.Render("[y] Commit now  [n] Skip commit  [esc] Back"))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}

// renderFinalStep shows the final completion
func (m Model) renderFinalStep() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("🚀 All Done!"))
	content.WriteString("\n\n")

	content.WriteString(StatusCleanStyle.Render("🎉 gren is now ready to use!"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("Quick start:"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeSelectedStyle.Width(m.width-8).Render("Press 'n' to create your first worktree"))
	content.WriteString("\n\n")

	content.WriteString(WorktreePathStyle.Render("Each worktree will be automatically set up with:"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   • Copied environment and configuration files"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   • Installed dependencies"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   • Custom setup commands"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("Documentation:"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   • Configuration: .gren/config.yml"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   • Setup script: .gren/post-create.sh"))
	content.WriteString("\n\n")

	content.WriteString(HelpStyle.Render("[enter] Go to dashboard  [q] Quit"))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}