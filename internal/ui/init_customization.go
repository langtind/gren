package ui

import (
	"fmt"
	"strings"
)

// renderCustomizationStep shows the customization interface
func (m Model) renderCustomizationStep() string {
	if m.initState.customizationMode == "" {
		return m.renderCustomizationMenu()
	}

	switch m.initState.customizationMode {
	case "worktree":
		return m.renderWorktreeCustomization()
	case "patterns":
		return m.renderPatternsCustomization()
	case "postcreate":
		return m.renderPostCreateCustomization()
	default:
		return m.renderCustomizationMenu()
	}
}

// renderCustomizationMenu shows the main customization menu
func (m Model) renderCustomizationMenu() string {
	var b strings.Builder

	b.WriteString(WizardHeader("Customize"))
	b.WriteString("\n\n")

	options := []struct {
		name string
		desc string
	}{
		{"Worktree location", fmt.Sprintf("Currently: %s", m.initState.worktreeDir)},
		{"Files to copy", fmt.Sprintf("%d files detected", len(m.initState.detectedFiles))},
		{"Setup command", fmt.Sprintf("Currently: %s", m.initState.postCreateCmd)},
	}

	for i, opt := range options {
		b.WriteString(WizardOption(opt.name, i == m.initState.selected))
		b.WriteString("\n")
		if i == m.initState.selected {
			b.WriteString(WizardDescStyle.Render("   " + opt.desc))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(WizardHelpBar("↑↓ select", "enter edit", "esc back"))

	return m.wrapWizardContent(b.String())
}

// renderWorktreeCustomization shows worktree directory customization
func (m Model) renderWorktreeCustomization() string {
	var b strings.Builder

	b.WriteString(WizardHeader("Worktree Location"))
	b.WriteString("\n\n")

	b.WriteString(WizardSubtitleStyle.Render("Where to create worktrees"))
	b.WriteString("\n\n")

	// Input field
	displayText := m.initState.editingText
	if displayText == "" {
		displayText = m.initState.worktreeDir
	}
	b.WriteString(WizardInputStyle.Width(50).Render(displayText + "▮"))
	b.WriteString("\n\n")

	// Examples
	b.WriteString(WizardDescStyle.Render("Examples:"))
	b.WriteString("\n")
	b.WriteString(WizardDescStyle.Render("  ../worktrees    (sibling directory)"))
	b.WriteString("\n")
	b.WriteString(WizardDescStyle.Render("  ~/dev/trees     (absolute path)"))
	b.WriteString("\n\n")

	b.WriteString(WizardHelpBar("type path", "enter save", "esc cancel"))

	return m.wrapWizardContent(b.String())
}

// renderPatternsCustomization shows detected files
func (m Model) renderPatternsCustomization() string {
	var b strings.Builder

	b.WriteString(WizardHeader("Files to Copy"))
	b.WriteString("\n\n")

	if len(m.initState.detectedFiles) == 0 {
		b.WriteString(WizardDescStyle.Render("No gitignored files detected."))
		b.WriteString("\n\n")
	} else {
		b.WriteString(WizardSubtitleStyle.Render("These files will be copied to new worktrees:"))
		b.WriteString("\n\n")

		for _, file := range m.initState.detectedFiles {
			icon := "📄"
			if file.IsGitIgnored {
				icon = "🔒"
			}
			b.WriteString(WizardDescStyle.Render(fmt.Sprintf("  %s %s", icon, file.Path)))
			b.WriteString("\n")
			if file.Description != "" {
				b.WriteString(WizardDescStyle.Render(fmt.Sprintf("     %s", file.Description)))
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}

	b.WriteString(WizardDescStyle.Render("Edit .gren/post-create.sh to customize file copying"))
	b.WriteString("\n\n")

	b.WriteString(WizardHelpBar("esc back"))

	return m.wrapWizardContent(b.String())
}

// renderPostCreateCustomization shows post-create command customization
func (m Model) renderPostCreateCustomization() string {
	var b strings.Builder

	b.WriteString(WizardHeader("Setup Command"))
	b.WriteString("\n\n")

	b.WriteString(WizardSubtitleStyle.Render("Command to run after creating worktree"))
	b.WriteString("\n\n")

	// Input field
	displayText := m.initState.editingText
	if displayText == "" {
		displayText = m.initState.postCreateCmd
	}
	b.WriteString(WizardInputStyle.Width(50).Render(displayText + "▮"))
	b.WriteString("\n\n")

	// Examples based on detected package manager
	b.WriteString(WizardDescStyle.Render("Examples:"))
	b.WriteString("\n")
	b.WriteString(WizardDescStyle.Render("  npm install"))
	b.WriteString("\n")
	b.WriteString(WizardDescStyle.Render("  bun install"))
	b.WriteString("\n")
	b.WriteString(WizardDescStyle.Render("  go mod download"))
	b.WriteString("\n")
	b.WriteString(WizardDescStyle.Render("  pip install -r requirements.txt"))
	b.WriteString("\n\n")

	b.WriteString(WizardHelpBar("type command", "enter save", "esc cancel"))

	return m.wrapWizardContent(b.String())
}
