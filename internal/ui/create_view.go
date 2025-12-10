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

// ═══════════════════════════════════════════════════════════════════════════
// Step 1: Branch Mode Selection
// ═══════════════════════════════════════════════════════════════════════════

func (m Model) renderBranchModeStep() string {
	// Build header
	header := m.renderWizardHeader("New Worktree")

	// Build content
	var content strings.Builder

	// Error display
	if m.err != nil {
		content.WriteString(ErrorStyle.Render("Error: " + m.err.Error()))
		content.WriteString("\n\n")
	}

	// Subtitle
	content.WriteString(WizardSubtitleStyle.Render("Choose branch type"))
	content.WriteString("\n\n")

	// Mode options
	modes := []struct {
		name string
		desc string
	}{
		{"Create new branch", "Start fresh from an existing branch"},
		{"Use existing branch", "Checkout a branch that already exists"},
	}

	for i, mode := range modes {
		content.WriteString(WizardOption(mode.name, i == m.createState.selectedMode))
		content.WriteString("\n")
		if i == m.createState.selectedMode {
			content.WriteString(WizardDescStyle.Render("   " + mode.desc))
			content.WriteString("\n")
		}
	}

	// Build footer
	footer := m.renderWizardFooter("↑↓", "select", "enter", "confirm", "esc", "cancel")

	// Calculate content height to fill available space
	contentHeight := m.height - 4 - FooterHeight // header lines + footer
	if contentHeight < 5 {
		contentHeight = 5
	}

	// Style content to fill space
	contentStyled := lipgloss.NewStyle().
		Width(m.width-4).
		Height(contentHeight).
		Padding(1, 2).
		Render(content.String())

	// Combine with vertical join
	return lipgloss.JoinVertical(lipgloss.Left, header, contentStyled, footer)
}

// ═══════════════════════════════════════════════════════════════════════════
// Step 2: Branch Name Input
// ═══════════════════════════════════════════════════════════════════════════

func (m Model) renderBranchNameStep() string {
	// Build header
	header := m.renderWizardHeader("New Worktree")

	// Build content
	var content strings.Builder

	content.WriteString(WizardSubtitleStyle.Render("Enter branch name"))
	content.WriteString("\n\n")

	// Input field
	cursor := "▮"
	inputContent := m.createState.branchName + cursor
	content.WriteString(WizardInputStyle.Width(40).Render(inputContent))
	content.WriteString("\n\n")

	// Validation
	if m.createState.branchName != "" {
		if isValidBranchName(m.createState.branchName) {
			content.WriteString(WizardSuccessStyle.Render("✓ Valid branch name"))
		} else {
			content.WriteString(ErrorStyle.Render("✗ Invalid branch name"))
			content.WriteString("\n")
			content.WriteString(WizardDescStyle.Render("  Use letters, numbers, dashes, underscores, slashes"))
		}
		content.WriteString("\n\n")
	}

	// Examples
	content.WriteString(WizardDescStyle.Render("Examples: feature/auth, hotfix/bug-123, experiment/new-ui"))

	// Build footer
	footer := m.renderWizardFooter("type", "name", "enter", "continue", "esc", "back")

	// Calculate content height
	contentHeight := m.height - 4 - FooterHeight
	if contentHeight < 5 {
		contentHeight = 5
	}

	// Style content
	contentStyled := lipgloss.NewStyle().
		Width(m.width-4).
		Height(contentHeight).
		Padding(1, 2).
		Render(content.String())

	return lipgloss.JoinVertical(lipgloss.Left, header, contentStyled, footer)
}

// ═══════════════════════════════════════════════════════════════════════════
// Step 3: Existing Branch Selection
// ═══════════════════════════════════════════════════════════════════════════

