package ui

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Layout breakpoints
const (
	NarrowWidthThreshold = 160 // Below this: vertical layout (in terminal columns)
)

// isNarrowLayout returns true if the screen is too narrow for horizontal layout
func (m Model) isNarrowLayout() bool {
	return m.width < NarrowWidthThreshold
}

// dashboardView renders the main dashboard with modern table layout
func (m Model) dashboardView() string {
	// Handle error/loading states first
	if m.err != nil {
		return m.renderErrorState()
	}
	if m.repoInfo == nil {
		return m.renderLoadingState()
	}
	if !m.repoInfo.IsGitRepo {
		return m.renderNotGitRepoState()
	}

	// Build layout components
	header := m.renderHeader()
	content := m.renderContent()
	footer := m.renderFooter()

	// GitHub status line (between content and footer)
	var githubStatus string
	if m.githubLoading {
		spinnerText := m.githubSpinner.View() + " " + HeaderInfoStyle.Render("Fetching GitHub info...")
		githubStatus = lipgloss.NewStyle().
			Width(m.width).
			Align(lipgloss.Center).
			Padding(1, 0).
			Render(spinnerText)
	}

	// Combine all parts
	if githubStatus != "" {
		return lipgloss.JoinVertical(lipgloss.Left, header, content, githubStatus, footer)
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
}

// ═══════════════════════════════════════════════════════════════════════════
// Header
// ═══════════════════════════════════════════════════════════════════════════

// getLogoLines returns the ASCII art logo as styled lines
func getLogoLines() []string {
	return []string{
		"   ██████╗ ██████╗ ███████╗███╗   ██╗",
		"  ██╔════╝ ██╔══██╗██╔════╝████╗  ██║",
		"  ██║  ███╗██████╔╝█████╗  ██╔██╗ ██║",
		"  ██║   ██║██╔══██╗██╔══╝  ██║╚██╗██║",
		"  ╚██████╔╝██║  ██║███████╗██║ ╚████║",
		"   ╚═════╝ ╚═╝  ╚═╝╚══════╝╚═╝  ╚═══╝",
	}
}

func (m Model) renderHeader() string {
	// Get logo
	logoLines := getLogoLines()

	// Build info section (to the right of logo)
	var infoLines []string

	// Empty line for spacing
	infoLines = append(infoLines, "")

	// Line 2: Repo name
	if m.repoInfo != nil && m.repoInfo.Name != "" {
		infoLines = append(infoLines, HeaderInfoStyle.Render("repo: ")+WorktreeNameStyle.Render(m.repoInfo.Name))
	} else {
		infoLines = append(infoLines, "")
	}

	// Line 3: Branch
	if m.repoInfo != nil && m.repoInfo.CurrentBranch != "" {
		infoLines = append(infoLines, HeaderInfoStyle.Render("branch: ")+HeaderBranchStyle.Render(m.repoInfo.CurrentBranch))
	} else {
		infoLines = append(infoLines, "")
	}

	// Line 4: Worktree count
	if len(m.worktrees) > 0 {
		infoLines = append(infoLines, HeaderInfoStyle.Render(fmt.Sprintf("%d worktrees", len(m.worktrees))))
	} else {
		infoLines = append(infoLines, "")
	}

	// Line 5: Version
	if m.version != "" {
		infoLines = append(infoLines, HeaderInfoStyle.Render(m.version))
	} else {
		infoLines = append(infoLines, "")
	}

	// Empty line for spacing
	infoLines = append(infoLines, "")

	// Combine logo and info side by side
	logoWidth := 38 // Width of the big ASCII logo (with left padding)
	var headerLines []string

	maxLines := len(logoLines)
	if len(infoLines) > maxLines {
		maxLines = len(infoLines)
	}

	for i := 0; i < maxLines; i++ {
		logoLine := ""
		if i < len(logoLines) {
			logoLine = LogoStyle.Render(logoLines[i])
		}
		// Pad logo line to consistent width
		padding := logoWidth - lipgloss.Width(logoLine)
		if padding < 0 {
			padding = 0
		}
		logoPadded := logoLine + strings.Repeat(" ", padding)

		infoLine := ""
		if i < len(infoLines) {
			infoLine = infoLines[i]
		}

		headerLines = append(headerLines, logoPadded+"  "+infoLine)
	}

	header := strings.Join(headerLines, "\n")

	return header + "\n"
}

// ═══════════════════════════════════════════════════════════════════════════
// Content Area
// ═══════════════════════════════════════════════════════════════════════════

func (m Model) renderContent() string {
	if !m.repoInfo.IsInitialized {
		return m.renderNotInitializedState()
	}
	if len(m.worktrees) == 0 {
		return m.renderEmptyState()
	}
	return m.renderWorktreeTable()
}

func (m Model) renderWorktreeTable() string {
	totalWidth := m.width - 4

	// Calculate available height for content
	contentHeight := m.height - HeaderHeight - FooterHeight - 4
	if contentHeight < 5 {
		contentHeight = 5
	}

	// Sort worktrees: current first, then by last commit (most recent first)
	sortedWorktrees := m.getSortedWorktrees()

	// Get selected worktree for preview
	var selectedWorktree *Worktree
	if m.selected >= 0 && m.selected < len(sortedWorktrees) {
		selectedWorktree = &sortedWorktrees[m.selected]
	}

	// Check if we should use narrow/vertical layout
	if m.isNarrowLayout() {
		return m.renderNarrowLayout(sortedWorktrees, selectedWorktree, totalWidth, contentHeight)
	}

	// Wide layout: horizontal table + preview
	return m.renderWideLayout(sortedWorktrees, selectedWorktree, totalWidth, contentHeight)
}

// renderNarrowLayout renders vertical layout for narrow screens (list on top, preview below)
func (m Model) renderNarrowLayout(sortedWorktrees []Worktree, selectedWorktree *Worktree, width, height int) string {
	// Section header style - white/bright with icon
	sectionHeaderStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ffffff")).
		Bold(true)

	// Calculate heights: list section takes up to 40%, details get the rest
	listContentHeight := len(sortedWorktrees)
	maxListHeight := height * 40 / 100
	if listContentHeight > maxListHeight {
		listContentHeight = maxListHeight
	}
	if listContentHeight < 2 {
		listContentHeight = 2
	}

	// Heights: list header (1) + list content + separator (1) + details header (1) + details content
	listHeaderHeight := 1
	separatorHeight := 1
	detailsHeaderHeight := 1
	detailsContentHeight := height - listHeaderHeight - listContentHeight - separatorHeight - detailsHeaderHeight

	if detailsContentHeight < 5 {
		detailsContentHeight = 5
	}

	// Render list section with icon
	listHeader := sectionHeaderStyle.Render("◈ Worktrees")
	list := m.renderNarrowWorktreeList(sortedWorktrees, width, listContentHeight)

	// Separator line
	separatorLine := strings.Repeat("─", width)
	separator := lipgloss.NewStyle().
		Foreground(ColorBorderDim).
		Render(separatorLine)

	// Details header with icon
	detailsHeader := sectionHeaderStyle.Render("◇ Details")

	// Render preview/details panel (full width in narrow mode)
	preview := m.renderNarrowPreviewPanel(selectedWorktree, width, detailsContentHeight)

	// Stack vertically
	return lipgloss.JoinVertical(lipgloss.Left,
		listHeader,
		list,
		separator,
		detailsHeader,
		preview,
	)
}

// renderNarrowPreviewPanel renders a compact preview for narrow screens
func (m Model) renderNarrowPreviewPanel(wt *Worktree, width, height int) string {
	if wt == nil {
		return lipgloss.NewStyle().
			Width(width).
			Height(height).
			Foreground(ColorTextMuted).
			Render("No worktree selected")
	}

	var b strings.Builder

	// Compact format: key-value pairs
	labelStyle := lipgloss.NewStyle().Foreground(ColorTextMuted)
	valueStyle := lipgloss.NewStyle().Foreground(ColorText)
	branchStyle := lipgloss.NewStyle().Foreground(ColorSecondary)
	statusStyle := lipgloss.NewStyle().Foreground(ColorSuccess)

	// Branch
	b.WriteString(labelStyle.Render("Branch: "))
	b.WriteString(branchStyle.Render(wt.Branch))
	b.WriteString("\n")

	// Path (shortened)
	path := shortenPath(wt.Path, width-8)
	b.WriteString(labelStyle.Render("Path:   "))
	b.WriteString(valueStyle.Render(path))
	b.WriteString("\n")

	// Status
	b.WriteString(labelStyle.Render("Status: "))
	statusText, statusStyleInfo := getWorktreeStatusInfo(wt)
	b.WriteString(statusStyleInfo.Render(statusText))
	b.WriteString("\n")

	// Last commit
	if wt.LastCommit != "" {
		b.WriteString(labelStyle.Render("Commit: "))
		b.WriteString(valueStyle.Render(wt.LastCommit))
	}

	// If there's status detail (modified/staged/untracked)
	if wt.StagedCount > 0 || wt.ModifiedCount > 0 || wt.UntrackedCount > 0 {
		b.WriteString("\n")
		var details []string
		if wt.StagedCount > 0 {
			details = append(details, fmt.Sprintf("%d staged", wt.StagedCount))
		}
		if wt.ModifiedCount > 0 {
			details = append(details, fmt.Sprintf("%d modified", wt.ModifiedCount))
		}
		if wt.UntrackedCount > 0 {
			details = append(details, fmt.Sprintf("%d untracked", wt.UntrackedCount))
		}
		b.WriteString(labelStyle.Render("        "))
		b.WriteString(statusStyle.Render(strings.Join(details, ", ")))
	}

	// Show PR info in narrow mode
	if wt.PRNumber > 0 {
		b.WriteString("\n")
		prStyle := lipgloss.NewStyle().Foreground(ColorSecondary)
		b.WriteString(prStyle.Render(fmt.Sprintf("PR #%d %s", wt.PRNumber, wt.PRState)))
	}

	// Show brief stale reason in narrow mode
	if wt.BranchStatus == "stale" {
		b.WriteString("\n")
		staleStyle := lipgloss.NewStyle().Foreground(ColorTextMuted).Italic(true)
		reason := "Branch is stale"
		switch wt.StaleReason {
		case "merged_locally":
			reason = "Merged into main"
		case "no_unique_commits":
			reason = "No unique commits"
		case "remote_gone":
			reason = "Remote branch deleted"
		case "pr_merged":
			reason = "PR merged"
		case "pr_closed":
			reason = "PR closed"
		}
		b.WriteString(staleStyle.Render(reason))
	}

	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Render(b.String())
}

// getWorktreeStatusInfo returns status text and style for a worktree
func getWorktreeStatusInfo(wt *Worktree) (string, lipgloss.Style) {
	if wt.Status == "missing" {
		return "Missing", lipgloss.NewStyle().Foreground(ColorError)
	}
	if wt.BranchStatus == "stale" {
		return "Stale", lipgloss.NewStyle().Foreground(ColorTextMuted)
	}
	if wt.StagedCount > 0 || wt.ModifiedCount > 0 {
		return "Modified", lipgloss.NewStyle().Foreground(ColorWarning)
	}
	if wt.UntrackedCount > 0 {
		return "Untracked", lipgloss.NewStyle().Foreground(ColorWarning)
	}
	return "Clean", lipgloss.NewStyle().Foreground(ColorSuccess)
}

// renderWideLayout renders horizontal layout for wide screens (table + preview side by side)
func (m Model) renderWideLayout(sortedWorktrees []Worktree, selectedWorktree *Worktree, totalWidth, contentHeight int) string {
	// Split: 65% table, 35% preview
	tableWidth := totalWidth * 65 / 100
	previewWidth := totalWidth - tableWidth - 3 // -3 for separator

	// Build table
	var tableRows []string

	// Table header
	header := m.renderTableHeader(tableWidth)
	tableRows = append(tableRows, header)

	// Separator line
	separator := lipgloss.NewStyle().
		Foreground(ColorBorderDim).
		Render(strings.Repeat("─", tableWidth))
	tableRows = append(tableRows, separator)

	// Worktree rows
	for i, wt := range sortedWorktrees {
		row := m.renderWorktreeRow(wt, i == m.selected, tableWidth)
		tableRows = append(tableRows, row)
	}

	tableContent := lipgloss.JoinVertical(lipgloss.Left, tableRows...)
	table := lipgloss.NewStyle().
		Width(tableWidth).
		Height(contentHeight).
		Render(tableContent)

	// Build preview panel
	preview := m.renderPreviewPanel(selectedWorktree, previewWidth, contentHeight)

	// Vertical separator
	sep := lipgloss.NewStyle().
		Foreground(ColorBorderDim).
		Render(strings.Repeat("│\n", contentHeight))

	// Combine table and preview
	return lipgloss.JoinHorizontal(lipgloss.Top, table, " "+sep+" ", preview)
}

// getSortedWorktrees returns worktrees sorted: current first, then by recency
func (m Model) getSortedWorktrees() []Worktree {
	sorted := make([]Worktree, len(m.worktrees))
	copy(sorted, m.worktrees)

	sort.SliceStable(sorted, func(i, j int) bool {
		// Current worktree always first
		if sorted[i].IsCurrent && !sorted[j].IsCurrent {
			return true
		}
		if !sorted[i].IsCurrent && sorted[j].IsCurrent {
			return false
		}
		// Then sort by commit recency (this is approximate based on string)
		return commitTimeScore(sorted[i].LastCommit) > commitTimeScore(sorted[j].LastCommit)
	})

	return sorted
}

// getSelectedWorktree returns the worktree at the current selection index from the sorted list.
// This must be used instead of m.worktrees[m.selected] to ensure the selection matches
// the displayed (sorted) order.
func (m Model) getSelectedWorktree() *Worktree {
	sorted := m.getSortedWorktrees()
	if m.selected < 0 || m.selected >= len(sorted) {
		return nil
	}
	return &sorted[m.selected]
}

// commitTimeScore returns a rough score for sorting (higher = more recent)
func commitTimeScore(timeStr string) int {
	if timeStr == "" {
		return 0
	}
	// Parse the shortened time format and return approximate seconds ago
	if strings.Contains(timeStr, "s ago") {
		return 1000000
	}
	if strings.Contains(timeStr, "m ago") {
		return 100000
	}
	if strings.Contains(timeStr, "h ago") {
		return 10000
	}
	if strings.Contains(timeStr, "d ago") {
		return 1000
	}
	if strings.Contains(timeStr, "w ago") {
		return 100
	}
	if strings.Contains(timeStr, "mo ago") {
		return 10
	}
	if strings.Contains(timeStr, "y ago") {
		return 1
	}
	return 0
}

// renderNarrowWorktreeList renders a simple list for narrow screens
func (m Model) renderNarrowWorktreeList(sortedWorktrees []Worktree, width, height int) string {
	var rows []string

	for i, wt := range sortedWorktrees {
		// Current indicator
		indicator := "  "
		if wt.IsCurrent {
			indicator = "• "
		}

		// Calculate widths for name and branch
		availableWidth := width - 4 // Account for indicator and spacing
		nameWidth := availableWidth * 45 / 100
		branchWidth := availableWidth - nameWidth - 2

		// Truncate name and branch
		name := truncate(wt.Name, nameWidth)
		branch := truncate(wt.Branch, branchWidth)

		// Style based on selection and current status
		var nameStyle, branchStyle lipgloss.Style
		if i == m.selected {
			// Selected row - use current style if current, otherwise primary
			if wt.IsCurrent {
				nameStyle = DashboardNameCurrentStyle
			} else {
				nameStyle = lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
			}
			branchStyle = DashboardBranchStyle
		} else {
			// Normal row
			if wt.IsCurrent {
				nameStyle = DashboardNameCurrentStyle
			} else {
				nameStyle = DashboardNameStyle
			}
			branchStyle = DashboardBranchStyle
		}

		rowContent := fmt.Sprintf("%s%-*s  %s",
			indicator,
			nameWidth, nameStyle.Render(name),
			branchStyle.Render(branch))

		rows = append(rows, rowContent)
	}

	// Join rows and ensure height
	content := strings.Join(rows, "\n")
	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Render(content)
}

func (m Model) renderTableHeader(width int) string {
	// Column widths (proportional) - no NAME column, branch is the identifier
	branchWidth := width * 35 / 100
	lastCommitWidth := width * 12 / 100
	statusWidth := width * 12 / 100
	pathWidth := width - branchWidth - lastCommitWidth - statusWidth

	branchCol := TableHeaderStyle.Width(branchWidth).Render("BRANCH")
	lastCommitCol := TableHeaderStyle.Width(lastCommitWidth).Render("LAST COMMIT")
	statusCol := TableHeaderStyle.Width(statusWidth).Render("STATUS")
	pathCol := TableHeaderStyle.Width(pathWidth).Render("PATH")

	return lipgloss.JoinHorizontal(lipgloss.Top, branchCol, lastCommitCol, statusCol, pathCol)
}

func (m Model) renderWorktreeRow(wt Worktree, selected bool, width int) string {
	// Column widths (proportional) - must match header
	branchWidth := width * 35 / 100
	lastCommitWidth := width * 12 / 100
	statusWidth := width * 12 / 100
	pathWidth := width - branchWidth - lastCommitWidth - statusWidth

	// Branch with current indicator
	branch := wt.Branch
	if wt.IsCurrent {
		branch = "● " + branch
	} else {
		branch = "  " + branch
	}

	// Shorten path (use ~ for home directory)
	// Add [main] suffix for main worktree
	mainTag := ""
	if wt.IsMain {
		mainTag = " [main]"
	}
	path := shortenPath(wt.Path, pathWidth-2-len(mainTag))

	// Style based on selection and current status
	var rowStyle lipgloss.Style
	var bgColor lipgloss.AdaptiveColor
	if wt.IsCurrent && selected {
		rowStyle = TableRowCurrentSelectedStyle
		bgColor = ColorBgCurrentSelected
	} else if wt.IsCurrent {
		rowStyle = TableRowCurrentStyle
		bgColor = ColorBgCurrent
	} else if selected {
		rowStyle = TableRowSelectedStyle
		bgColor = ColorBgSelected
	} else {
		rowStyle = TableRowStyle
		bgColor = lipgloss.AdaptiveColor{} // No background for normal rows
	}

	// Status badge with details - pass background color for consistent styling
	status := StatusBadgeDetailed(wt.Status, wt.BranchStatus, wt.StagedCount, wt.ModifiedCount, wt.UntrackedCount, wt.UnpushedCount, wt.PRNumber, wt.PRState, bgColor)

	// Use Dashboard-specific styles for consistent coloring
	var branchStyle lipgloss.Style
	if wt.IsCurrent {
		branchStyle = DashboardNameCurrentStyle
	} else {
		branchStyle = DashboardBranchStyle
	}

	// Build path with optional [main] tag - combine into single string for consistent background
	pathText := path + mainTag

	branchCol := rowStyle.Width(branchWidth).Render(branchStyle.Render(truncate(branch, branchWidth-2)))
	lastCommitCol := rowStyle.Width(lastCommitWidth).Render(DashboardCommitStyle.Render(truncate(wt.LastCommit, lastCommitWidth-2)))
	statusCol := rowStyle.Width(statusWidth).Render(status)
	pathCol := rowStyle.Width(pathWidth).Render(DashboardPathStyle.Render(pathText))

	return lipgloss.JoinHorizontal(lipgloss.Top, branchCol, lastCommitCol, statusCol, pathCol)
}

// shortenPath replaces home directory with ~ and truncates if needed
func shortenPath(path string, maxLen int) string {
	// Replace home directory with ~
	if home, err := os.UserHomeDir(); err == nil {
		if strings.HasPrefix(path, home) {
			path = "~" + strings.TrimPrefix(path, home)
		}
	}

	// Truncate from the beginning if too long
	if len(path) > maxLen && maxLen > 3 {
		path = "..." + path[len(path)-(maxLen-3):]
	}

	return path
}

// ═══════════════════════════════════════════════════════════════════════════
// Footer / Help Bar
// ═══════════════════════════════════════════════════════════════════════════

func (m Model) renderFooter() string {
	width := m.width - 2

	if m.repoInfo == nil || !m.repoInfo.IsGitRepo {
		return FooterBarStyle.Width(width).Render(HelpItem("q", "quit"))
	}

	if !m.repoInfo.IsInitialized {
		items := HelpItem("i", "init") + "  " + HelpItem("q", "quit")
		return FooterBarStyle.Width(width).Render(items)
	}

	// Group shortcuts logically with separators
	nav := HelpItem("↑↓", "nav")
	actions := HelpItem("n", "new") + " " + HelpItem("d", "del") + " " + HelpItem("t", "tools")
	open := HelpItem("enter", "open") + " " + HelpItem("g", "goto")
	other := HelpItem("c", "cfg") + " " + HelpItem("?", "help") + " " + HelpItem("q", "quit")

	sep := HelpSeparatorStyle.Render(" │ ")
	helpText := nav + sep + actions + sep + open + sep + other

	return FooterBarStyle.Width(width).Render(helpText)
}

// ═══════════════════════════════════════════════════════════════════════════
// State Views
// ═══════════════════════════════════════════════════════════════════════════

func (m Model) renderErrorState() string {
	width := m.width - 4
	height := m.height - 6

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		"",
		ErrorStyle.Render("Error"),
		"",
		HelpTextStyle.Render(m.err.Error()),
	)

	box := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(content)

	footer := FooterBarStyle.Width(width - 2).Render(HelpItem("q", "quit"))

	return lipgloss.JoinVertical(lipgloss.Left, box, footer)
}

