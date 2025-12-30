package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderHelpOverlay renders the help screen as a modal overlay
func (m Model) renderHelpOverlay(baseView string) string {
	helpContent := m.renderHelpContent()
	return m.renderWithModal(baseView, helpContent)
}

// renderHelpContent renders the help content
func (m Model) renderHelpContent() string {
	var b strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true)
	b.WriteString(titleStyle.Render("Keyboard Shortcuts"))
	b.WriteString("\n\n")

	// Define help sections
	sections := []struct {
		title string
		items []struct {
			key  string
			desc string
		}
	}{
		{
			title: "Navigation",
			items: []struct {
				key  string
				desc string
			}{
				{"↑/k", "Move up"},
				{"↓/j", "Move down"},
				{"enter", "Open in... menu"},
				{"g", "Go to worktree directory"},
			},
		},
		{
			title: "Worktrees",
			items: []struct {
				key  string
				desc string
			}{
				{"n", "New worktree"},
				{"d", "Delete worktree"},
				{"m", "Compare/merge changes from worktree"},
				{"t", "Tools menu (cleanup, prune, refresh)"},
			},
		},
		{
			title: "Configuration",
			items: []struct {
				key  string
				desc string
			}{
				{"i", "Initialize gren"},
				{"c", "Edit config files"},
			},
		},
		{
			title: "General",
			items: []struct {
				key  string
				desc string
			}{
				{"?", "Toggle help"},
				{"q", "Quit"},
				{"esc", "Back / Close"},
			},
		},
	}

	// Styles
	sectionStyle := lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true)

	keyStyle := lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Width(8)

	descStyle := lipgloss.NewStyle().
		Foreground(ColorText)

	// Render sections
	for i, section := range sections {
		b.WriteString(sectionStyle.Render(section.title))
		b.WriteString("\n")

		for _, item := range section.items {
			b.WriteString("  ")
			b.WriteString(keyStyle.Render(item.key))
			b.WriteString(descStyle.Render(item.desc))
			b.WriteString("\n")
		}

		if i < len(sections)-1 {
			b.WriteString("\n")
		}
	}

	// Legend section
	b.WriteString("\n")
	b.WriteString(sectionStyle.Render("Legend"))
	b.WriteString("\n")

	legendItems := []struct {
		symbol string
		desc   string
	}{
		{"●", "Current worktree (you are here)"},
		{"[main]", "Main worktree (original repo)"},
		{"+N", "Staged files"},
		{"~N", "Modified files"},
		{"?N", "Untracked files"},
		{"↑N", "Unpushed commits"},
		{"✓", "Clean (no changes)"},
		{"💤", "Stale branch (merged/closed PR)"},
		{"#N", "Pull request number"},
	}

	symbolStyle := lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Width(8)

	for _, item := range legendItems {
		b.WriteString("  ")
		b.WriteString(symbolStyle.Render(item.symbol))
		b.WriteString(descStyle.Render(item.desc))
		b.WriteString("\n")
	}

	// Footer
	b.WriteString("\n")
	footerStyle := lipgloss.NewStyle().Foreground(ColorTextMuted)
	b.WriteString(footerStyle.Render("Press ? or esc to close"))

	return b.String()
}

// renderCompareHelpOverlay renders help for the compare view
func (m Model) renderCompareHelpOverlay(baseView string) string {
	helpContent := m.renderCompareHelpContent()
	return m.renderWithModalWidth(baseView, helpContent, 70, ColorPrimary)
}

// renderCompareHelpContent renders help content specific to compare view
func (m Model) renderCompareHelpContent() string {
	var b strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true)
	b.WriteString(titleStyle.Render("Compare & Merge"))
	b.WriteString("\n\n")

	// Description
	descStyle := lipgloss.NewStyle().
		Foreground(ColorText)
	b.WriteString(descStyle.Render("Compare changes from another worktree and selectively"))
	b.WriteString("\n")
	b.WriteString(descStyle.Render("apply them to your current worktree."))
	b.WriteString("\n\n")

	// When to use
	sectionStyle := lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true)
	b.WriteString(sectionStyle.Render("When to use this?"))
	b.WriteString("\n")

	useCases := []string{
		"• Cherry-pick specific file changes without git merge",
		"• Pull changes from an experimental branch",
		"• Copy config or setup changes between worktrees",
		"• Review and selectively apply uncommitted work",
	}
	for _, uc := range useCases {
		b.WriteString(descStyle.Render("  " + uc))
		b.WriteString("\n")
	}

	// Why not PR?
	b.WriteString("\n")
	b.WriteString(sectionStyle.Render("Why not use a PR?"))
	b.WriteString("\n")
	whyNot := []string{
		"• PRs merge entire branches - this picks individual files",
		"• Works with uncommitted changes (no commit needed)",
		"• Faster for quick experiments and prototypes",
		"• No git history pollution for throwaway changes",
	}
	for _, w := range whyNot {
		b.WriteString(descStyle.Render("  " + w))
		b.WriteString("\n")
	}

	// Keyboard shortcuts
	b.WriteString("\n")
	b.WriteString(sectionStyle.Render("Keyboard Shortcuts"))
	b.WriteString("\n")

	keyStyle := lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Width(10)

	shortcuts := []struct {
		key  string
		desc string
	}{
		{"↑/↓/j/k", "Navigate files / Scroll diff"},
		{"→/l", "Focus diff panel"},
		{"←/h", "Back to file list"},
		{"space", "Toggle file selection"},
		{"a", "Select/deselect all"},
		{"y", "Apply selected files"},
		{"esc", "Back"},
	}

	for _, s := range shortcuts {
		b.WriteString("  ")
		b.WriteString(keyStyle.Render(s.key))
		b.WriteString(descStyle.Render(s.desc))
		b.WriteString("\n")
	}

	// Footer
	b.WriteString("\n")
	footerStyle := lipgloss.NewStyle().Foreground(ColorTextMuted)
	b.WriteString(footerStyle.Render("Press ? or esc to close"))

	return b.String()
}
