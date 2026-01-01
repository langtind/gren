package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRenderToolsMenu(t *testing.T) {
	m := Model{
		worktrees: []Worktree{
			{Branch: "main", IsMain: true},
			{Branch: "feature/test", PRNumber: 123, PRState: "OPEN"},
		},
		selected: 1, // Select feature branch with PR
	}

	result := m.renderToolsMenu()

	t.Run("contains title", func(t *testing.T) {
		if !strings.Contains(result, "Tools") {
			t.Error("Tools menu should contain title 'Tools'")
		}
	})

	t.Run("contains refresh action", func(t *testing.T) {
		if !strings.Contains(result, "Refresh") {
			t.Error("Tools menu should contain 'Refresh' action")
		}
	})

	t.Run("contains cleanup action", func(t *testing.T) {
		if !strings.Contains(result, "Cleanup") {
			t.Error("Tools menu should contain 'Cleanup' action")
		}
	})

	t.Run("contains prune action", func(t *testing.T) {
		if !strings.Contains(result, "Prune") {
			t.Error("Tools menu should contain 'Prune' action")
		}
	})

	t.Run("shows PR option when branch has PR", func(t *testing.T) {
		if !strings.Contains(result, "Open PR") {
			t.Error("Tools menu should show 'Open PR' when selected branch has PR")
		}
	})
}

func TestRenderToolsMenuNoPR(t *testing.T) {
	m := Model{
		worktrees: []Worktree{
			{Branch: "main", IsMain: true},
			{Branch: "feature/no-pr", PRNumber: 0},
		},
		selected: 1, // Select branch without PR
	}

	result := m.renderToolsMenu()

	// Open PR option should NOT appear for branches without PR
	if strings.Contains(result, "Open PR") {
		t.Error("Tools menu should NOT show 'Open PR' when selected branch has no PR")
	}
}

func TestHandleToolsKeysEscape(t *testing.T) {
	m := Model{
		currentView: ToolsView,
		keys:        DefaultKeyMap(),
	}

	msg := tea.KeyMsg{Type: tea.KeyEscape}
	newModel, cmd := m.handleToolsKeys(msg)

	if newModel.currentView != DashboardView {
		t.Errorf("ESC should return to DashboardView, got %v", newModel.currentView)
	}
	if cmd != nil {
		t.Error("ESC should not return a command")
	}
}

func TestHandleToolsKeysCleanup(t *testing.T) {
	m := Model{
		currentView: ToolsView,
		worktrees: []Worktree{
			{Branch: "main", IsMain: true, BranchStatus: "active"},
			{Branch: "feature/stale", BranchStatus: "stale", StaleReason: "pr_merged"},
		},
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}
	newModel, _ := m.handleToolsKeys(msg)

	if newModel.currentView != CleanupView {
		t.Errorf("'c' should switch to CleanupView, got %v", newModel.currentView)
	}
	if newModel.cleanupState == nil {
		t.Error("cleanupState should be initialized")
	}
}

func TestHandleToolsKeysCleanupNoStale(t *testing.T) {
	m := Model{
		currentView: ToolsView,
		worktrees: []Worktree{
			{Branch: "main", IsMain: true, BranchStatus: "active"},
			{Branch: "feature/active", BranchStatus: "active"},
		},
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}
	newModel, _ := m.handleToolsKeys(msg)

	// Should return to dashboard if no stale worktrees
	if newModel.currentView != DashboardView {
		t.Errorf("'c' with no stale worktrees should return to DashboardView, got %v", newModel.currentView)
	}
}

func TestGetToolActions(t *testing.T) {
	t.Run("without PR", func(t *testing.T) {
		actions := getToolActions(false, false)

		hasRefresh := false
		hasCleanup := false
		hasOpenPR := false

		for _, a := range actions {
			if strings.Contains(a.Name, "Refresh") {
				hasRefresh = true
			}
			if strings.Contains(a.Name, "Cleanup") {
				hasCleanup = true
			}
			if strings.Contains(a.Name, "Open PR") {
				hasOpenPR = true
			}
		}

		if !hasRefresh {
			t.Error("Should have Refresh action")
		}
		if !hasCleanup {
			t.Error("Should have Cleanup action")
		}
		if hasOpenPR {
			t.Error("Should NOT have Open PR action when hasPR=false")
		}
	})

	t.Run("with PR", func(t *testing.T) {
		actions := getToolActions(true, false)

		hasOpenPR := false
		for _, a := range actions {
			if strings.Contains(a.Name, "Open PR") {
				hasOpenPR = true
			}
		}

		if !hasOpenPR {
			t.Error("Should have Open PR action when hasPR=true")
		}
	})

	t.Run("with selected worktree", func(t *testing.T) {
		actions := getToolActions(false, true)

		hasMerge := false
		for _, a := range actions {
			if strings.Contains(a.Name, "Merge to main") {
				hasMerge = true
			}
		}

		if !hasMerge {
			t.Error("Should have Merge to main action when hasSelectedWorktree=true")
		}
	})
}
