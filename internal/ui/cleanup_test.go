package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

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
			{Branch: "feature/stale1", BranchStatus: "stale", StaleReason: "pr_merged"},         // Safe: pr_merged + clean
			{Branch: "feature/stale2", BranchStatus: "stale", StaleReason: "no_unique_commits"}, // Unsafe: no_unique_commits
			{Branch: "feature/current", IsCurrent: true, BranchStatus: "stale"},                 // Should be excluded (current)
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

	// Only SAFE worktrees should be pre-selected:
	// - pr_merged + clean = pre-selected
	// - no_unique_commits = NOT pre-selected (could be new branch)
	if len(newModel.cleanupState.selectedIndices) != 1 {
		t.Errorf("Expected 1 safe worktree to be pre-selected, got %d", len(newModel.cleanupState.selectedIndices))
	}

	// Verify the correct worktree is selected (the pr_merged one)
	if !newModel.cleanupState.selectedIndices[0] {
		t.Error("Expected pr_merged worktree (index 0) to be pre-selected")
	}
	if newModel.cleanupState.selectedIndices[1] {
		t.Error("no_unique_commits worktree (index 1) should NOT be pre-selected")
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

// TestCleanupPreSelectionSafety tests that worktrees with uncommitted changes
// are NOT pre-selected for deletion, even if they are marked as stale.
// This prevents accidental data loss from work-in-progress branches.
func TestCleanupPreSelectionSafety(t *testing.T) {
	t.Run("worktree with no_unique_commits but uncommitted changes should NOT be pre-selected", func(t *testing.T) {
		m := Model{
			currentView: ToolsView,
			worktrees: []Worktree{
				{Branch: "main", IsMain: true, BranchStatus: "active"},
				{
					Branch:        "feature/wip",
					BranchStatus:  "stale",
					StaleReason:   "no_unique_commits",
					ModifiedCount: 3, // Has uncommitted changes!
				},
			},
		}

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}
		newModel, _ := m.handleToolsKeys(msg)

		if newModel.cleanupState == nil {
			t.Fatal("cleanupState should be initialized")
		}

		// Should NOT be pre-selected because it has uncommitted changes
		if newModel.cleanupState.selectedIndices[0] {
			t.Error("Worktree with uncommitted changes should NOT be pre-selected, even with no_unique_commits")
		}
	})

	t.Run("worktree with no_unique_commits and clean should NOT be pre-selected", func(t *testing.T) {
		// A clean worktree with no_unique_commits could be a new branch - user should decide
		m := Model{
			currentView: ToolsView,
			worktrees: []Worktree{
				{Branch: "main", IsMain: true, BranchStatus: "active"},
				{
					Branch:         "feature/new-branch",
					BranchStatus:   "stale",
					StaleReason:    "no_unique_commits",
					ModifiedCount:  0,
					StagedCount:    0,
					UntrackedCount: 0,
				},
			},
		}

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}
		newModel, _ := m.handleToolsKeys(msg)

		if newModel.cleanupState == nil {
			t.Fatal("cleanupState should be initialized")
		}

		// Should NOT be pre-selected - could be a new branch user just started
		if newModel.cleanupState.selectedIndices[0] {
			t.Error("Worktree with no_unique_commits (even if clean) should NOT be pre-selected")
		}
	})

	t.Run("worktree with pr_merged but uncommitted changes should NOT be pre-selected", func(t *testing.T) {
		m := Model{
			currentView: ToolsView,
			worktrees: []Worktree{
				{Branch: "main", IsMain: true, BranchStatus: "active"},
				{
					Branch:         "feature/merged-but-dirty",
					BranchStatus:   "stale",
					StaleReason:    "pr_merged",
					PRNumber:       123,
					PRState:        "MERGED",
					ModifiedCount:  2, // Has uncommitted changes!
					UntrackedCount: 1,
				},
			},
		}

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}
		newModel, _ := m.handleToolsKeys(msg)

		if newModel.cleanupState == nil {
			t.Fatal("cleanupState should be initialized")
		}

		// Should NOT be pre-selected because it has uncommitted changes
		if newModel.cleanupState.selectedIndices[0] {
			t.Error("Worktree with uncommitted changes should NOT be pre-selected, even with pr_merged")
		}
	})

	t.Run("worktree with pr_merged and clean SHOULD be pre-selected", func(t *testing.T) {
		m := Model{
			currentView: ToolsView,
			worktrees: []Worktree{
				{Branch: "main", IsMain: true, BranchStatus: "active"},
				{
					Branch:         "feature/merged-and-clean",
					BranchStatus:   "stale",
					StaleReason:    "pr_merged",
					PRNumber:       123,
					PRState:        "MERGED",
					ModifiedCount:  0,
					StagedCount:    0,
					UntrackedCount: 0,
				},
			},
		}

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}
		newModel, _ := m.handleToolsKeys(msg)

		if newModel.cleanupState == nil {
			t.Fatal("cleanupState should be initialized")
		}

		// SHOULD be pre-selected - PR is merged and worktree is clean
		if !newModel.cleanupState.selectedIndices[0] {
			t.Error("Worktree with pr_merged and clean status SHOULD be pre-selected")
		}
	})

	t.Run("mixed worktrees: only safe ones should be pre-selected", func(t *testing.T) {
		m := Model{
			currentView: ToolsView,
			worktrees: []Worktree{
				{Branch: "main", IsMain: true, BranchStatus: "active"},
				// Index 0 in staleWorktrees: SAFE - pr_merged + clean
				{
					Branch:        "feature/safe-to-delete",
					BranchStatus:  "stale",
					StaleReason:   "pr_merged",
					PRNumber:      100,
					PRState:       "MERGED",
					ModifiedCount: 0,
				},
				// Index 1 in staleWorktrees: UNSAFE - no_unique_commits + dirty
				{
					Branch:        "feature/wip-dirty",
					BranchStatus:  "stale",
					StaleReason:   "no_unique_commits",
					ModifiedCount: 3,
				},
				// Index 2 in staleWorktrees: UNSAFE - pr_merged but dirty
				{
					Branch:        "feature/merged-but-dirty",
					BranchStatus:  "stale",
					StaleReason:   "pr_merged",
					PRNumber:      101,
					PRState:       "MERGED",
					ModifiedCount: 1,
				},
				// Index 3 in staleWorktrees: UNSAFE - no_unique_commits (could be new branch)
				{
					Branch:        "feature/new-empty-branch",
					BranchStatus:  "stale",
					StaleReason:   "no_unique_commits",
					ModifiedCount: 0,
				},
			},
		}

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}
		newModel, _ := m.handleToolsKeys(msg)

		if newModel.cleanupState == nil {
			t.Fatal("cleanupState should be initialized")
		}

		// Should have 4 stale worktrees total
		if len(newModel.cleanupState.staleWorktrees) != 4 {
			t.Errorf("Expected 4 stale worktrees, got %d", len(newModel.cleanupState.staleWorktrees))
		}

		// Only the first one (pr_merged + clean) should be pre-selected
		if !newModel.cleanupState.selectedIndices[0] {
			t.Error("Index 0 (pr_merged + clean) SHOULD be pre-selected")
		}
		if newModel.cleanupState.selectedIndices[1] {
			t.Error("Index 1 (no_unique_commits + dirty) should NOT be pre-selected")
		}
		if newModel.cleanupState.selectedIndices[2] {
			t.Error("Index 2 (pr_merged + dirty) should NOT be pre-selected")
		}
		if newModel.cleanupState.selectedIndices[3] {
			t.Error("Index 3 (no_unique_commits + clean) should NOT be pre-selected")
		}

		// Selection count should be 1
		selectedCount := 0
		for _, selected := range newModel.cleanupState.selectedIndices {
			if selected {
				selectedCount++
			}
		}
		if selectedCount != 1 {
			t.Errorf("Expected 1 pre-selected worktree, got %d", selectedCount)
		}
	})
}

