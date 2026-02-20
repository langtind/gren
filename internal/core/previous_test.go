package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/langtind/gren/internal/config"
	"github.com/langtind/gren/internal/git"
)

// setupPreviousWorktreeTest creates two linked worktrees for testing the previous worktree feature.
func setupPreviousWorktreeTest(t *testing.T) (mainDir, wtDir string, wm *WorktreeManager, cleanup func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "gren-previous-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		os.RemoveAll(dir)
		t.Fatalf("failed to init git repo: %v", err)
	}

	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test User").Run()

	testFile := filepath.Join(dir, "README.md")
	os.WriteFile(testFile, []byte("# Test\n"), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "Initial commit").Run()

	// Create a linked worktree on a feature branch
	wtPath := filepath.Join(dir, ".worktrees", "feature-test")
	os.MkdirAll(filepath.Dir(wtPath), 0755)
	if out, err := exec.Command("git", "-C", dir, "worktree", "add", "-b", "feature/test", wtPath).CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		t.Fatalf("failed to create worktree: %v: %s", err, out)
	}

	originalDir, _ := os.Getwd()
	os.Chdir(dir)

	wm = NewWorktreeManager(git.NewLocalRepository(), config.NewManager())

	cleanup = func() {
		os.Chdir(originalDir)
		os.RemoveAll(dir)
	}

	return dir, wtPath, wm, cleanup
}

func TestGetPreviousWorktreePath_NoPrevious(t *testing.T) {
	_, _, wm, cleanup := setupPreviousWorktreeTest(t)
	defer cleanup()

	path, err := wm.GetPreviousWorktreePath()
	if err != nil {
		t.Fatalf("GetPreviousWorktreePath() error: %v", err)
	}
	if path != "" {
		t.Errorf("GetPreviousWorktreePath() = %q, want empty string when no previous set", path)
	}
}

func TestSetAndGetPreviousWorktreePath(t *testing.T) {
	mainDir, _, wm, cleanup := setupPreviousWorktreeTest(t)
	defer cleanup()

	if err := wm.SetPreviousWorktreePath(mainDir); err != nil {
		t.Fatalf("SetPreviousWorktreePath() error: %v", err)
	}

	path, err := wm.GetPreviousWorktreePath()
	if err != nil {
		t.Fatalf("GetPreviousWorktreePath() error: %v", err)
	}
	if path != mainDir {
		t.Errorf("GetPreviousWorktreePath() = %q, want %q", path, mainDir)
	}
}

func TestSetPreviousWorktreePath_Overwrites(t *testing.T) {
	mainDir, wtDir, wm, cleanup := setupPreviousWorktreeTest(t)
	defer cleanup()

	// Set once
	if err := wm.SetPreviousWorktreePath(mainDir); err != nil {
		t.Fatalf("SetPreviousWorktreePath() first call error: %v", err)
	}

	// Overwrite with new path
	if err := wm.SetPreviousWorktreePath(wtDir); err != nil {
		t.Fatalf("SetPreviousWorktreePath() second call error: %v", err)
	}

	path, err := wm.GetPreviousWorktreePath()
	if err != nil {
		t.Fatalf("GetPreviousWorktreePath() error: %v", err)
	}
	if path != wtDir {
		t.Errorf("GetPreviousWorktreePath() = %q, want %q (latest value)", path, wtDir)
	}
}

func TestListWorktrees_MarksPreviousWorktree(t *testing.T) {
	mainDir, wtDir, wm, cleanup := setupPreviousWorktreeTest(t)
	defer cleanup()

	// Set the linked worktree as previous
	if err := wm.SetPreviousWorktreePath(wtDir); err != nil {
		t.Fatalf("SetPreviousWorktreePath() error: %v", err)
	}

	ctx := context.Background()
	worktrees, err := wm.ListWorktrees(ctx)
	if err != nil {
		t.Fatalf("ListWorktrees() error: %v", err)
	}

	// Resolve symlinks for comparison (macOS: /var/folders → /private/var/folders)
	resolvedWtDir, _ := filepath.EvalSymlinks(wtDir)
	if resolvedWtDir == "" {
		resolvedWtDir = wtDir
	}

	var foundPrevious bool
	for _, wt := range worktrees {
		if wt.IsPrevious {
			foundPrevious = true
			resolvedPath, _ := filepath.EvalSymlinks(wt.Path)
			if resolvedPath == "" {
				resolvedPath = wt.Path
			}
			if resolvedPath != resolvedWtDir {
				t.Errorf("IsPrevious worktree resolved path = %q, want %q", resolvedPath, resolvedWtDir)
			}
		}
	}
	if !foundPrevious {
		t.Errorf("expected one worktree to have IsPrevious=true after SetPreviousWorktreePath(%q)", wtDir)
	}

	_ = mainDir // used for context
}

func TestListWorktrees_NoPreviousSet(t *testing.T) {
	_, _, wm, cleanup := setupPreviousWorktreeTest(t)
	defer cleanup()

	ctx := context.Background()
	worktrees, err := wm.ListWorktrees(ctx)
	if err != nil {
		t.Fatalf("ListWorktrees() error: %v", err)
	}

	for _, wt := range worktrees {
		if wt.IsPrevious {
			t.Errorf("worktree %q has IsPrevious=true but no previous was set", wt.Path)
		}
	}
}

func TestListWorktrees_CurrentNotPrevious(t *testing.T) {
	mainDir, _, wm, cleanup := setupPreviousWorktreeTest(t)
	defer cleanup()

	// Set current (main) worktree as previous — should not mark IsPrevious on the current worktree
	if err := wm.SetPreviousWorktreePath(mainDir); err != nil {
		t.Fatalf("SetPreviousWorktreePath() error: %v", err)
	}

	ctx := context.Background()
	worktrees, err := wm.ListWorktrees(ctx)
	if err != nil {
		t.Fatalf("ListWorktrees() error: %v", err)
	}

	for _, wt := range worktrees {
		if wt.IsCurrent && wt.IsPrevious {
			t.Errorf("current worktree should not have IsPrevious=true (path: %q)", wt.Path)
		}
	}
}
