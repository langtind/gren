package core

import (
	"context"
	"encoding/json"
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

	// Stale detection fields
	BranchStatus string // "active", "stale", or "" if not yet checked
	StaleReason  string // "merged_locally", "no_unique_commits", "remote_gone", "pr_merged", "pr_closed"

	// GitHub PR fields (populated async, empty if gh unavailable or no PR)
	PRNumber int    // PR number, 0 if no PR
	PRState  string // "OPEN", "MERGED", "CLOSED", "DRAFT", "" if unknown
	PRURL    string // Full URL to PR for "Open in browser"
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
// Returns a warning message (if any) and an error
func (wm *WorktreeManager) CreateWorktree(ctx context.Context, req CreateWorktreeRequest) (warning string, err error) {
	logging.Info("CreateWorktree called: name=%s, branch=%s, base=%s, isNew=%v", req.Name, req.Branch, req.BaseBranch, req.IsNewBranch)

	// Check prerequisites
	if err := wm.CheckPrerequisites(); err != nil {
		logging.Error("Prerequisites check failed: %v", err)
		return "", err
	}

	// Fetch latest from origin to ensure we have up-to-date remote refs
	wm.FetchOrigin()

	// Load configuration
	cfg, err := wm.configManager.Load()
	if err != nil {
		logging.Error("Failed to load configuration: %v", err)
		return "", fmt.Errorf("failed to load configuration: %w", err)
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
				return "", fmt.Errorf("failed to get repo info: %w", err)
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
			return "", fmt.Errorf("failed to create worktree directory: %w", err)
		}
	}

	// Create the git worktree with smart branch detection
	var cmd *exec.Cmd
	branchName := req.Branch
	if branchName == "" {
		branchName = req.Name
	}

	// Get sync status for the branch (uses fresh data from fetch)
	syncStatus := wm.GetBranchSyncStatus(branchName)
	warning = syncStatus.Warning

	// Check if branch is already checked out in another worktree
	if syncStatus.LocalExists {
		worktreeListCmd := exec.Command("git", "worktree", "list")
		listOutput, _ := worktreeListCmd.Output()
		if strings.Contains(string(listOutput), "["+branchName+"]") {
			logging.Error("Branch already checked out in another worktree: %s", branchName)
			return "", fmt.Errorf("branch '%s' is already checked out in another worktree", branchName)
		}
	}

	var gitCmd string
	if syncStatus.LocalExists || syncStatus.RemoteExists {
		// Branch exists - use the best source ref (local if ahead, remote otherwise)
		sourceRef := syncStatus.SourceRef

		if syncStatus.LocalExists && !syncStatus.RemoteExists {
			// Local-only branch - use directly
			gitCmd = fmt.Sprintf("git worktree add %s %s", worktreePath, branchName)
			logging.Info("Using local-only branch: %s", branchName)
			cmd = exec.Command("git", "worktree", "add", worktreePath, branchName)
		} else if !syncStatus.LocalExists && syncStatus.RemoteExists {
			// Remote-only branch - create tracking branch
			gitCmd = fmt.Sprintf("git worktree add --track -b %s %s %s", branchName, worktreePath, sourceRef)
			logging.Info("Creating local branch from remote: %s", sourceRef)
			cmd = exec.Command("git", "worktree", "add", "--track", "-b", branchName, worktreePath, sourceRef)
		} else if syncStatus.Ahead > 0 {
			// Local has unpushed commits - use local branch
			gitCmd = fmt.Sprintf("git worktree add %s %s", worktreePath, branchName)
			logging.Info("Using local branch (has %d unpushed commits): %s", syncStatus.Ahead, branchName)
			cmd = exec.Command("git", "worktree", "add", worktreePath, branchName)
		} else {
			// Local is in sync or behind - use remote for latest code
			gitCmd = fmt.Sprintf("git worktree add --track -b %s %s %s", branchName, worktreePath, sourceRef)
			logging.Info("Using remote branch for latest code: %s", sourceRef)
			cmd = exec.Command("git", "worktree", "add", "--track", "-b", branchName, worktreePath, sourceRef)
		}
	} else if req.IsNewBranch {
		// Branch doesn't exist - create new from base
		baseBranch := req.BaseBranch
		if baseBranch == "" {
			// Get recommended base branch
			baseBranch, err = wm.gitRepo.GetRecommendedBaseBranch(ctx)
			if err != nil {
				logging.Error("Failed to get recommended base branch: %v", err)
				return "", fmt.Errorf("failed to get recommended base branch: %w", err)
			}
		}

		// Check sync status of base branch to use latest
		baseStatus := wm.GetBranchSyncStatus(baseBranch)
		baseRef := baseStatus.SourceRef
		if baseRef == "" {
			baseRef = baseBranch // Fallback to branch name
		}
		if baseStatus.Warning != "" && warning == "" {
			warning = baseStatus.Warning
		}

		gitCmd = fmt.Sprintf("git worktree add -b %s %s %s", branchName, worktreePath, baseRef)
		logging.Info("Creating new branch '%s' from base '%s'", branchName, baseRef)
		cmd = exec.Command("git", "worktree", "add", "-b", branchName, worktreePath, baseRef)
	} else {
		// User explicitly wanted existing branch but it doesn't exist
		logging.Error("Branch not found locally or on remote: %s", branchName)
		return "", fmt.Errorf("branch '%s' not found locally or on remote", branchName)
	}

	logging.Debug("Running: %s", gitCmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logging.Error("git worktree add failed: %v, output: %s", err, string(output))
		if len(output) == 0 {
			return "", fmt.Errorf("git worktree add failed: %w", err)
		}
		return "", fmt.Errorf("git worktree add failed: %s", string(output))
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
			return "", fmt.Errorf("failed to get repo root: %w", err)
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
	return warning, nil
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

	// Detect main worktree dynamically by checking if .git is a directory (not a file)
	// In main worktree: .git is a directory
	// In linked worktrees: .git is a file containing "gitdir: /path/to/.git/worktrees/name"
	for i := range worktrees {
		gitPath := filepath.Join(worktrees[i].Path, ".git")
		if info, err := os.Stat(gitPath); err == nil && info.IsDir() {
			worktrees[i].IsMain = true
		}
	}

	// Enrich worktrees with status information
	for i := range worktrees {
		wm.enrichWorktreeStatus(&worktrees[i])
	}

	// Build stale cache once (runs git commands only once for all worktrees)
	cache := wm.buildStaleCache()

	// Enrich with stale status using cached data
	for i := range worktrees {
		wm.enrichStaleStatusCached(&worktrees[i], cache)
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

// BranchSyncStatus represents the sync status between local and remote branch
type BranchSyncStatus struct {
	LocalExists  bool
	RemoteExists bool
	Ahead        int    // Commits in local not in remote
	Behind       int    // Commits in remote not in local
	SourceRef    string // Best ref to use (origin/branch or branch)
	Warning      string // Warning message if local has unpushed commits
}

// GetBranchSyncStatus checks sync status between local and remote branch
// Should be called AFTER git fetch
func (wm *WorktreeManager) GetBranchSyncStatus(branch string) BranchSyncStatus {
	status := BranchSyncStatus{}

	// Check if branch exists locally
	localCheckCmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	status.LocalExists = localCheckCmd.Run() == nil

	// Check if branch exists on remote
	remoteCheckCmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/remotes/origin/"+branch)
	status.RemoteExists = remoteCheckCmd.Run() == nil

	logging.Debug("GetBranchSyncStatus: branch=%s, local=%v, remote=%v", branch, status.LocalExists, status.RemoteExists)

	// If only local exists, use local
	if status.LocalExists && !status.RemoteExists {
		status.SourceRef = branch
		logging.Debug("GetBranchSyncStatus: local-only branch, using %s", status.SourceRef)
		return status
	}

	// If only remote exists, use remote
	if !status.LocalExists && status.RemoteExists {
		status.SourceRef = "origin/" + branch
		logging.Debug("GetBranchSyncStatus: remote-only branch, using %s", status.SourceRef)
		return status
	}

	// If neither exists, return empty (caller will handle)
	if !status.LocalExists && !status.RemoteExists {
		logging.Debug("GetBranchSyncStatus: branch not found locally or remote")
		return status
	}

	// Both exist - check ahead/behind
	aheadCmd := exec.Command("git", "rev-list", "--count", "origin/"+branch+".."+branch)
	if output, err := aheadCmd.Output(); err == nil {
		fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &status.Ahead)
	}

	behindCmd := exec.Command("git", "rev-list", "--count", branch+"..origin/"+branch)
	if output, err := behindCmd.Output(); err == nil {
		fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &status.Behind)
	}

	logging.Debug("GetBranchSyncStatus: ahead=%d, behind=%d", status.Ahead, status.Behind)

	// Determine source ref based on sync status
	if status.Ahead > 0 {
		// Local has unpushed commits - use local to preserve them
		status.SourceRef = branch
		status.Warning = fmt.Sprintf("%s has %d unpushed commit(s) - using local version", branch, status.Ahead)
		logging.Info("GetBranchSyncStatus: %s", status.Warning)
	} else {
		// In sync or behind - safe to use remote (get latest)
		status.SourceRef = "origin/" + branch
		logging.Debug("GetBranchSyncStatus: using remote %s", status.SourceRef)
	}

	return status
}

// FetchOrigin runs git fetch origin to update remote tracking branches
func (wm *WorktreeManager) FetchOrigin() error {
	logging.Debug("FetchOrigin: running git fetch origin")
	cmd := exec.Command("git", "fetch", "origin")
	output, err := cmd.CombinedOutput()
	if err != nil {
		logging.Warn("FetchOrigin: git fetch origin failed: %v, output: %s", err, string(output))
		// Don't fail - might be offline or no remote configured
		return nil
	}
	logging.Debug("FetchOrigin: success")
	return nil
}

// staleCache holds pre-fetched data for stale detection to avoid repeated git calls
type staleCache struct {
	mergedBranches map[string]bool // branches merged into main/master
	goneBranches   map[string]bool // branches with deleted remote tracking
	baseBranch     string          // which base branch was found (main or master)
}

// buildStaleCache fetches stale-related git data once for all worktrees
func (wm *WorktreeManager) buildStaleCache() *staleCache {
	cache := &staleCache{
		mergedBranches: make(map[string]bool),
		goneBranches:   make(map[string]bool),
	}

	// Get merged branches (try main first, then master)
	for _, baseBranch := range []string{"main", "master"} {
		cmd := exec.Command("git", "branch", "--merged", baseBranch)
		output, err := cmd.Output()
		if err != nil {
			logging.Debug("buildStaleCache: git branch --merged %s failed: %v", baseBranch, err)
			continue
		}

		cache.baseBranch = baseBranch
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			line = strings.TrimPrefix(line, "* ")
			line = strings.TrimPrefix(line, "+ ")
			if line != "" && line != baseBranch {
				cache.mergedBranches[line] = true
			}
		}
		logging.Debug("buildStaleCache: found %d merged branches into %s", len(cache.mergedBranches), baseBranch)
		break // Found a valid base branch
	}

	// Get branches with gone remotes
	cmd := exec.Command("git", "branch", "-vv")
	output, err := cmd.Output()
	if err != nil {
		logging.Debug("buildStaleCache: git branch -vv failed: %v", err)
		return cache
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, ": gone]") {
			// Extract branch name from line
			trimmed := strings.TrimSpace(line)
			trimmed = strings.TrimPrefix(trimmed, "* ")
			trimmed = strings.TrimPrefix(trimmed, "+ ")
			// Branch name is the first word
			parts := strings.Fields(trimmed)
			if len(parts) > 0 {
				cache.goneBranches[parts[0]] = true
			}
		}
	}
	logging.Debug("buildStaleCache: found %d branches with gone remotes", len(cache.goneBranches))

	return cache
}

