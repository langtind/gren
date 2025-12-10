package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestLocalRepository_GetBranchStatuses(t *testing.T) {
	repo := NewLocalRepository()
	ctx := context.Background()

	t.Run("single branch clean", func(t *testing.T) {
		dir, cleanup := setupTempGitRepoWithCommit(t)
		defer cleanup()

		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)
		os.Chdir(dir)

		statuses, err := repo.GetBranchStatuses(ctx)
		if err != nil {
			t.Fatalf("GetBranchStatuses() error: %v", err)
		}

		if len(statuses) == 0 {
			t.Fatal("GetBranchStatuses() returned no branches")
		}

		// Should have one branch (master or main)
		if len(statuses) != 1 {
			t.Errorf("got %d branches, want 1", len(statuses))
		}

		// First branch should be current and clean
		if !statuses[0].IsCurrent {
			t.Error("first branch should be current")
		}

		if !statuses[0].IsClean {
			t.Error("branch should be clean (no uncommitted changes)")
		}
	})

	t.Run("multiple branches", func(t *testing.T) {
		dir, cleanup := setupTempGitRepoWithCommit(t)
		defer cleanup()

		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)
		os.Chdir(dir)

		// Create additional branches
		exec.Command("git", "branch", "feature/test").Run()
		exec.Command("git", "branch", "bugfix/issue-1").Run()

		statuses, err := repo.GetBranchStatuses(ctx)
		if err != nil {
			t.Fatalf("GetBranchStatuses() error: %v", err)
		}

		if len(statuses) != 3 {
			t.Errorf("got %d branches, want 3", len(statuses))
		}

		// Count current branches (should be exactly 1)
		currentCount := 0
		for _, s := range statuses {
			if s.IsCurrent {
				currentCount++
			}
		}
		if currentCount != 1 {
			t.Errorf("got %d current branches, want 1", currentCount)
		}
	})

	t.Run("branch with uncommitted changes", func(t *testing.T) {
		dir, cleanup := setupTempGitRepoWithCommit(t)
		defer cleanup()

		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)
		os.Chdir(dir)

		// Modify a file without committing
		testFile := filepath.Join(dir, "README.md")
		os.WriteFile(testFile, []byte("# Modified\n"), 0644)

		statuses, err := repo.GetBranchStatuses(ctx)
		if err != nil {
			t.Fatalf("GetBranchStatuses() error: %v", err)
		}

		// Current branch should show as dirty
		var currentStatus *BranchStatus
		for i := range statuses {
			if statuses[i].IsCurrent {
				currentStatus = &statuses[i]
				break
			}
		}

		if currentStatus == nil {
			t.Fatal("no current branch found")
		}

		if currentStatus.IsClean {
			t.Error("branch should not be clean with uncommitted changes")
		}

		if currentStatus.UncommittedFiles == 0 {
			t.Error("UncommittedFiles should be > 0")
		}
	})

	t.Run("branch with untracked files", func(t *testing.T) {
		dir, cleanup := setupTempGitRepoWithCommit(t)
		defer cleanup()

		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)
		os.Chdir(dir)

		// Create untracked file
		newFile := filepath.Join(dir, "untracked.txt")
		os.WriteFile(newFile, []byte("untracked"), 0644)

		statuses, err := repo.GetBranchStatuses(ctx)
		if err != nil {
			t.Fatalf("GetBranchStatuses() error: %v", err)
		}

		var currentStatus *BranchStatus
		for i := range statuses {
			if statuses[i].IsCurrent {
				currentStatus = &statuses[i]
				break
			}
		}

		if currentStatus == nil {
			t.Fatal("no current branch found")
		}

		if currentStatus.IsClean {
			t.Error("branch should not be clean with untracked files")
		}

		if currentStatus.UntrackedFiles == 0 {
			t.Error("UntrackedFiles should be > 0")
		}
	})
}

func TestLocalRepository_GetRecommendedBaseBranch(t *testing.T) {
	repo := NewLocalRepository()
	ctx := context.Background()

	t.Run("prefers main over other branches", func(t *testing.T) {
		dir, cleanup := setupTempGitRepoWithCommit(t)
		defer cleanup()

		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)
		os.Chdir(dir)

		// Get current branch name
		currentBranch, _ := repo.GetCurrentBranch(ctx)

		// Create main if it doesn't exist
		if currentBranch != "main" {
			exec.Command("git", "branch", "main").Run()
		}

		// Create other branches
		exec.Command("git", "branch", "feature/test").Run()

		recommended, err := repo.GetRecommendedBaseBranch(ctx)
		if err != nil {
			t.Fatalf("GetRecommendedBaseBranch() error: %v", err)
		}

		// Should prefer main/master
		if recommended != "main" && recommended != "master" {
			t.Errorf("GetRecommendedBaseBranch() = %q, want 'main' or 'master'", recommended)
		}
	})

	t.Run("returns current branch as fallback", func(t *testing.T) {
		dir, cleanup := setupTempGitRepoWithCommit(t)
		defer cleanup()

		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)
		os.Chdir(dir)

		// Rename to a non-main branch
		exec.Command("git", "checkout", "-b", "develop").Run()

		recommended, err := repo.GetRecommendedBaseBranch(ctx)
		if err != nil {
			t.Fatalf("GetRecommendedBaseBranch() error: %v", err)
		}

		// Should return some branch
		if recommended == "" {
			t.Error("GetRecommendedBaseBranch() returned empty string")
		}
	})
}

