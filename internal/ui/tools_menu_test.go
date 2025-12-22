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

// TestCleanupSubmoduleIndicator tests that submodule indicator is shown in cleanup UI
func TestCleanupSubmoduleIndicator(t *testing.T) {
	t.Run("shows submodule indicator for worktree with submodules", func(t *testing.T) {
		m := Model{
			cleanupState: &CleanupState{
				staleWorktrees: []Worktree{
					{Branch: "feature/with-submodules", BranchStatus: "stale", StaleReason: "pr_merged", HasSubmodules: true},
					{Branch: "feature/no-submodules", BranchStatus: "stale", StaleReason: "no_unique_commits", HasSubmodules: false},
				},
				selectedIndices: map[int]bool{0: true, 1: true},
				cursorIndex:     0,
			},
		}

		result := m.renderCleanupConfirmation()

		// Should show submodule indicator (📦) for worktree with submodules
		if !strings.Contains(result, "📦") {
			t.Error("Should show 📦 indicator for worktree with submodules")
		}

		// Should show legend when any worktree has submodules
		if !strings.Contains(result, "has submodules") {
			t.Error("Should show legend explaining 📦 indicator")
		}
	})

	t.Run("shows force delete label change when submodules selected", func(t *testing.T) {
		m := Model{
			cleanupState: &CleanupState{
				staleWorktrees: []Worktree{
					{Branch: "feature/with-submodules", BranchStatus: "stale", StaleReason: "pr_merged", HasSubmodules: true},
				},
				selectedIndices: map[int]bool{0: true}, // Submodule worktree selected
				cursorIndex:     0,
			},
		}

		result := m.renderCleanupConfirmation()

		// Should show "required for submodules" in force delete label
		if !strings.Contains(result, "required for submodules") {
			t.Error("Force delete label should mention 'required for submodules' when submodule worktree is selected")
		}
	})

	t.Run("shows normal force delete label when no submodules selected", func(t *testing.T) {
		m := Model{
			cleanupState: &CleanupState{
				staleWorktrees: []Worktree{
					{Branch: "feature/no-submodules", BranchStatus: "stale", StaleReason: "pr_merged", HasSubmodules: false},
				},
				selectedIndices: map[int]bool{0: true},
				cursorIndex:     0,
			},
		}

		result := m.renderCleanupConfirmation()

		// Should show normal force delete label (not "required for submodules")
		if strings.Contains(result, "required for submodules") {
			t.Error("Force delete label should NOT mention 'required for submodules' when no submodule worktrees are selected")
		}
		if !strings.Contains(result, "ignore uncommitted changes") {
			t.Error("Force delete label should mention 'ignore uncommitted changes' when no submodules")
		}
	})

	t.Run("no submodule legend when no worktrees have submodules", func(t *testing.T) {
		m := Model{
			cleanupState: &CleanupState{
				staleWorktrees: []Worktree{
					{Branch: "feature/no-submodules1", BranchStatus: "stale", StaleReason: "pr_merged", HasSubmodules: false},
					{Branch: "feature/no-submodules2", BranchStatus: "stale", StaleReason: "no_unique_commits", HasSubmodules: false},
				},
				selectedIndices: map[int]bool{0: true, 1: true},
				cursorIndex:     0,
			},
		}

		result := m.renderCleanupConfirmation()

		// Should NOT show submodule indicator or legend
		if strings.Contains(result, "📦") {
			t.Error("Should NOT show 📦 indicator when no worktrees have submodules")
		}
	})
}

// TestCleanupAutoForceDeleteForSubmodules tests that force delete is auto-enabled for submodules
func TestCleanupAutoForceDeleteForSubmodules(t *testing.T) {
	t.Run("auto-enables force delete when confirming with submodules selected", func(t *testing.T) {
		m := Model{
			currentView: CleanupView,
			cleanupState: &CleanupState{
				staleWorktrees: []Worktree{
					{Branch: "feature/with-submodules", BranchStatus: "stale", HasSubmodules: true},
					{Branch: "feature/no-submodules", BranchStatus: "stale", HasSubmodules: false},
				},
				selectedIndices: map[int]bool{0: true}, // Only submodule worktree selected
				cursorIndex:     0,
				forceDelete:     false, // Not manually enabled
			},
		}

		msg := tea.KeyMsg{Type: tea.KeyEnter}
		newModel, cmd := m.handleCleanupKeys(msg)

		// Should auto-enable forceDelete
		if !newModel.cleanupState.forceDelete {
			t.Error("forceDelete should be auto-enabled when confirming with submodule worktrees selected")
		}

		// Should start cleanup
		if cmd == nil {
			t.Error("Should return cleanup command")
		}
		if !newModel.cleanupState.confirmed {
			t.Error("Should set confirmed=true")
		}
	})

	t.Run("does not auto-enable force delete when no submodules selected", func(t *testing.T) {
		m := Model{
			currentView: CleanupView,
			cleanupState: &CleanupState{
				staleWorktrees: []Worktree{
					{Branch: "feature/with-submodules", BranchStatus: "stale", HasSubmodules: true},
					{Branch: "feature/no-submodules", BranchStatus: "stale", HasSubmodules: false},
				},
				selectedIndices: map[int]bool{1: true}, // Only non-submodule worktree selected
				cursorIndex:     1,
				forceDelete:     false,
			},
		}

		msg := tea.KeyMsg{Type: tea.KeyEnter}
		newModel, _ := m.handleCleanupKeys(msg)

		// Should NOT auto-enable forceDelete
		if newModel.cleanupState.forceDelete {
			t.Error("forceDelete should NOT be auto-enabled when no submodule worktrees are selected")
		}
	})

	t.Run("preserves manually enabled force delete", func(t *testing.T) {
		m := Model{
			currentView: CleanupView,
			cleanupState: &CleanupState{
				staleWorktrees: []Worktree{
					{Branch: "feature/no-submodules", BranchStatus: "stale", HasSubmodules: false},
				},
				selectedIndices: map[int]bool{0: true},
				cursorIndex:     0,
				forceDelete:     true, // Manually enabled
			},
		}

		msg := tea.KeyMsg{Type: tea.KeyEnter}
		newModel, _ := m.handleCleanupKeys(msg)

		// Should preserve forceDelete
		if !newModel.cleanupState.forceDelete {
			t.Error("forceDelete should remain true when manually enabled")
		}
	})
}