func (m Model) renderLoadingState() string {
	width := m.width - 4
	height := m.height - 6

	content := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(HelpTextStyle.Render("Loading..."))

	footer := FooterBarStyle.Width(width - 2).Render(HelpItem("q", "quit"))

	return lipgloss.JoinVertical(lipgloss.Left, content, footer)
}

func (m Model) renderNotGitRepoState() string {
	width := m.width - 4
	height := m.height - 6

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		"",
		ErrorStyle.Render("Not a git repository"),
		"",
		HelpTextStyle.Render("Please run gren from within a git repository."),
	)

	box := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(content)

	footer := FooterBarStyle.Width(width - 2).Render(HelpItem("q", "quit"))

	return lipgloss.JoinVertical(lipgloss.Left, box, footer)
}

func (m Model) renderNotInitializedState() string {
	width := m.width - 4
	height := m.height - HeaderHeight - FooterHeight - 2

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		"",
		TitleStyle.Render("Welcome to gren"),
		"",
		HelpTextStyle.Render("Worktree management is not initialized for this project."),
		"",
		HelpTextStyle.Render("Press "+HelpKeyStyle.Render("i")+" to initialize and create a .gren/ configuration."),
	)

	box := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(content)

	return box
}

func (m Model) renderEmptyState() string {
	width := m.width - 4
	height := m.height - HeaderHeight - FooterHeight - 2

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		"",
		TitleStyle.Render("No worktrees yet"),
		"",
		HelpTextStyle.Render("Press "+HelpKeyStyle.Render("n")+" to create your first worktree."),
	)

	box := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(content)

	return box
}

