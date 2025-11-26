package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// createView renders the create worktree wizard
func (m Model) createView() string {
	if m.createState == nil {
		return "Loading..."
	}

	switch m.createState.currentStep {
	case CreateStepBranchMode:
		return m.renderBranchModeStep()
	case CreateStepBranchName:
		return m.renderBranchNameStep()
	case CreateStepExistingBranch:
		return m.renderExistingBranchStep()
	case CreateStepBaseBranch:
		return m.renderBaseBranchStep()
	case CreateStepConfirm:
		return m.renderConfirmStep()
	case CreateStepCreating:
		return m.renderCreatingStep()
	case CreateStepComplete:
		return m.renderCreateCompleteStep()
	default:
		return "Unknown step"
	}
}

// renderBranchModeStep shows branch mode selection
func (m Model) renderBranchModeStep() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("🌱 Create New Worktree"))
	content.WriteString("\n\n")

	// Show error if any
	if m.err != nil {
		content.WriteString(ErrorStyle.Render(fmt.Sprintf("❌ %s", m.err.Error())))
		content.WriteString("\n\n")
	}

	content.WriteString(WorktreeNameStyle.Render("Choose branch type:"))
	content.WriteString("\n\n")

	// Mode options
	modes := []struct {
		name string
		icon string
	}{
		{"Create new branch", "🌿"},
		{"Use existing branch", "🔄"},
	}

	for i, mode := range modes {
		prefix := "  "
		if i == m.createState.selectedMode {
			prefix = "▶ "
		}

		modeLine := fmt.Sprintf("%s%s %s", prefix, mode.icon, mode.name)

		// Apply color styling for selected item
		if i == m.createState.selectedMode {
			content.WriteString(WorktreeNameStyle.Foreground(PrimaryColor).Render(modeLine))
		} else {
			content.WriteString(modeLine)
		}
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(HelpStyle.Render("[↑↓] Navigate  [enter] Select  [esc] Cancel"))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}

// renderBranchNameStep shows branch name input
func (m Model) renderBranchNameStep() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("🌱 Create New Worktree"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("Enter branch name:"))
	content.WriteString("\n\n")

	// Branch name input
	inputStyle := WorktreeItemStyle
	if m.createState.branchName == "" {
		inputStyle = WorktreeSelectedStyle
	}

	branchInput := fmt.Sprintf("🌿 %s▮", m.createState.branchName)
	content.WriteString(inputStyle.Width(m.width - 8).Render(branchInput))
	content.WriteString("\n\n")

	// Validation hints
	if m.createState.branchName != "" {
		if isValidBranchName(m.createState.branchName) {
			content.WriteString(StatusCleanStyle.Render("✅ Valid branch name"))
		} else {
			content.WriteString(ErrorStyle.Render("❌ Invalid branch name"))
			content.WriteString("\n")
			content.WriteString(WorktreePathStyle.Render("Use only letters, numbers, dashes, and slashes"))
		}
		content.WriteString("\n\n")
	}

	content.WriteString(WorktreePathStyle.Render("Examples: feature/auth, hotfix/bug-123, experiments/new-ui"))
	content.WriteString("\n\n")

	content.WriteString(HelpStyle.Render("[type] Enter name  [enter] Continue  [esc] Cancel"))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}

