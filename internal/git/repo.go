package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// RepoInfo contains information about the git repository.
type RepoInfo struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	IsGitRepo     bool   `json:"is_git_repo"`
	IsInitialized bool   `json:"is_initialized"`
	CurrentBranch string `json:"current_branch"`
}

// Repository defines the interface for git repository operations.
type Repository interface {
	GetRepoInfo(ctx context.Context) (*RepoInfo, error)
	IsGitRepo(ctx context.Context) (bool, error)
	GetRepoName(ctx context.Context) (string, error)
	GetCurrentBranch(ctx context.Context) (string, error)
	GetBranchStatuses(ctx context.Context) ([]BranchStatus, error)
	GetRecommendedBaseBranch(ctx context.Context) (string, error)
}

// LocalRepository implements Repository for local git repositories.
type LocalRepository struct {
	timeout time.Duration
}

// NewLocalRepository creates a new LocalRepository with default timeout.
func NewLocalRepository() *LocalRepository {
	return &LocalRepository{
		timeout: 5 * time.Second,
	}
}

// GetRepoInfo returns comprehensive repository information.
func (r *LocalRepository) GetRepoInfo(ctx context.Context) (*RepoInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	info := &RepoInfo{}

	// Check if we're in a git repository
	isGit, err := r.IsGitRepo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to check if directory is git repo: %w", err)
	}
	info.IsGitRepo = isGit

	if info.IsGitRepo {
		if info.Name, err = r.GetRepoName(ctx); err != nil {
			return nil, fmt.Errorf("failed to get repo name: %w", err)
		}

		if info.Path, err = r.getRepoPath(ctx); err != nil {
			return nil, fmt.Errorf("failed to get repo path: %w", err)
		}

		if info.CurrentBranch, err = r.GetCurrentBranch(ctx); err != nil {
			// Don't fail completely if we can't get branch name
			info.CurrentBranch = ""
		}
	} else {
		// Fallback to current directory name
		name, err := getCurrentDirectory()
		if err != nil {
			return nil, fmt.Errorf("failed to get current directory: %w", err)
		}
		info.Name = name

		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
		info.Path = wd
	}

	// Check if gren has been initialized
	info.IsInitialized = isInitialized()

	return info, nil
}

// IsGitRepo checks if current directory is a git repository.
func (r *LocalRepository) IsGitRepo(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--git-dir")
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return false, fmt.Errorf("git command timed out")
	}
	return err == nil, nil
}

// GetRepoName returns the name of the current git repository.
func (r *LocalRepository) GetRepoName(ctx context.Context) (string, error) {
	repoPath, err := r.getRepoPath(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get repo path: %w", err)
	}

	name := filepath.Base(repoPath)
	if name == "" || name == "." || name == "/" {
		return "", fmt.Errorf("invalid repository path: %s", repoPath)
	}

	return name, nil
}

// getRepoPath returns the full path to the git repository root.
func (r *LocalRepository) getRepoPath(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("git command timed out")
		}
		return "", fmt.Errorf("failed to get repository root: %w", err)
	}

	path := strings.TrimSpace(string(output))
	if path == "" {
		return "", fmt.Errorf("empty repository path")
	}

	return path, nil
}

// GetCurrentBranch returns the name of the current git branch.
func (r *LocalRepository) GetCurrentBranch(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "branch", "--show-current")
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("git command timed out")
		}
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}

	branch := strings.TrimSpace(string(output))
	// Empty branch name is valid (detached HEAD)
	return branch, nil
}

// getCurrentDirectory returns the current working directory name.
func getCurrentDirectory() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	name := filepath.Base(wd)
	if name == "" || name == "." {
		return "", fmt.Errorf("invalid working directory: %s", wd)
	}

	return name, nil
}

// isInitialized checks if gren has been initialized in this repo.
func isInitialized() bool {
	_, err := os.Stat(".gren")
	return err == nil
}