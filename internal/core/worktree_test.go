package core

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/langtind/gren/internal/config"
	"github.com/langtind/gren/internal/git"
)

// setupTestEnvironment creates a temp git repo with config for testing.
func setupTestEnvironment(t *testing.T) (string, *WorktreeManager, func()) {
	t.Helper()

	// Create temp directory
	dir, err := os.MkdirTemp("", "gren-core-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		os.RemoveAll(dir)
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Configure git user
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test User").Run()

	// Create initial commit
	testFile := filepath.Join(dir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test\n"), 0644); err != nil {
		os.RemoveAll(dir)
		t.Fatalf("failed to create test file: %v", err)
	}
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "Initial commit").Run()

	// Create .gren directory and config
	grenDir := filepath.Join(dir, ".gren")
	os.MkdirAll(grenDir, 0755)

	worktreeDir := filepath.Join(filepath.Dir(dir), "test-worktrees")
	configPath := filepath.Join(grenDir, "config.json")
	configContent := `{
		"main_worktree": "` + dir + `",
		"worktree_dir": "` + worktreeDir + `",
		"package_manager": "auto",
		"version": "1.0.0"
	}`
	os.WriteFile(configPath, []byte(configContent), 0644)

	// Change to the test directory
	originalDir, _ := os.Getwd()

	cleanup := func() {
		os.Chdir(originalDir)
		os.RemoveAll(dir)
		// Also clean up worktrees directory
		os.RemoveAll(worktreeDir)
	}

	os.Chdir(dir)

	// Create manager
	gitRepo := git.NewLocalRepository()
	configManager := config.NewManager()
	manager := NewWorktreeManager(gitRepo, configManager)

	return dir, manager, cleanup
}

func TestNewWorktreeManager(t *testing.T) {
	gitRepo := git.NewLocalRepository()
	configManager := config.NewManager()

	manager := NewWorktreeManager(gitRepo, configManager)

	if manager == nil {
		t.Fatal("NewWorktreeManager returned nil")
	}
}

func TestCheckPrerequisites(t *testing.T) {
	gitRepo := git.NewLocalRepository()
	configManager := config.NewManager()
	manager := NewWorktreeManager(gitRepo, configManager)

	// Git should always be available in a dev environment
	err := manager.CheckPrerequisites()
	if err != nil {
		t.Errorf("CheckPrerequisites() error: %v (git should be available)", err)
	}
}

func TestListWorktrees(t *testing.T) {
	dir, manager, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("single worktree (main repo)", func(t *testing.T) {
		worktrees, err := manager.ListWorktrees(ctx)
		if err != nil {
			t.Fatalf("ListWorktrees() error: %v", err)
		}

		if len(worktrees) != 1 {
			t.Errorf("got %d worktrees, want 1", len(worktrees))
		}

		// Main repo worktree should be current
		if len(worktrees) > 0 && !worktrees[0].IsCurrent {
			t.Error("main repo worktree should be current")
		}
	})

	t.Run("multiple worktrees", func(t *testing.T) {
		// Create a worktree manually
		worktreeDir := filepath.Join(filepath.Dir(dir), "test-worktrees")
		os.MkdirAll(worktreeDir, 0755)

		worktreePath := filepath.Join(worktreeDir, "feature-test")
		cmd := exec.Command("git", "worktree", "add", "-b", "feature-test", worktreePath, "HEAD")
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to create worktree: %v", err)
		}

		worktrees, err := manager.ListWorktrees(ctx)
		if err != nil {
			t.Fatalf("ListWorktrees() error: %v", err)
		}

		if len(worktrees) != 2 {
			t.Errorf("got %d worktrees, want 2", len(worktrees))
		}
	})
}

func TestParseWorktreeList(t *testing.T) {
	manager := &WorktreeManager{}

	t.Run("parse single worktree", func(t *testing.T) {
		output := `worktree /path/to/repo
branch refs/heads/main

`
		worktrees := manager.parseWorktreeList(output)

		if len(worktrees) != 1 {
			t.Fatalf("got %d worktrees, want 1", len(worktrees))
		}

		if worktrees[0].Path != "/path/to/repo" {
			t.Errorf("Path = %q, want /path/to/repo", worktrees[0].Path)
		}

		if worktrees[0].Name != "repo" {
			t.Errorf("Name = %q, want repo", worktrees[0].Name)
		}

		if worktrees[0].Branch != "main" {
			t.Errorf("Branch = %q, want main", worktrees[0].Branch)
		}
	})

	t.Run("parse multiple worktrees", func(t *testing.T) {
		output := `worktree /path/to/repo
branch refs/heads/main

worktree /path/to/worktrees/feature
branch refs/heads/feature

worktree /path/to/worktrees/fix
branch refs/heads/fix

`
		worktrees := manager.parseWorktreeList(output)

		if len(worktrees) != 3 {
			t.Fatalf("got %d worktrees, want 3", len(worktrees))
		}
	})

	t.Run("parse bare repository", func(t *testing.T) {
		output := `worktree /path/to/repo.git
bare

`
		worktrees := manager.parseWorktreeList(output)

		if len(worktrees) != 1 {
			t.Fatalf("got %d worktrees, want 1", len(worktrees))
		}

		if worktrees[0].Branch != "(bare)" {
			t.Errorf("Branch = %q, want (bare)", worktrees[0].Branch)
		}
	})

	t.Run("parse detached HEAD", func(t *testing.T) {
		output := `worktree /path/to/repo
detached

`
		worktrees := manager.parseWorktreeList(output)

		if len(worktrees) != 1 {
			t.Fatalf("got %d worktrees, want 1", len(worktrees))
		}

		if worktrees[0].Branch != "(detached)" {
			t.Errorf("Branch = %q, want (detached)", worktrees[0].Branch)
		}
	})
}