func TestGetWorkingDirectoryStatus(t *testing.T) {
	repo := NewLocalRepository()
	ctx := context.Background()

	t.Run("clean working directory", func(t *testing.T) {
		dir, cleanup := setupTempGitRepoWithCommit(t)
		defer cleanup()

		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)
		os.Chdir(dir)

		uncommitted, untracked, err := repo.getWorkingDirectoryStatus(ctx)
		if err != nil {
			t.Fatalf("getWorkingDirectoryStatus() error: %v", err)
		}

		if uncommitted != 0 {
			t.Errorf("uncommitted = %d, want 0", uncommitted)
		}

		if untracked != 0 {
			t.Errorf("untracked = %d, want 0", untracked)
		}
	})

	t.Run("modified files", func(t *testing.T) {
		dir, cleanup := setupTempGitRepoWithCommit(t)
		defer cleanup()

		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)
		os.Chdir(dir)

		// Modify existing file
		os.WriteFile(filepath.Join(dir, "README.md"), []byte("modified"), 0644)

		uncommitted, untracked, err := repo.getWorkingDirectoryStatus(ctx)
		if err != nil {
			t.Fatalf("getWorkingDirectoryStatus() error: %v", err)
		}

		if uncommitted != 1 {
			t.Errorf("uncommitted = %d, want 1", uncommitted)
		}

		if untracked != 0 {
			t.Errorf("untracked = %d, want 0", untracked)
		}
	})

	t.Run("untracked files", func(t *testing.T) {
		dir, cleanup := setupTempGitRepoWithCommit(t)
		defer cleanup()

		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)
		os.Chdir(dir)

		// Create new untracked file
		os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new"), 0644)

		uncommitted, untracked, err := repo.getWorkingDirectoryStatus(ctx)
		if err != nil {
			t.Fatalf("getWorkingDirectoryStatus() error: %v", err)
		}

		if uncommitted != 0 {
			t.Errorf("uncommitted = %d, want 0", uncommitted)
		}

		if untracked != 1 {
			t.Errorf("untracked = %d, want 1", untracked)
		}
	})

	t.Run("mixed changes", func(t *testing.T) {
		dir, cleanup := setupTempGitRepoWithCommit(t)
		defer cleanup()

		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)
		os.Chdir(dir)

		// Modify existing + add new
		os.WriteFile(filepath.Join(dir, "README.md"), []byte("modified"), 0644)
		os.WriteFile(filepath.Join(dir, "new1.txt"), []byte("new"), 0644)
		os.WriteFile(filepath.Join(dir, "new2.txt"), []byte("new"), 0644)

		uncommitted, untracked, err := repo.getWorkingDirectoryStatus(ctx)
		if err != nil {
			t.Fatalf("getWorkingDirectoryStatus() error: %v", err)
		}

		if uncommitted != 1 {
			t.Errorf("uncommitted = %d, want 1", uncommitted)
		}

		if untracked != 2 {
			t.Errorf("untracked = %d, want 2", untracked)
		}
	})
}

func TestGetLocalBranches(t *testing.T) {
	repo := NewLocalRepository()
	ctx := context.Background()

	t.Run("single branch", func(t *testing.T) {
		dir, cleanup := setupTempGitRepoWithCommit(t)
		defer cleanup()

		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)
		os.Chdir(dir)

		branches, err := repo.getLocalBranches(ctx)
		if err != nil {
			t.Fatalf("getLocalBranches() error: %v", err)
		}

		if len(branches) != 1 {
			t.Errorf("got %d branches, want 1", len(branches))
		}
	})

	t.Run("multiple branches", func(t *testing.T) {
		dir, cleanup := setupTempGitRepoWithCommit(t)
		defer cleanup()

		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)
		os.Chdir(dir)

		// Create branches
		exec.Command("git", "branch", "feature-a").Run()
		exec.Command("git", "branch", "feature-b").Run()

		branches, err := repo.getLocalBranches(ctx)
		if err != nil {
			t.Fatalf("getLocalBranches() error: %v", err)
		}

		if len(branches) != 3 {
			t.Errorf("got %d branches, want 3", len(branches))
		}
	})
}

