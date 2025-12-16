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
		cleanupState: &CleanupState{
			staleWorktrees: []Worktree{
				{Branch: "feature/stale1", BranchStatus: "stale", StaleReason: "pr_merged", PRNumber: 95, PRState: "MERGED"},
				{Branch: "feature/stale2", BranchStatus: "stale", StaleReason: "no_unique_commits"},
			},
			selectedIndices: map[int]bool{0: true, 1: true}, // Both selected by default
			cursorIndex:     0,
		},
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
		if !strings.Contains(result, "enter") {
			t.Error("Should contain enter key prompt for confirmation")
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				currentView: CleanupView,
				cleanupState: &CleanupState{
					staleWorktrees: []Worktree{
						{Branch: "feature/stale", BranchStatus: "stale"},
					},
					selectedIndices: map[int]bool{0: true},
					cursorIndex:     0,
				},
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
		currentView: CleanupView,
		cleanupState: &CleanupState{
			staleWorktrees: []Worktree{
				{Branch: "feature/stale", BranchStatus: "stale"},
			},
			selectedIndices: map[int]bool{0: true},
			cursorIndex:     0,
		},
		worktrees: []Worktree{
			{Branch: "feature/stale", BranchStatus: "stale"},
		},
	}

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	_, cmd := m.handleCleanupKeys(msg)

	if cmd == nil {
		t.Error("'enter' should return a cleanup command when items are selected")
	}
}

func TestHandleCleanupKeysConfirmNoSelection(t *testing.T) {
	m := Model{
		currentView: CleanupView,
		cleanupState: &CleanupState{
			staleWorktrees: []Worktree{
				{Branch: "feature/stale", BranchStatus: "stale"},
			},
			selectedIndices: map[int]bool{}, // Nothing selected
			cursorIndex:     0,
		},
	}

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	newModel, cmd := m.handleCleanupKeys(msg)

	// Should not start cleanup when nothing is selected
	if cmd != nil {
		t.Error("'enter' should not return a command when nothing is selected")
	}
	if newModel.cleanupState.confirmed {
		t.Error("Should not set confirmed=true when nothing is selected")
	}
}

func TestHandleCleanupKeysNavigation(t *testing.T) {
	tests := []struct {
		name           string
		key            tea.KeyMsg
		startIndex     int
		expectedIndex  int
		totalWorktrees int
	}{
		{"down arrow", tea.KeyMsg{Type: tea.KeyDown}, 0, 1, 3},
		{"j key", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, 0, 1, 3},
		{"up arrow", tea.KeyMsg{Type: tea.KeyUp}, 1, 0, 3},
		{"k key", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}, 1, 0, 3},
		{"down at end", tea.KeyMsg{Type: tea.KeyDown}, 2, 2, 3}, // Should stay at last
		{"up at start", tea.KeyMsg{Type: tea.KeyUp}, 0, -1, 3},  // Should go to force delete option
		{"up at force", tea.KeyMsg{Type: tea.KeyUp}, -1, -1, 3}, // Should stay at force delete (top)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			staleWorktrees := make([]Worktree, tt.totalWorktrees)
			for i := 0; i < tt.totalWorktrees; i++ {
				staleWorktrees[i] = Worktree{Branch: "feature/stale" + string(rune('1'+i)), BranchStatus: "stale"}
			}

			m := Model{
				currentView: CleanupView,
				cleanupState: &CleanupState{
					staleWorktrees:  staleWorktrees,
					selectedIndices: map[int]bool{0: true, 1: true, 2: true},
					cursorIndex:     tt.startIndex,
				},
			}

			newModel, _ := m.handleCleanupKeys(tt.key)

			if newModel.cleanupState.cursorIndex != tt.expectedIndex {
				t.Errorf("%s: expected cursor at %d, got %d", tt.name, tt.expectedIndex, newModel.cleanupState.cursorIndex)
			}
		})
	}
}

