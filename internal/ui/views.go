package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) renderMergeView() string {
	if m.mergeState == nil {
		return ""
	}

	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	keyStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary)
	labelStyle := lipgloss.NewStyle().Foreground(ColorText)
	mutedStyle := lipgloss.NewStyle().Foreground(ColorTextMuted)

	switch m.mergeState.currentStep {
	case MergeStepConfirm:
		b.WriteString(titleStyle.Render("Merge to Main"))
		b.WriteString("\n\n")

		if m.mergeState.sourceWorktree != nil {
			b.WriteString(labelStyle.Render("Source: "))
			b.WriteString(m.mergeState.sourceWorktree.Branch)
			b.WriteString("\n")
		}
		b.WriteString(labelStyle.Render("Target: "))
		b.WriteString(m.mergeState.targetBranch)
		b.WriteString("\n\n")

		b.WriteString(labelStyle.Render("Options:"))
		b.WriteString("\n")

		squashCheck := "[ ]"
		if m.mergeState.squash {
			squashCheck = "[✓]"
		}
		b.WriteString(fmt.Sprintf("  %s %s Squash commits\n", keyStyle.Render("s"), squashCheck))

		rebaseCheck := "[ ]"
		if m.mergeState.rebase {
			rebaseCheck = "[✓]"
		}
		b.WriteString(fmt.Sprintf("  %s %s Rebase before merge\n", keyStyle.Render("r"), rebaseCheck))

		removeCheck := "[ ]"
		if m.mergeState.remove {
			removeCheck = "[✓]"
		}
		b.WriteString(fmt.Sprintf("  %s %s Remove worktree after merge\n", keyStyle.Render("d"), removeCheck))

		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("Press enter to merge • esc to cancel"))

	case MergeStepInProgress:
		b.WriteString(titleStyle.Render("Merging..."))
		b.WriteString("\n\n")
		if m.mergeState.progressMsg != "" {
			b.WriteString(m.mergeState.progressMsg)
		} else {
			b.WriteString("Please wait...")
		}

	case MergeStepComplete:
		if m.mergeState.err != nil {
			b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ColorError).Render("✗ Merge Failed"))
			b.WriteString("\n\n")
			b.WriteString(m.mergeState.err.Error())
		} else {
			b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ColorSuccess).Render("✓ Merge Complete"))
			b.WriteString("\n\n")
			b.WriteString(m.mergeState.result)
		}
		b.WriteString("\n\n")
		b.WriteString(mutedStyle.Render("Press enter or esc to close"))
	}

	return b.String()
}

func (m Model) renderForEachView() string {
	if m.forEachState == nil {
		return ""
	}

	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	labelStyle := lipgloss.NewStyle().Foreground(ColorText)
	mutedStyle := lipgloss.NewStyle().Foreground(ColorTextMuted)
	inputStyle := lipgloss.NewStyle().Foreground(ColorSecondary)

	if m.forEachState.inputMode {
		b.WriteString(titleStyle.Render("Run in All Worktrees"))
		b.WriteString("\n\n")

		b.WriteString(labelStyle.Render("Command: "))
		b.WriteString(inputStyle.Render(m.forEachState.command))
		b.WriteString("█")
		b.WriteString("\n\n")

		skipMainCheck := "[ ]"
		if m.forEachState.skipMain {
			skipMainCheck = "[✓]"
		}
		b.WriteString(fmt.Sprintf("  %s Skip main worktree (tab to toggle)\n", skipMainCheck))

		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("Type command and press enter • esc to cancel"))
	} else if m.forEachState.inProgress {
		b.WriteString(titleStyle.Render("Running..."))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("Processing worktree %d...", m.forEachState.currentIndex+1))
	} else if len(m.forEachState.results) > 0 {
		successCount := 0
		failCount := 0
		for _, r := range m.forEachState.results {
			if r.Success {
				successCount++
			} else {
				failCount++
			}
		}

		if failCount == 0 {
			b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ColorSuccess).Render("✓ Complete"))
		} else {
			b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ColorWarning).Render("⚠ Complete with errors"))
		}
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("Success: %d, Failed: %d\n\n", successCount, failCount))

		for _, r := range m.forEachState.results {
			statusIcon := "✓"
			statusColor := ColorSuccess
			if !r.Success {
				statusIcon = "✗"
				statusColor = ColorError
			}
			b.WriteString(lipgloss.NewStyle().Foreground(statusColor).Render(statusIcon))
			b.WriteString(" ")
			b.WriteString(r.Worktree)
			b.WriteString("\n")
		}

		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("Press enter or esc to close"))
	}

	return b.String()
}

func (m Model) renderStepCommitView() string {
	if m.stepCommitState == nil {
		return ""
	}

	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
	labelStyle := lipgloss.NewStyle().Foreground(ColorText)
	mutedStyle := lipgloss.NewStyle().Foreground(ColorTextMuted)
	inputStyle := lipgloss.NewStyle().Foreground(ColorSecondary)

	switch m.stepCommitState.currentStep {
	case StepCommitStepOptions:
		b.WriteString(titleStyle.Render("Commit Changes"))
		b.WriteString("\n\n")
		b.WriteString(labelStyle.Render("Stage all changes and create a commit."))
		b.WriteString("\n\n")

		llmCheck := "[ ]"
		if m.stepCommitState.useLLM {
			llmCheck = "[✓]"
		}
		b.WriteString(fmt.Sprintf("  %s Generate message with AI (tab to toggle)\n", llmCheck))

		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("Press enter to continue • esc to cancel"))

	case StepCommitStepGenerating:
		b.WriteString(titleStyle.Render("Generating Commit Message..."))
		b.WriteString("\n\n")
		b.WriteString("Please wait while the AI generates a commit message.")
		b.WriteString("\n\n")
		b.WriteString(mutedStyle.Render("esc to cancel"))

	case StepCommitStepMessage:
		if m.stepCommitState.useLLM {
			b.WriteString(titleStyle.Render("Review Commit Message"))
			b.WriteString("\n\n")
			b.WriteString(labelStyle.Render("AI-generated message (edit if needed):"))
		} else {
			b.WriteString(titleStyle.Render("Commit Message"))
			b.WriteString("\n\n")
			b.WriteString(labelStyle.Render("Message: "))
		}
		b.WriteString("\n")
		b.WriteString(inputStyle.Render(m.stepCommitState.message))
		b.WriteString("█")
		b.WriteString("\n\n")
		b.WriteString(mutedStyle.Render("Edit message and press enter to commit • esc to go back"))

	case StepCommitStepInProgress:
		b.WriteString(titleStyle.Render("Committing..."))
		b.WriteString("\n\n")
		b.WriteString("Please wait...")

	case StepCommitStepComplete:
		if m.stepCommitState.err != nil {
			b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ColorError).Render("✗ Commit Failed"))
			b.WriteString("\n\n")
			b.WriteString(m.stepCommitState.err.Error())
		} else {
			b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(ColorSuccess).Render("✓ Commit Complete"))
			b.WriteString("\n\n")
			b.WriteString(m.stepCommitState.result)
		}
		b.WriteString("\n\n")
		b.WriteString(mutedStyle.Render("Press enter or esc to close"))
	}

	return b.String()
}

func (m Model) settingsView() string {
	title := TitleStyle.Render("⚙️ Settings")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		"Configuration: [coming soon]",
		"",
		HelpStyle.Render("[esc] Back to dashboard"),
	)

	return HeaderStyle.Width(m.width - 4).Render(content)
}