func (m Model) renderExistingBranchStep() string {
	// Build header
	header := m.renderWizardHeader("Select Branch")

	// Get branches to display
	branches := m.createState.filteredAvailableBranches
	if len(branches) == 0 && m.createState.searchQuery == "" {
		branches = m.createState.availableBranches
	}

	// Build content
	var content strings.Builder

	// Search input
	hasSearch := m.createState.isSearching || m.createState.searchQuery != ""
	if hasSearch {
		searchStyle := WizardInputStyle.Width(30)
		if m.createState.isSearching {
			content.WriteString(searchStyle.Render("/ " + m.createState.searchQuery + "▮"))
		} else {
			content.WriteString(WizardDescStyle.Render("Filter: " + m.createState.searchQuery))
		}
		content.WriteString("\n\n")
	}

	// Build footer based on state
	var footer string
	if len(branches) == 0 {
		if m.createState.searchQuery != "" {
			content.WriteString(WizardDescStyle.Render("No branches match your search"))
		} else {
			content.WriteString(WizardDescStyle.Render("No available branches found"))
			content.WriteString("\n")
			content.WriteString(WizardDescStyle.Render("All branches may already have worktrees"))
		}
		footer = m.renderWizardFooter("esc", "back")
	} else {
		// Calculate dynamic maxVisible - use all available space
		headerLines := 3 // header + padding
		searchLines := 0
		if hasSearch {
			searchLines = 3
		}
		footerLines := FooterHeight + 1

		maxVisible := m.height - headerLines - searchLines - footerLines
		if maxVisible < 5 {
			maxVisible = 5
		}
		// No upper limit - fill the screen

		// Branch list
		content.WriteString(m.renderBranchList(branches, maxVisible, false))

		// Footer based on search state
		if m.createState.isSearching {
			footer = m.renderWizardFooter("type", "filter", "enter", "select", "esc", "cancel")
		} else {
			footer = m.renderWizardFooter("↑↓", "select", "/", "search", "enter", "confirm", "esc", "back")
		}
	}

	// Calculate content height
	contentHeight := m.height - 3 - FooterHeight
	if contentHeight < 5 {
		contentHeight = 5
	}

	// Style content
	contentStyled := lipgloss.NewStyle().
		Width(m.width-4).
		Height(contentHeight).
		Padding(0, 2).
		Render(content.String())

	return lipgloss.JoinVertical(lipgloss.Left, header, contentStyled, footer)
}

// ═══════════════════════════════════════════════════════════════════════════
// Step 4: Base Branch Selection
// ═══════════════════════════════════════════════════════════════════════════

func (m Model) renderBaseBranchStep() string {
	// Build header
	header := m.renderWizardHeader("Select Base Branch")

	// Get branches
	branches := m.createState.filteredBranches
	if len(branches) == 0 && m.createState.searchQuery == "" {
		branches = m.createState.branchStatuses
	}

	// Build content
	var content strings.Builder

	content.WriteString(WizardSubtitleStyle.Render(fmt.Sprintf("Create '%s' from:", m.createState.branchName)))
	content.WriteString("\n\n")

	// Search input
	hasSearch := m.createState.isSearching || m.createState.searchQuery != ""
	if hasSearch {
		searchStyle := WizardInputStyle.Width(30)
		if m.createState.isSearching {
			content.WriteString(searchStyle.Render("/ " + m.createState.searchQuery + "▮"))
		} else {
			content.WriteString(WizardDescStyle.Render("Filter: " + m.createState.searchQuery))
		}
		content.WriteString("\n\n")
	}

	// Build footer based on state
	var footer string
	if len(branches) == 0 && m.createState.searchQuery != "" {
		content.WriteString(WizardDescStyle.Render("No branches match your search"))
		footer = m.renderWizardFooter("esc", "cancel", "backspace", "edit")
	} else {
		// Calculate dynamic maxVisible
		headerLines := 5 // header + subtitle + padding
		searchLines := 0
		if hasSearch {
			searchLines = 3
		}
		warningLines := 0
		if len(branches) > 0 && m.createState.selectedBranch < len(branches) && !branches[m.createState.selectedBranch].IsClean {
			warningLines = 4
		}
		footerLines := FooterHeight + 1

		maxVisible := m.height - headerLines - searchLines - warningLines - footerLines
		if maxVisible < 5 {
			maxVisible = 5
		}
		// No upper limit

		// Branch list
		content.WriteString(m.renderBranchList(branches, maxVisible, true))

		// Warning for dirty branch
		if len(branches) > 0 && m.createState.selectedBranch < len(branches) {
			selectedBranch := branches[m.createState.selectedBranch]
			if !selectedBranch.IsClean {
				content.WriteString("\n")
				warning := fmt.Sprintf("⚠ '%s' has uncommitted changes\n  Worktree will be based on last commit only", selectedBranch.Name)
				content.WriteString(WizardWarningStyle.Render(warning))
			}
		}

		// Footer based on state
		if m.createState.isSearching {
			footer = m.renderWizardFooter("type", "filter", "enter", "select", "esc", "cancel")
		} else {
			needsWarningAccept := len(branches) > 0 &&
				m.createState.selectedBranch < len(branches) &&
				!branches[m.createState.selectedBranch].IsClean &&
				!m.createState.warningAccepted

			if needsWarningAccept {
				footer = m.renderWizardFooter("↑↓", "select", "y", "accept", "/", "search", "esc", "back")
			} else {
				footer = m.renderWizardFooter("↑↓", "select", "/", "search", "enter", "confirm", "esc", "back")
			}
		}
	}

	// Calculate content height
	contentHeight := m.height - 3 - FooterHeight
	if contentHeight < 5 {
		contentHeight = 5
	}

	// Style content
	contentStyled := lipgloss.NewStyle().
		Width(m.width-4).
		Height(contentHeight).
		Padding(0, 2).
		Render(content.String())

	return lipgloss.JoinVertical(lipgloss.Left, header, contentStyled, footer)
}

