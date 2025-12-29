package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestIsBranchMerged(t *testing.T) {
	dir, manager, cleanup := setupTestEnvironment(t)
	defer cleanup()

	t.Run("merged branch shows no unique commits after merge", func(t *testing.T) {
		// Create a branch with a commit, then merge it
		// After merging, commits are in main so hasUniqueCommits=false
		exec.Command("git", "-C", dir, "checkout", "-b", "merged-branch").Run()
		testFile := filepath.Join(dir, "merged-file.txt")
		os.WriteFile(testFile, []byte("merged content"), 0644)
		exec.Command("git", "-C", dir, "add", ".").Run()
		exec.Command("git", "-C", dir, "commit", "-m", "Merged commit").Run()
		exec.Command("git", "-C", dir, "checkout", "-").Run() // back to main/master
		exec.Command("git", "-C", dir, "merge", "--no-ff", "merged-branch", "-m", "Merge merged-branch").Run()

		merged, hasUniqueCommits := manager.isBranchMerged("merged-branch")
		if !merged {
			t.Error("isBranchMerged() merged=false for merged branch, want true")
		}
		// After merging, commits are now in main, so hasUniqueCommits is false
		if hasUniqueCommits {
			t.Error("isBranchMerged() hasUniqueCommits=true after merge, want false (commits are now in main)")
		}
	})

	t.Run("empty branch returns merged=true, hasUniqueCommits=false", func(t *testing.T) {
		// Create a branch with no unique commits
		exec.Command("git", "-C", dir, "checkout", "-b", "empty-branch").Run()
		exec.Command("git", "-C", dir, "checkout", "-").Run()

		merged, hasUniqueCommits := manager.isBranchMerged("empty-branch")
		if !merged {
			t.Error("isBranchMerged() merged=false for empty branch, want true")
		}
		if hasUniqueCommits {
			t.Error("isBranchMerged() hasUniqueCommits=true for empty branch, want false")
		}
	})

	t.Run("unmerged branch returns false", func(t *testing.T) {
		// Create a branch with a new commit (not merged)
		exec.Command("git", "-C", dir, "checkout", "-b", "unmerged-branch").Run()
		testFile := filepath.Join(dir, "unmerged-file.txt")
		os.WriteFile(testFile, []byte("unmerged content"), 0644)
		exec.Command("git", "-C", dir, "add", ".").Run()
		exec.Command("git", "-C", dir, "commit", "-m", "Unmerged commit").Run()
		exec.Command("git", "-C", dir, "checkout", "-").Run()

		merged, _ := manager.isBranchMerged("unmerged-branch")
		if merged {
			t.Error("isBranchMerged() merged=true for unmerged branch, want false")
		}
	})

	t.Run("nonexistent branch returns false", func(t *testing.T) {
		merged, _ := manager.isBranchMerged("nonexistent-branch-xyz")
		if merged {
			t.Error("isBranchMerged() merged=true for nonexistent branch, want false")
		}
	})
}

