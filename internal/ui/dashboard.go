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
	NarrowWidthThreshold = 100 // Below this: vertical layout
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

	// Combine all parts
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
	// Calculate heights: list takes up to half, preview gets the rest
	listHeight := len(sortedWorktrees)
	if listHeight > height/2 {
		listHeight = height / 2
	}
	if listHeight < 3 {
		listHeight = 3
	}
	previewHeight := height - listHeight - 1 // -1 for separator

	// Render list
	list := m.renderNarrowWorktreeList(sortedWorktrees, width, listHeight)

	// Horizontal separator
	separator := lipgloss.NewStyle().
		Foreground(ColorBorderDim).
		Render(strings.Repeat("─", width))

	// Render preview panel (full width in narrow mode)
	preview := m.renderPreviewPanel(selectedWorktree, width, previewHeight)

	// Stack vertically
	return lipgloss.JoinVertical(lipgloss.Left, list, separator, preview)
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

		// Style based on selection
		var rowContent string
		if i == m.selected {
			// Selected row - highlight
			nameStyle := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
			branchStyle := lipgloss.NewStyle().Foreground(ColorSecondary)
			rowContent = fmt.Sprintf("%s%-*s  %s",
				indicator,
				nameWidth, nameStyle.Render(name),
				branchStyle.Render(branch))
		} else {
			// Normal row
			nameStyle := lipgloss.NewStyle().Foreground(ColorText)
			branchStyle := lipgloss.NewStyle().Foreground(ColorTextMuted)
			rowContent = fmt.Sprintf("%s%-*s  %s",
				indicator,
				nameWidth, nameStyle.Render(name),
				branchStyle.Render(branch))
		}

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
	// Column widths (proportional)
	nameWidth := width * 20 / 100
	branchWidth := width * 22 / 100
	lastCommitWidth := width * 12 / 100
	statusWidth := width * 12 / 100
	pathWidth := width - nameWidth - branchWidth - lastCommitWidth - statusWidth

	nameCol := TableHeaderStyle.Width(nameWidth).Render("NAME")
	branchCol := TableHeaderStyle.Width(branchWidth).Render("BRANCH")
	lastCommitCol := TableHeaderStyle.Width(lastCommitWidth).Render("LAST COMMIT")
	statusCol := TableHeaderStyle.Width(statusWidth).Render("STATUS")
	pathCol := TableHeaderStyle.Width(pathWidth).Render("PATH")

	return lipgloss.JoinHorizontal(lipgloss.Top, nameCol, branchCol, lastCommitCol, statusCol, pathCol)
}

