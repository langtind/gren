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
	"github.com/langtind/gren/internal/logging"
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
	Name        string // Worktree name/directory
	Branch      string // Branch name (empty to create new from base)
	BaseBranch  string // Base branch to create from (if creating new branch)
	IsNewBranch bool   // Whether to create a new branch
	WorktreeDir string // Base directory for worktrees
}

// WorktreeInfo represents basic worktree information
type WorktreeInfo struct {
	Name           string
	Path           string
	Branch         string
	IsCurrent      bool
	IsMain         bool   // True if this is the main worktree (where .git directory lives)
	Status         string // "clean", "modified", "untracked", "mixed", "unpushed", "missing"
	LastCommit     string // Relative time of last commit (e.g., "2 hours ago")
	StagedCount    int    // Number of staged files (ready to commit)
	ModifiedCount  int    // Number of modified files (not staged)
	UntrackedCount int    // Number of untracked files
	UnpushedCount  int    // Number of unpushed commits
}

// CheckPrerequisites verifies that required tools are available
func (wm *WorktreeManager) CheckPrerequisites() error {
	var missing []string

	// git is required
	if _, err := exec.LookPath("git"); err != nil {
		missing = append(missing, "git")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required tools: %s\n\nPlease install these tools and try again", strings.Join(missing, ", "))
	}

	return nil
}

