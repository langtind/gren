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

func TestRenderCleanupConfirmation(t *testing.T) {
	m := Model{
		cleanupState: &CleanupState{},
		worktrees: []Worktree{
			{Branch: "main", IsMain: true, BranchStatus: "active"},
			{Branch: "feature/stale1", BranchStatus: "stale", StaleReason: "pr_merged", PRNumber: 95, PRState: "MERGED"},
			{Branch: "feature/stale2", BranchStatus: "stale", StaleReason: "no_unique_commits"},
			{Branch: "feature/active", BranchStatus: "active"},
		},
	}

	result := m.renderCleanupConfirmation()

	t.Run("contains warning header", func(t *testing.T) {
		if !strings.Contains(result, "Cleanup Stale Worktrees") {
			t.Error("Cleanup confirmation should contain header")
		}
	})

	t.Run("lists stale worktrees", func(t *testing.T) {
		if !strings.Contains(result, "feature/stale1") {
			t.Error("Should list stale1")
		}
		if !strings.Contains(result, "feature/stale2") {
			t.Error("Should list stale2")
		}
	})

	t.Run("shows PR info when available", func(t *testing.T) {
		if !strings.Contains(result, "#95") {
			t.Error("Should show PR number")
		}
		if !strings.Contains(result, "MERGED") {
			t.Error("Should show PR state")
		}
	})

	t.Run("excludes active worktrees", func(t *testing.T) {
		if strings.Contains(result, "feature/active") {
			t.Error("Should NOT list active worktrees")
		}
	})

	t.Run("excludes main worktree", func(t *testing.T) {
		// Main should never appear even if it had stale status
		// This is more of a sanity check
		if strings.Count(result, "main") > 0 && strings.Contains(result, "• main") {
			t.Error("Should NOT list main worktree in cleanup")
		}
	})

	t.Run("contains confirmation prompt", func(t *testing.T) {
		if !strings.Contains(result, "confirm") {
			t.Error("Should contain confirmation prompt")
		}
	})
}

func TestRenderCleanupConfirmationEmpty(t *testing.T) {
	m := Model{
		cleanupState: &CleanupState{},
		worktrees: []Worktree{
			{Branch: "main", IsMain: true, BranchStatus: "active"},
			{Branch: "feature/active", BranchStatus: "active"},
		},
	}

	result := m.renderCleanupConfirmation()

	if result != "" {
		t.Errorf("Should return empty string when no stale worktrees, got %q", result)
	}
}

func TestHandleCleanupKeysCancel(t *testing.T) {
	tests := []struct {
		name string
		key  tea.KeyMsg
	}{
		{"escape", tea.KeyMsg{Type: tea.KeyEscape}},
		{"n key", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				currentView:  CleanupView,
				cleanupState: &CleanupState{},
			}

			newModel, _ := m.handleCleanupKeys(tt.key)

			if newModel.currentView != DashboardView {
				t.Errorf("%s should return to DashboardView, got %v", tt.name, newModel.currentView)
			}
			if newModel.cleanupState != nil {
				t.Errorf("%s should clear cleanupState", tt.name)
			}
		})
	}
}

func TestHandleCleanupKeysConfirm(t *testing.T) {
	m := Model{
		currentView:  CleanupView,
		cleanupState: &CleanupState{},
		worktrees: []Worktree{
			{Branch: "feature/stale", BranchStatus: "stale"},
		},
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
	_, cmd := m.handleCleanupKeys(msg)

	if cmd == nil {
		t.Error("'y' should return a cleanup command")
	}
}

func TestGetToolActions(t *testing.T) {
	t.Run("without PR", func(t *testing.T) {
		actions := getToolActions(false)

		// Should have basic actions but not Open PR
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
		actions := getToolActions(true)

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
}