// enrichStaleStatusCached checks if a worktree's branch is stale using cached data
func (wm *WorktreeManager) enrichStaleStatusCached(wt *WorktreeInfo, cache *staleCache) {
	logging.Debug("enrichStaleStatusCached: checking branch %q", wt.Branch)

	// Skip main worktree and missing worktrees
	if wt.IsMain || wt.Status == "missing" {
		logging.Debug("enrichStaleStatusCached: skipping %q (isMain=%v, status=%s)", wt.Branch, wt.IsMain, wt.Status)
		wt.BranchStatus = "active"
		return
	}

	// Skip detached HEAD or bare repos
	if wt.Branch == "(detached)" || wt.Branch == "(bare)" {
		logging.Debug("enrichStaleStatusCached: skipping %q (detached/bare)", wt.Branch)
		wt.BranchStatus = "active"
		return
	}

	// Check 1: Is branch merged into main/master?
	if cache.mergedBranches[wt.Branch] {
		wt.BranchStatus = "stale"
		// Check if branch has unique commits (still need per-branch check for this)
		if wm.branchHasUniqueCommits(wt.Branch) {
			logging.Info("enrichStaleStatusCached: branch %q is merged into %s", wt.Branch, cache.baseBranch)
			wt.StaleReason = "merged_locally"
		} else {
			logging.Info("enrichStaleStatusCached: branch %q has no unique commits", wt.Branch)
			wt.StaleReason = "no_unique_commits"
		}
		return
	}

	// Check 2: Is remote branch gone?
	if cache.goneBranches[wt.Branch] {
		logging.Info("enrichStaleStatusCached: branch %q has gone remote", wt.Branch)
		wt.BranchStatus = "stale"
		wt.StaleReason = "remote_gone"
		return
	}

	logging.Debug("enrichStaleStatusCached: branch %q is active", wt.Branch)
	wt.BranchStatus = "active"
}