// ═══════════════════════════════════════════════════════════════════════════
// Helpers
// ═══════════════════════════════════════════════════════════════════════════

func truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// Utility function to get status icon and color (legacy compatibility)
func getStatusDisplay(status string) (string, lipgloss.Style) {
	switch status {
	case "clean":
		return "● Clean", StatusCleanStyle
	case "modified":
		return "● Modified", StatusModifiedStyle
	case "building":
		return "◐ Building", StatusBuildingStyle
	default:
		return "● Clean", StatusCleanStyle
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// Preview Panel
// ═══════════════════════════════════════════════════════════════════════════

// renderPreviewPanel renders the right-side preview panel with worktree details
func (m Model) renderPreviewPanel(wt *Worktree, width, height int) string {
	if wt == nil {
		return lipgloss.NewStyle().
			Width(width).
			Height(height).
			Foreground(ColorTextMuted).
			Align(lipgloss.Center, lipgloss.Center).
			Render("No worktree selected")
	}

	var lines []string

	// Title: Worktree name
	titleStyle := lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true).
		MarginBottom(1)

	// Build title with indicators
	title := wt.Name
	var suffix string
	if wt.IsCurrent && wt.IsMain {
		title = "● " + title
		suffix = " (current, main)"
	} else if wt.IsCurrent {
		title = "● " + title
		suffix = " (current)"
	} else if wt.IsMain {
		suffix = " (main)"
	}
	lines = append(lines, titleStyle.Render(title+suffix))

	lines = append(lines, "") // spacing

	// Use consistent label and value styles
	labelStyle := lipgloss.NewStyle().Foreground(ColorTextMuted)

	// Branch info
	lines = append(lines, labelStyle.Render("Branch"))
	lines = append(lines, "  "+DashboardBranchStyle.Render(wt.Branch))
	lines = append(lines, "")

	// Path
	lines = append(lines, labelStyle.Render("Path"))
	shortPath := shortenPath(wt.Path, width-4)
	lines = append(lines, "  "+DashboardPathStyle.Render(shortPath))
	lines = append(lines, "")

	// Last commit
	lines = append(lines, labelStyle.Render("Last Commit"))
	if wt.LastCommit != "" {
		lines = append(lines, "  "+DashboardCommitStyle.Render(wt.LastCommit))
	} else {
		lines = append(lines, "  "+DashboardPathStyle.Render("unknown"))
	}
	lines = append(lines, "")

	// Status details
	lines = append(lines, labelStyle.Render("Status"))
	if wt.BranchStatus == "stale" {
		lines = append(lines, "  "+lipgloss.NewStyle().Foreground(ColorTextMuted).Render("💤 Stale"))
	} else if wt.StagedCount == 0 && wt.ModifiedCount == 0 && wt.UntrackedCount == 0 && wt.UnpushedCount == 0 {
		lines = append(lines, "  "+StatusCleanStyle.Render("✓ Clean"))
	} else {
		if wt.StagedCount > 0 {
			lines = append(lines, "  "+StatusCleanStyle.Render(fmt.Sprintf("+%d staged", wt.StagedCount)))
		}
		if wt.ModifiedCount > 0 {
			lines = append(lines, "  "+StatusModifiedStyle.Render(fmt.Sprintf("~%d modified", wt.ModifiedCount)))
		}
		if wt.UntrackedCount > 0 {
			lines = append(lines, "  "+StatusModifiedStyle.Render(fmt.Sprintf("?%d untracked", wt.UntrackedCount)))
		}
		if wt.UnpushedCount > 0 {
			lines = append(lines, "  "+StatusUnpushedStyle.Render(fmt.Sprintf("↑%d unpushed", wt.UnpushedCount)))
		}
	}

	// Show PR info if available
	if wt.PRNumber > 0 {
		lines = append(lines, "")
		prHeaderStyle := lipgloss.NewStyle().Foreground(ColorSecondary).Bold(true)
		lines = append(lines, prHeaderStyle.Render("Pull Request"))

		prStyle := lipgloss.NewStyle().Foreground(ColorText)
		stateStyle := lipgloss.NewStyle()
		switch wt.PRState {
		case "OPEN":
			stateStyle = stateStyle.Foreground(ColorSuccess)
		case "DRAFT":
			stateStyle = stateStyle.Foreground(ColorTextMuted)
		case "MERGED":
			stateStyle = stateStyle.Foreground(ColorPrimary)
		case "CLOSED":
			stateStyle = stateStyle.Foreground(ColorError)
		}

		lines = append(lines, "  "+prStyle.Render(fmt.Sprintf("#%d", wt.PRNumber))+" "+stateStyle.Render(wt.PRState))
		lines = append(lines, "  "+lipgloss.NewStyle().Foreground(ColorTextMuted).Render("Press 't' → 'p' to open in browser"))
	}

	// Show "Why stale?" explanation if worktree is stale
	if wt.BranchStatus == "stale" {
		lines = append(lines, "")
		staleHeaderStyle := lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
		lines = append(lines, staleHeaderStyle.Render("Why stale?"))

		// Explanation based on reason
		explanation := ""
		suggestion := ""
		switch wt.StaleReason {
		case "merged_locally":
			explanation = "This branch has been merged into main."
			suggestion = "Safe to delete - work is preserved in main."
		case "no_unique_commits":
			explanation = "This branch has no unique commits."
			suggestion = "Empty or already merged - safe to delete."
		case "remote_gone":
			explanation = "Remote branch was deleted (likely after merge)."
			suggestion = "Press 't' → 'c' to cleanup, or 'd' to delete."
		case "pr_merged":
			explanation = "Pull request was merged."
			suggestion = "Safe to delete - work is in main."
		case "pr_closed":
			explanation = "Pull request was closed without merging."
			suggestion = "Review if work should be preserved."
		default:
			explanation = "Branch appears to be stale."
			suggestion = "Consider cleaning up this worktree."
		}

		explanationStyle := lipgloss.NewStyle().Foreground(ColorText)
		suggestionStyle := lipgloss.NewStyle().Foreground(ColorTextMuted).Italic(true)

		lines = append(lines, "  "+explanationStyle.Render(explanation))
		lines = append(lines, "  "+suggestionStyle.Render(suggestion))
	}

	// Get recent commits for this worktree
	lines = append(lines, "")
	lines = append(lines, labelStyle.Render("Recent Commits"))
	commits := getRecentCommits(wt.Path, 3, width-4)
	if len(commits) > 0 {
		for _, commit := range commits {
			lines = append(lines, "  "+commit)
		}
	} else {
		lines = append(lines, "  "+WorktreePathStyle.Render("No commits"))
	}

	content := strings.Join(lines, "\n")

	// Apply panel styling
	panelStyle := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Padding(0, 1)

	return panelStyle.Render(content)
}

// getRecentCommits retrieves the most recent commits for a worktree
func getRecentCommits(worktreePath string, count int, maxWidth int) []string {
	if worktreePath == "" {
		return nil
	}

	// Run git log to get recent commits
	cmd := exec.Command("git", "-C", worktreePath, "log", "--oneline", "-n", fmt.Sprintf("%d", count), "--format=%h %s")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	outputStr := strings.TrimSpace(string(output))
	if outputStr == "" {
		return nil
	}

	commitLines := strings.Split(outputStr, "\n")
	var result []string

	commitStyle := lipgloss.NewStyle().Foreground(ColorTextMuted)
	hashStyle := lipgloss.NewStyle().Foreground(ColorSecondary)

	for _, line := range commitLines {
		if line == "" {
			continue
		}
		// Split into hash and message
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			hash := parts[0]
			msg := parts[1]
			// Truncate message if needed
			maxMsgLen := maxWidth - len(hash) - 3
			if maxMsgLen < 10 {
				maxMsgLen = 10
			}
			if len(msg) > maxMsgLen {
				msg = msg[:maxMsgLen-3] + "..."
			}
			result = append(result, hashStyle.Render(hash)+" "+commitStyle.Render(msg))
		} else {
			result = append(result, commitStyle.Render(truncate(line, maxWidth)))
		}
	}

	return result
}