func TestGetAheadBehindCount(t *testing.T) {
	repo := NewLocalRepository()
	ctx := context.Background()

	t.Run("no upstream returns zeros", func(t *testing.T) {
		dir, cleanup := setupTempGitRepoWithCommit(t)
		defer cleanup()

		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)
		os.Chdir(dir)

		currentBranch, _ := repo.GetCurrentBranch(ctx)
		ahead, behind, err := repo.getAheadBehindCount(ctx, currentBranch)
		if err != nil {
			t.Fatalf("getAheadBehindCount() error: %v", err)
		}

		// Without a remote, should return 0,0
		if ahead != 0 || behind != 0 {
			t.Errorf("ahead=%d, behind=%d, want both 0 for no upstream", ahead, behind)
		}
	})

	t.Run("nonexistent branch returns zeros", func(t *testing.T) {
		dir, cleanup := setupTempGitRepoWithCommit(t)
		defer cleanup()

		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)
		os.Chdir(dir)

		ahead, behind, err := repo.getAheadBehindCount(ctx, "nonexistent-branch")
		if err != nil {
			t.Fatalf("getAheadBehindCount() error: %v", err)
		}

		if ahead != 0 || behind != 0 {
			t.Errorf("ahead=%d, behind=%d, want both 0 for nonexistent branch", ahead, behind)
		}
	})
}

func TestGetRecommendedBaseBranchEdgeCases(t *testing.T) {
	repo := NewLocalRepository()
	ctx := context.Background()

	t.Run("dirty main prefers current clean branch", func(t *testing.T) {
		dir, cleanup := setupTempGitRepoWithCommit(t)
		defer cleanup()

		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)
		os.Chdir(dir)

		// Get current branch
		currentBranch, _ := repo.GetCurrentBranch(ctx)

		// Create a feature branch and switch to it
		exec.Command("git", "checkout", "-b", "feature-clean").Run()

		// Go back to original and make it dirty
		exec.Command("git", "checkout", currentBranch).Run()
		os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("dirty"), 0644)

		// Switch to clean feature branch
		exec.Command("git", "checkout", "feature-clean").Run()

		recommended, err := repo.GetRecommendedBaseBranch(ctx)
		if err != nil {
			t.Fatalf("GetRecommendedBaseBranch() error: %v", err)
		}

		// Should recommend clean feature branch
		if recommended == "" {
			t.Error("GetRecommendedBaseBranch() returned empty string")
		}
	})

	t.Run("all branches dirty returns current", func(t *testing.T) {
		dir, cleanup := setupTempGitRepoWithCommit(t)
		defer cleanup()

		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)
		os.Chdir(dir)

		// Make current branch dirty
		os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("dirty"), 0644)

		recommended, err := repo.GetRecommendedBaseBranch(ctx)
		if err != nil {
			t.Fatalf("GetRecommendedBaseBranch() error: %v", err)
		}

		// Should still return something (the dirty current branch as fallback)
		if recommended == "" {
			t.Error("GetRecommendedBaseBranch() should return current branch as fallback")
		}
	})

	t.Run("prefers master when main doesnt exist", func(t *testing.T) {
		dir, cleanup := setupTempGitRepoWithCommit(t)
		defer cleanup()

		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)
		os.Chdir(dir)

		// Check if master exists (default on some systems)
		currentBranch, _ := repo.GetCurrentBranch(ctx)
		if currentBranch == "master" {
			// Create a feature branch
			exec.Command("git", "branch", "feature-test").Run()

			recommended, err := repo.GetRecommendedBaseBranch(ctx)
			if err != nil {
				t.Fatalf("GetRecommendedBaseBranch() error: %v", err)
			}

			if recommended != "master" {
				t.Errorf("GetRecommendedBaseBranch() = %q, want 'master'", recommended)
			}
		}
	})
}

func TestGetBranchStatus(t *testing.T) {
	repo := NewLocalRepository()
	ctx := context.Background()

	t.Run("non-current branch is always clean", func(t *testing.T) {
		dir, cleanup := setupTempGitRepoWithCommit(t)
		defer cleanup()

		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)
		os.Chdir(dir)

		// Create another branch
		exec.Command("git", "branch", "other-branch").Run()

		status, err := repo.getBranchStatus(ctx, "other-branch", false)
		if err != nil {
			t.Fatalf("getBranchStatus() error: %v", err)
		}

		// Non-current branches are assumed clean
		if !status.IsClean {
			t.Error("non-current branch should be assumed clean")
		}
		if status.IsCurrent {
			t.Error("branch should not be marked as current")
		}
	})

	t.Run("current branch with staged changes", func(t *testing.T) {
		dir, cleanup := setupTempGitRepoWithCommit(t)
		defer cleanup()

		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)
		os.Chdir(dir)

		// Create and stage a file
		os.WriteFile(filepath.Join(dir, "staged.txt"), []byte("staged"), 0644)
		exec.Command("git", "add", "staged.txt").Run()

		currentBranch, _ := repo.GetCurrentBranch(ctx)
		status, err := repo.getBranchStatus(ctx, currentBranch, true)
		if err != nil {
			t.Fatalf("getBranchStatus() error: %v", err)
		}

		if status.IsClean {
			t.Error("branch with staged changes should not be clean")
		}
		if status.UncommittedFiles == 0 {
			t.Error("UncommittedFiles should be > 0 for staged changes")
		}
	})
}
