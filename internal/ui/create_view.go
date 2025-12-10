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
	var b strings.Builder

	// Header
	b.WriteString(WizardHeader("New Worktree"))
	b.WriteString("\n\n")

	// Error display
	if m.err != nil {
		b.WriteString(ErrorStyle.Render("Error: " + m.err.Error()))
		b.WriteString("\n\n")
	}

	// Subtitle
	b.WriteString(WizardSubtitleStyle.Render("Choose branch type"))
	b.WriteString("\n\n")

	// Mode options
	modes := []struct {
		name string
		desc string
	}{
		{"Create new branch", "Start fresh from an existing branch"},
		{"Use existing branch", "Checkout a branch that already exists"},
	}

	for i, mode := range modes {
		b.WriteString(WizardOption(mode.name, i == m.createState.selectedMode))
		b.WriteString("\n")
		if i == m.createState.selectedMode {
			b.WriteString(WizardDescStyle.Render("   " + mode.desc))
			b.WriteString("\n")
		}
	}

	// Help
	b.WriteString("\n")
	b.WriteString(WizardHelpBar("↑↓ select", "enter confirm", "esc cancel"))

	return m.wrapWizardContent(b.String())
}

// ═══════════════════════════════════════════════════════════════════════════
// Step 2: Branch Name Input
// ═══════════════════════════════════════════════════════════════════════════

func (m Model) renderBranchNameStep() string {
	var b strings.Builder

	b.WriteString(WizardHeader("New Worktree"))
	b.WriteString("\n\n")

	b.WriteString(WizardSubtitleStyle.Render("Enter branch name"))
	b.WriteString("\n\n")

	// Input field
	cursor := "▮"
	inputContent := m.createState.branchName + cursor
	b.WriteString(WizardInputStyle.Width(40).Render(inputContent))
	b.WriteString("\n\n")

	// Validation
	if m.createState.branchName != "" {
		if isValidBranchName(m.createState.branchName) {
			b.WriteString(WizardSuccessStyle.Render("✓ Valid branch name"))
		} else {
			b.WriteString(ErrorStyle.Render("✗ Invalid branch name"))
			b.WriteString("\n")
			b.WriteString(WizardDescStyle.Render("  Use letters, numbers, dashes, underscores, slashes"))
		}
		b.WriteString("\n\n")
	}

	// Examples
	b.WriteString(WizardDescStyle.Render("Examples: feature/auth, hotfix/bug-123, experiment/new-ui"))
	b.WriteString("\n\n")

	b.WriteString(WizardHelpBar("type name", "enter continue", "esc back"))

	return m.wrapWizardContent(b.String())
}

// ═══════════════════════════════════════════════════════════════════════════
// Step 3: Existing Branch Selection
// ═══════════════════════════════════════════════════════════════════════════

func (m Model) renderExistingBranchStep() string {
	var b strings.Builder

	b.WriteString(WizardHeader("Select Branch"))
	b.WriteString("\n\n")

	// Search input
	if m.createState.isSearching || m.createState.searchQuery != "" {
		searchStyle := WizardInputStyle.Width(30)
		if m.createState.isSearching {
			b.WriteString(searchStyle.Render("/ " + m.createState.searchQuery + "▮"))
		} else {
			b.WriteString(WizardDescStyle.Render("Filter: " + m.createState.searchQuery))
		}
		b.WriteString("\n\n")
	}

	// Get branches to display
	branches := m.createState.filteredAvailableBranches
	if len(branches) == 0 && m.createState.searchQuery == "" {
		branches = m.createState.availableBranches
	}

	if len(branches) == 0 {
		if m.createState.searchQuery != "" {
			b.WriteString(WizardDescStyle.Render("No branches match your search"))
		} else {
			b.WriteString(WizardDescStyle.Render("No available branches found"))
			b.WriteString("\n")
			b.WriteString(WizardDescStyle.Render("All branches may already have worktrees"))
		}
		b.WriteString("\n\n")
		b.WriteString(WizardHelpBar("esc back"))
		return m.wrapWizardContent(b.String())
	}

	// Calculate visible window
	maxVisible := m.height - 18
	if maxVisible < 5 {
		maxVisible = 5
	}
	if maxVisible > 15 {
		maxVisible = 15
	}

	// Branch list
	b.WriteString(m.renderBranchList(branches, maxVisible, false))

	// Help
	b.WriteString("\n")
	if m.createState.isSearching {
		b.WriteString(WizardHelpBar("type filter", "enter select", "esc cancel"))
	} else {
		b.WriteString(WizardHelpBar("↑↓ select", "/ search", "enter confirm", "esc back"))
	}

	return m.wrapWizardContent(b.String())
}

// ═══════════════════════════════════════════════════════════════════════════
// Step 4: Base Branch Selection
// ═══════════════════════════════════════════════════════════════════════════