// TestCleanupForceDeleteRequiredForUncommittedChanges tests that deleting worktrees
// with uncommitted changes requires force delete.
func TestCleanupForceDeleteRequiredForUncommittedChanges(t *testing.T) {
	t.Run("UI shows warning for worktrees with uncommitted changes", func(t *testing.T) {
		m := Model{
			cleanupState: &CleanupState{
				staleWorktrees: []Worktree{
					{
						Branch:        "feature/wip",
						BranchStatus:  "stale",
						StaleReason:   "no_unique_commits",
						ModifiedCount: 3,
					},
				},
				selectedIndices: map[int]bool{0: true},
				cursorIndex:     0,
			},
		}

		result := m.renderCleanupConfirmation()

		// Should show warning about uncommitted changes
		if !strings.Contains(result, "uncommitted") {
			t.Error("Should show warning about uncommitted changes in the UI")
		}
	})

	t.Run("worktree with uncommitted changes shows requires force indicator", func(t *testing.T) {
		m := Model{
			cleanupState: &CleanupState{
				staleWorktrees: []Worktree{
					{
						Branch:         "feature/dirty",
						BranchStatus:   "stale",
						StaleReason:    "no_unique_commits",
						ModifiedCount:  2,
						UntrackedCount: 1,
					},
				},
				selectedIndices: map[int]bool{0: true},
				cursorIndex:     0,
			},
		}

		result := m.renderCleanupConfirmation()

		// Should indicate that force delete is required
		if !strings.Contains(result, "requires force") || !strings.Contains(result, "⚠") {
			t.Error("Should indicate that worktree with uncommitted changes requires force delete")
		}
	})
}

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
