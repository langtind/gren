package ui

import (
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/langtind/gren/internal/git"
)

func TestGetSortedWorktrees(t *testing.T) {
	// Create worktrees in unsorted order
	// The bug: when displayed, list is sorted (current first, then by recency)
	// but selection uses the original unsorted m.worktrees slice
	model := Model{
		worktrees: []Worktree{
			{Name: "feature-old", Path: "/path/feature-old", IsCurrent: false, LastCommit: "3d ago"},
			{Name: "main", Path: "/path/main", IsCurrent: true, LastCommit: "1h ago"},
			{Name: "feature-new", Path: "/path/feature-new", IsCurrent: false, LastCommit: "30m ago"},
		},
	}

	sorted := model.getSortedWorktrees()

	// Current worktree (main) should be first
	if sorted[0].Name != "main" {
		t.Errorf("sorted[0].Name = %q, want %q (current worktree should be first)", sorted[0].Name, "main")
	}

	// Then sorted by recency: feature-new (30m) before feature-old (3d)
	if sorted[1].Name != "feature-new" {
		t.Errorf("sorted[1].Name = %q, want %q (more recent should come before older)", sorted[1].Name, "feature-new")
	}

	if sorted[2].Name != "feature-old" {
		t.Errorf("sorted[2].Name = %q, want %q", sorted[2].Name, "feature-old")
	}
}

func TestSelectedWorktreeMatchesSortedList(t *testing.T) {
	// This test verifies that getSelectedWorktree returns from the sorted list,
	// not the original unsorted list.
	model := Model{
		worktrees: []Worktree{
			{Name: "feature-old", Path: "/path/feature-old", IsCurrent: false, LastCommit: "3d ago"},
			{Name: "main", Path: "/path/main", IsCurrent: true, LastCommit: "1h ago"},
			{Name: "feature-new", Path: "/path/feature-new", IsCurrent: false, LastCommit: "30m ago"},
		},
		selected: 1, // User selected index 1 in the displayed (sorted) list
	}

	sorted := model.getSortedWorktrees()

	// What the user sees at index 1 in the sorted list
	expectedWorktree := sorted[1] // Should be "feature-new"

	// getSelectedWorktree should return from sorted list
	actualWorktree := model.getSelectedWorktree()

	if actualWorktree == nil {
		t.Fatal("getSelectedWorktree() returned nil")
	}

	if actualWorktree.Name != expectedWorktree.Name {
		t.Errorf("getSelectedWorktree().Name = %q, want %q (from sorted list)", actualWorktree.Name, expectedWorktree.Name)
	}

	// Verify we're NOT getting from unsorted list
	unsortedWorktree := model.worktrees[model.selected]
	if unsortedWorktree.Name == expectedWorktree.Name {
		t.Log("Note: unsorted and sorted happen to match at this index")
	} else {
		t.Logf("Correctly using sorted list: unsorted[1]=%q, sorted[1]=%q",
			unsortedWorktree.Name, expectedWorktree.Name)
	}
}

func TestGetSelectedWorktreeFromSortedList(t *testing.T) {
	// Test the helper function that should be used for correct selection
	model := Model{
		worktrees: []Worktree{
			{Name: "feature-old", Path: "/path/feature-old", IsCurrent: false, LastCommit: "3d ago"},
			{Name: "main", Path: "/path/main", IsCurrent: true, LastCommit: "1h ago"},
			{Name: "feature-new", Path: "/path/feature-new", IsCurrent: false, LastCommit: "30m ago"},
		},
	}

	tests := []struct {
		selected int
		wantName string
	}{
		{0, "main"},        // Current worktree first
		{1, "feature-new"}, // Then by recency
		{2, "feature-old"}, // Oldest last
	}

	for _, tt := range tests {
		model.selected = tt.selected
		got := model.getSelectedWorktree()

		if got == nil {
			t.Errorf("getSelectedWorktree() with selected=%d returned nil", tt.selected)
			continue
		}

		if got.Name != tt.wantName {
			t.Errorf("getSelectedWorktree() with selected=%d: got %q, want %q",
				tt.selected, got.Name, tt.wantName)
		}
	}
}

func TestCannotDeleteCurrentWorktree(t *testing.T) {
	// Create a model with current worktree selected
	model := Model{
		currentView: DashboardView,
		repoInfo: &git.RepoInfo{
			IsGitRepo:     true,
			IsInitialized: true,
		},
		worktrees: []Worktree{
			{Name: "main", Path: "/path/main", IsCurrent: true, LastCommit: "1h ago"},
			{Name: "feature", Path: "/path/feature", IsCurrent: false, LastCommit: "2h ago"},
		},
		selected: 0, // Current worktree is first in sorted order
		keys:     DefaultKeyMap(),
	}

	// Simulate pressing 'd' to delete
	deleteKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}

	// Update the model
	updatedModel, cmd := model.Update(deleteKey)
	m := updatedModel.(Model)

	// Should show status message instead of entering delete view
	if m.statusMessage == "" {
		t.Error("Expected status message to be set when trying to delete current worktree")
	}
	if m.statusMessage != "⚠️ Cannot delete current worktree" {
		t.Errorf("Unexpected status message: %q", m.statusMessage)
	}
	if m.currentView != DashboardView {
		t.Error("Should stay in dashboard view, not enter delete view")
	}

	// Should return a command to clear the status
	if cmd == nil {
		t.Error("Expected a command to clear status message after delay")
	}
}

func TestDeleteNonCurrentWorktreeAllowed(t *testing.T) {
	// Create a model with non-current worktree selected
	model := Model{
		currentView: DashboardView,
		repoInfo: &git.RepoInfo{
			IsGitRepo:     true,
			IsInitialized: true,
		},
		worktrees: []Worktree{
			{Name: "main", Path: "/path/main", IsCurrent: true, LastCommit: "1h ago"},
			{Name: "feature", Path: "/path/feature", IsCurrent: false, LastCommit: "2h ago"},
		},
		selected: 1, // Feature worktree (not current)
		keys:     DefaultKeyMap(),
	}

	// Simulate pressing 'd' to delete
	deleteKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}

	// Update the model
	updatedModel, _ := model.Update(deleteKey)
	m := updatedModel.(Model)

	// Should enter delete view (not show status message)
	if m.statusMessage != "" {
		t.Errorf("Should not show status message for non-current worktree, got: %q", m.statusMessage)
	}
	if m.currentView != DeleteView {
		t.Error("Should enter delete view for non-current worktree")
	}
}

func TestClearStatusMessage(t *testing.T) {
	model := Model{
		statusMessage: "Test message",
	}

	// Send clearStatusMsg
	updatedModel, _ := model.Update(clearStatusMsg{})
	m := updatedModel.(Model)

	if m.statusMessage != "" {
		t.Errorf("Status message should be cleared, got: %q", m.statusMessage)
	}
}

func TestClearStatusAfterCommand(t *testing.T) {
	// Test that clearStatusAfter returns a command that sends clearStatusMsg
	cmd := clearStatusAfter(1 * time.Millisecond)
	if cmd == nil {
		t.Fatal("clearStatusAfter should return a command")
	}

	// The command should be a tick that eventually returns clearStatusMsg
	// We can't easily test the timing, but we can verify it's not nil
}

// Helper to check if key matches
func init() {
	// Suppress unused import error for key package
	_ = key.NewBinding()
}