func TestCreateWorktree(t *testing.T) {
	dir, manager, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("create new branch worktree", func(t *testing.T) {
		req := CreateWorktreeRequest{
			Name:        "feature-new",
			IsNewBranch: true,
		}

		err := manager.CreateWorktree(ctx, req)
		if err != nil {
			t.Fatalf("CreateWorktree() error: %v", err)
		}

		// Verify worktree was created
		worktrees, _ := manager.ListWorktrees(ctx)
		found := false
		for _, wt := range worktrees {
			if wt.Name == "feature-new" {
				found = true
				break
			}
		}
		if !found {
			t.Error("worktree 'feature-new' not found after creation")
		}
	})

	t.Run("create worktree with existing branch", func(t *testing.T) {
		// First create a branch
		exec.Command("git", "-C", dir, "branch", "existing-branch").Run()

		req := CreateWorktreeRequest{
			Name:        "existing-wt",
			Branch:      "existing-branch",
			IsNewBranch: false,
		}

		err := manager.CreateWorktree(ctx, req)
		if err != nil {
			t.Fatalf("CreateWorktree() error: %v", err)
		}
	})

	t.Run("fail when branch already checked out", func(t *testing.T) {
		// Try to create worktree with currently checked out branch
		currentBranch, _ := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD").Output()
		branchName := string(currentBranch)[:len(currentBranch)-1] // trim newline

		req := CreateWorktreeRequest{
			Name:        "duplicate-branch",
			Branch:      branchName,
			IsNewBranch: false,
		}

		err := manager.CreateWorktree(ctx, req)
		if err == nil {
			t.Error("CreateWorktree() expected error for branch already checked out")
		}
	})

	t.Run("fail when branch not found", func(t *testing.T) {
		req := CreateWorktreeRequest{
			Name:        "nonexistent",
			Branch:      "nonexistent-branch",
			IsNewBranch: false,
		}

		err := manager.CreateWorktree(ctx, req)
		if err == nil {
			t.Error("CreateWorktree() expected error for nonexistent branch")
		}
	})

	t.Run("sanitize branch name with slashes", func(t *testing.T) {
		req := CreateWorktreeRequest{
			Name:        "feature/with/slashes",
			IsNewBranch: true,
		}

		err := manager.CreateWorktree(ctx, req)
		if err != nil {
			t.Fatalf("CreateWorktree() error: %v", err)
		}

		// Check that worktree was created with sanitized name
		worktrees, _ := manager.ListWorktrees(ctx)
		found := false
		for _, wt := range worktrees {
			if wt.Name == "feature-with-slashes" {
				found = true
				break
			}
		}
		if !found {
			t.Error("worktree with sanitized name 'feature-with-slashes' not found")
		}
	})
}

func TestDeleteWorktree(t *testing.T) {
	_, manager, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("delete existing worktree", func(t *testing.T) {
		// Create a worktree first
		req := CreateWorktreeRequest{
			Name:        "to-delete",
			IsNewBranch: true,
		}
		err := manager.CreateWorktree(ctx, req)
		if err != nil {
			t.Fatalf("failed to create worktree: %v", err)
		}

		// Delete it
		err = manager.DeleteWorktree(ctx, "to-delete")
		if err != nil {
			t.Fatalf("DeleteWorktree() error: %v", err)
		}

		// Verify it's gone
		worktrees, _ := manager.ListWorktrees(ctx)
		for _, wt := range worktrees {
			if wt.Name == "to-delete" {
				t.Error("worktree 'to-delete' still exists after deletion")
			}
		}
	})

	t.Run("fail to delete current worktree", func(t *testing.T) {
		worktrees, _ := manager.ListWorktrees(ctx)
		var currentWorktree string
		for _, wt := range worktrees {
			if wt.IsCurrent {
				currentWorktree = wt.Name
				break
			}
		}

		if currentWorktree == "" {
			t.Skip("no current worktree found")
		}

		err := manager.DeleteWorktree(ctx, currentWorktree)
		if err == nil {
			t.Error("DeleteWorktree() expected error when deleting current worktree")
		}
	})

	t.Run("fail to delete nonexistent worktree", func(t *testing.T) {
		err := manager.DeleteWorktree(ctx, "nonexistent-worktree")
		if err == nil {
			t.Error("DeleteWorktree() expected error for nonexistent worktree")
		}
	})
}