// ═══════════════════════════════════════════════════════════════════════════
// Step 5: Confirmation
// ═══════════════════════════════════════════════════════════════════════════

func (m Model) renderConfirmStep() string {
	// Build header
	header := m.renderWizardHeader("Confirm Creation")

	// Build content
	var content strings.Builder

	// Summary box
	sanitizedName := sanitizeBranchForPath(m.createState.branchName)
	worktreePath := m.getWorktreePath(m.createState.branchName)

	summaryStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(0, 1)

	var summary strings.Builder
	summary.WriteString(WizardSubtitleStyle.Render("Worktree: ") + sanitizedName + "\n")
	if m.createState.createMode == CreateModeNewBranch {
		summary.WriteString(WizardSubtitleStyle.Render("Branch:   ") + WorktreeBranchStyle.Render(m.createState.branchName) + "\n")
		summary.WriteString(WizardSubtitleStyle.Render("Based on: ") + m.createState.baseBranch + "\n")
	} else {
		summary.WriteString(WizardSubtitleStyle.Render("Branch:   ") + WorktreeBranchStyle.Render(m.createState.branchName) + "\n")
	}
	summary.WriteString(WizardSubtitleStyle.Render("Path:     ") + WizardDescStyle.Render(worktreePath))

	content.WriteString(summaryStyle.Render(summary.String()))
	content.WriteString("\n\n")

	// Post-create info
	content.WriteString(WizardDescStyle.Render("After creation, .gren/post-create.sh will run"))

	// Build footer
	footer := m.renderWizardFooter("enter", "create", "esc", "back")

	// Calculate content height
	contentHeight := m.height - 4 - FooterHeight
	if contentHeight < 5 {
		contentHeight = 5
	}

	// Style content
	contentStyled := lipgloss.NewStyle().
		Width(m.width-4).
		Height(contentHeight).
		Padding(1, 2).
		Render(content.String())

	return lipgloss.JoinVertical(lipgloss.Left, header, contentStyled, footer)
}

// ═══════════════════════════════════════════════════════════════════════════
// Step 6: Creating (Progress)
// ═══════════════════════════════════════════════════════════════════════════