// ═══════════════════════════════════════════════════════════════════════════
// Modal Rendering
// ═══════════════════════════════════════════════════════════════════════════

// renderWithModal overlays a modal on top of a base view (default width 50)
func (m Model) renderWithModal(baseView, modalContent string) string {
	return m.renderWithModalWidth(baseView, modalContent, 50, ColorPrimary)
}

// renderWithModalWidth overlays a modal with custom width and border color
func (m Model) renderWithModalWidth(baseView, modalContent string, width int, borderColor lipgloss.TerminalColor) string {
	// Style for the modal box
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1, 2).
		Width(width)

	styledModal := modalStyle.Render(modalContent)
	modalLines := strings.Split(styledModal, "\n")

	// Split base view into lines
	baseLines := strings.Split(baseView, "\n")

	// Calculate modal position (centered)
	modalVisualWidth := lipgloss.Width(modalLines[0])
	startX := (m.width - modalVisualWidth) / 2
	startY := (m.height - len(modalLines)) / 2

	if startX < 0 {
		startX = 0
	}
	if startY < 0 {
		startY = 0
	}

	// Overlay modal on base view
	result := make([]string, len(baseLines))
	copy(result, baseLines)

	// Ensure we have enough lines
	for len(result) < startY+len(modalLines) {
		result = append(result, strings.Repeat(" ", m.width))
	}

	// Overlay each modal line - use simple padding approach
	for i, modalLine := range modalLines {
		y := startY + i
		if y >= 0 && y < len(result) {
			// Build line: left padding + modal + right side is ignored (modal covers it)
			leftPad := strings.Repeat(" ", startX)
			result[y] = leftPad + modalLine
		}
	}

	return strings.Join(result, "\n")
}

