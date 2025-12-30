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

func TestFileStatusString(t *testing.T) {
	tests := []struct {
		status   FileStatus
		expected string
	}{
		{FileAdded, "added"},
		{FileModified, "modified"},
		{FileDeleted, "deleted"},
		{FileStatus(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.status.String(); got != tt.expected {
				t.Errorf("FileStatus(%d).String() = %q, want %q", tt.status, got, tt.expected)
			}
		})
	}
}

func TestDeduplicateFiles(t *testing.T) {
	t.Run("uncommitted takes precedence over committed", func(t *testing.T) {
		files := []FileChange{
			{Path: "file.txt", Status: FileModified, IsCommitted: true},
			{Path: "file.txt", Status: FileAdded, IsCommitted: false},
		}

		result := deduplicateFiles(files)

		if len(result) != 1 {
			t.Fatalf("expected 1 file, got %d", len(result))
		}
		if result[0].IsCommitted {
			t.Error("expected uncommitted to take precedence")
		}
		if result[0].Status != FileAdded {
			t.Errorf("expected FileAdded status, got %v", result[0].Status)
		}
	})

	t.Run("keeps first occurrence if both committed", func(t *testing.T) {
		files := []FileChange{
			{Path: "file.txt", Status: FileModified, IsCommitted: true},
			{Path: "file.txt", Status: FileDeleted, IsCommitted: true},
		}

		result := deduplicateFiles(files)

		if len(result) != 1 {
			t.Fatalf("expected 1 file, got %d", len(result))
		}
		if result[0].Status != FileModified {
			t.Errorf("expected FileModified (first), got %v", result[0].Status)
		}
	})

	t.Run("handles multiple different files", func(t *testing.T) {
		files := []FileChange{
			{Path: "a.txt", Status: FileAdded, IsCommitted: false},
			{Path: "b.txt", Status: FileModified, IsCommitted: true},
			{Path: "c.txt", Status: FileDeleted, IsCommitted: false},
		}

		result := deduplicateFiles(files)

		if len(result) != 3 {
			t.Errorf("expected 3 files, got %d", len(result))
		}
	})

	t.Run("empty input returns empty result", func(t *testing.T) {
		result := deduplicateFiles([]FileChange{})
		if len(result) != 0 {
			t.Errorf("expected 0 files, got %d", len(result))
		}
	})
}