func (m Model) renderCreatingStep() string {
	// Build header (no step indicator for progress view)
	header := WizardHeader("Creating Worktree")

	// Build content
	var content strings.Builder

	// Use animated spinner
	spinnerView := m.createState.spinner.View()
	content.WriteString(spinnerView + " Creating worktree and running post-create hook...")
	content.WriteString("\n\n")

	content.WriteString(WizardDescStyle.Render("Branch: " + m.createState.branchName))
	content.WriteString("\n")
	content.WriteString(WizardDescStyle.Render("Path: " + m.getWorktreePath(m.createState.branchName)))

	// Calculate content height
	contentHeight := m.height - 4 - FooterHeight
	if contentHeight < 5 {
		contentHeight = 5
	}

	// Style content
	contentStyled := lipgloss.NewStyle().
		Width(m.width-4).
		Height(contentHeight).
		Padding(1, 2).
		Render(content.String())

	// Empty footer for progress view
	footer := FooterBarStyle.Width(m.width - 2).Render("")

	return lipgloss.JoinVertical(lipgloss.Left, header, contentStyled, footer)
}

// ═══════════════════════════════════════════════════════════════════════════
// Step 7: Complete
// ═══════════════════════════════════════════════════════════════════════════

func (m Model) renderCreateCompleteStep() string {
	// Build header (no step indicator for completion view)
	header := WizardHeader("Worktree Created")

	// Build content
	var content strings.Builder

	content.WriteString(WizardSuccessStyle.Render("✓ " + m.createState.branchName))
	content.WriteString("\n\n")

	// Path
	worktreePath := m.getWorktreePath(m.createState.branchName)
	content.WriteString(WizardDescStyle.Render("Path: " + worktreePath))
	content.WriteString("\n\n")

	// Actions
	content.WriteString(WizardSubtitleStyle.Render("Open in:"))
	content.WriteString("\n\n")

	actions := m.getAvailableActions()
	if len(actions) == 1 {
		content.WriteString(WizardDescStyle.Render("No editors detected in PATH"))
		content.WriteString("\n\n")
	}

	for i, action := range actions {
		label := action.Icon + " " + action.Name
		content.WriteString(WizardOption(label, i == m.createState.selectedAction))
		content.WriteString("\n")
	}

	// Build footer
	footer := m.renderWizardFooter("↑↓", "select", "enter", "open", "esc", "dashboard")

	// Calculate content height
	contentHeight := m.height - 4 - FooterHeight
	if contentHeight < 5 {
		contentHeight = 5
	}

	// Style content
	contentStyled := lipgloss.NewStyle().
		Width(m.width-4).
		Height(contentHeight).
		Padding(1, 2).
		Render(content.String())

	return lipgloss.JoinVertical(lipgloss.Left, header, contentStyled, footer)
}

// ═══════════════════════════════════════════════════════════════════════════
// Helper Functions
// ═══════════════════════════════════════════════════════════════════════════

// wrapWizardContent wraps wizard content in a consistent container
func (m Model) wrapWizardContent(content string) string {
	// Calculate container dimensions
	width := m.width - 4
	if width > 80 {
		width = 80
	}

	containerStyle := lipgloss.NewStyle().
		Width(width).
		Padding(1, 2)

	return containerStyle.Render(content)
}

// getWizardStepInfo returns current step and total steps for the wizard
func (m Model) getWizardStepInfo() (current int, total int) {
	if m.createState == nil {
		return 0, 0
	}

	isNewBranch := m.createState.createMode == CreateModeNewBranch

	switch m.createState.currentStep {
	case CreateStepBranchMode:
		// At step 1, we don't know the flow yet, so show 1 of ?
		// Actually, we show based on selected mode
		if m.createState.selectedMode == 0 { // New branch selected
			return 1, 4
		}
		return 1, 3
	case CreateStepBranchName:
		return 2, 4 // Only in new branch flow
	case CreateStepExistingBranch:
		return 2, 3 // Only in existing branch flow
	case CreateStepBaseBranch:
		return 3, 4 // Only in new branch flow
	case CreateStepConfirm:
		if isNewBranch {
			return 4, 4
		}
		return 3, 3
	default:
		return 0, 0 // Creating/Complete don't show step indicator
	}
}

