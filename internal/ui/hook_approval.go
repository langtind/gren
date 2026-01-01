package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/langtind/gren/internal/config"
	"github.com/langtind/gren/internal/core"
)

// handleHookApprovalKeys handles keyboard input for the hook approval modal
func (m Model) handleHookApprovalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("tab"))):
		// Toggle between Approve and Skip
		m.hookApprovalState.selectedIndex = (m.hookApprovalState.selectedIndex + 1) % 2
		return m, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("left", "h"))):
		// Move to Approve
		m.hookApprovalState.selectedIndex = 0
		return m, nil

	case key.Matches(msg, key.NewBinding(key.WithKeys("right", "l"))):
		// Move to Skip
		m.hookApprovalState.selectedIndex = 1
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		// Confirm selection
		if m.hookApprovalState.selectedIndex == 0 {
			// Approve and run hooks
			return m.approveAndRunHooks()
		} else {
			// Skip hooks
			m.skipHooks()
		}
		return m, nil

	case key.Matches(msg, m.keys.Back):
		// Esc = Skip hooks
		m.skipHooks()
		return m, nil
	}

	return m, nil
}

// renderHookApprovalOverlay renders the hook approval modal overlay
func (m Model) renderHookApprovalOverlay(baseView string) string {
	if m.hookApprovalState == nil || !m.hookApprovalState.visible {
		return baseView
	}

	// Build modal content
	var content strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorWarning)
	content.WriteString(titleStyle.Render("Hook needs approval"))
	content.WriteString("\n\n")

	// Description
	content.WriteString("The following commands will run:\n\n")

	// List commands
	cmdStyle := lipgloss.NewStyle().
		Foreground(ColorSecondary).
		PaddingLeft(2)
	for _, cmd := range m.hookApprovalState.commands {
		content.WriteString(cmdStyle.Render(fmt.Sprintf("• %s", cmd)))
		content.WriteString("\n")
	}
	content.WriteString("\n")

	// Buttons
	approveStyle := lipgloss.NewStyle().
		Padding(0, 2).
		Background(ColorSuccess).
		Foreground(lipgloss.Color("#ffffff"))
	skipStyle := lipgloss.NewStyle().
		Padding(0, 2).
		Background(ColorTextMuted).
		Foreground(lipgloss.Color("#ffffff"))

	if m.hookApprovalState.selectedIndex == 0 {
		approveStyle = approveStyle.Bold(true).Underline(true)
	} else {
		skipStyle = skipStyle.Bold(true).Underline(true)
	}

	content.WriteString(approveStyle.Render("Approve & Run"))
	content.WriteString("  ")
	content.WriteString(skipStyle.Render("Skip"))
	content.WriteString("\n\n")

	// Help text
	helpStyle := lipgloss.NewStyle().
		Foreground(ColorTextMuted).
		Italic(true)
	content.WriteString(helpStyle.Render("Tab: switch • Enter: confirm • Esc: skip"))

	// Modal box
	modalWidth := 50
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorWarning).
		Padding(1, 2).
		Width(modalWidth)

	modal := modalStyle.Render(content.String())

	// Center the modal
	return m.centerOverlay(baseView, modal)
}

// centerOverlay centers a modal on top of a base view
func (m Model) centerOverlay(baseView, modal string) string {
	modalLines := strings.Split(modal, "\n")
	modalHeight := len(modalLines)
	modalWidth := 0
	for _, line := range modalLines {
		if lipgloss.Width(line) > modalWidth {
			modalWidth = lipgloss.Width(line)
		}
	}

	baseLines := strings.Split(baseView, "\n")

	// Calculate position
	startY := (m.height - modalHeight) / 2
	startX := (m.width - modalWidth) / 2

	if startY < 0 {
		startY = 0
	}
	if startX < 0 {
		startX = 0
	}

	// Overlay modal on base
	result := make([]string, len(baseLines))
	copy(result, baseLines)

	for i, modalLine := range modalLines {
		targetY := startY + i
		if targetY >= 0 && targetY < len(result) {
			baseLine := result[targetY]
			baseWidth := lipgloss.Width(baseLine)

			// Pad base line if needed
			if baseWidth < startX+modalWidth {
				baseLine = baseLine + strings.Repeat(" ", startX+modalWidth-baseWidth)
			}

			// Insert modal line
			prefix := ""
			if startX > 0 && len(baseLine) >= startX {
				prefix = baseLine[:startX]
			} else if startX > 0 {
				prefix = strings.Repeat(" ", startX)
			}

			suffix := ""
			endX := startX + lipgloss.Width(modalLine)
			if endX < len(baseLine) {
				suffix = baseLine[endX:]
			}

			result[targetY] = prefix + modalLine + suffix
		}
	}

	return strings.Join(result, "\n")
}

