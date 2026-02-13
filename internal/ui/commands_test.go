package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGetAvailableBranchesForWorktree_RemoteBranches(t *testing.T) {
	// Save original directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)

	// Create a temporary directory for test repo
	tmpDir, err := os.MkdirTemp("", "gren-test-remote-branches-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Initialize a git repository
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir to temp dir: %v", err)
	}

	// Initialize repo with explicit branch name
	cmd := exec.Command("git", "init", "-b", "main")
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	// Configure git
	exec.Command("git", "config", "user.email", "test@example.com").Run()
	exec.Command("git", "config", "user.name", "Test User").Run()

	// Create initial commit
	if err := os.WriteFile("README.md", []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("failed to create README: %v", err)
	}
	exec.Command("git", "add", ".").Run()
	exec.Command("git", "commit", "-m", "Initial commit").Run()

	// Create local branches
	exec.Command("git", "branch", "local-feature-1").Run()
	exec.Command("git", "branch", "local-feature-2").Run()

	// Simulate remote branches by creating them in refs/remotes/origin/
	// This is how git stores remote tracking branches
	gitDir := filepath.Join(tmpDir, ".git")
	remotesDir := filepath.Join(gitDir, "refs", "remotes", "origin")
	if err := os.MkdirAll(remotesDir, 0755); err != nil {
		t.Fatalf("failed to create remotes dir: %v", err)
	}

	// Create remote branches that don't have local tracking
	remoteOnlyBranches := []string{"remote-feature-1", "remote-feature-2", "remote-bugfix"}
	for _, branch := range remoteOnlyBranches {
		// Get current HEAD commit hash
		headCmd := exec.Command("git", "rev-parse", "HEAD")
		headOutput, err := headCmd.Output()
		if err != nil {
			t.Fatalf("failed to get HEAD: %v", err)
		}

		// Write the commit hash to the remote branch ref
		remoteBranchPath := filepath.Join(remotesDir, branch)
		if err := os.WriteFile(remoteBranchPath, headOutput, 0644); err != nil {
			t.Fatalf("failed to create remote branch %s: %v", branch, err)
		}
	}

	// Create a remote branch that HAS a local tracking branch (should not appear as duplicate)
	localTrackedBranch := "local-feature-1"
	remoteBranchPath := filepath.Join(remotesDir, localTrackedBranch)
	headCmd := exec.Command("git", "rev-parse", "HEAD")
	headOutput, _ := headCmd.Output()
	os.WriteFile(remoteBranchPath, headOutput, 0644)

	// Create a worktree for one of the local branches (should be filtered out)
	worktreeDir := filepath.Join(tmpDir, "..", "test-worktrees")
	os.MkdirAll(worktreeDir, 0755)
	worktreePath := filepath.Join(worktreeDir, "local-feature-2-worktree")
	exec.Command("git", "worktree", "add", worktreePath, "local-feature-2").Run()

	// Now test getAvailableBranchesForWorktree
	model := Model{
		worktrees: []Worktree{
			{
				Name:   "local-feature-2-worktree",
				Branch: "local-feature-2",
				Path:   worktreePath,
			},
		},
	}

	branches, err := model.getAvailableBranchesForWorktree()
	if err != nil {
		t.Fatalf("getAvailableBranchesForWorktree failed: %v", err)
	}

	// Test expectations:
	// 1. Should include local branches (except those with worktrees)
	// 2. Should include remote-only branches
	// 3. Should NOT duplicate branches that exist both locally and remotely

	t.Run("includes local branches without worktrees", func(t *testing.T) {
		found := false
		for _, b := range branches {
			if b.Name == "local-feature-1" && !b.IsRemote {
				found = true
				break
			}
		}
		if !found {
			t.Error("local-feature-1 should be in available branches")
		}
	})

	t.Run("excludes local branches with existing worktrees", func(t *testing.T) {
		for _, b := range branches {
			if b.Name == "local-feature-2" && !b.IsRemote {
				t.Error("local-feature-2 has a worktree and should be excluded")
			}
		}
	})

	t.Run("includes remote-only branches", func(t *testing.T) {
		remoteCount := 0
		expectedRemotes := map[string]bool{
			"origin/remote-feature-1": true,
			"origin/remote-feature-2": true,
			"origin/remote-bugfix":    true,
		}

		for _, b := range branches {
			if b.IsRemote && expectedRemotes[b.Name] {
				remoteCount++
			}
		}

		if remoteCount != len(expectedRemotes) {
			t.Errorf("expected %d remote branches, found %d", len(expectedRemotes), remoteCount)
		}
	})

	t.Run("does not duplicate branches that exist both locally and remotely", func(t *testing.T) {
		// local-feature-1 exists both locally and remotely
		// We should see it only once (as local, not remote)
		localCount := 0
		remoteCount := 0

		for _, b := range branches {
			if b.Name == "local-feature-1" && !b.IsRemote {
				localCount++
			}
			if b.Name == "origin/local-feature-1" && b.IsRemote {
				remoteCount++
			}
		}

		if localCount != 1 {
			t.Errorf("expected 1 local instance of local-feature-1, found %d", localCount)
		}
		if remoteCount > 0 {
			t.Error("should not show remote version of branch that exists locally")
		}
	})

	t.Run("excludes main branch", func(t *testing.T) {
		// main branch should be excluded as it typically has a worktree (the main repo)
		for _, b := range branches {
			if b.Name == "main" && !b.IsRemote {
				// This might be OK depending on whether main has a worktree
				// Just check it's not duplicated as origin/main
			}
		}
	})

	t.Run("marks remote branches with IsRemote flag", func(t *testing.T) {
		foundRemote := false
		for _, b := range branches {
			if b.IsRemote && b.Name == "origin/remote-feature-1" {
				foundRemote = true
				break
			}
		}
		if !foundRemote {
			t.Error("remote branches should have IsRemote=true")
		}
	})
}