// renderDeleteModal renders delete confirmation as a modal overlay
func (m Model) renderDeleteModal(baseView string) string {
	return m.renderWithModalWidth(baseView, m.renderDeleteConfirmModal(), 70, ColorWarning)
}

// renderOpenInModal renders the "Open in..." modal content
func (m Model) renderOpenInModal() string {
	if m.openInState == nil {
		return "Loading..."
	}

	var content strings.Builder

	// Title
	title := TitleStyle.Render("Open in...")
	content.WriteString(title)
	content.WriteString("\n\n")

	// Get selected worktree name
	if wt := m.getSelectedWorktree(); wt != nil {
		content.WriteString(WorktreePathStyle.Render(wt.Name))
		content.WriteString("\n\n")
	}

	// Action list
	for i, action := range m.openInState.actions {
		prefix := "  "
		if i == m.openInState.selectedIndex {
			prefix = "▶ "
			content.WriteString(MenuItemSelectedStyle.Render(fmt.Sprintf("%s%s %s", prefix, action.Icon, action.Name)))
		} else {
			content.WriteString(MenuItemStyle.Render(fmt.Sprintf("%s%s %s", prefix, action.Icon, action.Name)))
		}
		content.WriteString("\n")
	}

	// Help text
	content.WriteString("\n")
	content.WriteString(HelpTextStyle.Render("↑↓ select • enter open • esc close"))

	return content.String()
}