func TestHandleCleanupKeysToggleSelection(t *testing.T) {
	t.Run("deselect item", func(t *testing.T) {
		m := Model{
			currentView: CleanupView,
			cleanupState: &CleanupState{
				staleWorktrees: []Worktree{
					{Branch: "feature/stale1", BranchStatus: "stale"},
					{Branch: "feature/stale2", BranchStatus: "stale"},
				},
				selectedIndices: map[int]bool{0: true, 1: true}, // Both selected
				cursorIndex:     0,
			},
		}

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}}
		newModel, _ := m.handleCleanupKeys(msg)

		// Index 0 should be deselected
		if newModel.cleanupState.selectedIndices[0] {
			t.Error("Space should deselect item at cursor")
		}
		// Index 1 should still be selected
		if !newModel.cleanupState.selectedIndices[1] {
			t.Error("Other items should remain selected")
		}
	})

	t.Run("select item", func(t *testing.T) {
		m := Model{
			currentView: CleanupView,
			cleanupState: &CleanupState{
				staleWorktrees: []Worktree{
					{Branch: "feature/stale1", BranchStatus: "stale"},
					{Branch: "feature/stale2", BranchStatus: "stale"},
				},
				selectedIndices: map[int]bool{1: true}, // Only 1 selected
				cursorIndex:     0,
			},
		}

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}}
		newModel, _ := m.handleCleanupKeys(msg)

		// Index 0 should now be selected
		if !newModel.cleanupState.selectedIndices[0] {
			t.Error("Space should select item at cursor")
		}
		// Index 1 should still be selected
		if !newModel.cleanupState.selectedIndices[1] {
			t.Error("Other items should remain selected")
		}
	})
}

func TestCleanupStateInitialization(t *testing.T) {
	// This tests the initialization in tools_menu.go when 'c' is pressed
	m := Model{
		currentView: ToolsView,
		worktrees: []Worktree{
			{Branch: "main", IsMain: true, BranchStatus: "active"},
			{Branch: "feature/stale1", BranchStatus: "stale", StaleReason: "pr_merged"},
			{Branch: "feature/stale2", BranchStatus: "stale", StaleReason: "no_unique_commits"},
			{Branch: "feature/current", IsCurrent: true, BranchStatus: "stale"}, // Should be excluded
		},
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}
	newModel, _ := m.handleToolsKeys(msg)

	if newModel.cleanupState == nil {
		t.Fatal("cleanupState should be initialized")
	}

	// Should have 2 stale worktrees (excluding main and current)
	if len(newModel.cleanupState.staleWorktrees) != 2 {
		t.Errorf("Expected 2 stale worktrees, got %d", len(newModel.cleanupState.staleWorktrees))
	}

	// All should be selected by default
	if len(newModel.cleanupState.selectedIndices) != 2 {
		t.Errorf("Expected all 2 worktrees to be selected by default, got %d", len(newModel.cleanupState.selectedIndices))
	}

	// Cursor should start at 0
	if newModel.cleanupState.cursorIndex != 0 {
		t.Errorf("Expected cursor at 0, got %d", newModel.cleanupState.cursorIndex)
	}

	// Should transition to CleanupView
	if newModel.currentView != CleanupView {
		t.Errorf("Expected CleanupView, got %v", newModel.currentView)
	}
}

func TestRenderCleanupConfirmationWithSelection(t *testing.T) {
	m := Model{
		cleanupState: &CleanupState{
			staleWorktrees: []Worktree{
				{Branch: "feature/stale1", BranchStatus: "stale", StaleReason: "pr_merged", PRNumber: 95, PRState: "MERGED"},
				{Branch: "feature/stale2", BranchStatus: "stale", StaleReason: "no_unique_commits"},
			},
			selectedIndices: map[int]bool{0: true, 1: true}, // Both selected
			cursorIndex:     0,
		},
	}

	result := m.renderCleanupConfirmation()

	t.Run("shows selection count", func(t *testing.T) {
		if !strings.Contains(result, "(2/2 selected)") {
			t.Error("Should show selection count")
		}
	})

	t.Run("shows checkboxes", func(t *testing.T) {
		if !strings.Contains(result, "[✓]") {
			t.Error("Should show checked checkbox for selected items")
		}
	})

	t.Run("shows cursor indicator", func(t *testing.T) {
		if !strings.Contains(result, "> ") {
			t.Error("Should show cursor indicator")
		}
	})

	t.Run("shows navigation help", func(t *testing.T) {
		if !strings.Contains(result, "space") {
			t.Error("Should show space toggle help")
		}
		if !strings.Contains(result, "navigate") {
			t.Error("Should show navigation help")
		}
	})
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
