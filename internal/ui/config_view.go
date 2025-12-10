package ui

import (
	"strings"
)

// configView renders the configuration file selector
func (m Model) configView() string {
	if m.configState == nil {
		return "Loading..."
	}

	var b strings.Builder

	b.WriteString(WizardHeader("Configuration"))
	b.WriteString("\n\n")

	if len(m.configState.files) == 0 {
		b.WriteString(WizardDescStyle.Render("No configuration files found."))
		b.WriteString("\n")
		b.WriteString(WizardDescStyle.Render("Run 'gren init' to create configuration."))
		b.WriteString("\n\n")
		b.WriteString(WizardHelpBar("esc back"))
		return m.wrapWizardContent(b.String())
	}

	b.WriteString(WizardSubtitleStyle.Render("Select file to edit"))
	b.WriteString("\n\n")

	// File list
	for i, file := range m.configState.files {
		label := file.Icon + " " + file.Name
		b.WriteString(WizardOption(label, i == m.configState.selectedIndex))
		b.WriteString("\n")

		// Show description for selected item
		if i == m.configState.selectedIndex && file.Description != "" {
			b.WriteString(WizardDescStyle.Render("   " + file.Description))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(WizardHelpBar("↑↓ select", "enter open", "esc back"))

	return m.wrapWizardContent(b.String())
}