func TestIsRemoteBranchGone(t *testing.T) {
	dir, manager, cleanup := setupTestEnvironment(t)
	defer cleanup()

	t.Run("branch without tracking returns false", func(t *testing.T) {
		// Create a local branch without remote tracking
		exec.Command("git", "-C", dir, "checkout", "-b", "local-only-branch").Run()
		exec.Command("git", "-C", dir, "checkout", "-").Run()

		result := manager.isRemoteBranchGone("local-only-branch")
		if result {
			t.Error("isRemoteBranchGone() = true for local-only branch, want false")
		}
	})

	t.Run("branch with active remote returns false", func(t *testing.T) {
		// Set up a remote
		remoteDir, err := os.MkdirTemp("", "gren-remote-test-*")
		if err != nil {
			t.Fatalf("failed to create remote dir: %v", err)
		}
		defer os.RemoveAll(remoteDir)

		exec.Command("git", "-C", remoteDir, "init", "--bare").Run()
		exec.Command("git", "-C", dir, "remote", "add", "origin", remoteDir).Run()

		// Create branch and push
		exec.Command("git", "-C", dir, "checkout", "-b", "remote-active").Run()
		exec.Command("git", "-C", dir, "push", "-u", "origin", "remote-active").Run()
		exec.Command("git", "-C", dir, "checkout", "-").Run()

		result := manager.isRemoteBranchGone("remote-active")
		if result {
			t.Error("isRemoteBranchGone() = true for branch with active remote, want false")
		}
	})

	t.Run("branch with gone remote returns true", func(t *testing.T) {
		// Set up a remote if not already done
		remoteDir, err := os.MkdirTemp("", "gren-remote-gone-*")
		if err != nil {
			t.Fatalf("failed to create remote dir: %v", err)
		}
		defer os.RemoveAll(remoteDir)

		// Remove existing origin if any and add new one
		exec.Command("git", "-C", dir, "remote", "remove", "origin").Run()
		exec.Command("git", "-C", remoteDir, "init", "--bare").Run()
		exec.Command("git", "-C", dir, "remote", "add", "origin", remoteDir).Run()

		// Create branch, push, then delete remote branch
		exec.Command("git", "-C", dir, "checkout", "-b", "remote-gone-branch").Run()
		exec.Command("git", "-C", dir, "push", "-u", "origin", "remote-gone-branch").Run()
		exec.Command("git", "-C", dir, "checkout", "-").Run()

		// Delete the remote branch
		exec.Command("git", "-C", dir, "push", "origin", "--delete", "remote-gone-branch").Run()

		// Fetch to update tracking info
		exec.Command("git", "-C", dir, "fetch", "--prune").Run()

		result := manager.isRemoteBranchGone("remote-gone-branch")
		if !result {
			t.Error("isRemoteBranchGone() = false for branch with gone remote, want true")
		}
	})
}

func TestEnrichStaleStatus(t *testing.T) {
	dir, manager, cleanup := setupTestEnvironment(t)
	defer cleanup()

	t.Run("main worktree stays active", func(t *testing.T) {
		wt := &WorktreeInfo{
			Name:   "main",
			Branch: "main",
			IsMain: true,
			Status: "clean",
		}

		manager.enrichStaleStatus(wt)

		if wt.BranchStatus != "active" {
			t.Errorf("BranchStatus = %q, want 'active'", wt.BranchStatus)
		}
		if wt.StaleReason != "" {
			t.Errorf("StaleReason = %q, want empty", wt.StaleReason)
		}
	})

	t.Run("missing worktree stays active", func(t *testing.T) {
		wt := &WorktreeInfo{
			Name:   "missing-wt",
			Branch: "some-branch",
			IsMain: false,
			Status: "missing",
		}

		manager.enrichStaleStatus(wt)

		if wt.BranchStatus != "active" {
			t.Errorf("BranchStatus = %q, want 'active'", wt.BranchStatus)
		}
	})

	t.Run("detached HEAD stays active", func(t *testing.T) {
		wt := &WorktreeInfo{
			Name:   "detached-wt",
			Branch: "(detached)",
			IsMain: false,
			Status: "clean",
		}

		manager.enrichStaleStatus(wt)

		if wt.BranchStatus != "active" {
			t.Errorf("BranchStatus = %q, want 'active'", wt.BranchStatus)
		}
	})

	t.Run("bare repo stays active", func(t *testing.T) {
		wt := &WorktreeInfo{
			Name:   "bare-wt",
			Branch: "(bare)",
			IsMain: false,
			Status: "clean",
		}

		manager.enrichStaleStatus(wt)

		if wt.BranchStatus != "active" {
			t.Errorf("BranchStatus = %q, want 'active'", wt.BranchStatus)
		}
	})

	t.Run("branch with no unique commits becomes stale", func(t *testing.T) {
		// Create an empty branch (no commits) - shows as stale with no_unique_commits
		exec.Command("git", "-C", dir, "checkout", "-b", "stale-empty").Run()
		exec.Command("git", "-C", dir, "checkout", "-").Run()

		wt := &WorktreeInfo{
			Name:   "stale-empty",
			Branch: "stale-empty",
			IsMain: false,
			Status: "clean",
		}

		manager.enrichStaleStatus(wt)

		if wt.BranchStatus != "stale" {
			t.Errorf("BranchStatus = %q, want 'stale'", wt.BranchStatus)
		}
		// Empty branches show as "no_unique_commits"
		if wt.StaleReason != "no_unique_commits" {
			t.Errorf("StaleReason = %q, want 'no_unique_commits'", wt.StaleReason)
		}
	})

	t.Run("active branch stays active", func(t *testing.T) {
		// Create a branch with unmerged commits
		exec.Command("git", "-C", dir, "checkout", "-b", "active-branch").Run()
		testFile := filepath.Join(dir, "active-file.txt")
		os.WriteFile(testFile, []byte("active content"), 0644)
		exec.Command("git", "-C", dir, "add", ".").Run()
		exec.Command("git", "-C", dir, "commit", "-m", "Active commit").Run()
		exec.Command("git", "-C", dir, "checkout", "-").Run()

		wt := &WorktreeInfo{
			Name:   "active-branch",
			Branch: "active-branch",
			IsMain: false,
			Status: "clean",
		}

		manager.enrichStaleStatus(wt)

		if wt.BranchStatus != "active" {
			t.Errorf("BranchStatus = %q, want 'active'", wt.BranchStatus)
		}
		if wt.StaleReason != "" {
			t.Errorf("StaleReason = %q, want empty", wt.StaleReason)
		}
	})
}