// renderStepIndicator renders the dot-style step indicator
func (m Model) renderStepIndicator() string {
	current, total := m.getWizardStepInfo()
	if current == 0 || total == 0 {
		return ""
	}

	var dots strings.Builder
	for i := 1; i <= total; i++ {
		if i == current {
			dots.WriteString(WizardListItemSelectedStyle.Render("●"))
		} else if i < current {
			dots.WriteString(StatusCleanStyle.Render("●")) // Completed steps in green
		} else {
			dots.WriteString(WizardDescStyle.Render("○"))
		}
		if i < total {
			dots.WriteString(" ")
		}
	}
	return dots.String()
}

// renderWizardFooter renders a footer bar in dashboard style
func (m Model) renderWizardFooter(items ...string) string {
	sep := HelpSeparatorStyle.Render(" │ ")
	var helpItems []string
	for i := 0; i+1 < len(items); i += 2 {
		helpItems = append(helpItems, HelpItem(items[i], items[i+1]))
	}
	return FooterBarStyle.Width(m.width - 2).Render(strings.Join(helpItems, sep))
}

// renderWizardHeader renders the header with title and step indicator
func (m Model) renderWizardHeader(title string) string {
	titleText := WizardHeader(title)
	stepIndicator := m.renderStepIndicator()

	if stepIndicator == "" {
		return titleText
	}

	// Calculate padding to right-align step indicator
	headerWidth := m.width - 6 // Account for padding
	titleWidth := lipgloss.Width(titleText)
	indicatorWidth := lipgloss.Width(stepIndicator)
	padding := headerWidth - titleWidth - indicatorWidth
	if padding < 1 {
		padding = 1
	}

	return titleText + strings.Repeat(" ", padding) + stepIndicator
}

// renderBranchList renders a scrollable list of branches
func (m Model) renderBranchList(branches []BranchStatus, maxVisible int, showWarnings bool) string {
	var b strings.Builder

	totalBranches := len(branches)
	scrollOffset := m.createState.scrollOffset

	// Ensure scroll offset is valid
	if scrollOffset > totalBranches-maxVisible {
		scrollOffset = totalBranches - maxVisible
	}
	if scrollOffset < 0 {
		scrollOffset = 0
	}

	// Scroll indicator (top)
	if scrollOffset > 0 {
		b.WriteString(WizardDescStyle.Render(fmt.Sprintf("  ↑ %d more", scrollOffset)))
		b.WriteString("\n")
	}

	// Branch items
	endIndex := scrollOffset + maxVisible
	if endIndex > totalBranches {
		endIndex = totalBranches
	}

	for i := scrollOffset; i < endIndex; i++ {
		branch := branches[i]
		selected := i == m.createState.selectedBranch

		// Build branch line
		var line strings.Builder

		// Status indicator
		if branch.IsClean {
			line.WriteString(StatusCleanStyle.Render("✓"))
		} else {
			line.WriteString(StatusWarningStyle.Render("!"))
		}
		line.WriteString(" ")

		// Branch name
		line.WriteString(branch.Name)

		// Current indicator
		if branch.IsCurrent {
			line.WriteString(WizardDescStyle.Render(" (current)"))
		}

		// Ahead/behind
		if branch.AheadCount > 0 {
			line.WriteString(StatusUnpushedStyle.Render(fmt.Sprintf(" ↑%d", branch.AheadCount)))
		}
		if branch.BehindCount > 0 {
			line.WriteString(StatusModifiedStyle.Render(fmt.Sprintf(" ↓%d", branch.BehindCount)))
		}

		// Uncommitted changes
		if !branch.IsClean {
			changes := branch.UncommittedFiles + branch.UntrackedFiles
			if changes > 0 {
				line.WriteString(WizardDescStyle.Render(fmt.Sprintf(" ~%d", changes)))
			}
		}

		b.WriteString(WizardOption(line.String(), selected))
		b.WriteString("\n")
	}

	// Scroll indicator (bottom)
	remaining := totalBranches - endIndex
	if remaining > 0 {
		b.WriteString(WizardDescStyle.Render(fmt.Sprintf("  ↓ %d more", remaining)))
		b.WriteString("\n")
	}

	return b.String()
}