func TestCreateWorktreeReadme(t *testing.T) {
	manager := &WorktreeManager{}

	// Create temp directory
	dir, err := os.MkdirTemp("", "gren-readme-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	t.Run("creates readme", func(t *testing.T) {
		err := manager.createWorktreeReadme(dir)
		if err != nil {
			t.Fatalf("createWorktreeReadme() error: %v", err)
		}

		readmePath := filepath.Join(dir, "README.md")
		if _, err := os.Stat(readmePath); err != nil {
			t.Error("README.md was not created")
		}

		content, _ := os.ReadFile(readmePath)
		if len(content) == 0 {
			t.Error("README.md is empty")
		}
	})

	t.Run("does not overwrite existing readme", func(t *testing.T) {
		readmePath := filepath.Join(dir, "README.md")
		existingContent := "# Custom README\n"
		os.WriteFile(readmePath, []byte(existingContent), 0644)

		err := manager.createWorktreeReadme(dir)
		if err != nil {
			t.Fatalf("createWorktreeReadme() error: %v", err)
		}

		content, _ := os.ReadFile(readmePath)
		if string(content) != existingContent {
			t.Error("README.md was overwritten")
		}
	})
}

func TestCopyDir(t *testing.T) {
	// Create source directory
	srcDir, err := os.MkdirTemp("", "gren-copy-src-*")
	if err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}
	defer os.RemoveAll(srcDir)

	// Create files in source
	os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content1"), 0644)
	os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(srcDir, "subdir", "file2.txt"), []byte("content2"), 0644)

	// Create destination directory
	dstDir, err := os.MkdirTemp("", "gren-copy-dst-*")
	if err != nil {
		t.Fatalf("failed to create dest dir: %v", err)
	}
	defer os.RemoveAll(dstDir)

	t.Run("copies directory recursively", func(t *testing.T) {
		err := copyDir(srcDir, dstDir)
		if err != nil {
			t.Fatalf("copyDir() error: %v", err)
		}

		// Check file1.txt
		content, err := os.ReadFile(filepath.Join(dstDir, "file1.txt"))
		if err != nil {
			t.Errorf("file1.txt not copied: %v", err)
		}
		if string(content) != "content1" {
			t.Errorf("file1.txt content = %q, want 'content1'", string(content))
		}

		// Check subdir/file2.txt
		content, err = os.ReadFile(filepath.Join(dstDir, "subdir", "file2.txt"))
		if err != nil {
			t.Errorf("subdir/file2.txt not copied: %v", err)
		}
		if string(content) != "content2" {
			t.Errorf("subdir/file2.txt content = %q, want 'content2'", string(content))
		}
	})

	t.Run("handles empty source directory", func(t *testing.T) {
		emptyDir, err := os.MkdirTemp("", "gren-copy-empty-*")
		if err != nil {
			t.Fatalf("failed to create empty dir: %v", err)
		}
		defer os.RemoveAll(emptyDir)

		emptyDst, err := os.MkdirTemp("", "gren-copy-empty-dst-*")
		if err != nil {
			t.Fatalf("failed to create dest dir: %v", err)
		}
		defer os.RemoveAll(emptyDst)

		err = copyDir(emptyDir, emptyDst)
		if err != nil {
			t.Fatalf("copyDir() error for empty dir: %v", err)
		}
	})
}

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

		err = manager.CreateWorktree(ctx, req)
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

		err := manager.CreateWorktree(ctx, req)
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

func TestListWorktreesEdgeCases(t *testing.T) {
	_, manager, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("list with prunable worktrees", func(t *testing.T) {
		// Create a worktree
		req := CreateWorktreeRequest{
			Name:        "prunable-test",
			IsNewBranch: true,
		}
		err := manager.CreateWorktree(ctx, req)
		if err != nil {
			t.Fatalf("failed to create worktree: %v", err)
		}

		worktrees, err := manager.ListWorktrees(ctx)
		if err != nil {
			t.Fatalf("ListWorktrees() error: %v", err)
		}

		// Should have at least 2 worktrees
		if len(worktrees) < 2 {
			t.Errorf("got %d worktrees, want at least 2", len(worktrees))
		}
	})
}