// renderExistingBranchStep shows existing branch selection
func (m Model) renderExistingBranchStep() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("🔄 Select Existing Branch"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("Select branch to checkout:"))
	content.WriteString("\n\n")

	// Show search input
	if m.createState.isSearching {
		content.WriteString(WorktreeNameStyle.Foreground(PrimaryColor).Render(fmt.Sprintf("🔍 %s▮", m.createState.searchQuery)))
		content.WriteString("\n\n")
	} else if m.createState.searchQuery != "" {
		// Show active filter (search mode exited but filter still active)
		content.WriteString(HelpStyle.Render(fmt.Sprintf("🔍 Filtered: %s", m.createState.searchQuery)))
		content.WriteString("\n\n")
	}

	// Use filtered branches if available
	branches := m.createState.filteredAvailableBranches
	if len(branches) == 0 && m.createState.searchQuery == "" {
		branches = m.createState.availableBranches
	}

	if len(branches) == 0 && m.createState.searchQuery == "" {
		content.WriteString(WorktreePathStyle.Render("No available branches found."))
		content.WriteString("\n")
		content.WriteString(WorktreePathStyle.Render("All branches may already have worktrees."))
		content.WriteString("\n\n")
		content.WriteString(HelpStyle.Render("[esc] Back"))
		return HeaderStyle.Width(m.width - 4).Render(content.String())
	}

	// Show "no matches" if search returned nothing
	if len(branches) == 0 && m.createState.searchQuery != "" {
		content.WriteString(WorktreePathStyle.Render("No branches match your search."))
		content.WriteString("\n\n")
		content.WriteString(HelpStyle.Render("[esc] Cancel search  [backspace] Edit"))
		return HeaderStyle.Width(m.width - 4).Render(content.String())
	}

	// Calculate visible window size
	maxVisible := m.height - 17
	if maxVisible < 5 {
		maxVisible = 5
	}
	if maxVisible > 20 {
		maxVisible = 20
	}

	totalBranches := len(branches)
	scrollOffset := m.createState.scrollOffset

	// Ensure scroll offset is valid
	if scrollOffset > totalBranches-maxVisible {
		scrollOffset = totalBranches - maxVisible
	}
	if scrollOffset < 0 {
		scrollOffset = 0
	}

	// Show scroll indicator at top if needed
	if scrollOffset > 0 {
		content.WriteString(HelpStyle.Render(fmt.Sprintf("  ↑ %d more above", scrollOffset)))
		content.WriteString("\n")
	}

	// Branch list with status indicators (simple list style)
	endIndex := scrollOffset + maxVisible
	if endIndex > totalBranches {
		endIndex = totalBranches
	}

	for i := scrollOffset; i < endIndex; i++ {
		status := branches[i]
		// Status indicator
		statusIcon := "🟢"
		statusText := ""
		if !status.IsClean {
			statusIcon = "⚠️"
			statusText = fmt.Sprintf(" (%d uncommitted, %d untracked)",
				status.UncommittedFiles, status.UntrackedFiles)
		}

		// Current branch indicator
		currentIndicator := ""
		if status.IsCurrent {
			currentIndicator = " (current)"
		}

		// Ahead/behind indicator
		aheadBehind := ""
		if status.AheadCount > 0 {
			aheadBehind += fmt.Sprintf(" ↑%d", status.AheadCount)
		}
		if status.BehindCount > 0 {
			aheadBehind += fmt.Sprintf(" ↓%d", status.BehindCount)
		}

		// Selection prefix
		prefix := "  "
		if i == m.createState.selectedBranch {
			prefix = "▶ "
		}

		branchLine := fmt.Sprintf("%s%s %s%s%s%s",
			prefix, statusIcon, status.Name, currentIndicator, statusText, aheadBehind)

		// Apply color styling for selected item
		if i == m.createState.selectedBranch {
			content.WriteString(WorktreeNameStyle.Foreground(PrimaryColor).Render(branchLine))
		} else {
			content.WriteString(branchLine)
		}
		content.WriteString("\n")
	}

	// Show scroll indicator at bottom if needed
	remaining := totalBranches - endIndex
	if remaining > 0 {
		content.WriteString(HelpStyle.Render(fmt.Sprintf("  ↓ %d more below", remaining)))
		content.WriteString("\n")
	}

	content.WriteString("\n")
	if m.createState.isSearching {
		content.WriteString(HelpStyle.Render("[enter] Select  [esc] Cancel search"))
	} else {
		content.WriteString(HelpStyle.Render("[enter] Continue  [/] Search  [↑↓] Select  [esc] Back"))
	}

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}