// enrichStaleStatus checks if a worktree's branch is stale (merged or remote gone)
// Deprecated: Use enrichStaleStatusCached with buildStaleCache for better performance
func (wm *WorktreeManager) enrichStaleStatus(wt *WorktreeInfo) {
	logging.Debug("enrichStaleStatus: checking branch %q", wt.Branch)

	// Skip main worktree and missing worktrees
	if wt.IsMain || wt.Status == "missing" {
		logging.Debug("enrichStaleStatus: skipping %q (isMain=%v, status=%s)", wt.Branch, wt.IsMain, wt.Status)
		wt.BranchStatus = "active"
		return
	}

	// Skip detached HEAD or bare repos
	if wt.Branch == "(detached)" || wt.Branch == "(bare)" {
		logging.Debug("enrichStaleStatus: skipping %q (detached/bare)", wt.Branch)
		wt.BranchStatus = "active"
		return
	}

	// Check 1: Is branch merged into main/master?
	merged, hasUniqueCommits := wm.isBranchMerged(wt.Branch)
	if merged {
		wt.BranchStatus = "stale"
		if hasUniqueCommits {
			logging.Info("enrichStaleStatus: branch %q is merged into main/master", wt.Branch)
			wt.StaleReason = "merged_locally"
		} else {
			logging.Info("enrichStaleStatus: branch %q has no unique commits", wt.Branch)
			wt.StaleReason = "no_unique_commits"
		}
		return
	}

	// Check 2: Is remote branch gone (deleted after merge)?
	if wm.isRemoteBranchGone(wt.Branch) {
		logging.Info("enrichStaleStatus: branch %q has gone remote", wt.Branch)
		wt.BranchStatus = "stale"
		wt.StaleReason = "remote_gone"
		return
	}

	logging.Debug("enrichStaleStatus: branch %q is active", wt.Branch)
	wt.BranchStatus = "active"
}

