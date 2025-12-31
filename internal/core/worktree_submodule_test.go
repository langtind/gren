package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestSubmoduleDetection tests that worktrees with submodules are detected
func TestSubmoduleDetection(t *testing.T) {
	dir, manager, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("detect HasSubmodules when .gitmodules exists", func(t *testing.T) {
		// Create a worktree
		req := CreateWorktreeRequest{
			Name:        "submodule-test",
			IsNewBranch: true,
		}
		_, _, err := manager.CreateWorktree(ctx, req)
		if err != nil {
			t.Fatalf("failed to create worktree: %v", err)
		}

		// Find the worktree path
		worktrees, _ := manager.ListWorktrees(ctx)
		var worktreePath string
		for _, wt := range worktrees {
			if wt.Name == "submodule-test" {
				worktreePath = wt.Path
				// Before adding .gitmodules, should not have submodules
				if wt.HasSubmodules {
					t.Error("HasSubmodules should be false before adding .gitmodules")
				}
				break
			}
		}

		if worktreePath == "" {
			t.Fatal("could not find worktree path")
		}

		// Create a .gitmodules file to simulate submodule
		gitmodulesPath := filepath.Join(worktreePath, ".gitmodules")
		err = os.WriteFile(gitmodulesPath, []byte(`[submodule "test-submodule"]
	path = test-submodule
	url = https://github.com/example/test.git
`), 0644)
		if err != nil {
			t.Fatalf("failed to create .gitmodules: %v", err)
		}

		// Re-list worktrees to check detection
		worktrees, err = manager.ListWorktrees(ctx)
		if err != nil {
			t.Fatalf("ListWorktrees() error: %v", err)
		}

		// Find the worktree and check HasSubmodules
		var foundWt *WorktreeInfo
		for i := range worktrees {
			if worktrees[i].Name == "submodule-test" {
				foundWt = &worktrees[i]
				break
			}
		}

		if foundWt == nil {
			t.Fatal("worktree not found after re-listing")
		}

		if !foundWt.HasSubmodules {
			t.Error("HasSubmodules should be true after adding .gitmodules")
		}

		// Clean up
		manager.DeleteWorktree(ctx, "submodule-test", true)
	})

	t.Run("HasSubmodules false when no .gitmodules", func(t *testing.T) {
		// Create a worktree without submodules
		req := CreateWorktreeRequest{
			Name:        "no-submodule-test",
			IsNewBranch: true,
		}
		_, _, err := manager.CreateWorktree(ctx, req)
		if err != nil {
			t.Fatalf("failed to create worktree: %v", err)
		}

		worktrees, _ := manager.ListWorktrees(ctx)
		for _, wt := range worktrees {
			if wt.Name == "no-submodule-test" {
				if wt.HasSubmodules {
					t.Error("HasSubmodules should be false for worktree without .gitmodules")
				}
				break
			}
		}

		// Clean up
		manager.DeleteWorktree(ctx, "no-submodule-test", true)
	})

	// Test worktree with submodule detection and deletion (mock .gitmodules file)
	t.Run("delete worktree with submodule config", func(t *testing.T) {
		// Create a worktree
		req := CreateWorktreeRequest{
			Name:        "submodule-test",
			IsNewBranch: true,
		}
		_, _, err := manager.CreateWorktree(ctx, req)
		if err != nil {
			t.Fatalf("failed to create worktree: %v", err)
		}

		// Find the worktree path
		worktrees, _ := manager.ListWorktrees(ctx)
		var worktreePath string
		for _, wt := range worktrees {
			if wt.Name == "submodule-test" {
				worktreePath = wt.Path
				break
			}
		}

		if worktreePath == "" {
			t.Fatal("could not find worktree path")
		}

		// Create a mock .gitmodules file (simulates submodule presence)
		gitmodulesPath := filepath.Join(worktreePath, ".gitmodules")
		err = os.WriteFile(gitmodulesPath, []byte(`[submodule "test-submodule"]
	path = test-submodule
	url = https://github.com/example/test.git
`), 0644)
		if err != nil {
			t.Fatalf("failed to create .gitmodules: %v", err)
		}

		// Re-list and verify detection
		worktrees, _ = manager.ListWorktrees(ctx)
		var foundWt *WorktreeInfo
		for i := range worktrees {
			if worktrees[i].Name == "submodule-test" {
				foundWt = &worktrees[i]
				break
			}
		}

		if foundWt == nil {
			t.Fatal("worktree not found")
		}

		if !foundWt.HasSubmodules {
			t.Error("HasSubmodules should be true for worktree with .gitmodules")
		}

		// Delete the worktree with force flag (needed for submodules)
		err = manager.DeleteWorktree(ctx, "submodule-test", true)
		if err != nil {
			t.Errorf("DeleteWorktree() should succeed for worktree with submodules: %v", err)
		}

		// Verify it's gone
		worktrees, _ = manager.ListWorktrees(ctx)
		for _, wt := range worktrees {
			if wt.Name == "submodule-test" {
				t.Error("worktree still exists after deletion")
			}
		}
	})

	// Test in main worktree to ensure it handles submodules there too
	t.Run("main worktree with submodule detected", func(t *testing.T) {
		// Add .gitmodules to main worktree
		gitmodulesPath := filepath.Join(dir, ".gitmodules")
		err := os.WriteFile(gitmodulesPath, []byte(`[submodule "main-submodule"]
	path = main-submodule
	url = https://github.com/example/test.git
`), 0644)
		if err != nil {
			t.Fatalf("failed to create .gitmodules in main: %v", err)
		}
		defer os.Remove(gitmodulesPath)

		worktrees, err := manager.ListWorktrees(ctx)
		if err != nil {
			t.Fatalf("ListWorktrees() error: %v", err)
		}

		// Find the main worktree
		var mainWt *WorktreeInfo
		for i := range worktrees {
			if worktrees[i].IsMain {
				mainWt = &worktrees[i]
				break
			}
		}

		if mainWt == nil {
			t.Fatal("main worktree not found")
		}

		if !mainWt.HasSubmodules {
			t.Error("HasSubmodules should be true for main worktree with .gitmodules")
		}
	})
}
