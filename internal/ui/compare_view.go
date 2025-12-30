package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderCompareView renders the compare worktrees view with split layout
func (m Model) renderCompareView() string {
	if m.compareState == nil {
		return "No compare state"
	}

	state := m.compareState

	// Styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7D56F4"))

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	selectedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("#7D56F4")).
		Foreground(lipgloss.Color("#FFFFFF"))

	addedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A6E3A1"))

	modifiedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F9E2AF"))

	deletedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F38BA8"))

	diffAddStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A6E3A1"))

	diffDelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#F38BA8"))

	diffHeaderStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#89B4FA")).
		Bold(true)

	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6C7086"))

	// Show completion or error message
	if state.applyComplete {
		var b strings.Builder
		b.WriteString(titleStyle.Render(fmt.Sprintf("Compare: %s → current", state.sourceWorktree)))
		b.WriteString("\n\n")
		if state.applyError != "" {
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#F38BA8")).Render(
				fmt.Sprintf("Error: %s", state.applyError)))
		} else {
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#A6E3A1")).Render(
				fmt.Sprintf("Successfully applied %d file(s)", state.appliedCount)))
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

	// Calculate dimensions
	totalWidth := m.width
	if totalWidth < 80 {
		totalWidth = 80
	}
	totalHeight := m.height - 6 // Leave room for header and footer

	// Left panel (file list) gets 35% width, right panel (diff) gets 65%
	leftWidth := totalWidth * 35 / 100
	if leftWidth < 30 {
		leftWidth = 30
	}
	rightWidth := totalWidth - leftWidth - 4 // Account for borders and spacing

	// Build left panel (file list)
	var leftContent strings.Builder
	leftContent.WriteString(dimStyle.Render("FILES") + "\n")
	leftContent.WriteString(strings.Repeat("─", leftWidth-4) + "\n")

	visibleFiles := totalHeight - 4
	if visibleFiles < 3 {
		visibleFiles = 3
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
			padding := leftWidth - 4 - len(line)
			if padding > 0 {
				line += strings.Repeat(" ", padding)
			}
			line = selectedStyle.Render(line)
		} else {
			line = fmt.Sprintf("%s %s %s", checkbox, pathStyle.Render(icon), pathStyle.Render(displayPath))
		}

		leftContent.WriteString(line + "\n")
	}

	// Show scroll indicator if needed
	if len(state.files) > visibleFiles {
		scrollInfo := fmt.Sprintf("[%d/%d]", state.selectedIndex+1, len(state.files))
		leftContent.WriteString(dimStyle.Render(scrollInfo))
	}

	// Build right panel (diff viewer)
	var rightContent strings.Builder
	diffLabel := "DIFF"
	if state.diffFocused {
		diffLabel = "DIFF (focused)"
	}
	rightContent.WriteString(dimStyle.Render(diffLabel) + "\n")
	rightContent.WriteString(strings.Repeat("─", rightWidth-4) + "\n")

	if state.diffContent == "" {
		rightContent.WriteString(dimStyle.Render("Loading diff..."))
	} else {
		// Render diff with syntax highlighting
		diffLines := strings.Split(state.diffContent, "\n")
		visibleDiffLines := totalHeight - 4

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
				rightContent.WriteString(diffAddStyle.Render(line) + "\n")
			} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
				rightContent.WriteString(diffDelStyle.Render(line) + "\n")
			} else if strings.HasPrefix(line, "@@") {
				rightContent.WriteString(diffHeaderStyle.Render(line) + "\n")
			} else if strings.HasPrefix(line, "diff ") || strings.HasPrefix(line, "index ") ||
				strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") {
				rightContent.WriteString(diffHeaderStyle.Render(line) + "\n")
			} else {
				rightContent.WriteString(line + "\n")
			}
		}

		// Show scroll indicator for diff
		if len(diffLines) > visibleDiffLines {
			scrollInfo := fmt.Sprintf("[line %d/%d]", startLine+1, len(diffLines))
			rightContent.WriteString(dimStyle.Render(scrollInfo))
		}
	}

	// Create boxed panels
	leftPanel := boxStyle.Width(leftWidth - 2).Render(leftContent.String())

	rightBoxStyle := boxStyle
	if state.diffFocused {
		rightBoxStyle = rightBoxStyle.BorderForeground(lipgloss.Color("#A6E3A1")) // Green when focused
	}
	rightPanel := rightBoxStyle.Width(rightWidth - 2).Render(rightContent.String())

	// Join panels horizontally
	panels := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, " ", rightPanel)

	// Build final view
	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf("Compare: %s → current worktree", state.sourceWorktree)))
	b.WriteString("\n\n")
	b.WriteString(panels)
	b.WriteString("\n\n")

	// Selection count
	selectedCount := 0
	for _, f := range state.files {
		if f.Selected {
			selectedCount++
		}
	}
	b.WriteString(fmt.Sprintf("%d of %d files selected", selectedCount, len(state.files)))
	b.WriteString("\n")

	// Help footer
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6C7086"))
	if state.diffFocused {
		b.WriteString(helpStyle.Render("[↑/↓/j/k] Scroll diff  [esc/q] Exit focus"))
	} else {
		b.WriteString(helpStyle.Render("[↑/↓/j/k] Navigate  [space] Toggle  [a] All  [enter] Focus diff  [y] Apply  [q] Back"))
	}

	return b.String()
}
