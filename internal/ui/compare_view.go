package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderCompareView renders the compare worktrees view with split layout
func (m Model) renderCompareView() string {
	if m.compareState == nil {
		// Show loading state while compare is being initialized
		titleStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary)
		return fmt.Sprintf("%s %s", m.compareSpinner.View(), titleStyle.Render("Loading comparison..."))
	}

	state := m.compareState

	// Styles - using shared color constants from styles.go
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(0, 1)

	selectedStyle := lipgloss.NewStyle().
		Background(ColorBgSelected).
		Foreground(ColorTextPrimary)

	addedStyle := lipgloss.NewStyle().
		Foreground(ColorSuccess)

	modifiedStyle := lipgloss.NewStyle().
		Foreground(ColorWarning)

	deletedStyle := lipgloss.NewStyle().
		Foreground(ColorError)

	diffAddStyle := lipgloss.NewStyle().
		Foreground(ColorSuccess)

	diffDelStyle := lipgloss.NewStyle().
		Foreground(ColorError)

	diffHeaderStyle := lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true)

	dimStyle := lipgloss.NewStyle().
		Foreground(ColorTextMuted)

	// Show completion or error message
	if state.applyComplete {
		var b strings.Builder
		b.WriteString(titleStyle.Render(fmt.Sprintf("Compare: %s → current", state.sourceWorktree)))
		b.WriteString("\n\n")
		if state.applyError != "" {
			b.WriteString(ErrorStyle.Render(fmt.Sprintf("Error: %s", state.applyError)))
		} else {
			b.WriteString(SuccessStyle.Render(fmt.Sprintf("Successfully applied %d file(s)", state.appliedCount)))
		}
		b.WriteString("\n\n")
		b.WriteString("Press [q] or [esc] to return to dashboard")
		return b.String()
	}

	if state.applyInProgress {
		return titleStyle.Render("Applying changes...")
	}

	if len(state.files) == 0 {
		var b strings.Builder
		b.WriteString(titleStyle.Render(fmt.Sprintf("Compare: %s → current", state.sourceWorktree)))
		b.WriteString("\n\n")
		b.WriteString("No changes found between worktrees.\n\n")
		b.WriteString("Press [q] or [esc] to return to dashboard")
		return b.String()
	}

	// Calculate dimensions - fixed layout
	totalWidth := m.width
	if totalWidth < 80 {
		totalWidth = 80
	}
	// Reserve: 1 title, 1 blank, panels, 1 selection count, 1 footer = 4 lines
	panelHeight := m.height - 4
	if panelHeight < 10 {
		panelHeight = 10
	}

	// Left panel (file list) gets 35% width, right panel (diff) gets 65%
	leftWidth := totalWidth * 35 / 100
	if leftWidth < 30 {
		leftWidth = 30
	}
	rightWidth := totalWidth - leftWidth - 4 // Account for borders and spacing

	// Content height inside panels (subtract border and header)
	contentHeight := panelHeight - 4
	if contentHeight < 3 {
		contentHeight = 3
	}

	// Build left panel (file list)
	var leftLines []string
	filesLabel := "FILES"
	if !state.diffFocused {
		filesLabel = "FILES (focused)"
	}
	leftLines = append(leftLines, dimStyle.Render(filesLabel))
	leftLines = append(leftLines, strings.Repeat("─", leftWidth-4))

	visibleFiles := contentHeight - 2 // Subtract header lines
	if visibleFiles < 1 {
		visibleFiles = 1
	}

	startIdx := state.scrollOffset
	endIdx := startIdx + visibleFiles
	if endIdx > len(state.files) {
		endIdx = len(state.files)
	}

	for i := startIdx; i < endIdx; i++ {
		file := state.files[i]

		// Checkbox
		checkbox := "○"
		if file.Selected {
			checkbox = "●"
		}

		// Status icon and color
		var icon string
		var pathStyle lipgloss.Style
		switch file.Status {
		case "added":
			icon = "+"
			pathStyle = addedStyle
		case "modified":
			icon = "~"
			pathStyle = modifiedStyle
		case "deleted":
			icon = "-"
			pathStyle = deletedStyle
		default:
			icon = "?"
			pathStyle = dimStyle
		}

		// Truncate path if needed
		displayPath := file.Path
		maxPathLen := leftWidth - 10
		if len(displayPath) > maxPathLen {
			displayPath = "..." + displayPath[len(displayPath)-maxPathLen+3:]
		}

		line := fmt.Sprintf("%s %s %s", checkbox, icon, displayPath)

		if i == state.selectedIndex {
			// Pad to fill width for selection highlight
			// Use lipgloss.Width for correct Unicode character width calculation
			padding := leftWidth - 4 - lipgloss.Width(line)
			if padding > 0 {
				line += strings.Repeat(" ", padding)
			}
			line = selectedStyle.Render(line)
		} else {
			line = fmt.Sprintf("%s %s %s", checkbox, pathStyle.Render(icon), pathStyle.Render(displayPath))
		}

		leftLines = append(leftLines, line)
	}

	// Pad to fill height
	for len(leftLines) < contentHeight {
		leftLines = append(leftLines, "")
	}

	// Show scroll indicator if needed (replace last line)
	if len(state.files) > visibleFiles {
		scrollInfo := fmt.Sprintf("[%d/%d]", state.selectedIndex+1, len(state.files))
		leftLines[len(leftLines)-1] = dimStyle.Render(scrollInfo)
	}

	leftContent := strings.Join(leftLines, "\n")

	// Build right panel (diff viewer)
	var rightLines []string
	diffLabel := "DIFF"
	if state.diffFocused {
		diffLabel = "DIFF (focused)"
	}
	rightLines = append(rightLines, dimStyle.Render(diffLabel))
	rightLines = append(rightLines, strings.Repeat("─", rightWidth-4))

	visibleDiffLines := contentHeight - 2 // Subtract header lines
	if visibleDiffLines < 1 {
		visibleDiffLines = 1
	}

	if state.diffContent == "" {
		rightLines = append(rightLines, dimStyle.Render("Loading diff..."))
	} else {
		// Render diff with syntax highlighting
		diffLines := strings.Split(state.diffContent, "\n")

		startLine := state.diffScrollOffset
		endLine := startLine + visibleDiffLines
		if endLine > len(diffLines) {
			endLine = len(diffLines)
		}

		for i := startLine; i < endLine; i++ {
			line := diffLines[i]
			// Truncate long lines
			if len(line) > rightWidth-4 {
				line = line[:rightWidth-7] + "..."
			}

			// Color based on diff type
			if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
				rightLines = append(rightLines, diffAddStyle.Render(line))
			} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
				rightLines = append(rightLines, diffDelStyle.Render(line))
			} else if strings.HasPrefix(line, "@@") {
				rightLines = append(rightLines, diffHeaderStyle.Render(line))
			} else if strings.HasPrefix(line, "diff ") || strings.HasPrefix(line, "index ") ||
				strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") {
				rightLines = append(rightLines, diffHeaderStyle.Render(line))
			} else {
				rightLines = append(rightLines, line)
			}
		}

		// Show scroll indicator for diff
		if len(diffLines) > visibleDiffLines {
			scrollInfo := fmt.Sprintf("[line %d/%d]", startLine+1, len(diffLines))
			// Pad and add scroll info
			for len(rightLines) < contentHeight-1 {
				rightLines = append(rightLines, "")
			}
			rightLines = append(rightLines, dimStyle.Render(scrollInfo))
		}
	}

	// Pad to fill height
	for len(rightLines) < contentHeight {
		rightLines = append(rightLines, "")
	}

	rightContent := strings.Join(rightLines, "\n")

	// Create boxed panels with fixed height
	leftBoxStyle := boxStyle
	if !state.diffFocused {
		leftBoxStyle = leftBoxStyle.BorderForeground(ColorSuccess) // Green when focused
	}
	leftPanel := leftBoxStyle.Width(leftWidth - 2).Height(panelHeight - 2).Render(leftContent)

	rightBoxStyle := boxStyle
	if state.diffFocused {
		rightBoxStyle = rightBoxStyle.BorderForeground(ColorSuccess) // Green when focused
	}
	rightPanel := rightBoxStyle.Width(rightWidth - 2).Height(panelHeight - 2).Render(rightContent)

	// Join panels horizontally
	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, " ", rightPanel)

	// Build final view
	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf("Compare: %s → current worktree", state.sourceWorktree)))
	b.WriteString("\n")
	b.WriteString(panels)
	b.WriteString("\n")

	// Selection count
	selectedCount := 0
	for _, f := range state.files {
		if f.Selected {
			selectedCount++
		}
	}
	b.WriteString(fmt.Sprintf("%d of %d files selected", selectedCount, len(state.files)))
	b.WriteString("\n")

	// Help footer - same style as dashboard
	width := m.width - 2
	sep := HelpSeparatorStyle.Render(" │ ")

	var footer string
	if state.diffFocused {
		nav := HelpItem("↑↓jk", "scroll")
		other := HelpItem("←h", "back") + " " + HelpItem("?", "help") + " " + HelpItem("esc", "exit")
		footer = nav + sep + other
	} else {
		nav := HelpItem("↑↓jk", "nav")
		selection := HelpItem("space", "toggle") + " " + HelpItem("a", "all")
		actions := HelpItem("→l", "focus") + " " + HelpItem("y", "apply")
		other := HelpItem("?", "help") + " " + HelpItem("esc", "back")
		footer = nav + sep + selection + sep + actions + sep + other
	}
	b.WriteString(FooterBarStyle.Width(width).Render(footer))

	return b.String()
}
