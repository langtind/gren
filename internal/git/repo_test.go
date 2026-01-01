package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupTempGitRepo creates a temporary git repository for testing.
func setupTempGitRepo(t *testing.T) (string, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "gren-git-test-*")
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

	cleanup := func() {
		os.RemoveAll(dir)
	}

	return dir, cleanup
}

// setupTempGitRepoWithCommit creates a temp git repo with an initial commit.
func setupTempGitRepoWithCommit(t *testing.T) (string, func()) {
	t.Helper()

	dir, cleanup := setupTempGitRepo(t)

	// Create a file
	testFile := filepath.Join(dir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test\n"), 0644); err != nil {
		cleanup()
		t.Fatalf("failed to create test file: %v", err)
	}

	// Add and commit
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "Initial commit").Run()

	return dir, cleanup
}

func TestLocalRepository_IsGitRepo(t *testing.T) {
	repo := NewLocalRepository()
	ctx := context.Background()

	t.Run("valid git repo", func(t *testing.T) {
		dir, cleanup := setupTempGitRepo(t)
		defer cleanup()

		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)
		os.Chdir(dir)

		isGit, err := repo.IsGitRepo(ctx)
		if err != nil {
			t.Errorf("IsGitRepo() error: %v", err)
		}
		if !isGit {
			t.Error("IsGitRepo() = false, want true")
		}
	})

	t.Run("not a git repo", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "not-git-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(dir)

		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)
		os.Chdir(dir)

		isGit, err := repo.IsGitRepo(ctx)
		if err != nil {
			t.Errorf("IsGitRepo() error: %v", err)
		}
		if isGit {
			t.Error("IsGitRepo() = true, want false")
		}
	})
}

func TestLocalRepository_GetRepoName(t *testing.T) {
	repo := NewLocalRepository()
	ctx := context.Background()

	t.Run("valid repo name", func(t *testing.T) {
		dir, cleanup := setupTempGitRepo(t)
		defer cleanup()

		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)
		os.Chdir(dir)

		name, err := repo.GetRepoName(ctx)
		if err != nil {
			t.Errorf("GetRepoName() error: %v", err)
		}
		if name == "" {
			t.Error("GetRepoName() returned empty string")
		}
		// Name should be the directory name
		expectedPrefix := "gren-git-test-"
		if len(name) < len(expectedPrefix) {
			t.Errorf("GetRepoName() = %q, expected to start with %q", name, expectedPrefix)
		}
	})
}

func TestLocalRepository_GetCurrentBranch(t *testing.T) {
	repo := NewLocalRepository()
	ctx := context.Background()

	t.Run("main branch after commit", func(t *testing.T) {
		dir, cleanup := setupTempGitRepoWithCommit(t)
		defer cleanup()

		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)
		os.Chdir(dir)

		branch, err := repo.GetCurrentBranch(ctx)
		if err != nil {
			t.Errorf("GetCurrentBranch() error: %v", err)
		}
		// Branch should be master or main depending on git config
		if branch != "master" && branch != "main" {
			t.Errorf("GetCurrentBranch() = %q, want 'master' or 'main'", branch)
		}
	})

	t.Run("empty branch before first commit", func(t *testing.T) {
		dir, cleanup := setupTempGitRepo(t)
		defer cleanup()

		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)
		os.Chdir(dir)

		branch, err := repo.GetCurrentBranch(ctx)
		if err != nil {
			t.Errorf("GetCurrentBranch() error: %v", err)
		}
		// Branch might be empty or show the initial branch name
		// This is fine - we just want no error
		_ = branch
	})
}

func TestLocalRepository_GetRepoInfo(t *testing.T) {
	repo := NewLocalRepository()
	ctx := context.Background()

	t.Run("complete repo info", func(t *testing.T) {
		dir, cleanup := setupTempGitRepoWithCommit(t)
		defer cleanup()

		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)
		os.Chdir(dir)

		info, err := repo.GetRepoInfo(ctx)
		if err != nil {
			t.Fatalf("GetRepoInfo() error: %v", err)
		}

		if !info.IsGitRepo {
			t.Error("IsGitRepo = false, want true")
		}

		if info.Name == "" {
			t.Error("Name is empty")
		}

		if info.Path == "" {
			t.Error("Path is empty")
		}

		if info.CurrentBranch == "" {
			t.Error("CurrentBranch is empty (should have branch after commit)")
		}
	})

	t.Run("non-git directory", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "not-git-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(dir)

		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)
		os.Chdir(dir)

		info, err := repo.GetRepoInfo(ctx)
		if err != nil {
			t.Fatalf("GetRepoInfo() error: %v", err)
		}

		if info.IsGitRepo {
			t.Error("IsGitRepo = true, want false for non-git directory")
		}

		// Should still have name and path from current directory
		if info.Name == "" {
			t.Error("Name should be set even for non-git directory")
		}
	})
}

func TestIsInitialized(t *testing.T) {
	t.Run("not initialized - no git repo", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "no-gren-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(dir)

		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)
		os.Chdir(dir)

		// Not a git repo, so isInitialized should return false
		if isInitialized() {
			t.Error("isInitialized() = true, want false")
		}
	})

	t.Run("not initialized - git repo without .gren", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "git-no-gren-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(dir)

		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)
		os.Chdir(dir)

		// Initialize git repo
		exec.Command("git", "init").Run()

		if isInitialized() {
			t.Error("isInitialized() = true, want false")
		}
	})

	t.Run("initialized with .gren", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "with-gren-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(dir)

		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)
		os.Chdir(dir)

		// Initialize git repo and create .gren directory
		exec.Command("git", "init").Run()
		os.Mkdir(".gren", 0755)

		if !isInitialized() {
			t.Error("isInitialized() = false, want true")
		}
	})

	t.Run("initialized - check from subdirectory", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "gren-subdir-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(dir)

		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)

		// Initialize git repo and create .gren directory at repo root
		os.Chdir(dir)
		exec.Command("git", "init").Run()
		os.Mkdir(".gren", 0755)

		// Create and change to subdirectory
		subdir := filepath.Join(dir, "subdir")
		os.Mkdir(subdir, 0755)
		os.Chdir(subdir)

		// Should still detect initialization from subdirectory
		if !isInitialized() {
			t.Error("isInitialized() = false from subdirectory, want true")
		}
	})
}