// CreateWorktree creates a new worktree with the given parameters
func (wm *WorktreeManager) CreateWorktree(ctx context.Context, req CreateWorktreeRequest) error {
	logging.Info("CreateWorktree called: name=%s, branch=%s, base=%s, isNew=%v", req.Name, req.Branch, req.BaseBranch, req.IsNewBranch)

	// Check prerequisites
	if err := wm.CheckPrerequisites(); err != nil {
		logging.Error("Prerequisites check failed: %v", err)
		return err
	}

	// Load configuration
	cfg, err := wm.configManager.Load()
	if err != nil {
		logging.Error("Failed to load configuration: %v", err)
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Determine worktree path
	worktreeDir := req.WorktreeDir
	if worktreeDir == "" {
		if cfg.WorktreeDir != "" {
			worktreeDir = cfg.WorktreeDir
			logging.Debug("Using worktree_dir from config: %s", worktreeDir)
		} else {
			// Default to ../repo-worktrees
			repoInfo, err := wm.gitRepo.GetRepoInfo(ctx)
			if err != nil {
				logging.Error("Failed to get repo info: %v", err)
				return fmt.Errorf("failed to get repo info: %w", err)
			}
			worktreeDir = fmt.Sprintf("../%s-worktrees", repoInfo.Name)
			logging.Debug("Using default worktree_dir: %s", worktreeDir)
		}
	}

	// Sanitize worktree name: replace / with - to avoid nested directories
	worktreeName := strings.ReplaceAll(req.Name, "/", "-")
	worktreePath := filepath.Join(worktreeDir, worktreeName)
	logging.Debug("Worktree path: %s", worktreePath)

	// Create worktree directory if it doesn't exist
	if _, err := os.Stat(worktreeDir); os.IsNotExist(err) {
		logging.Debug("Creating worktree directory: %s", worktreeDir)
		if err := os.MkdirAll(worktreeDir, 0755); err != nil {
			logging.Error("Failed to create worktree directory: %v", err)
			return fmt.Errorf("failed to create worktree directory: %w", err)
		}
	}

	// Create the git worktree with smart branch detection
	var cmd *exec.Cmd
	branchName := req.Branch
	if branchName == "" {
		branchName = req.Name
	}

	// Check if branch exists locally
	localCheckCmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branchName)
	branchExistsLocally := localCheckCmd.Run() == nil

	// Check if branch exists on remote
	remoteCheckCmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/remotes/origin/"+branchName)
	branchExistsRemote := remoteCheckCmd.Run() == nil

	logging.Debug("Branch detection: local=%v, remote=%v", branchExistsLocally, branchExistsRemote)

	// Check if branch is already checked out in another worktree
	if branchExistsLocally {
		worktreeListCmd := exec.Command("git", "worktree", "list")
		listOutput, _ := worktreeListCmd.Output()
		if strings.Contains(string(listOutput), "["+branchName+"]") {
			logging.Error("Branch already checked out in another worktree: %s", branchName)
			return fmt.Errorf("branch '%s' is already checked out in another worktree", branchName)
		}
	}

	var gitCmd string
	if branchExistsLocally {
		// Branch exists locally - use it directly
		gitCmd = fmt.Sprintf("git worktree add %s %s", worktreePath, branchName)
		logging.Info("Using existing local branch: %s", branchName)
		cmd = exec.Command("git", "worktree", "add", worktreePath, branchName)
	} else if branchExistsRemote {
		// Branch exists on remote - create tracking branch
		gitCmd = fmt.Sprintf("git worktree add --track -b %s %s origin/%s", branchName, worktreePath, branchName)
		logging.Info("Creating local branch from remote: origin/%s", branchName)
		cmd = exec.Command("git", "worktree", "add", "--track", "-b", branchName, worktreePath, "origin/"+branchName)
	} else if req.IsNewBranch {
		// Branch doesn't exist - create new from base
		baseBranch := req.BaseBranch
		if baseBranch == "" {
			// Get recommended base branch
			baseBranch, err = wm.gitRepo.GetRecommendedBaseBranch(ctx)
			if err != nil {
				logging.Error("Failed to get recommended base branch: %v", err)
				return fmt.Errorf("failed to get recommended base branch: %w", err)
			}
		}
		gitCmd = fmt.Sprintf("git worktree add -b %s %s %s", branchName, worktreePath, baseBranch)
		logging.Info("Creating new branch '%s' from base '%s'", branchName, baseBranch)
		cmd = exec.Command("git", "worktree", "add", "-b", branchName, worktreePath, baseBranch)
	} else {
		// User explicitly wanted existing branch but it doesn't exist
		logging.Error("Branch not found locally or on remote: %s", branchName)
		return fmt.Errorf("branch '%s' not found locally or on remote", branchName)
	}

	logging.Debug("Running: %s", gitCmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logging.Error("git worktree add failed: %v, output: %s", err, string(output))
		if len(output) == 0 {
			return fmt.Errorf("git worktree add failed: %w", err)
		}
		return fmt.Errorf("git worktree add failed: %s", string(output))
	}

	// Initialize submodules in the new worktree
	if _, err := os.Stat(".gitmodules"); err == nil {
		submoduleCmd := exec.Command("git", "-C", worktreePath, "submodule", "update", "--init", "--recursive")
		if err := submoduleCmd.Run(); err != nil {
			logging.Warn("Failed to initialize submodules: %v", err)
		}
	}

	// Symlink .gren/ configuration to worktree if it exists
	repoRoot, err := wm.getRepoRoot()
	if err == nil {
		srcGrenDir := filepath.Join(repoRoot, ".gren")
		if _, err := os.Stat(srcGrenDir); err == nil {
			destGrenDir := filepath.Join(worktreePath, ".gren")
			// Create symlink (relative path for portability)
			relPath, err := filepath.Rel(worktreePath, srcGrenDir)
			if err == nil {
				if err := os.Symlink(relPath, destGrenDir); err != nil {
					// Log but don't fail for this
					logging.Warn("Failed to symlink .gren configuration: %v", err)
				}
			}
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

		// Resolve hook path relative to repo root (not worktree - the hook creates the .gren symlink)
		hookPath := cfg.PostCreateHook
		fullHookPath := filepath.Join(repoRoot, hookPath)
		logging.Debug("Post-create hook path: %s (full: %s)", hookPath, fullHookPath)

		// Check if hook exists
		if _, err := os.Stat(fullHookPath); err != nil {
			logging.Error("Post-create hook not found: %s", fullHookPath)
			logging.Warn("Post-create hook not found: %s", fullHookPath)
		} else {
			logging.Info("Running post-create hook: %s", fullHookPath)
			// Convert worktreePath to absolute for the hook
			absWorktreePath := worktreePath
			if !filepath.IsAbs(worktreePath) {
				absWorktreePath = filepath.Join(repoRoot, worktreePath)
			}
			hookCmd := exec.Command(fullHookPath, absWorktreePath, branchName, baseBranch, repoRoot)
			hookCmd.Dir = absWorktreePath
			// Capture output instead of printing to stdout/stderr
			// This prevents TUI corruption when running in interactive mode
			hookOutput, hookErr := hookCmd.CombinedOutput()
			if hookErr != nil {
				logging.Error("Post-create hook failed: %v, output: %s", hookErr, string(hookOutput))
			} else {
				logging.Info("Post-create hook completed successfully")
				logging.Debug("Post-create hook output: %s", string(hookOutput))
			}
		}
	}

	logging.Info("Created worktree '%s' at %s", req.Name, worktreePath)
	return nil
}

// ListWorktrees returns a list of all worktrees with full status information
func (wm *WorktreeManager) ListWorktrees(ctx context.Context) ([]WorktreeInfo, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	output, err := cmd.CombinedOutput()
	if err != nil {
		logging.Error("git worktree list failed: %v, output: %s", err, string(output))
		return nil, fmt.Errorf("failed to list worktrees: %w (git output: %s)", err, string(output))
	}

	worktrees := wm.parseWorktreeList(string(output))

	// Mark main worktree using config if available, otherwise detect by .git directory
	cfg, _ := wm.configManager.Load()
	mainWorktreePath := ""
	if cfg != nil && cfg.MainWorktree != "" {
		mainWorktreePath = cfg.MainWorktree
	}

	for i := range worktrees {
		// Check if this is the main worktree
		if mainWorktreePath != "" && worktrees[i].Path == mainWorktreePath {
			worktrees[i].IsMain = true
		} else if mainWorktreePath == "" {
			// Fallback: detect main worktree by checking if .git is a directory (not a file)
			gitPath := filepath.Join(worktrees[i].Path, ".git")
			if info, err := os.Stat(gitPath); err == nil && info.IsDir() {
				worktrees[i].IsMain = true
			}
		}
	}

	// Enrich worktrees with status information
	for i := range worktrees {
		wm.enrichWorktreeStatus(&worktrees[i])
	}

	return worktrees, nil
}

// enrichWorktreeStatus adds detailed status information to a worktree
func (wm *WorktreeManager) enrichWorktreeStatus(wt *WorktreeInfo) {
	// Skip if worktree is missing
	if wt.Status == "missing" {
		return
	}

	// Get file counts
	wt.StagedCount, wt.ModifiedCount, wt.UntrackedCount = getFileCounts(wt.Path, wt.IsCurrent)

	// Get unpushed count
	wt.UnpushedCount = getUnpushedCount(wt.Path, wt.IsCurrent)

	// Determine status based on counts
	hasModified := wt.StagedCount > 0 || wt.ModifiedCount > 0
	hasUntracked := wt.UntrackedCount > 0

	if hasModified && hasUntracked {
		wt.Status = "mixed"
	} else if hasModified {
		wt.Status = "modified"
	} else if hasUntracked {
		wt.Status = "untracked"
	} else if wt.UnpushedCount > 0 || isNotPushedToRemote(wt.Path, wt.IsCurrent) {
		wt.Status = "unpushed"
	} else {
		wt.Status = "clean"
	}

	// Get last commit time
	wt.LastCommit = getLastCommitTime(wt.Path)
}

// getFileCounts returns staged, modified, and untracked file counts
func getFileCounts(worktreePath string, isCurrent bool) (staged, modified, untracked int) {
	var cmd *exec.Cmd
	if isCurrent {
		cmd = exec.Command("git", "status", "--porcelain")
	} else {
		cmd = exec.Command("git", "-C", worktreePath, "status", "--porcelain")
	}

	output, err := cmd.Output()
	if err != nil {
		return 0, 0, 0
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if len(line) < 2 {
			continue
		}
		indexStatus := line[0] // Staging area (index) status
		workStatus := line[1]  // Working tree status

		// Untracked files
		if indexStatus == '?' && workStatus == '?' {
			untracked++
			continue
		}

		// Staged changes (first column has M, A, D, R, C)
		if indexStatus != ' ' && indexStatus != '?' {
			staged++
		}

		// Unstaged changes (second column has M, D)
		if workStatus != ' ' && workStatus != '?' {
			modified++
		}
	}

	return staged, modified, untracked
}

// getUnpushedCount returns the number of unpushed commits
func getUnpushedCount(worktreePath string, isCurrent bool) int {
	var cmd *exec.Cmd
	if isCurrent {
		cmd = exec.Command("git", "log", "@{u}..HEAD", "--oneline")
	} else {
		cmd = exec.Command("git", "-C", worktreePath, "log", "@{u}..HEAD", "--oneline")
	}

	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) > 0 && lines[0] != "" {
		return len(lines)
	}
	return 0
}