// renderBaseBranchStep shows base branch selection with warnings
func (m Model) renderBaseBranchStep() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("🌳 Select Base Branch"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render(fmt.Sprintf("Create '%s' from:", m.createState.branchName)))
	content.WriteString("\n\n")

	// Show search input
	if m.createState.isSearching {
		content.WriteString(WorktreeNameStyle.Foreground(PrimaryColor).Render(fmt.Sprintf("🔍 %s▮", m.createState.searchQuery)))
		content.WriteString("\n\n")
	} else if m.createState.searchQuery != "" {
		// Show active filter (search mode exited but filter still active)
		content.WriteString(HelpStyle.Render(fmt.Sprintf("🔍 Filtered: %s", m.createState.searchQuery)))
		content.WriteString("\n\n")
	}

	// Use filtered branches if search is active, otherwise use all branches
	branches := m.createState.filteredBranches
	if len(branches) == 0 && m.createState.searchQuery == "" {
		branches = m.createState.branchStatuses
	}

	// Show "no matches" if search returned nothing
	if len(branches) == 0 && m.createState.searchQuery != "" {
		content.WriteString(WorktreePathStyle.Render("No branches match your search."))
		content.WriteString("\n\n")
		content.WriteString(HelpStyle.Render("[esc] Cancel search  [backspace] Edit"))
		return HeaderStyle.Width(m.width - 4).Render(content.String())
	}

	// Calculate visible window size (leave room for header, footer, warnings)
	maxVisible := m.height - 17 // Adjusted for search input
	if maxVisible < 5 {
		maxVisible = 5
	}
	if maxVisible > 20 {
		maxVisible = 20
	}

	totalBranches := len(branches)
	scrollOffset := m.createState.scrollOffset

	// Ensure scroll offset is valid
	if scrollOffset > totalBranches-maxVisible {
		scrollOffset = totalBranches - maxVisible
	}
	if scrollOffset < 0 {
		scrollOffset = 0
	}

	// Show scroll indicator at top if needed
	if scrollOffset > 0 {
		content.WriteString(HelpStyle.Render(fmt.Sprintf("  ↑ %d more above", scrollOffset)))
		content.WriteString("\n")
	}

	// Branch list with status indicators (simple list style)
	endIndex := scrollOffset + maxVisible
	if endIndex > totalBranches {
		endIndex = totalBranches
	}

	for i := scrollOffset; i < endIndex; i++ {
		status := branches[i]
		// Status indicator
		statusIcon := "🟢"
		statusText := ""
		if !status.IsClean {
			statusIcon = "⚠️"
			statusText = fmt.Sprintf(" (%d uncommitted, %d untracked)",
				status.UncommittedFiles, status.UntrackedFiles)
		}

		// Current branch indicator
		currentIndicator := ""
		if status.IsCurrent {
			currentIndicator = " (current)"
		}

		// Ahead/behind indicator
		aheadBehind := ""
		if status.AheadCount > 0 {
			aheadBehind += fmt.Sprintf(" ↑%d", status.AheadCount)
		}
		if status.BehindCount > 0 {
			aheadBehind += fmt.Sprintf(" ↓%d", status.BehindCount)
		}

		// Selection prefix
		prefix := "  "
		if i == m.createState.selectedBranch {
			prefix = "▶ "
		}

		branchLine := fmt.Sprintf("%s%s %s%s%s%s",
			prefix, statusIcon, status.Name, currentIndicator, statusText, aheadBehind)

		// Apply color styling for selected item
		if i == m.createState.selectedBranch {
			content.WriteString(WorktreeNameStyle.Foreground(PrimaryColor).Render(branchLine))
		} else {
			content.WriteString(branchLine)
		}
		content.WriteString("\n")
	}

	// Show scroll indicator at bottom if needed
	remaining := totalBranches - endIndex
	if remaining > 0 {
		content.WriteString(HelpStyle.Render(fmt.Sprintf("  ↓ %d more below", remaining)))
		content.WriteString("\n")
	}

	// Warning for dirty branches
	if len(branches) > 0 && m.createState.selectedBranch < len(branches) {
		selectedStatus := branches[m.createState.selectedBranch]
		if !selectedStatus.IsClean {
			content.WriteString("\n")

			warningStyle := ErrorStyle.Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#f59e0b")).
				Padding(1, 2)

			warningText := fmt.Sprintf("⚠️  Warning: '%s' has uncommitted changes\n"+
				"   Worktree will be based on last commit only\n"+
				"   Local changes stay in current repository", selectedStatus.Name)

			content.WriteString(warningStyle.Render(warningText))
			content.WriteString("\n\n")

			if m.createState.isSearching {
				content.WriteString(HelpStyle.Render("[enter] Select  [esc] Cancel search"))
			} else if !m.createState.warningAccepted {
				content.WriteString(HelpStyle.Render("[y] Accept warning  [/] Search  [↑↓] Navigate  [esc] Back"))
			} else {
				content.WriteString(HelpStyle.Render("[enter] Continue  [/] Search  [↑↓] Navigate  [esc] Back"))
			}
		} else {
			content.WriteString("\n")
			if m.createState.isSearching {
				content.WriteString(HelpStyle.Render("[enter] Select  [esc] Cancel search"))
			} else {
				content.WriteString(HelpStyle.Render("[enter] Continue  [/] Search  [↑↓] Navigate  [esc] Back"))
			}
		}
	}

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}