func TestStaleStatusInListWorktrees(t *testing.T) {
	dir, manager, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("ListWorktrees includes stale status", func(t *testing.T) {
		// Create a branch with a commit, then merge it
		exec.Command("git", "-C", dir, "checkout", "-b", "stale-in-list").Run()
		// Add a commit so there's something to merge
		testFile := filepath.Join(dir, "stale-list-file.txt")
		os.WriteFile(testFile, []byte("stale list content"), 0644)
		exec.Command("git", "-C", dir, "add", ".").Run()
		exec.Command("git", "-C", dir, "commit", "-m", "Stale list commit").Run()
		exec.Command("git", "-C", dir, "checkout", "-").Run()
		exec.Command("git", "-C", dir, "merge", "--no-ff", "stale-in-list", "-m", "Merge stale-in-list").Run()

		// Create a worktree for the merged branch
		worktreeDir := filepath.Join(filepath.Dir(dir), "test-worktrees")
		os.MkdirAll(worktreeDir, 0755)
		worktreePath := filepath.Join(worktreeDir, "stale-in-list")
		exec.Command("git", "-C", dir, "worktree", "add", worktreePath, "stale-in-list").Run()

		worktrees, err := manager.ListWorktrees(ctx)
		if err != nil {
			t.Fatalf("ListWorktrees() error: %v", err)
		}

		// Find the stale worktree
		var staleWt *WorktreeInfo
		for i := range worktrees {
			if worktrees[i].Branch == "stale-in-list" {
				staleWt = &worktrees[i]
				break
			}
		}

		if staleWt == nil {
			t.Fatal("stale-in-list worktree not found")
		}

		if staleWt.BranchStatus != "stale" {
			t.Errorf("BranchStatus = %q, want 'stale'", staleWt.BranchStatus)
		}
		// After merging, commits are in main, so shows as "no_unique_commits"
		if staleWt.StaleReason != "no_unique_commits" {
			t.Errorf("StaleReason = %q, want 'no_unique_commits'", staleWt.StaleReason)
		}
	})
}