func (m Model) renderBaseBranchStep() string {
	var b strings.Builder

	b.WriteString(WizardHeader("Select Base Branch"))
	b.WriteString("\n\n")

	b.WriteString(WizardSubtitleStyle.Render(fmt.Sprintf("Create '%s' from:", m.createState.branchName)))
	b.WriteString("\n\n")

	// Search input
	if m.createState.isSearching || m.createState.searchQuery != "" {
		searchStyle := WizardInputStyle.Width(30)
		if m.createState.isSearching {
			b.WriteString(searchStyle.Render("/ " + m.createState.searchQuery + "▮"))
		} else {
			b.WriteString(WizardDescStyle.Render("Filter: " + m.createState.searchQuery))
		}
		b.WriteString("\n\n")
	}

	// Get branches
	branches := m.createState.filteredBranches
	if len(branches) == 0 && m.createState.searchQuery == "" {
		branches = m.createState.branchStatuses
	}

	if len(branches) == 0 && m.createState.searchQuery != "" {
		b.WriteString(WizardDescStyle.Render("No branches match your search"))
		b.WriteString("\n\n")
		b.WriteString(WizardHelpBar("esc cancel", "backspace edit"))
		return m.wrapWizardContent(b.String())
	}

	// Calculate visible window
	maxVisible := m.height - 20
	if maxVisible < 5 {
		maxVisible = 5
	}
	if maxVisible > 12 {
		maxVisible = 12
	}

	// Branch list
	b.WriteString(m.renderBranchList(branches, maxVisible, true))

	// Warning for dirty branch
	if len(branches) > 0 && m.createState.selectedBranch < len(branches) {
		selectedBranch := branches[m.createState.selectedBranch]
		if !selectedBranch.IsClean {
			b.WriteString("\n")
			warning := fmt.Sprintf("⚠ '%s' has uncommitted changes\n  Worktree will be based on last commit only", selectedBranch.Name)
			b.WriteString(WizardWarningStyle.Render(warning))
			b.WriteString("\n")
		}
	}

	// Help
	b.WriteString("\n")
	if m.createState.isSearching {
		b.WriteString(WizardHelpBar("type filter", "enter select", "esc cancel"))
	} else {
		needsWarningAccept := len(branches) > 0 &&
			m.createState.selectedBranch < len(branches) &&
			!branches[m.createState.selectedBranch].IsClean &&
			!m.createState.warningAccepted

		if needsWarningAccept {
			b.WriteString(WizardHelpBar("↑↓ select", "y accept warning", "/ search", "esc back"))
		} else {
			b.WriteString(WizardHelpBar("↑↓ select", "/ search", "enter confirm", "esc back"))
		}
	}

	return m.wrapWizardContent(b.String())
}

// ═══════════════════════════════════════════════════════════════════════════
// Step 5: Confirmation
// ═══════════════════════════════════════════════════════════════════════════

func (m Model) renderConfirmStep() string {
	var b strings.Builder

	b.WriteString(WizardHeader("Confirm Creation"))
	b.WriteString("\n\n")

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

	b.WriteString(summaryStyle.Render(summary.String()))
	b.WriteString("\n\n")

	// Post-create info
	b.WriteString(WizardDescStyle.Render("After creation, .gren/post-create.sh will run"))
	b.WriteString("\n\n")

	b.WriteString(WizardHelpBar("enter create", "esc back"))

	return m.wrapWizardContent(b.String())
}

// ═══════════════════════════════════════════════════════════════════════════
// Step 6: Creating (Progress)
// ═══════════════════════════════════════════════════════════════════════════

func (m Model) renderCreatingStep() string {
	var b strings.Builder

	b.WriteString(WizardHeader("Creating Worktree"))
	b.WriteString("\n\n")

	// Progress indicators
	spinnerStyle := lipgloss.NewStyle().Foreground(ColorAccent)
	doneStyle := lipgloss.NewStyle().Foreground(ColorSuccess)
	pendingStyle := WizardDescStyle

	b.WriteString(spinnerStyle.Render("◐ Creating git worktree..."))
	b.WriteString("\n")
	b.WriteString(pendingStyle.Render("○ Running post-create hook..."))
	b.WriteString("\n")
	b.WriteString(pendingStyle.Render("○ Installing dependencies..."))
	b.WriteString("\n\n")

	_ = doneStyle // Will be used when we track actual progress

	b.WriteString(WizardDescStyle.Render("Path: " + m.getWorktreePath(m.createState.branchName)))

	return m.wrapWizardContent(b.String())
}

// ═══════════════════════════════════════════════════════════════════════════
// Step 7: Complete
// ═══════════════════════════════════════════════════════════════════════════

func (m Model) renderCreateCompleteStep() string {
	var b strings.Builder

	b.WriteString(WizardHeader("Worktree Created"))
	b.WriteString("\n\n")

	b.WriteString(WizardSuccessStyle.Render("✓ " + m.createState.branchName))
	b.WriteString("\n\n")

	// Path
	worktreePath := m.getWorktreePath(m.createState.branchName)
	b.WriteString(WizardDescStyle.Render("Path: " + worktreePath))
	b.WriteString("\n\n")

	// Actions
	b.WriteString(WizardSubtitleStyle.Render("Open in:"))
	b.WriteString("\n\n")

	actions := m.getAvailableActions()
	if len(actions) == 1 {
		b.WriteString(WizardDescStyle.Render("No editors detected in PATH"))
		b.WriteString("\n\n")
	}

	for i, action := range actions {
		label := action.Icon + " " + action.Name
		b.WriteString(WizardOption(label, i == m.createState.selectedAction))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(WizardHelpBar("↑↓ select", "enter open", "esc dashboard"))

	return m.wrapWizardContent(b.String())
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