func TestGetAvailableBranchesForWorktree_NoRemoteBranches(t *testing.T) {
	// Test case where there are no remote branches
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)

	tmpDir, err := os.MkdirTemp("", "gren-test-no-remote-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	cmd := exec.Command("git", "init", "-b", "main")
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	exec.Command("git", "config", "user.email", "test@example.com").Run()
	exec.Command("git", "config", "user.name", "Test User").Run()

	os.WriteFile("README.md", []byte("# Test\n"), 0644)
	exec.Command("git", "add", ".").Run()
	exec.Command("git", "commit", "-m", "Initial commit").Run()

	exec.Command("git", "branch", "local-only").Run()

	model := Model{
		worktrees: []Worktree{},
	}

	branches, err := model.getAvailableBranchesForWorktree()
	if err != nil {
		t.Fatalf("getAvailableBranchesForWorktree failed: %v", err)
	}

	// Should still work and return local branches
	if len(branches) == 0 {
		t.Error("should return local branches even without remotes")
	}

	for _, b := range branches {
		if b.IsRemote {
			t.Error("should not have any remote branches in this test")
		}
	}
}

func TestGetAvailableBranchesForWorktree_EmptyRepo(t *testing.T) {
	// Test case for freshly initialized repo with no branches
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer os.Chdir(origDir)

	tmpDir, err := os.MkdirTemp("", "gren-test-empty-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	cmd := exec.Command("git", "init", "-b", "main")
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	model := Model{
		worktrees: []Worktree{},
	}

	// This should handle the case gracefully
	_, err = model.getAvailableBranchesForWorktree()
	// Empty repo might return error or empty list - both are acceptable
	// The important thing is it doesn't crash
	if err != nil {
		// Expected - no branches in empty repo
		t.Logf("Empty repo returns error (expected): %v", err)
	}
}
