package git

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// BranchStatus represents the status of a git branch.
type BranchStatus struct {
	Name             string `json:"name"`
	IsClean          bool   `json:"is_clean"`
	UncommittedFiles int    `json:"uncommitted_files"`
	UntrackedFiles   int    `json:"untracked_files"`
	AheadCount       int    `json:"ahead_count"`
	BehindCount      int    `json:"behind_count"`
	IsCurrent        bool   `json:"is_current"`
}

// GetBranchStatuses returns the status of all local branches.
func (r *LocalRepository) GetBranchStatuses(ctx context.Context) ([]BranchStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	// Get all local branches
	branches, err := r.getLocalBranches(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get local branches: %w", err)
	}

	// Get current branch
	currentBranch, err := r.GetCurrentBranch(ctx)
	if err != nil {
		// Continue even if we can't get current branch
		currentBranch = ""
	}

	var statuses []BranchStatus
	for _, branch := range branches {
		status, err := r.getBranchStatus(ctx, branch, branch == currentBranch)
		if err != nil {
			// Log error but continue with other branches
			continue
		}
		statuses = append(statuses, status)
	}

	return statuses, nil
}

// getLocalBranches returns all local branch names.
func (r *LocalRepository) getLocalBranches(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "branch", "--format=%(refname:short)")
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("git command timed out")
		}
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var branches []string
	for _, line := range lines {
		branch := strings.TrimSpace(line)
		if branch != "" {
			branches = append(branches, branch)
		}
	}

	return branches, nil
}

// getBranchStatus returns the detailed status of a specific branch.
func (r *LocalRepository) getBranchStatus(ctx context.Context, branch string, isCurrent bool) (BranchStatus, error) {
	status := BranchStatus{
		Name:      branch,
		IsCurrent: isCurrent,
	}

	if isCurrent {
		// For current branch, check working directory status
		uncommitted, untracked, err := r.getWorkingDirectoryStatus(ctx)
		if err != nil {
			return status, fmt.Errorf("failed to get working directory status: %w", err)
		}

		status.UncommittedFiles = uncommitted
		status.UntrackedFiles = untracked
		status.IsClean = uncommitted == 0 && untracked == 0
	} else {
		// For other branches, assume they're clean (we can't easily check without checkout)
		status.IsClean = true
	}

	// Get ahead/behind count compared to upstream
	ahead, behind, err := r.getAheadBehindCount(ctx, branch)
	if err != nil {
		// Continue without ahead/behind info if it fails
		ahead, behind = 0, 0
	}

	status.AheadCount = ahead
	status.BehindCount = behind

	return status, nil
}

// getWorkingDirectoryStatus returns the number of uncommitted and untracked files.
func (r *LocalRepository) getWorkingDirectoryStatus(ctx context.Context) (uncommitted, untracked int, err error) {
	// Get git status in porcelain format
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return 0, 0, fmt.Errorf("git command timed out")
		}
		return 0, 0, fmt.Errorf("failed to get git status: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if len(line) < 2 {
			continue
		}

		indexStatus := line[0]
		workTreeStatus := line[1]

		// Check if file is untracked
		if indexStatus == '?' && workTreeStatus == '?' {
			untracked++
		} else if indexStatus != ' ' || workTreeStatus != ' ' {
			// File has changes in index or working tree
			uncommitted++
		}
	}

	return uncommitted, untracked, nil
}

// getAheadBehindCount returns how many commits the branch is ahead/behind its upstream.
func (r *LocalRepository) getAheadBehindCount(ctx context.Context, branch string) (ahead, behind int, err error) {
	// Try to get the upstream branch
	cmd := exec.CommandContext(ctx, "git", "rev-list", "--left-right", "--count", branch+"...origin/"+branch)
	output, err := cmd.Output()
	if err != nil {
		// No upstream or other error, return 0,0
		return 0, 0, nil
	}

	parts := strings.Fields(strings.TrimSpace(string(output)))
	if len(parts) != 2 {
		return 0, 0, nil
	}

	ahead, err = strconv.Atoi(parts[0])
	if err != nil {
		ahead = 0
	}

	behind, err = strconv.Atoi(parts[1])
	if err != nil {
		behind = 0
	}

	return ahead, behind, nil
}

// GetRecommendedBaseBranch returns the best branch to use as base for new worktrees.
func (r *LocalRepository) GetRecommendedBaseBranch(ctx context.Context) (string, error) {
	statuses, err := r.GetBranchStatuses(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get branch statuses: %w", err)
	}

	// Priority order for base branch selection:
	// 1. main/master if clean
	// 2. current branch if clean
	// 3. first clean branch alphabetically
	// 4. current branch with warning if all are dirty

	var currentBranch string
	var cleanBranches []string
	var allBranches []string

	for _, status := range statuses {
		allBranches = append(allBranches, status.Name)

		if status.IsCurrent {
			currentBranch = status.Name
		}

		if status.IsClean {
			cleanBranches = append(cleanBranches, status.Name)
		}
	}

	// 1. Prefer main/master if clean
	for _, branch := range cleanBranches {
		if branch == "main" || branch == "master" {
			return branch, nil
		}
	}

	// 2. Prefer current branch if clean
	if currentBranch != "" {
		for _, branch := range cleanBranches {
			if branch == currentBranch {
				return branch, nil
			}
		}
	}

	// 3. First clean branch alphabetically
	if len(cleanBranches) > 0 {
		return cleanBranches[0], nil
	}

	// 4. Current branch as fallback (even if dirty)
	if currentBranch != "" {
		return currentBranch, nil
	}

	// 5. Any branch as last resort
	if len(allBranches) > 0 {
		return allBranches[0], nil
	}

	return "", fmt.Errorf("no branches found")
}