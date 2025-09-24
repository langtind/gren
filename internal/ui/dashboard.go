package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// dashboardView renders the main dashboard
func (m Model) dashboardView() string {
	var content strings.Builder

	// Header
	header := TitleStyle.Render("🌿 gren - Git Worktree Manager")

	// Project name with status
	var projectStatus string
	if m.err != nil {
		projectStatus = "(error loading repository info)"
	} else if m.repoInfo == nil {
		projectStatus = "(loading...)"
	} else if !m.repoInfo.IsGitRepo {
		projectStatus = "(not a git repository)"
	} else {
		projectStatus = m.repoInfo.Name
		if m.repoInfo.CurrentBranch != "" {
			projectStatus += " (on " + m.repoInfo.CurrentBranch + ")"
		}
	}
	subtitle := SubtitleStyle.Render(fmt.Sprintf("Project: %s", projectStatus))

	content.WriteString(header)
	content.WriteString("\n")
	content.WriteString(subtitle)
	content.WriteString("\n\n")

	// Handle error state
	if m.err != nil {
		message := lipgloss.JoinVertical(
			lipgloss.Center,
			ErrorStyle.Render("❌ Error"),
			"",
			WorktreePathStyle.Render(m.err.Error()),
		)
		content.WriteString(message)
		content.WriteString("\n\n")
		content.WriteString(HelpStyle.Render("[q] Quit"))
		return HeaderStyle.Width(m.width - 4).Render(content.String())
	}

	// Handle loading state
	if m.repoInfo == nil {
		message := lipgloss.JoinVertical(
			lipgloss.Center,
			WorktreeNameStyle.Render("Loading repository information..."),
		)
		content.WriteString(message)
		content.WriteString("\n\n")
		content.WriteString(HelpStyle.Render("[q] Quit"))
		return HeaderStyle.Width(m.width - 4).Render(content.String())
	}

	// Check initialization status first, then worktrees
	if !m.repoInfo.IsGitRepo {
		// Not in a git repo
		message := lipgloss.JoinVertical(
			lipgloss.Center,
			ErrorStyle.Render("❌ Not in a git repository"),
			"",
			WorktreePathStyle.Render("Please run gren from within a git repository."),
		)
		content.WriteString(message)
		content.WriteString("\n\n")
		content.WriteString(HelpStyle.Render("[q] Quit"))
	} else if !m.repoInfo.IsInitialized {
		// In a git repo but not initialized - always show init message
		message := lipgloss.JoinVertical(
			lipgloss.Center,
			WorktreeNameStyle.Render("Worktree management not initialized."),
			"",
			WorktreePathStyle.Render("Press 'i' to initialize worktree management for this project."),
			WorktreePathStyle.Render("This will create a .gren/ configuration directory."),
		)
		content.WriteString(message)
		content.WriteString("\n\n")
		content.WriteString(HelpStyle.Render("[i] Initialize  [q] Quit"))
	} else if len(m.worktrees) == 0 {
		// Initialized but no worktrees yet
		message := lipgloss.JoinVertical(
			lipgloss.Center,
			WorktreeNameStyle.Render("No worktrees created yet."),
			"",
			WorktreePathStyle.Render("Press 'n' to create your first worktree."),
		)
		content.WriteString(message)
		content.WriteString("\n\n")
		content.WriteString(HelpStyle.Render("[n] New worktree  [c] Config  [q] Quit"))
	} else {
		// Show worktrees list
		for i, wt := range m.worktrees {
			var item strings.Builder

			// Icon based on worktree type
			icon := "🌿"
			if wt.Name == "main" || wt.Name == "master" {
				icon = "📁"
			}

			// Current worktree indicator
			currentIndicator := ""
			if wt.IsCurrent {
				currentIndicator = " (current)"
			}

			// Status indicator with proper detection
			var statusStyle lipgloss.Style
			var statusText string
			switch wt.Status {
			case "clean":
				statusStyle = StatusCleanStyle
				statusText = "🟢 Clean"
			case "modified":
				statusStyle = StatusModifiedStyle
				statusText = "🟡 Modified"
			case "untracked":
				statusStyle = StatusModifiedStyle // Use same style for now
				statusText = "🔴 Untracked"
			case "mixed":
				statusStyle = StatusModifiedStyle
				statusText = "📝 Changes"
			default:
				statusStyle = StatusCleanStyle
				statusText = "🟢 Clean"
			}

			// Build the worktree item content
			nameAndStatus := lipgloss.JoinHorizontal(
				lipgloss.Top,
				WorktreeNameStyle.Render(fmt.Sprintf("%s %s%s", icon, wt.Name, currentIndicator)),
				lipgloss.NewStyle().Render(strings.Repeat(" ", 20)), // Spacer
				statusStyle.Render(statusText),
			)

			pathInfo := WorktreePathStyle.Render(fmt.Sprintf("    %s", wt.Path))
			branchInfo := WorktreeBranchStyle.Render(fmt.Sprintf("    │ Branch: %s", wt.Branch))

			// Combine all parts (removed the confusing extra info)
			itemContent := lipgloss.JoinVertical(
				lipgloss.Left,
				nameAndStatus,
				pathInfo,
				branchInfo,
			)

			// Apply selection styling with consistent width
			var itemStyle lipgloss.Style
			if i == m.selected {
				itemStyle = WorktreeSelectedStyle.Width(m.width - 8)
			} else {
				itemStyle = WorktreeItemStyle.Width(m.width - 8)
			}

			item.WriteString(itemStyle.Render(itemContent))
			item.WriteString("\n")

			content.WriteString(item.String())
		}

		// Help text for worktrees view
		helpText := HelpStyle.Render("[n] New  [d] Delete  [c] Config  [↑↓] Navigate  [enter] Open in...  [q] Quit")
		content.WriteString("\n")
		content.WriteString(helpText)
	}

	// Wrap everything in a border
	return HeaderStyle.Width(m.width - 4).Render(content.String())
}

// Utility function to get status icon and color
func getStatusDisplay(status string) (string, lipgloss.Style) {
	switch status {
	case "clean":
		return "🟢 Clean", StatusCleanStyle
	case "modified":
		return "⚡ Modified", StatusModifiedStyle
	case "building":
		return "🔄 Building", StatusBuildingStyle
	default:
		return "🟢 Clean", StatusCleanStyle
	}
}