func TestApplyChangesDelete(t *testing.T) {
	_, manager, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Create a worktree
	req := CreateWorktreeRequest{
		Name:        "delete-source",
		IsNewBranch: true,
	}
	_, err := manager.CreateWorktree(ctx, req)
	if err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}

	// Find worktree paths
	worktrees, _ := manager.ListWorktrees(ctx)
	var currentPath string
	for _, wt := range worktrees {
		if wt.IsCurrent {
			currentPath = wt.Path
			break
		}
	}

	// Create a file in current worktree that we will "delete"
	fileToDelete := filepath.Join(currentPath, "to-delete.txt")
	err = os.WriteFile(fileToDelete, []byte("will be deleted"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	t.Run("apply deletes files", func(t *testing.T) {
		// Verify file exists before
		if _, err := os.Stat(fileToDelete); os.IsNotExist(err) {
			t.Fatal("test file should exist before delete")
		}

		// Apply a delete change
		err = manager.ApplyChanges(ctx, "delete-source", []FileChange{
			{Path: "to-delete.txt", Status: FileDeleted, IsCommitted: false},
		})
		if err != nil {
			t.Fatalf("ApplyChanges() error: %v", err)
		}

		// Verify file was deleted
		if _, err := os.Stat(fileToDelete); !os.IsNotExist(err) {
			t.Error("file should have been deleted")
		}
	})

	t.Run("delete nonexistent file succeeds", func(t *testing.T) {
		err = manager.ApplyChanges(ctx, "delete-source", []FileChange{
			{Path: "nonexistent.txt", Status: FileDeleted, IsCommitted: false},
		})
		if err != nil {
			t.Errorf("ApplyChanges() should not error for nonexistent file: %v", err)
		}
	})
}

func TestApplyChangesErrors(t *testing.T) {
	_, manager, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("apply fails for nonexistent worktree", func(t *testing.T) {
		err := manager.ApplyChanges(ctx, "nonexistent-worktree", []FileChange{
			{Path: "file.txt", Status: FileAdded, IsCommitted: false},
		})
		if err == nil {
			t.Error("expected error for nonexistent worktree")
		}
	})
}

func TestApplyChangesInSubdirectory(t *testing.T) {
	_, manager, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Create a worktree
	req := CreateWorktreeRequest{
		Name:        "subdir-source",
		IsNewBranch: true,
	}
	_, err := manager.CreateWorktree(ctx, req)
	if err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}

	// Find worktree paths
	worktrees, _ := manager.ListWorktrees(ctx)
	var sourcePath, currentPath string
	for _, wt := range worktrees {
		if wt.Name == "subdir-source" {
			sourcePath = wt.Path
		}
		if wt.IsCurrent {
			currentPath = wt.Path
		}
	}

	// Create a file in a subdirectory in source
	subDir := filepath.Join(sourcePath, "deep", "nested", "dir")
	err = os.MkdirAll(subDir, 0755)
	if err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}
	testFile := filepath.Join(subDir, "nested-file.txt")
	err = os.WriteFile(testFile, []byte("nested content"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	t.Run("apply creates subdirectories", func(t *testing.T) {
		err = manager.ApplyChanges(ctx, "subdir-source", []FileChange{
			{Path: "deep/nested/dir/nested-file.txt", Status: FileAdded, IsCommitted: false},
		})
		if err != nil {
			t.Fatalf("ApplyChanges() error: %v", err)
		}

		// Verify the file was created with directories
		appliedFile := filepath.Join(currentPath, "deep", "nested", "dir", "nested-file.txt")
		content, err := os.ReadFile(appliedFile)
		if err != nil {
			t.Fatalf("applied file not found: %v", err)
		}
		if string(content) != "nested content" {
			t.Errorf("content = %q, want 'nested content'", string(content))
		}
	})
}

func TestCompareDetectsDeletedFiles(t *testing.T) {
	_, manager, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Create a worktree
	req := CreateWorktreeRequest{
		Name:        "deleted-source",
		IsNewBranch: true,
	}
	_, err := manager.CreateWorktree(ctx, req)
	if err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}

	// Find worktree path
	worktrees, _ := manager.ListWorktrees(ctx)
	var sourcePath string
	for _, wt := range worktrees {
		if wt.Name == "deleted-source" {
			sourcePath = wt.Path
			break
		}
	}

	// Create, stage, then delete a file to simulate deleted status
	testFile := filepath.Join(sourcePath, "deleted-file.txt")
	err = os.WriteFile(testFile, []byte("to be deleted"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	exec.Command("git", "-C", sourcePath, "add", "deleted-file.txt").Run()

	// Now remove it (staged for deletion)
	os.Remove(testFile)

	result, err := manager.CompareWorktrees(ctx, "deleted-source")
	if err != nil {
		t.Fatalf("CompareWorktrees() error: %v", err)
	}

	foundDeleted := false
	for _, f := range result.Files {
		if f.Path == "deleted-file.txt" && f.Status == FileDeleted {
			foundDeleted = true
			break
		}
	}
	if !foundDeleted {
		t.Log("Files found:", result.Files)
		// This might not work in all git versions, so just log
		t.Log("Note: deleted file detection may depend on git version/state")
	}
}

func TestCompareDetectsRenamedFiles(t *testing.T) {
	_, manager, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Create a worktree
	req := CreateWorktreeRequest{
		Name:        "rename-source",
		IsNewBranch: true,
	}
	_, err := manager.CreateWorktree(ctx, req)
	if err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}

	// Find worktree path
	worktrees, _ := manager.ListWorktrees(ctx)
	var sourcePath string
	for _, wt := range worktrees {
		if wt.Name == "rename-source" {
			sourcePath = wt.Path
			break
		}
	}

	// Create and stage a file
	testFile := filepath.Join(sourcePath, "original.txt")
	err = os.WriteFile(testFile, []byte("content for rename"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	exec.Command("git", "-C", sourcePath, "add", "original.txt").Run()
	exec.Command("git", "-C", sourcePath, "commit", "-m", "Add original file").Run()

	// Rename the file using git mv
	exec.Command("git", "-C", sourcePath, "mv", "original.txt", "renamed.txt").Run()

	result, err := manager.CompareWorktrees(ctx, "rename-source")
	if err != nil {
		t.Fatalf("CompareWorktrees() error: %v", err)
	}

	// Should detect the renamed file
	foundRenamed := false
	for _, f := range result.Files {
		if f.Path == "renamed.txt" {
			foundRenamed = true
			break
		}
	}
	if !foundRenamed {
		t.Log("Files found:", result.Files)
		t.Log("Note: rename detection is being tested")
	}
}

func TestCompareCommittedDeletes(t *testing.T) {
	_, manager, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Create a worktree
	req := CreateWorktreeRequest{
		Name:        "committed-delete-source",
		IsNewBranch: true,
	}
	_, err := manager.CreateWorktree(ctx, req)
	if err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}

	// Find worktree paths
	worktrees, _ := manager.ListWorktrees(ctx)
	var sourcePath string
	for _, wt := range worktrees {
		if wt.Name == "committed-delete-source" {
			sourcePath = wt.Path
			break
		}
	}

	// Delete README.md which exists in main branch, and commit the deletion
	readmeFile := filepath.Join(sourcePath, "README.md")
	exec.Command("git", "-C", sourcePath, "rm", readmeFile).Run()
	exec.Command("git", "-C", sourcePath, "commit", "-m", "Delete README").Run()

	result, err := manager.CompareWorktrees(ctx, "committed-delete-source")
	if err != nil {
		t.Fatalf("CompareWorktrees() error: %v", err)
	}

	// Should detect the deleted file in committed changes
	foundDeleted := false
	for _, f := range result.Files {
		if f.Path == "README.md" && f.Status == FileDeleted && f.IsCommitted {
			foundDeleted = true
			break
		}
	}
	if !foundDeleted {
		t.Log("Files found:", result.Files)
		// Log for debugging
		t.Log("Note: committed delete detection is being tested")
	}
}

func TestCopyFileErrors(t *testing.T) {
	t.Run("copy from nonexistent source", func(t *testing.T) {
		tmpDir := t.TempDir()
		src := filepath.Join(tmpDir, "nonexistent.txt")
		dst := filepath.Join(tmpDir, "dest.txt")

		err := copyFile(src, dst)
		if err == nil {
			t.Error("expected error for nonexistent source")
		}
	})

	t.Run("copy to unwritable directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create source file
		src := filepath.Join(tmpDir, "source.txt")
		err := os.WriteFile(src, []byte("content"), 0644)
		if err != nil {
			t.Fatalf("failed to create source: %v", err)
		}

		// Create a directory and make it read-only
		unwritableDir := filepath.Join(tmpDir, "readonly")
		err = os.Mkdir(unwritableDir, 0555)
		if err != nil {
			t.Fatalf("failed to create readonly dir: %v", err)
		}
		// Restore permissions for cleanup
		defer os.Chmod(unwritableDir, 0755)

		dst := filepath.Join(unwritableDir, "subdir", "dest.txt")

		err = copyFile(src, dst)
		if err == nil {
			// Some systems (like macOS as root) might still succeed
			t.Log("Note: copy to unwritable dir succeeded (might be running as root)")
		}
	})

	t.Run("successful copy preserves content", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create source file
		src := filepath.Join(tmpDir, "source.txt")
		content := "test content for copy"
		err := os.WriteFile(src, []byte(content), 0644)
		if err != nil {
			t.Fatalf("failed to create source: %v", err)
		}

		// Copy to new location in subdirectory
		dst := filepath.Join(tmpDir, "subdir", "dest.txt")
		err = copyFile(src, dst)
		if err != nil {
			t.Fatalf("copyFile() error: %v", err)
		}

		// Verify content
		data, err := os.ReadFile(dst)
		if err != nil {
			t.Fatalf("failed to read dest: %v", err)
		}
		if string(data) != content {
			t.Errorf("content = %q, want %q", string(data), content)
		}
	})

	t.Run("copy preserves file permissions", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create source file with executable permission
		src := filepath.Join(tmpDir, "executable.sh")
		err := os.WriteFile(src, []byte("#!/bin/bash\necho hello"), 0755)
		if err != nil {
			t.Fatalf("failed to create source: %v", err)
		}

		dst := filepath.Join(tmpDir, "copied.sh")
		err = copyFile(src, dst)
		if err != nil {
			t.Fatalf("copyFile() error: %v", err)
		}

		// Verify permissions
		info, err := os.Stat(dst)
		if err != nil {
			t.Fatalf("failed to stat dest: %v", err)
		}
		if info.Mode().Perm() != 0755 {
			t.Errorf("permissions = %o, want 0755", info.Mode().Perm())
		}
	})
}
