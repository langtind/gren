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