// TestCleanupForceDeleteCheckboxAutoCheck tests that force checkbox appears checked when submodules are selected
func TestCleanupForceDeleteCheckboxAutoCheck(t *testing.T) {
	t.Run("force checkbox shown as checked when submodules selected", func(t *testing.T) {
		m := Model{
			cleanupState: &CleanupState{
				staleWorktrees: []Worktree{
					{Branch: "feature/with-submodules", BranchStatus: "stale", HasSubmodules: true},
				},
				selectedIndices: map[int]bool{0: true}, // Submodule selected
				cursorIndex:     0,
				forceDelete:     false, // Not manually toggled, but should appear checked
			},
		}

		result := m.renderCleanupConfirmation()

		// The force delete checkbox should appear checked when submodules are selected
		// Even though forceDelete=false, the visual should show [✓] because submodules are selected
		if !strings.Contains(result, "[✓]") {
			t.Error("Force delete checkbox should appear checked when submodule worktrees are selected")
		}
	})
}

// TestCleanupProgressShowsOnlySelectedWorktrees verifies that the progress view
// only displays worktrees that were actually selected for deletion, not all stale worktrees.
// Bug: When user selects 2 of 3 stale worktrees, progress showed all 3 with "0/3" instead of "0/2".
func TestCleanupProgressShowsOnlySelectedWorktrees(t *testing.T) {
	t.Run("progress shows only selected worktrees not all stale", func(t *testing.T) {
		m := Model{
			cleanupState: &CleanupState{
				staleWorktrees: []Worktree{
					{Branch: "feature/generate-endpoint", BranchStatus: "stale", StaleReason: "pr_merged"},
					{Branch: "vid-342-supabase-tag", BranchStatus: "stale", StaleReason: "pr_merged"},
					{Branch: "vid-359-fixing-failing-tests", BranchStatus: "stale", StaleReason: "pr_merged"},
				},
				// Only 2 of 3 worktrees selected (indices 1 and 2)
				selectedIndices: map[int]bool{1: true, 2: true},
				deletedIndices:  map[int]bool{},
				failedWorktrees: map[int]string{},
				inProgress:      true,
				currentIndex:    1, // Currently deleting first selected
				totalCleaned:    0,
				totalFailed:     0,
			},
		}

		result := m.renderCleanupProgress()

		// Should show "0/2" not "0/3" since only 2 worktrees were selected
		if strings.Contains(result, "0/3") {
			t.Error("Progress should show 0/2 (only selected count), but shows 0/3 (all stale count)")
		}
		if !strings.Contains(result, "0/2") {
			t.Errorf("Progress should show '0/2' for 2 selected worktrees, got: %s", result)
		}

		// Should NOT show the unselected worktree (feature/generate-endpoint)
		if strings.Contains(result, "feature/generate-endpoint") {
			t.Error("Progress should NOT show unselected worktree 'feature/generate-endpoint'")
		}

		// Should show the selected worktrees
		if !strings.Contains(result, "vid-342-supabase-tag") {
			t.Error("Progress should show selected worktree 'vid-342-supabase-tag'")
		}
		if !strings.Contains(result, "vid-359-fixing-failing-tests") {
			t.Error("Progress should show selected worktree 'vid-359-fixing-failing-tests'")
		}
	})

	t.Run("progress total matches selected count after partial deletion", func(t *testing.T) {
		m := Model{
			cleanupState: &CleanupState{
				staleWorktrees: []Worktree{
					{Branch: "feature/unselected", BranchStatus: "stale"},
					{Branch: "feature/selected1", BranchStatus: "stale"},
					{Branch: "feature/selected2", BranchStatus: "stale"},
				},
				selectedIndices: map[int]bool{1: true, 2: true}, // 2 selected
				deletedIndices:  map[int]bool{1: true},          // 1 already deleted
				failedWorktrees: map[int]string{},
				inProgress:      true,
				currentIndex:    2, // Now deleting second selected
				totalCleaned:    1,
				totalFailed:     0,
			},
		}

		result := m.renderCleanupProgress()

		// Should show "1/2" - 1 deleted out of 2 selected
		if !strings.Contains(result, "1/2") {
			t.Errorf("Progress should show '1/2' after 1 of 2 selected deleted, got: %s", result)
		}

		// Should NOT show unselected or deleted worktrees
		if strings.Contains(result, "feature/unselected") {
			t.Error("Should NOT show unselected worktree")
		}
		if strings.Contains(result, "feature/selected1") {
			t.Error("Should NOT show already deleted worktree")
		}

		// Should show the remaining selected worktree being deleted
		if !strings.Contains(result, "feature/selected2") {
			t.Error("Should show the currently deleting worktree")
		}
	})
}
