package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/key"
)

func TestDefaultKeyMap(t *testing.T) {
	km := DefaultKeyMap()

	t.Run("up binding", func(t *testing.T) {
		if !key.Matches(tea.KeyMsg{Type: tea.KeyUp}, km.Up) {
			t.Error("Up binding should match arrow up")
		}
	})

	t.Run("down binding", func(t *testing.T) {
		if !key.Matches(tea.KeyMsg{Type: tea.KeyDown}, km.Down) {
			t.Error("Down binding should match arrow down")
		}
	})

	t.Run("vim-style j binding", func(t *testing.T) {
		if !key.Matches(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, km.Down) {
			t.Error("Down binding should match 'j' key")
		}
	})

	t.Run("vim-style k binding", func(t *testing.T) {
		if !key.Matches(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}, km.Up) {
			t.Error("Up binding should match 'k' key")
		}
	})

	t.Run("enter binding", func(t *testing.T) {
		if !key.Matches(tea.KeyMsg{Type: tea.KeyEnter}, km.Enter) {
			t.Error("Enter binding should match enter key")
		}
	})

	t.Run("quit binding", func(t *testing.T) {
		if !key.Matches(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}, km.Quit) {
			t.Error("Quit binding should match 'q' key")
		}
	})

	t.Run("esc binding", func(t *testing.T) {
		if !key.Matches(tea.KeyMsg{Type: tea.KeyEsc}, km.Back) {
			t.Error("Back binding should match esc key")
		}
	})

	t.Run("new worktree binding", func(t *testing.T) {
		if !key.Matches(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}, km.New) {
			t.Error("New binding should match 'n' key")
		}
	})

	t.Run("delete binding", func(t *testing.T) {
		if !key.Matches(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}, km.Delete) {
			t.Error("Delete binding should match 'd' key")
		}
	})

	t.Run("init binding", func(t *testing.T) {
		if !key.Matches(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}}, km.Init) {
			t.Error("Init binding should match 'i' key")
		}
	})

	t.Run("config binding", func(t *testing.T) {
		if !key.Matches(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}, km.Config) {
			t.Error("Config binding should match 'c' key")
		}
	})

	t.Run("prune binding", func(t *testing.T) {
		if !key.Matches(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}, km.Prune) {
			t.Error("Prune binding should match 'p' key")
		}
	})

	t.Run("navigate binding", func(t *testing.T) {
		if !key.Matches(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}, km.Navigate) {
			t.Error("Navigate binding should match 'g' key")
		}
	})

	t.Run("help binding", func(t *testing.T) {
		if !key.Matches(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}, km.Help) {
			t.Error("Help binding should match '?' key")
		}
	})
}

func TestViewStateConstants(t *testing.T) {
	// Verify view states are unique
	states := []ViewState{
		DashboardView,
		CreateView,
		DeleteView,
		InitView,
		SettingsView,
		OpenInView,
		ConfigView,
	}

	seen := make(map[ViewState]bool)
	for _, state := range states {
		if seen[state] {
			t.Errorf("ViewState %d is duplicated", state)
		}
		seen[state] = true
	}
}

func TestInitStepConstants(t *testing.T) {
	// Verify init steps are unique and in order
	steps := []InitStep{
		InitStepWelcome,
		InitStepAnalysis,
		InitStepRecommendations,
		InitStepCustomization,
		InitStepPreview,
		InitStepCreated,
		InitStepExecuting,
		InitStepComplete,
		InitStepCommitConfirm,
		InitStepFinal,
		InitStepAIGenerating,
		InitStepAIResult,
	}

	seen := make(map[InitStep]bool)
	for _, step := range steps {
		if seen[step] {
			t.Errorf("InitStep %d is duplicated", step)
		}
		seen[step] = true
	}
}

func TestCreateStepConstants(t *testing.T) {
	// Verify create steps are unique
	steps := []CreateStep{
		CreateStepBranchMode,
		CreateStepBranchName,
		CreateStepExistingBranch,
		CreateStepBaseBranch,
		CreateStepConfirm,
		CreateStepCreating,
		CreateStepComplete,
	}

	seen := make(map[CreateStep]bool)
	for _, step := range steps {
		if seen[step] {
			t.Errorf("CreateStep %d is duplicated", step)
		}
		seen[step] = true
	}
}

func TestDeleteStepConstants(t *testing.T) {
	// Verify delete steps are unique
	steps := []DeleteStep{
		DeleteStepSelection,
		DeleteStepConfirm,
		DeleteStepDeleting,
		DeleteStepComplete,
	}

	seen := make(map[DeleteStep]bool)
	for _, step := range steps {
		if seen[step] {
			t.Errorf("DeleteStep %d is duplicated", step)
		}
		seen[step] = true
	}
}

func TestCreateModeConstants(t *testing.T) {
	// Verify create modes are unique
	modes := []CreateMode{
		CreateModeNewBranch,
		CreateModeExistingBranch,
	}

	seen := make(map[CreateMode]bool)
	for _, mode := range modes {
		if seen[mode] {
			t.Errorf("CreateMode %d is duplicated", mode)
		}
		seen[mode] = true
	}
}

func TestPostCreateActionInterface(t *testing.T) {
	action := PostCreateAction{
		Name:        "Open in VSCode",
		Icon:        "📝",
		Command:     "code",
		Args:        []string{"."},
		Available:   true,
		Description: "Open worktree in Visual Studio Code",
	}

	t.Run("FilterValue", func(t *testing.T) {
		if action.FilterValue() != "Open in VSCode" {
			t.Errorf("FilterValue() = %q, want %q", action.FilterValue(), "Open in VSCode")
		}
	})

	t.Run("Title", func(t *testing.T) {
		title := action.Title()
		if title != "📝 Open in VSCode" {
			t.Errorf("Title() = %q, want %q", title, "📝 Open in VSCode")
		}
	})

	t.Run("Desc", func(t *testing.T) {
		if action.Desc() != "Open worktree in Visual Studio Code" {
			t.Errorf("Desc() = %q, want %q", action.Desc(), "Open worktree in Visual Studio Code")
		}
	})
}

func TestWorktreeStruct(t *testing.T) {
	wt := Worktree{
		Name:           "feature-branch",
		Path:           "/path/to/worktree",
		Branch:         "refs/heads/feature-branch",
		Status:         "clean",
		IsCurrent:      true,
		LastCommit:     "2h ago",
		StagedCount:    1,
		ModifiedCount:  2,
		UntrackedCount: 3,
		UnpushedCount:  4,
	}

	if wt.Name != "feature-branch" {
		t.Errorf("Name = %q, want %q", wt.Name, "feature-branch")
	}
	if wt.IsCurrent != true {
		t.Error("IsCurrent should be true")
	}
	if wt.StagedCount != 1 {
		t.Errorf("StagedCount = %d, want 1", wt.StagedCount)
	}
}

func TestBranchStatusStruct(t *testing.T) {
	bs := BranchStatus{
		Name:             "main",
		IsClean:          false,
		UncommittedFiles: 5,
		UntrackedFiles:   2,
		IsCurrent:        true,
		AheadCount:       3,
		BehindCount:      1,
	}

	if bs.Name != "main" {
		t.Errorf("Name = %q, want %q", bs.Name, "main")
	}
	if bs.IsClean != false {
		t.Error("IsClean should be false")
	}
	if bs.UncommittedFiles != 5 {
		t.Errorf("UncommittedFiles = %d, want 5", bs.UncommittedFiles)
	}
	if bs.AheadCount != 3 {
		t.Errorf("AheadCount = %d, want 3", bs.AheadCount)
	}
}
