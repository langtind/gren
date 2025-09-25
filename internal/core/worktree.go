package core

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/langtind/gren/internal/config"
	"github.com/langtind/gren/internal/git"
)

// WorktreeManager handles worktree operations
type WorktreeManager struct {
	gitRepo       git.Repository
	configManager *config.Manager
}

// NewWorktreeManager creates a new WorktreeManager
func NewWorktreeManager(gitRepo git.Repository, configManager *config.Manager) *WorktreeManager {
	return &WorktreeManager{
		gitRepo:       gitRepo,
		configManager: configManager,
	}
}

// CreateWorktreeRequest contains parameters for creating a worktree
type CreateWorktreeRequest struct {
	Name         string // Worktree name/directory
	Branch       string // Branch name (empty to create new from base)
	BaseBranch   string // Base branch to create from (if creating new branch)
	IsNewBranch  bool   // Whether to create a new branch
	WorktreeDir  string // Base directory for worktrees
}

// WorktreeInfo represents basic worktree information
type WorktreeInfo struct {
	Name      string
	Path      string
	Branch    string
	IsCurrent bool
	Status    string
}

// CreateWorktree creates a new worktree with the given parameters
func (wm *WorktreeManager) CreateWorktree(ctx context.Context, req CreateWorktreeRequest) error {
	// Load configuration
	cfg, err := wm.configManager.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Determine worktree path
	worktreeDir := req.WorktreeDir
	if worktreeDir == "" {
		if cfg.WorktreeDir != "" {
			worktreeDir = cfg.WorktreeDir
		} else {
			// Default to ../repo-worktrees
			repoInfo, err := wm.gitRepo.GetRepoInfo(ctx)
			if err != nil {
				return fmt.Errorf("failed to get repo info: %w", err)
			}
			worktreeDir = fmt.Sprintf("../%s-worktrees", repoInfo.Name)
		}
	}

	worktreePath := filepath.Join(worktreeDir, req.Name)

	// Create worktree directory if it doesn't exist
	if err := os.MkdirAll(worktreeDir, 0755); err != nil {
		return fmt.Errorf("failed to create worktree directory: %w", err)
	}

	// Create the git worktree
	var cmd *exec.Cmd
	if req.IsNewBranch {
		branchName := req.Branch
		if branchName == "" {
			branchName = req.Name
		}
		baseBranch := req.BaseBranch
		if baseBranch == "" {
			// Get recommended base branch
			baseBranch, err = wm.gitRepo.GetRecommendedBaseBranch(ctx)
			if err != nil {
				return fmt.Errorf("failed to get recommended base branch: %w", err)
			}
		}
		cmd = exec.Command("git", "worktree", "add", "-b", branchName, worktreePath, baseBranch)
	} else {
		// Validate existing branch
		validateCmd := exec.Command("git", "rev-parse", "--verify", req.Branch)
		if err := validateCmd.Run(); err != nil {
			return fmt.Errorf("branch '%s' is not a valid git reference", req.Branch)
		}
		cmd = exec.Command("git", "worktree", "add", worktreePath, req.Branch)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		errorMsg := fmt.Sprintf("git worktree add failed: %s", string(output))
		if len(output) == 0 {
			errorMsg = fmt.Sprintf("git worktree add failed with exit code: %v", err)
		}
		return fmt.Errorf(errorMsg)
	}

	// Copy .gren/ configuration to worktree if it exists
	if _, err := os.Stat(".gren"); err == nil {
		destGrenDir := filepath.Join(worktreePath, ".gren")
		if err := copyDir(".gren", destGrenDir); err != nil {
			// Log but don't fail for this
			fmt.Printf("Warning: failed to copy .gren configuration: %v\n", err)
		}
	}

	// Run post-create hook if it exists and is configured
	if cfg.PostCreateHook != "" {
		branchName := req.Branch
		if req.IsNewBranch && branchName == "" {
			branchName = req.Name
		}
		baseBranch := req.BaseBranch

		// Get repo root
		repoRoot, err := wm.getRepoRoot()
		if err != nil {
			return fmt.Errorf("failed to get repo root: %w", err)
		}

		hookCmd := exec.Command(cfg.PostCreateHook, worktreePath, branchName, baseBranch, repoRoot)
		hookCmd.Dir = worktreePath
		if err := hookCmd.Run(); err != nil {
			fmt.Printf("Warning: post-create hook failed: %v\n", err)
		}
	}

	fmt.Printf("Created worktree '%s' at %s\n", req.Name, worktreePath)
	return nil
}

// ListWorktrees returns a list of all worktrees
func (wm *WorktreeManager) ListWorktrees(ctx context.Context) ([]WorktreeInfo, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	return wm.parseWorktreeList(string(output)), nil
}

// DeleteWorktree deletes a worktree by name or path
func (wm *WorktreeManager) DeleteWorktree(ctx context.Context, identifier string) error {
	worktrees, err := wm.ListWorktrees(ctx)
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	var targetWorktree *WorktreeInfo
	for _, wt := range worktrees {
		if wt.Name == identifier || wt.Path == identifier {
			targetWorktree = &wt
			break
		}
	}

	if targetWorktree == nil {
		return fmt.Errorf("worktree '%s' not found", identifier)
	}

	if targetWorktree.IsCurrent {
		return fmt.Errorf("cannot delete current worktree")
	}

	// Remove git worktree
	cmd := exec.Command("git", "worktree", "remove", targetWorktree.Path, "--force")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove worktree %s: %w", targetWorktree.Name, err)
	}

	// Delete local branch if it exists
	cmd = exec.Command("git", "branch", "-D", targetWorktree.Branch)
	cmd.Run() // Ignore errors for branch deletion

	fmt.Printf("Deleted worktree '%s'\n", targetWorktree.Name)
	return nil
}

// Helper functions

func (wm *WorktreeManager) parseWorktreeList(output string) []WorktreeInfo {
	var worktrees []WorktreeInfo
	lines := strings.Split(strings.TrimSpace(output), "\n")

	var current WorktreeInfo
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if current.Path != "" {
				worktrees = append(worktrees, current)
				current = WorktreeInfo{}
			}
			continue
		}

		if strings.HasPrefix(line, "worktree ") {
			current.Path = strings.TrimPrefix(line, "worktree ")
			current.Name = filepath.Base(current.Path)
		} else if strings.HasPrefix(line, "branch ") {
			current.Branch = strings.TrimPrefix(line, "branch ")
		} else if line == "bare" {
			current.Branch = "(bare)"
		} else if line == "detached" {
			current.Branch = "(detached)"
		}
	}

	// Add the last worktree if there's one pending
	if current.Path != "" {
		worktrees = append(worktrees, current)
	}

	// Mark current worktree
	currentPath, _ := os.Getwd()
	for i := range worktrees {
		if worktrees[i].Path == currentPath {
			worktrees[i].IsCurrent = true
		}
	}

	return worktrees
}

func (wm *WorktreeManager) getRepoRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get repository root: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// copyDir copies a directory recursively
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(dstPath, data, info.Mode())
	})
}