// isBranchMerged checks if a branch has been merged into main/master
// Returns: merged (bool), hasUniqueCommits (bool)
// - merged=true, hasUniqueCommits=true → branch was actually merged
// - merged=true, hasUniqueCommits=false → branch has no unique commits (empty branch)
func (wm *WorktreeManager) isBranchMerged(branch string) (merged bool, hasUniqueCommits bool) {
	logging.Debug("isBranchMerged: checking if %q is merged", branch)

	// First, check if branch has any unique commits compared to main/master
	hasUniqueCommits = wm.branchHasUniqueCommits(branch)
	logging.Debug("isBranchMerged: %q hasUniqueCommits=%v", branch, hasUniqueCommits)

	// Try main first, then master
	for _, baseBranch := range []string{"main", "master"} {
		cmd := exec.Command("git", "branch", "--merged", baseBranch)
		output, err := cmd.Output()
		if err != nil {
			logging.Debug("isBranchMerged: git branch --merged %s failed: %v", baseBranch, err)
			continue // This base branch might not exist
		}

		// Parse output to find if our branch is in the merged list
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			// Clean up the line (remove leading spaces, asterisk for current branch,
			// and + for branches checked out in other worktrees)
			line = strings.TrimSpace(line)
			line = strings.TrimPrefix(line, "* ")
			line = strings.TrimPrefix(line, "+ ")
			if line == branch {
				logging.Debug("isBranchMerged: %q is merged into %s", branch, baseBranch)
				return true, hasUniqueCommits
			}
		}
	}
	logging.Debug("isBranchMerged: %q is not merged", branch)
	return false, hasUniqueCommits
}