// showHookApproval shows the hook approval overlay for pending hooks
func (m *Model) showHookApproval(hookType config.HookType, worktreePath, branchName, baseBranch string) {
	wm := core.NewWorktreeManager(m.gitRepo, m.configManager)
	unapproved := wm.GetUnapprovedHooks(hookType)
	hasInteractive := wm.HasInteractiveHooks(hookType)

	if len(unapproved) == 0 {
		// All hooks already approved, run them directly
		// Note: If interactive, we still need to suspend TUI - handled by caller
		wm.RunPostCreateHookWithApproval(worktreePath, branchName, baseBranch, true)
		return
	}

	// Show approval modal
	m.hookApprovalState = &HookApprovalState{
		visible:        true,
		hookType:       string(hookType),
		commands:       unapproved,
		worktreePath:   worktreePath,
		branchName:     branchName,
		baseBranch:     baseBranch,
		selectedIndex:  0, // Default to "Approve"
		hasInteractive: hasInteractive,
	}
}

// approveAndRunHooks approves all pending hooks and runs them.
// For interactive hooks, returns a tea.Exec command to suspend the TUI.
func (m Model) approveAndRunHooks() (tea.Model, tea.Cmd) {
	if m.hookApprovalState == nil {
		return m, nil
	}

	state := m.hookApprovalState
	m.hookApprovalState = nil // Close modal

	if state.hasInteractive {
		// For interactive hooks, we need to suspend TUI and run hooks with terminal access
		// Create a shell command that runs the hooks
		wm := core.NewWorktreeManager(m.gitRepo, m.configManager)

		// First approve the hooks
		projectID, _ := config.GetProjectID()
		am := config.NewApprovalManager()
		am.ApproveAll(projectID, state.commands)

		// Create a command that runs the hooks
		// We use a callback to run hooks after TUI suspends
		return m, tea.ExecProcess(
			createHookRunnerCmd(wm, state),
			func(err error) tea.Msg {
				return hookExecutionCompleteMsg{err: err}
			},
		)
	}

	// For non-interactive hooks, run directly
	wm := core.NewWorktreeManager(m.gitRepo, m.configManager)
	wm.RunPostCreateHookWithApproval(
		state.worktreePath,
		state.branchName,
		state.baseBranch,
		true, // auto-approve
	)

	return m, nil
}

// hookExecutionCompleteMsg is sent when hook execution completes
type hookExecutionCompleteMsg struct {
	err error
}

// createHookRunnerCmd creates an exec.Cmd that runs the hooks.
// This is a workaround since we can't directly run Go code with tea.Exec.
func createHookRunnerCmd(wm *core.WorktreeManager, state *HookApprovalState) *exec.Cmd {
	// We'll use the gren binary itself to run hooks
	// This is a bit of a hack, but it works
	grenPath, _ := os.Executable()
	cmd := exec.Command(grenPath, "hook-run",
		"--type", state.hookType,
		"--path", state.worktreePath,
		"--branch", state.branchName,
		"--base", state.baseBranch,
	)
	return cmd
}

// skipHooks closes the approval modal without running hooks
func (m *Model) skipHooks() {
	m.hookApprovalState = nil
}
