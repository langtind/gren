package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCompareWorktrees(t *testing.T) {
	_, manager, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Create a worktree to compare against
	req := CreateWorktreeRequest{
		Name:        "compare-source",
		IsNewBranch: true,
	}
	_, err := manager.CreateWorktree(ctx, req)
	if err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}

	// Find the worktree path
	worktrees, _ := manager.ListWorktrees(ctx)
	var sourcePath string
	for _, wt := range worktrees {
		if wt.Name == "compare-source" {
			sourcePath = wt.Path
			break
		}
	}
	if sourcePath == "" {
		t.Fatal("could not find source worktree path")
	}

	t.Run("compare returns empty when no changes", func(t *testing.T) {
		result, err := manager.CompareWorktrees(ctx, "compare-source")
		if err != nil {
			t.Fatalf("CompareWorktrees() error: %v", err)
		}

		if len(result.Files) != 0 {
			t.Errorf("expected 0 changed files, got %d", len(result.Files))
		}
	})

	t.Run("compare detects uncommitted changes", func(t *testing.T) {
		// Create an uncommitted change in source worktree
		testFile := filepath.Join(sourcePath, "new-file.txt")
		err := os.WriteFile(testFile, []byte("new content"), 0644)
		if err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		result, err := manager.CompareWorktrees(ctx, "compare-source")
		if err != nil {
			t.Fatalf("CompareWorktrees() error: %v", err)
		}

		if len(result.Files) != 1 {
			t.Errorf("expected 1 changed file, got %d", len(result.Files))
		}

		if len(result.Files) > 0 {
			if result.Files[0].Path != "new-file.txt" {
				t.Errorf("expected file path 'new-file.txt', got %q", result.Files[0].Path)
			}
			if result.Files[0].Status != FileAdded {
				t.Errorf("expected status FileAdded, got %v", result.Files[0].Status)
			}
		}
	})

	t.Run("compare detects modified files", func(t *testing.T) {
		// Modify an existing file in source worktree
		readmeFile := filepath.Join(sourcePath, "README.md")
		err := os.WriteFile(readmeFile, []byte("# Modified README\nThis is different content.\n"), 0644)
		if err != nil {
			t.Fatalf("failed to modify test file: %v", err)
		}

		result, err := manager.CompareWorktrees(ctx, "compare-source")
		if err != nil {
			t.Fatalf("CompareWorktrees() error: %v", err)
		}

		// Should have the new file + modified README
		foundModified := false
		for _, f := range result.Files {
			if f.Path == "README.md" && f.Status == FileModified {
				foundModified = true
				break
			}
		}
		if !foundModified {
			t.Error("expected to find modified README.md")
		}
	})

	t.Run("compare fails for nonexistent worktree", func(t *testing.T) {
		_, err := manager.CompareWorktrees(ctx, "nonexistent")
		if err == nil {
			t.Error("expected error for nonexistent worktree")
		}
	})

	t.Run("compare fails for current worktree", func(t *testing.T) {
		worktrees, _ := manager.ListWorktrees(ctx)
		var currentName string
		for _, wt := range worktrees {
			if wt.IsCurrent {
				currentName = wt.Name
				break
			}
		}

		_, err := manager.CompareWorktrees(ctx, currentName)
		if err == nil {
			t.Error("expected error when comparing current worktree to itself")
		}
	})
}