// branchHasUniqueCommits checks if a branch currently has commits not in main/master
// Note: After a merge, this will return false even if the branch had commits before merging
func (wm *WorktreeManager) branchHasUniqueCommits(branch string) bool {
	// Try main first, then master
	for _, baseBranch := range []string{"main", "master"} {
		// Count commits in branch that are not in baseBranch
		cmd := exec.Command("git", "rev-list", "--count", baseBranch+".."+branch)
		output, err := cmd.Output()
		if err != nil {
			logging.Debug("branchHasUniqueCommits: git rev-list --count %s..%s failed: %v", baseBranch, branch, err)
			continue
		}

		countStr := strings.TrimSpace(string(output))
		if countStr != "0" {
			logging.Debug("branchHasUniqueCommits: %q has %s unique commits vs %s", branch, countStr, baseBranch)
			return true
		}
		logging.Debug("branchHasUniqueCommits: %q has no unique commits vs %s", branch, baseBranch)
		return false
	}
	return false
}

// isRemoteBranchGone checks if the remote tracking branch was deleted
func (wm *WorktreeManager) isRemoteBranchGone(branch string) bool {
	logging.Debug("isRemoteBranchGone: checking if %q remote is gone", branch)

	// Use git branch -vv to check tracking status
	cmd := exec.Command("git", "branch", "-vv")
	output, err := cmd.Output()
	if err != nil {
		logging.Debug("isRemoteBranchGone: git branch -vv failed: %v", err)
		return false
	}

	// Look for the branch line with [origin/branch: gone]
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		// Check if this line is for our branch
		trimmed := strings.TrimSpace(line)
		trimmed = strings.TrimPrefix(trimmed, "* ")

		// Line format: "branch-name hash [origin/branch: gone] commit message"
		// or: "branch-name hash [origin/branch] commit message"
		if strings.HasPrefix(trimmed, branch+" ") || strings.HasPrefix(trimmed, branch+"\t") {
			// Check if it contains ": gone]"
			if strings.Contains(line, ": gone]") {
				logging.Debug("isRemoteBranchGone: %q remote branch is gone", branch)
				return true
			}
			logging.Debug("isRemoteBranchGone: %q remote branch exists", branch)
			break
		}
	}
	logging.Debug("isRemoteBranchGone: %q has no remote tracking", branch)
	return false
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

