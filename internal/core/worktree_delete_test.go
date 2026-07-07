package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestDeleteWorktreeForceRemovesLeftoverContent guards that a forced delete
// completes even when the checkout still holds gitignored content (e.g.
// node_modules/ or .venv/) that a plain `git worktree remove` refuses to delete
// with "Directory not empty".
func TestDeleteWorktreeForceRemovesLeftoverContent(t *testing.T) {
	_, manager, cleanup := setupTestEnvironment(t)
	defer cleanup()
	ctx := context.Background()

	if _, _, err := manager.CreateWorktree(ctx, CreateWorktreeRequest{Name: "leftover-test", IsNewBranch: true}); err != nil {
		t.Fatalf("create worktree: %v", err)
	}

	var wtPath string
	wts, _ := manager.ListWorktrees(ctx)
	for _, wt := range wts {
		if wt.Name == "leftover-test" {
			wtPath = wt.Path
			break
		}
	}
	if wtPath == "" {
		t.Fatal("could not find the created worktree")
	}

	// Gitignored leftover content a plain `git worktree remove` won't delete.
	if err := os.WriteFile(filepath.Join(wtPath, ".gitignore"), []byte("node_modules/\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(wtPath, "node_modules"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wtPath, "node_modules", "pkg"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := manager.DeleteWorktree(ctx, "leftover-test", true); err != nil {
		t.Fatalf("force delete should succeed with leftover content: %v", err)
	}
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Errorf("worktree directory should be gone after force delete, stat err = %v", err)
	}
}