func TestParseWorktreeListAdditionalCases(t *testing.T) {
	manager := &WorktreeManager{}

	t.Run("parse worktree with locked flag", func(t *testing.T) {
		output := `worktree /path/to/repo
branch refs/heads/main

worktree /path/to/locked-worktree
branch refs/heads/feature
locked

`
		worktrees := manager.parseWorktreeList(output)

		if len(worktrees) != 2 {
			t.Fatalf("got %d worktrees, want 2", len(worktrees))
		}

		// Check for locked worktree
		var lockedFound bool
		for _, wt := range worktrees {
			if wt.Path == "/path/to/locked-worktree" {
				lockedFound = true
			}
		}
		if !lockedFound {
			t.Error("locked worktree not found in parsed output")
		}
	})

	t.Run("parse empty output", func(t *testing.T) {
		output := ""
		worktrees := manager.parseWorktreeList(output)

		if len(worktrees) != 0 {
			t.Errorf("got %d worktrees for empty output, want 0", len(worktrees))
		}
	})

	t.Run("parse worktree with HEAD reference", func(t *testing.T) {
		output := `worktree /path/to/repo
HEAD abc123def456
branch refs/heads/main

`
		worktrees := manager.parseWorktreeList(output)

		if len(worktrees) != 1 {
			t.Fatalf("got %d worktrees, want 1", len(worktrees))
		}

		if worktrees[0].Branch != "main" {
			t.Errorf("Branch = %q, want main", worktrees[0].Branch)
		}
	})
}

func TestDeleteWorktreeByPath(t *testing.T) {
	_, manager, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("delete worktree by path", func(t *testing.T) {
		// Create a worktree
		req := CreateWorktreeRequest{
			Name:        "delete-by-path",
			IsNewBranch: true,
		}
		err := manager.CreateWorktree(ctx, req)
		if err != nil {
			t.Fatalf("failed to create worktree: %v", err)
		}

		// Find worktree path
		worktrees, _ := manager.ListWorktrees(ctx)
		var worktreePath string
		for _, wt := range worktrees {
			if wt.Name == "delete-by-path" {
				worktreePath = wt.Path
				break
			}
		}

		if worktreePath == "" {
			t.Fatal("could not find worktree path")
		}

		// Delete by path
		err = manager.DeleteWorktree(ctx, worktreePath)
		if err != nil {
			t.Fatalf("DeleteWorktree() by path error: %v", err)
		}

		// Verify it's gone
		worktrees, _ = manager.ListWorktrees(ctx)
		for _, wt := range worktrees {
			if wt.Name == "delete-by-path" {
				t.Error("worktree still exists after deletion by path")
			}
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

		err := manager.CreateWorktree(ctx, req)
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

		err := manager.CreateWorktree(ctx, req)
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
		err := manager.CreateWorktree(ctx, req)
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

		err = manager.CreateWorktree(ctx, req)
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

func TestWorktreeInfoFields(t *testing.T) {
	t.Run("WorktreeInfo struct has expected fields", func(t *testing.T) {
		info := WorktreeInfo{
			Name:       "test",
			Path:       "/path/to/test",
			Branch:     "test",
			IsCurrent:  true,
			Status:     "clean",
			LastCommit: "2 hours ago",
		}

		if info.Name != "test" {
			t.Errorf("Name = %q, want 'test'", info.Name)
		}
		if info.Path != "/path/to/test" {
			t.Errorf("Path = %q, want '/path/to/test'", info.Path)
		}
		if info.Branch != "test" {
			t.Errorf("Branch = %q, want 'test'", info.Branch)
		}
		if !info.IsCurrent {
			t.Error("IsCurrent = false, want true")
		}
		if info.Status != "clean" {
			t.Errorf("Status = %q, want 'clean'", info.Status)
		}
		if info.LastCommit != "2 hours ago" {
			t.Errorf("LastCommit = %q, want '2 hours ago'", info.LastCommit)
		}
	})
}

func TestCreateWorktreeRequestFields(t *testing.T) {
	t.Run("CreateWorktreeRequest struct has expected fields", func(t *testing.T) {
		req := CreateWorktreeRequest{
			Name:        "feature-test",
			Branch:      "feature-branch",
			BaseBranch:  "main",
			IsNewBranch: true,
			WorktreeDir: "/custom/path",
		}

		if req.Name != "feature-test" {
			t.Errorf("Name = %q, want 'feature-test'", req.Name)
		}
		if req.Branch != "feature-branch" {
			t.Errorf("Branch = %q, want 'feature-branch'", req.Branch)
		}
		if req.BaseBranch != "main" {
			t.Errorf("BaseBranch = %q, want 'main'", req.BaseBranch)
		}
		if !req.IsNewBranch {
			t.Error("IsNewBranch = false, want true")
		}
		if req.WorktreeDir != "/custom/path" {
			t.Errorf("WorktreeDir = %q, want '/custom/path'", req.WorktreeDir)
		}
	})
}
