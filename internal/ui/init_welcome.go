package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderWelcomeStep shows the initial welcome
func (m Model) renderWelcomeStep() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("🚀 Welcome to gren"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("Git Worktree Manager"))
	content.WriteString("\n\n")

	intro := `gren helps you manage git worktrees efficiently by:

• Creating separate working directories for different branches
• Automatically copying environment files and configuration
• Running setup scripts for each new worktree
• Keeping your main branch clean while working on features`

	content.WriteString(WorktreePathStyle.Render(intro))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("This setup wizard will:"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("1. Analyze your project structure"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("2. Recommend configuration patterns"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("3. Create initial configuration files"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("4. Set up automation scripts"))
	content.WriteString("\n\n")

	content.WriteString(HelpStyle.Render("[enter] Start setup  [q] Quit"))

	// Add version info above the box, aligned to the right
	var finalResult strings.Builder
	if m.version != "" {
		versionText := fmt.Sprintf("version: %s", m.version)
		// Create right-aligned version text
		versionLine := lipgloss.NewStyle().
			Width(m.width - 4).
			Align(lipgloss.Right).
			Render(HelpStyle.Render(versionText))
		finalResult.WriteString(versionLine)
		finalResult.WriteString("\n")
	}

	// Wrap everything in a border
	result := HeaderStyle.Width(m.width - 4).Render(content.String())
	finalResult.WriteString(result)

	return finalResult.String()
}

// renderAnalysisStep shows project analysis in progress
func (m Model) renderAnalysisStep() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("🔍 Analyzing Project"))
	content.WriteString("\n\n")

	if !m.initState.analysisComplete {
		content.WriteString(SpinnerStyle.Render("⠋ Scanning project structure..."))
		content.WriteString("\n")
		content.WriteString(WorktreePathStyle.Render("⏳ Detecting configuration files..."))
		content.WriteString("\n")
		content.WriteString(WorktreePathStyle.Render("⏸️ Analyzing dependencies..."))
		content.WriteString("\n\n")

		content.WriteString(WorktreePathStyle.Render("This may take a moment..."))
	} else {
		content.WriteString(StatusCleanStyle.Render("✅ Analysis complete!"))
		content.WriteString("\n\n")
		content.WriteString(WorktreePathStyle.Render("Ready to show recommendations..."))
	}

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}