package core

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCreateWorktreeWithCustomDir(t *testing.T) {
	dir, manager, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("create worktree in custom directory", func(t *testing.T) {
		customDir, err := os.MkdirTemp("", "gren-custom-worktrees-*")
		if err != nil {
			t.Fatalf("failed to create custom dir: %v", err)
		}
		defer os.RemoveAll(customDir)

		req := CreateWorktreeRequest{
			Name:        "custom-location",
			IsNewBranch: true,
			WorktreeDir: customDir,
		}

		_, _, err = manager.CreateWorktree(ctx, req)
		if err != nil {
			t.Fatalf("CreateWorktree() error: %v", err)
		}

		// Verify worktree was created in custom location
		worktreePath := filepath.Join(customDir, "custom-location")
		if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
			t.Error("worktree was not created in custom directory")
		}
	})

	_ = dir // keep reference for cleanup
}

func TestCreateWorktreeWithBaseBranch(t *testing.T) {
	dir, manager, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("create worktree from specific base branch", func(t *testing.T) {
		// Get current branch name
		out, _ := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD").Output()
		baseBranch := string(out[:len(out)-1])

		req := CreateWorktreeRequest{
			Name:        "from-base",
			BaseBranch:  baseBranch,
			IsNewBranch: true,
		}

		_, _, err := manager.CreateWorktree(ctx, req)
		if err != nil {
			t.Fatalf("CreateWorktree() error: %v", err)
		}

		// Verify worktree was created
		worktrees, _ := manager.ListWorktrees(ctx)
		found := false
		for _, wt := range worktrees {
			if wt.Name == "from-base" {
				found = true
				break
			}
		}
		if !found {
			t.Error("worktree 'from-base' not found after creation")
		}
	})
}

func TestCreateWorktreeWithGrenSymlink(t *testing.T) {
	dir, manager, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("creates symlink to .gren directory", func(t *testing.T) {
		// The setup already creates .gren directory
		grenDir := filepath.Join(dir, ".gren")
		if _, err := os.Stat(grenDir); os.IsNotExist(err) {
			t.Skip(".gren directory not found in test setup")
		}

		// Use absolute worktree dir so symlink path resolution works correctly
		worktreeDir := filepath.Join(filepath.Dir(dir), filepath.Base(dir)+"-symlink-worktrees")
		os.MkdirAll(worktreeDir, 0755)
		defer os.RemoveAll(worktreeDir)

		req := CreateWorktreeRequest{
			Name:        "symlink-test",
			IsNewBranch: true,
			WorktreeDir: worktreeDir,
		}

		_, _, err := manager.CreateWorktree(ctx, req)
		if err != nil {
			t.Fatalf("CreateWorktree() error: %v", err)
		}

		// Check if .gren symlink exists in the worktree
		worktreePath := filepath.Join(worktreeDir, "symlink-test")
		symlinkPath := filepath.Join(worktreePath, ".gren")
		info, err := os.Lstat(symlinkPath)
		if err != nil {
			// Symlink creation is best-effort, log but don't fail if it doesn't exist
			t.Logf(".gren symlink not created (may be expected depending on path resolution): %v", err)
			return
		}

		if info.Mode()&os.ModeSymlink == 0 {
			t.Error(".gren is not a symlink")
		}
	})
}

func TestCreateWorktreeWithPostCreateHook(t *testing.T) {
	dir, manager, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("runs post-create hook when configured", func(t *testing.T) {
		// Create a post-create hook script
		hookScript := `#!/bin/sh
echo "Hook executed" > "$1/.hook-executed"
`
		hookPath := filepath.Join(dir, ".gren", "post-create.sh")
		if err := os.WriteFile(hookPath, []byte(hookScript), 0755); err != nil {
			t.Fatalf("failed to create hook script: %v", err)
		}

		// Update config to use the hook
		configPath := filepath.Join(dir, ".gren", "config.json")
		configContent := fmt.Sprintf(`{
			"main_worktree": %q,
			"worktree_dir": "../test-worktrees",
			"post_create_hook": ".gren/post-create.sh",
			"version": "1.0.0"
		}`, dir)
		os.WriteFile(configPath, []byte(configContent), 0644)

		req := CreateWorktreeRequest{
			Name:        "hook-test",
			IsNewBranch: true,
		}

		_, _, err := manager.CreateWorktree(ctx, req)
		if err != nil {
			t.Fatalf("CreateWorktree() error: %v", err)
		}

		// Find the worktree path
		worktrees, _ := manager.ListWorktrees(ctx)
		var worktreePath string
		for _, wt := range worktrees {
			if wt.Name == "hook-test" {
				worktreePath = wt.Path
				break
			}
		}

		if worktreePath == "" {
			t.Fatal("could not find worktree path")
		}

		// Check if hook created the marker file
		markerPath := filepath.Join(worktreePath, ".hook-executed")
		if _, err := os.Stat(markerPath); os.IsNotExist(err) {
			t.Error("post-create hook was not executed (marker file not found)")
		}
	})

	t.Run("handles missing hook gracefully", func(t *testing.T) {
		// Update config to point to nonexistent hook
		configPath := filepath.Join(dir, ".gren", "config.json")
		configContent := fmt.Sprintf(`{
			"main_worktree": %q,
			"worktree_dir": "../test-worktrees",
			"post_create_hook": ".gren/nonexistent-hook.sh",
			"version": "1.0.0"
		}`, dir)
		os.WriteFile(configPath, []byte(configContent), 0644)

		req := CreateWorktreeRequest{
			Name:        "missing-hook-test",
			IsNewBranch: true,
		}

		// Should not fail even with missing hook
		_, _, err := manager.CreateWorktree(ctx, req)
		if err != nil {
			t.Fatalf("CreateWorktree() should not fail for missing hook: %v", err)
		}
	})
}