// isNotPushedToRemote checks if branch doesn't exist on remote
func isNotPushedToRemote(worktreePath string, isCurrent bool) bool {
	// Get current branch
	var branchCmd *exec.Cmd
	if isCurrent {
		branchCmd = exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	} else {
		branchCmd = exec.Command("git", "-C", worktreePath, "rev-parse", "--abbrev-ref", "HEAD")
	}
	branchOutput, err := branchCmd.Output()
	if err != nil {
		return false
	}
	branch := strings.TrimSpace(string(branchOutput))

	if branch == "" || branch == "HEAD" {
		return false
	}

	// Check if branch exists on remote
	var remoteCheckCmd *exec.Cmd
	if isCurrent {
		remoteCheckCmd = exec.Command("git", "rev-parse", "--verify", "origin/"+branch)
	} else {
		remoteCheckCmd = exec.Command("git", "-C", worktreePath, "rev-parse", "--verify", "origin/"+branch)
	}
	return remoteCheckCmd.Run() != nil
}

// getLastCommitTime returns a human-readable relative time for the last commit
func getLastCommitTime(worktreePath string) string {
	cmd := exec.Command("git", "-C", worktreePath, "log", "-1", "--format=%cr")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	result := strings.TrimSpace(string(output))

	// Shorten common phrases for compact display
	replacements := map[string]string{
		" seconds ago": "s ago",
		" second ago":  "s ago",
		" minutes ago": "m ago",
		" minute ago":  "m ago",
		" hours ago":   "h ago",
		" hour ago":    "h ago",
		" days ago":    "d ago",
		" day ago":     "d ago",
		" weeks ago":   "w ago",
		" week ago":    "w ago",
		" months ago":  "mo ago",
		" month ago":   "mo ago",
		" years ago":   "y ago",
		" year ago":    "y ago",
	}
	for old, new := range replacements {
		result = strings.ReplaceAll(result, old, new)
	}

	return result
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

	// 1. Deinit submodules and track if worktree has submodules
	hasSubmodules := false
	if _, err := os.Stat(filepath.Join(targetWorktree.Path, ".gitmodules")); err == nil {
		hasSubmodules = true
		deinitCmd := exec.Command("git", "-C", targetWorktree.Path, "submodule", "deinit", "--all", "--force")
		output, err := deinitCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to deinit submodules in worktree '%s': %w\n\nThis can happen if submodules have uncommitted changes.\nTry running manually:\n  cd %s\n  git submodule deinit --all --force\n\nOutput: %s",
				targetWorktree.Name, err, targetWorktree.Path, string(output))
		}
	}

	// 2. Remove .gren symlink if it exists (created by gren during worktree creation)
	// This prevents "untracked files" errors during removal
	grenSymlink := filepath.Join(targetWorktree.Path, ".gren")
	if info, err := os.Lstat(grenSymlink); err == nil && info.Mode()&os.ModeSymlink != 0 {
		os.Remove(grenSymlink)
	}

	// 3. Remove worktree using git
	// Note: --force is required for worktrees with submodules (even after deinit)
	var cmd *exec.Cmd
	if hasSubmodules {
		cmd = exec.Command("git", "worktree", "remove", "--force", targetWorktree.Path)
	} else {
		cmd = exec.Command("git", "worktree", "remove", targetWorktree.Path)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to remove worktree '%s': %w\n\nOutput: %s\n\nIf the worktree has uncommitted changes, commit or stash them first.",
			targetWorktree.Name, err, string(output))
	}

	// Note: Branch is kept - user can delete manually if needed
	logging.Info("Deleted worktree '%s' (branch '%s' is preserved)", targetWorktree.Name, targetWorktree.Branch)
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
			branch := strings.TrimPrefix(line, "branch ")
			// Strip refs/heads/ prefix for cleaner display
			current.Branch = strings.TrimPrefix(branch, "refs/heads/")
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