func (m Model) renderWorktreeRow(wt Worktree, selected bool, width int) string {
	// Column widths (proportional) - must match header
	nameWidth := width * 20 / 100
	branchWidth := width * 22 / 100
	lastCommitWidth := width * 12 / 100
	statusWidth := width * 12 / 100
	pathWidth := width - nameWidth - branchWidth - lastCommitWidth - statusWidth

	// Name with indicator
	name := wt.Name
	if wt.IsCurrent {
		name = "● " + name
	} else {
		name = "  " + name
	}

	// Shorten path (use ~ for home directory)
	path := shortenPath(wt.Path, pathWidth-2)

	// Status badge with details
	status := StatusBadgeDetailed(wt.Status, wt.StagedCount, wt.ModifiedCount, wt.UntrackedCount, wt.UnpushedCount)

	// Style based on selection and current status
	var rowStyle lipgloss.Style
	if wt.IsCurrent && selected {
		rowStyle = TableRowCurrentSelectedStyle
	} else if wt.IsCurrent {
		rowStyle = TableRowCurrentStyle
	} else if selected {
		rowStyle = TableRowSelectedStyle
	} else {
		rowStyle = TableRowStyle
	}

	var nameStyle lipgloss.Style
	if wt.IsCurrent {
		nameStyle = WorktreeNameCurrentStyle
	} else {
		nameStyle = WorktreeNameStyle
	}

	nameCol := rowStyle.Width(nameWidth).Render(nameStyle.Render(truncate(name, nameWidth-2)))
	branchCol := rowStyle.Width(branchWidth).Render(WorktreeBranchStyle.Render(truncate(wt.Branch, branchWidth-2)))
	lastCommitCol := rowStyle.Width(lastCommitWidth).Render(WorktreePathStyle.Render(truncate(wt.LastCommit, lastCommitWidth-2)))
	statusCol := rowStyle.Width(statusWidth).Render(status)
	pathCol := rowStyle.Width(pathWidth).Render(WorktreePathStyle.Render(path))

	return lipgloss.JoinHorizontal(lipgloss.Top, nameCol, branchCol, lastCommitCol, statusCol, pathCol)
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
	actions := HelpItem("n", "new") + " " + HelpItem("d", "del") + " " + HelpItem("p", "prune")
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

	if wt.IsCurrent {
		lines = append(lines, titleStyle.Render("● "+wt.Name+" (current)"))
	} else {
		lines = append(lines, titleStyle.Render(wt.Name))
	}

	lines = append(lines, "") // spacing

	// Branch info
	labelStyle := lipgloss.NewStyle().Foreground(ColorTextMuted)
	valueStyle := lipgloss.NewStyle().Foreground(ColorText)

	lines = append(lines, labelStyle.Render("Branch"))
	lines = append(lines, "  "+WorktreeBranchStyle.Render(wt.Branch))
	lines = append(lines, "")

	// Path
	lines = append(lines, labelStyle.Render("Path"))
	shortPath := shortenPath(wt.Path, width-4)
	lines = append(lines, "  "+valueStyle.Render(shortPath))
	lines = append(lines, "")

	// Last commit
	lines = append(lines, labelStyle.Render("Last Commit"))
	if wt.LastCommit != "" {
		lines = append(lines, "  "+valueStyle.Render(wt.LastCommit))
	} else {
		lines = append(lines, "  "+WorktreePathStyle.Render("unknown"))
	}
	lines = append(lines, "")

	// Status details
	lines = append(lines, labelStyle.Render("Status"))
	if wt.StagedCount == 0 && wt.ModifiedCount == 0 && wt.UntrackedCount == 0 && wt.UnpushedCount == 0 {
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

// renderWithModal overlays a modal on top of a base view
func (m Model) renderWithModal(baseView, modalContent string) string {
	// Split base view into lines
	baseLines := strings.Split(baseView, "\n")

	// Modal dimensions
	modalWidth := 40
	modalHeight := strings.Count(modalContent, "\n") + 1

	// Calculate modal position (centered)
	startX := (m.width - modalWidth) / 2
	startY := (m.height - modalHeight) / 2

	if startX < 0 {
		startX = 0
	}
	if startY < 0 {
		startY = 0
	}

	// Style for the modal box
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(1, 2).
		Width(modalWidth)

	styledModal := modalStyle.Render(modalContent)
	modalLines := strings.Split(styledModal, "\n")

	// Overlay modal on base view
	result := make([]string, len(baseLines))
	copy(result, baseLines)

	// Ensure we have enough lines
	for len(result) < startY+len(modalLines) {
		result = append(result, strings.Repeat(" ", m.width))
	}

	// Overlay each modal line
	for i, modalLine := range modalLines {
		y := startY + i
		if y >= 0 && y < len(result) {
			baseLine := result[y]
			// Pad base line if needed
			for len(baseLine) < startX+lipgloss.Width(modalLine) {
				baseLine += " "
			}

			// Create new line with modal overlaid
			newLine := ""
			if startX > 0 && startX < len(baseLine) {
				newLine = baseLine[:startX]
			} else if startX > 0 {
				newLine = strings.Repeat(" ", startX)
			}
			newLine += modalLine

			// Add remaining part of base line if any
			endX := startX + lipgloss.Width(modalLine)
			if endX < len(baseLine) {
				newLine += baseLine[endX:]
			}

			result[y] = newLine
		}
	}

	return strings.Join(result, "\n")
}

// renderDeleteModal renders delete confirmation as a modal overlay
func (m Model) renderDeleteModal(baseView string) string {
	return m.renderWithModal(baseView, m.renderDeleteConfirmModal())
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
	if m.selected >= 0 && m.selected < len(m.worktrees) {
		wt := m.worktrees[m.selected]
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