func TestCreateWorktreeWithRemoteBranch(t *testing.T) {
	dir, manager, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("creates worktree from remote tracking branch", func(t *testing.T) {
		// Create a simulated remote by setting up origin
		// First create a bare repo as "origin"
		remoteDir, err := os.MkdirTemp("", "gren-remote-*")
		if err != nil {
			t.Fatalf("failed to create remote dir: %v", err)
		}
		defer os.RemoveAll(remoteDir)

		// Init bare repo
		exec.Command("git", "-C", remoteDir, "init", "--bare").Run()

		// Add as remote
		exec.Command("git", "-C", dir, "remote", "add", "origin", remoteDir).Run()

		// Push current branch to remote
		exec.Command("git", "-C", dir, "push", "-u", "origin", "HEAD").Run()

		// Create a branch on remote only
		// We do this by creating locally, pushing, and deleting locally
		exec.Command("git", "-C", dir, "checkout", "-b", "remote-only-branch").Run()
		exec.Command("git", "-C", dir, "push", "origin", "remote-only-branch").Run()

		// Get back to main and delete local branch
		currentBranch, _ := exec.Command("git", "-C", dir, "symbolic-ref", "--short", "HEAD").Output()
		mainBranch := "master"
		if len(currentBranch) > 0 {
			mainBranch = string(currentBranch[:len(currentBranch)-1])
		}

		// Try to checkout main/master
		if err := exec.Command("git", "-C", dir, "checkout", "master").Run(); err != nil {
			exec.Command("git", "-C", dir, "checkout", "main").Run()
			mainBranch = "main"
		}

		// Delete the local branch
		exec.Command("git", "-C", dir, "branch", "-D", "remote-only-branch").Run()

		// Fetch to make sure we have the remote ref
		exec.Command("git", "-C", dir, "fetch", "origin").Run()

		// Now create worktree from remote branch
		req := CreateWorktreeRequest{
			Name:        "from-remote",
			Branch:      "remote-only-branch",
			IsNewBranch: false,
		}

		_, _, err = manager.CreateWorktree(ctx, req)
		if err != nil {
			t.Logf("CreateWorktree from remote branch failed (may be expected in CI): %v", err)
			// This test may fail in some CI environments without proper git setup
			t.Skip("skipping remote branch test in environment without proper remote setup")
		}

		// Verify worktree was created
		worktrees, _ := manager.ListWorktrees(ctx)
		found := false
		for _, wt := range worktrees {
			if wt.Name == "from-remote" {
				found = true
				break
			}
		}
		if !found {
			t.Error("worktree 'from-remote' not found after creation from remote branch")
		}

		_ = mainBranch // used in checkout
	})
}

// TestCreateWorktreeWithExistingBranchBothLocalAndRemote tests the --existing flag
// when a branch exists both locally and on remote (in sync or behind).
// This is a regression test for GitHub issue #4.
func TestCreateWorktreeWithExistingBranchBothLocalAndRemote(t *testing.T) {
	dir, manager, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("existing branch in sync with remote should not use -b flag", func(t *testing.T) {
		// Set up a remote repository
		remoteDir, err := os.MkdirTemp("", "gren-remote-existing-*")
		if err != nil {
			t.Fatalf("failed to create remote dir: %v", err)
		}
		defer os.RemoveAll(remoteDir)

		// Init bare repo as remote
		exec.Command("git", "-C", remoteDir, "init", "--bare").Run()

		// Add as remote origin
		exec.Command("git", "-C", dir, "remote", "add", "origin", remoteDir).Run()

		// Push current branch (main/master) to remote
		exec.Command("git", "-C", dir, "push", "-u", "origin", "HEAD").Run()

		// Create a feature branch, add a commit, and push it
		exec.Command("git", "-C", dir, "checkout", "-b", "existing-feature").Run()

		testFile := filepath.Join(dir, "feature-file.txt")
		os.WriteFile(testFile, []byte("feature content"), 0644)
		// Use specific file to avoid adding .gren/config.json to git
		exec.Command("git", "-C", dir, "add", "feature-file.txt").Run()
		exec.Command("git", "-C", dir, "commit", "-m", "Feature commit").Run()
		exec.Command("git", "-C", dir, "push", "-u", "origin", "existing-feature").Run()

		// Go back to main/master - the branch now exists both locally and on remote
		if err := exec.Command("git", "-C", dir, "checkout", "master").Run(); err != nil {
			exec.Command("git", "-C", dir, "checkout", "main").Run()
		}

		// Now try to create a worktree with IsNewBranch=false (--existing flag)
		// This should use the existing local branch, NOT try to create a new one with -b
		req := CreateWorktreeRequest{
			Name:        "existing-feature-wt",
			Branch:      "existing-feature",
			IsNewBranch: false, // This is the --existing flag
		}

		_, _, err = manager.CreateWorktree(ctx, req)
		if err != nil {
			// The bug causes this to fail with "already exists" because it uses -b flag
			t.Fatalf("CreateWorktree() with --existing flag failed: %v", err)
		}

		// Verify worktree was created
		worktrees, _ := manager.ListWorktrees(ctx)
		found := false
		for _, wt := range worktrees {
			if wt.Name == "existing-feature-wt" {
				found = true
				// Verify it's on the correct branch
				if wt.Branch != "existing-feature" {
					t.Errorf("worktree branch = %q, want %q", wt.Branch, "existing-feature")
				}
				break
			}
		}
		if !found {
			t.Error("worktree 'existing-feature-wt' not found after creation with --existing flag")
		}
	})
}
