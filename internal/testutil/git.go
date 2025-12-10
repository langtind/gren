package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// CreateTempRepo creates a bare git repository in a temp directory.
// Returns the path and a cleanup function.
func CreateTempRepo(t *testing.T) (string, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "gren-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		os.RemoveAll(dir)
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Configure git user for commits
	configUser := exec.Command("git", "config", "user.email", "test@test.com")
	configUser.Dir = dir
	configUser.Run()

	configName := exec.Command("git", "config", "user.name", "Test User")
	configName.Dir = dir
	configName.Run()

	cleanup := func() {
		os.RemoveAll(dir)
	}

	return dir, cleanup
}

// CreateTempRepoWithCommit creates a git repo with an initial commit.
func CreateTempRepoWithCommit(t *testing.T) (string, func()) {
	t.Helper()

	dir, cleanup := CreateTempRepo(t)

	// Create a file and commit it
	testFile := filepath.Join(dir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test Repo\n"), 0644); err != nil {
		cleanup()
		t.Fatalf("failed to create test file: %v", err)
	}

	addCmd := exec.Command("git", "add", ".")
	addCmd.Dir = dir
	if err := addCmd.Run(); err != nil {
		cleanup()
		t.Fatalf("failed to git add: %v", err)
	}

	commitCmd := exec.Command("git", "commit", "-m", "Initial commit")
	commitCmd.Dir = dir
	if err := commitCmd.Run(); err != nil {
		cleanup()
		t.Fatalf("failed to git commit: %v", err)
	}

	return dir, cleanup
}

// CreateTempRepoWithBranches creates a git repo with multiple branches.
func CreateTempRepoWithBranches(t *testing.T, branches []string) (string, func()) {
	t.Helper()

	dir, cleanup := CreateTempRepoWithCommit(t)

	for _, branch := range branches {
		branchCmd := exec.Command("git", "branch", branch)
		branchCmd.Dir = dir
		if err := branchCmd.Run(); err != nil {
			cleanup()
			t.Fatalf("failed to create branch %s: %v", branch, err)
		}
	}

	return dir, cleanup
}

// AddWorktree adds a worktree to an existing repo.
func AddWorktree(t *testing.T, repoPath, worktreePath, branch string) {
	t.Helper()

	cmd := exec.Command("git", "worktree", "add", worktreePath, branch)
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add worktree: %v", err)
	}
}

// CreateFileInRepo creates a file with content in the repo.
func CreateFileInRepo(t *testing.T, repoPath, filename, content string) {
	t.Helper()

	filePath := filepath.Join(repoPath, filename)
	dir := filepath.Dir(filePath)
	if dir != repoPath {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create directory %s: %v", dir, err)
		}
	}

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create file %s: %v", filename, err)
	}
}

// CommitFile stages and commits a file in the repo.
func CommitFile(t *testing.T, repoPath, filename, message string) {
	t.Helper()

	addCmd := exec.Command("git", "add", filename)
	addCmd.Dir = repoPath
	if err := addCmd.Run(); err != nil {
		t.Fatalf("failed to git add %s: %v", filename, err)
	}

	commitCmd := exec.Command("git", "commit", "-m", message)
	commitCmd.Dir = repoPath
	if err := commitCmd.Run(); err != nil {
		t.Fatalf("failed to git commit: %v", err)
	}
}

// GetCurrentBranch returns the current branch name.
func GetCurrentBranch(t *testing.T, repoPath string) string {
	t.Helper()

	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get current branch: %v", err)
	}

	// Trim newline
	branch := string(output)
	if len(branch) > 0 && branch[len(branch)-1] == '\n' {
		branch = branch[:len(branch)-1]
	}

	return branch
}