// GitHubStatus represents the availability of GitHub CLI
type GitHubStatus int

const (
	GitHubUnchecked GitHubStatus = iota
	GitHubAvailable
	GitHubUnavailable
)

// CheckGitHubAvailability checks if gh CLI is installed and authenticated
func (wm *WorktreeManager) CheckGitHubAvailability() GitHubStatus {
	// Check if gh is installed
	if _, err := exec.LookPath("gh"); err != nil {
		logging.Debug("CheckGitHubAvailability: gh CLI not installed")
		return GitHubUnavailable
	}

	// Check if gh is authenticated
	cmd := exec.Command("gh", "auth", "status")
	if err := cmd.Run(); err != nil {
		logging.Debug("CheckGitHubAvailability: gh not authenticated: %v", err)
		return GitHubUnavailable
	}

	logging.Debug("CheckGitHubAvailability: gh CLI available and authenticated")
	return GitHubAvailable
}

// PRInfo holds GitHub PR information for a branch
type PRInfo struct {
	Number  int    `json:"number"`
	State   string `json:"state"` // OPEN, CLOSED, MERGED
	URL     string `json:"url"`
	IsDraft bool   `json:"isDraft"`
}

// FetchPRStatus fetches PR status for a branch using gh CLI
// Returns nil if no PR exists or gh is unavailable
func (wm *WorktreeManager) FetchPRStatus(branch string) *PRInfo {
	logging.Debug("FetchPRStatus: checking PR for branch %q", branch)

	// Use gh pr view to get PR info for this branch
	cmd := exec.Command("gh", "pr", "view", branch, "--json", "number,state,url,isDraft")
	output, err := cmd.Output()
	if err != nil {
		// No PR exists or other error - this is normal
		logging.Debug("FetchPRStatus: no PR for branch %q: %v", branch, err)
		return nil
	}

	var pr PRInfo
	if err := json.Unmarshal(output, &pr); err != nil {
		logging.Debug("FetchPRStatus: failed to parse PR info: %v", err)
		return nil
	}

	logging.Debug("FetchPRStatus: found PR #%d (%s) for branch %q", pr.Number, pr.State, branch)
	return &pr
}

// EnrichWithGitHubStatus fetches GitHub PR status for all worktrees
// This should be called async after initial worktree load
func (wm *WorktreeManager) EnrichWithGitHubStatus(worktrees []WorktreeInfo) {
	logging.Debug("EnrichWithGitHubStatus: enriching %d worktrees", len(worktrees))

	for i := range worktrees {
		wt := &worktrees[i]

		// Skip main worktree
		if wt.IsMain {
			continue
		}

		// Skip detached HEAD or bare repos
		if wt.Branch == "(detached)" || wt.Branch == "(bare)" {
			continue
		}

		pr := wm.FetchPRStatus(wt.Branch)
		if pr != nil {
			wt.PRNumber = pr.Number
			wt.PRURL = pr.URL

			// Handle draft state
			if pr.IsDraft {
				wt.PRState = "DRAFT"
			} else {
				wt.PRState = pr.State
			}

			// Update stale status based on PR state
			if pr.State == "MERGED" {
				wt.BranchStatus = "stale"
				wt.StaleReason = "pr_merged"
				logging.Info("EnrichWithGitHubStatus: branch %q has merged PR #%d", wt.Branch, pr.Number)
			} else if pr.State == "CLOSED" {
				wt.BranchStatus = "stale"
				wt.StaleReason = "pr_closed"
				logging.Info("EnrichWithGitHubStatus: branch %q has closed PR #%d", wt.Branch, pr.Number)
			}
		}
	}
}

// OpenPRInBrowser opens the PR for a branch in the default browser
func (wm *WorktreeManager) OpenPRInBrowser(branch string) error {
	logging.Debug("OpenPRInBrowser: opening PR for branch %q", branch)

	cmd := exec.Command("gh", "pr", "view", branch, "--web")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to open PR: %w", err)
	}

	return nil
}