func TestApplyChanges(t *testing.T) {
	_, manager, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Create a worktree with changes
	req := CreateWorktreeRequest{
		Name:        "apply-source",
		IsNewBranch: true,
	}
	_, err := manager.CreateWorktree(ctx, req)
	if err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}

	// Find the worktree path
	worktrees, _ := manager.ListWorktrees(ctx)
	var sourcePath string
	var currentPath string
	for _, wt := range worktrees {
		if wt.Name == "apply-source" {
			sourcePath = wt.Path
		}
		if wt.IsCurrent {
			currentPath = wt.Path
		}
	}

	// Create a new file in source
	testFile := filepath.Join(sourcePath, "apply-test.txt")
	err = os.WriteFile(testFile, []byte("content to apply"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	t.Run("apply single file", func(t *testing.T) {
		result, err := manager.CompareWorktrees(ctx, "apply-source")
		if err != nil {
			t.Fatalf("CompareWorktrees() error: %v", err)
		}

		// Find the file we want to apply
		var fileToApply *FileChange
		for i := range result.Files {
			if result.Files[i].Path == "apply-test.txt" {
				fileToApply = &result.Files[i]
				break
			}
		}
		if fileToApply == nil {
			t.Fatal("could not find apply-test.txt in changes")
		}

		// Apply just this file
		err = manager.ApplyChanges(ctx, "apply-source", []FileChange{*fileToApply})
		if err != nil {
			t.Fatalf("ApplyChanges() error: %v", err)
		}

		// Verify the file now exists in current worktree
		appliedFile := filepath.Join(currentPath, "apply-test.txt")
		content, err := os.ReadFile(appliedFile)
		if err != nil {
			t.Fatalf("applied file not found: %v", err)
		}
		if string(content) != "content to apply" {
			t.Errorf("applied file content = %q, want 'content to apply'", string(content))
		}
	})

	t.Run("apply all changes", func(t *testing.T) {
		// Create another file in source
		anotherFile := filepath.Join(sourcePath, "another-file.txt")
		err = os.WriteFile(anotherFile, []byte("another content"), 0644)
		if err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		result, err := manager.CompareWorktrees(ctx, "apply-source")
		if err != nil {
			t.Fatalf("CompareWorktrees() error: %v", err)
		}

		// Apply all changes
		err = manager.ApplyChanges(ctx, "apply-source", result.Files)
		if err != nil {
			t.Fatalf("ApplyChanges() error: %v", err)
		}

		// Verify the new file exists
		appliedFile := filepath.Join(currentPath, "another-file.txt")
		content, err := os.ReadFile(appliedFile)
		if err != nil {
			t.Fatalf("applied file not found: %v", err)
		}
		if string(content) != "another content" {
			t.Errorf("applied file content = %q, want 'another content'", string(content))
		}
	})

	t.Run("apply empty list does nothing", func(t *testing.T) {
		err := manager.ApplyChanges(ctx, "apply-source", []FileChange{})
		if err != nil {
			t.Fatalf("ApplyChanges() with empty list should not error: %v", err)
		}
	})
}

func TestCompareWorktreesWithCommittedChanges(t *testing.T) {
	dir, manager, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Create a worktree
	req := CreateWorktreeRequest{
		Name:        "committed-source",
		IsNewBranch: true,
	}
	_, err := manager.CreateWorktree(ctx, req)
	if err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}

	// Find the worktree path
	worktrees, _ := manager.ListWorktrees(ctx)
	var sourcePath string
	for _, wt := range worktrees {
		if wt.Name == "committed-source" {
			sourcePath = wt.Path
			break
		}
	}

	// Create and commit a change in source worktree
	testFile := filepath.Join(sourcePath, "committed-file.txt")
	err = os.WriteFile(testFile, []byte("committed content"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Stage and commit the change
	exec.Command("git", "-C", sourcePath, "add", "committed-file.txt").Run()
	exec.Command("git", "-C", sourcePath, "commit", "-m", "Add committed file").Run()

	t.Run("compare detects committed changes vs main", func(t *testing.T) {
		result, err := manager.CompareWorktrees(ctx, "committed-source")
		if err != nil {
			t.Fatalf("CompareWorktrees() error: %v", err)
		}

		foundCommitted := false
		for _, f := range result.Files {
			if f.Path == "committed-file.txt" {
				foundCommitted = true
				if !f.IsCommitted {
					t.Error("expected file to be marked as committed")
				}
				break
			}
		}
		if !foundCommitted {
			t.Error("expected to find committed-file.txt in comparison")
		}
	})

	t.Run("compare includes both committed and uncommitted changes", func(t *testing.T) {
		// Add an uncommitted change
		uncommittedFile := filepath.Join(sourcePath, "uncommitted.txt")
		err = os.WriteFile(uncommittedFile, []byte("uncommitted"), 0644)
		if err != nil {
			t.Fatalf("failed to create uncommitted file: %v", err)
		}

		result, err := manager.CompareWorktrees(ctx, "committed-source")
		if err != nil {
			t.Fatalf("CompareWorktrees() error: %v", err)
		}

		hasCommitted := false
		hasUncommitted := false
		for _, f := range result.Files {
			if f.Path == "committed-file.txt" && f.IsCommitted {
				hasCommitted = true
			}
			if f.Path == "uncommitted.txt" && !f.IsCommitted {
				hasUncommitted = true
			}
		}

		if !hasCommitted {
			t.Error("expected to find committed file")
		}
		if !hasUncommitted {
			t.Error("expected to find uncommitted file")
		}
	})

	_ = dir // Silence unused variable warning
}

func TestCompareResult(t *testing.T) {
	t.Run("CompareResult struct has expected fields", func(t *testing.T) {
		result := CompareResult{
			SourceWorktree: "source",
			TargetWorktree: "target",
			Files: []FileChange{
				{
					Path:        "test.txt",
					Status:      FileModified,
					IsCommitted: false,
				},
			},
		}

		if result.SourceWorktree != "source" {
			t.Errorf("SourceWorktree = %q, want 'source'", result.SourceWorktree)
		}
		if result.TargetWorktree != "target" {
			t.Errorf("TargetWorktree = %q, want 'target'", result.TargetWorktree)
		}
		if len(result.Files) != 1 {
			t.Errorf("Files count = %d, want 1", len(result.Files))
		}
	})
}

func TestFileChangeStatus(t *testing.T) {
	t.Run("FileStatus constants are defined", func(t *testing.T) {
		if FileAdded != 1 {
			t.Errorf("FileAdded = %d, want 1", FileAdded)
		}
		if FileModified != 2 {
			t.Errorf("FileModified = %d, want 2", FileModified)
		}
		if FileDeleted != 3 {
			t.Errorf("FileDeleted = %d, want 3", FileDeleted)
		}
	})
}