// renderConfirmStep shows final confirmation
func (m Model) renderConfirmStep() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("✅ Confirm Worktree Creation"))
	content.WriteString("\n\n")

	// Summary - different format for new vs existing branch
	sanitizedName := sanitizeBranchForPath(m.createState.branchName)
	worktreePath := m.getWorktreePath(m.createState.branchName)
	var summary string
	if m.createState.createMode == CreateModeNewBranch {
		summary = fmt.Sprintf("📁 Worktree: %s\n"+
			"🌿 New Branch: %s\n"+
			"🔗 Based on: %s\n"+
			"📍 Path: %s/",
			sanitizedName,
			m.createState.branchName,
			m.createState.baseBranch,
			worktreePath)
	} else {
		summary = fmt.Sprintf("📁 Worktree: %s\n"+
			"🔄 Existing Branch: %s\n"+
			"📍 Path: %s/",
			sanitizedName,
			m.createState.branchName,
			worktreePath)
	}

	content.WriteString(WorktreeItemStyle.Render(summary))
	content.WriteString("\n\n")

	// Post-create hook info
	content.WriteString(WorktreeNameStyle.Render("After creation:"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("📜 .gren/post-create.sh will run"))
	content.WriteString("\n\n")

	content.WriteString(HelpStyle.Render("[enter] Create Worktree  [esc] Back"))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}

// renderCreatingStep shows progress
func (m Model) renderCreatingStep() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("🔄 Creating Worktree"))
	content.WriteString("\n\n")

	content.WriteString(SpinnerStyle.Render("⠋ Creating git worktree..."))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("⏳ Running post-create hook..."))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("⏸️ Installing dependencies..."))
	content.WriteString("\n\n")

	content.WriteString(WorktreePathStyle.Render(fmt.Sprintf("📍 Path: %s/", m.getWorktreePath(m.createState.branchName))))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}

// renderCreateCompleteStep shows completion
func (m Model) renderCreateCompleteStep() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("🎉 Worktree Created!"))
	content.WriteString("\n\n")

	content.WriteString(StatusCleanStyle.Render(fmt.Sprintf("✅ Worktree '%s' ready", m.createState.branchName)))
	content.WriteString("\n\n")

	// Show worktree path
	worktreePath := m.getWorktreePath(m.createState.branchName)
	content.WriteString(WorktreePathStyle.Render(fmt.Sprintf("📍 Path: %s", worktreePath)))
	content.WriteString("\n\n")

	// Show message if no editors are available
	actions := m.getAvailableActions()
	if len(actions) == 1 { // Only "Return to dashboard" is available
		content.WriteString(WorktreePathStyle.Render("💡 No editors detected in PATH"))
		content.WriteString("\n")
		content.WriteString(WorktreePathStyle.Render("   You can manually navigate to the worktree directory"))
		content.WriteString("\n\n")
	}

	// Show available actions as simple list
	content.WriteString(WorktreeNameStyle.Render("What would you like to do next?"))
	content.WriteString("\n\n")

	for i, action := range actions {
		prefix := "  "
		if i == m.createState.selectedAction {
			prefix = "▶ "
			// Just change text color, no border/box
			content.WriteString(WorktreeNameStyle.Foreground(PrimaryColor).Render(fmt.Sprintf("%s%s %s", prefix, action.Icon, action.Name)))
		} else {
			content.WriteString(fmt.Sprintf("%s%s %s", prefix, action.Icon, action.Name))
		}
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(HelpStyle.Render("↑↓ Navigate • Enter Select • Esc Back"))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}
