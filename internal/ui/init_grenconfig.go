package ui

import (
	"strings"
)

// renderGrenConfigStep shows the .gren configuration tracking options
func (m Model) renderGrenConfigStep() string {
	var b strings.Builder

	b.WriteString(WizardHeader(".gren Configuration"))
	b.WriteString("\n\n")

	b.WriteString(WizardDescStyle.Render("How should .gren/ configuration be managed?"))
	b.WriteString("\n\n")

	// Options
	options := []struct {
		title string
		desc  string
	}{
		{
			title: "Track in git (Share with team)",
			desc:  "Commit .gren/ to version control so everyone uses the same setup",
		},
		{
			title: "Keep local (Add to .gitignore)",
			desc:  "Add .gren/ to .gitignore for personal configuration",
		},
	}

	for i, option := range options {
		selected := i == m.initState.selected

		// Title
		b.WriteString(WizardOption(option.title, selected))
		b.WriteString("\n")

		// Description (indented)
		if selected {
			b.WriteString(WizardDescStyle.Render("    " + option.desc))
		} else {
			b.WriteString(WizardDescStyle.Render("    " + option.desc))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(WizardHelpBar("↑↓ select", "enter confirm", "esc back"))

	return m.wrapWizardContent(b.String())
}
