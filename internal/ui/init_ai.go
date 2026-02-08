package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// aiScriptScrollParams calculates visible lines and max scroll offset for the AI script preview.
// Used by both the handler (for input clamping) and the renderer (for display).
const aiScriptReservedLines = 14 // header(2) + success(2) + options(5) + helpbar(2) + padding(3)

func aiScriptScrollParams(termHeight, totalLines int) (visibleLines, maxOffset int) {
	visibleLines = termHeight - aiScriptReservedLines
	if visibleLines < 5 {
		visibleLines = 5
	}
	if visibleLines > totalLines {
		visibleLines = totalLines
	}
	maxOffset = totalLines - visibleLines
	if maxOffset < 0 {
		maxOffset = 0
	}
	return visibleLines, maxOffset
}

// renderAIGeneratingStep shows the AI script generation in progress
func (m Model) renderAIGeneratingStep() string {
	var b strings.Builder

	b.WriteString(WizardHeader("Generating Setup Script"))
	b.WriteString("\n\n")

	b.WriteString(m.initState.aiSpinner.View() + " Generating with Claude Code...")
	b.WriteString("\n\n")

	b.WriteString(WizardDescStyle.Render("Claude is reading files and analyzing your project structure"))
	b.WriteString("\n")
	b.WriteString(WizardDescStyle.Render("This may take up to a minute"))
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

	scriptLines := strings.Split(m.initState.aiGeneratedScript, "\n")
	totalLines := len(scriptLines)

	b.WriteString(WizardHeader("Generated Script"))
	b.WriteString("\n\n")

	b.WriteString(WizardSuccessStyle.Render(fmt.Sprintf("Script generated (%d lines)", totalLines)))
	b.WriteString("\n\n")

	// Calculate visible area for script preview
	visibleLines, maxOffset := aiScriptScrollParams(m.height, totalLines)

	// Use local scroll offset for rendering (don't mutate state in View)
	scrollOffset := m.initState.aiScrollOffset
	if scrollOffset > maxOffset {
		scrollOffset = maxOffset
	}

	// Show script window with scroll
	codeStyle := lipgloss.NewStyle().Foreground(ColorTextMuted)
	lineNumStyle := lipgloss.NewStyle().Foreground(ColorBorder)

	for i := scrollOffset; i < scrollOffset+visibleLines && i < totalLines; i++ {
		lineNum := lineNumStyle.Render(fmt.Sprintf(" %3d ", i+1))
		b.WriteString(lineNum + codeStyle.Render(scriptLines[i]))
		b.WriteString("\n")
	}

	// Scroll indicator
	if totalLines > visibleLines {
		scrollInfo := fmt.Sprintf("  Lines %d-%d of %d",
			scrollOffset+1,
			min(scrollOffset+visibleLines, totalLines),
			totalLines)
		b.WriteString(lineNumStyle.Render(scrollInfo))
		b.WriteString("\n")
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

	helpItems := []string{"j/k scroll", "tab select", "enter confirm", "esc back"}
	b.WriteString(WizardHelpBar(helpItems...))

	return m.wrapWizardContent(b.String())
}
