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
	HasSubmodules  bool   // True if worktree contains .gitmodules (requires --force to delete)

	// Stale detection fields
	BranchStatus string // "active", "stale", or "" if not yet checked
	StaleReason  string // "merged_locally", "no_unique_commits", "remote_gone", "pr_merged", "pr_closed"

	// GitHub PR fields (populated async, empty if gh unavailable or no PR)
	PRNumber int    // PR number, 0 if no PR
	PRState  string // "OPEN", "MERGED", "CLOSED", "DRAFT", "" if unknown
	PRURL    string // Full URL to PR for "Open in browser"

	// CI status fields (populated async via gh CLI)
	CIStatus     string // "success", "failure", "pending", "error", "" if unknown
	CIConclusion string // Detailed conclusion from GitHub Actions
	ChecksURL    string // URL to checks page

	Marker MarkerType
}

type MergeOptions struct {
	Target string
	Squash bool
	Remove bool
	Verify bool
	Rebase bool
	Yes    bool
	Force  bool
}

type MergeResult struct {
	SourceBranch    string
	TargetBranch    string
	CommitsSquashed int
	WorktreeRemoved bool
	WorktreePath    string
	Skipped         bool
	SkipReason      string
}

// ForEachOptions contains parameters for running a command in all worktrees
type ForEachOptions struct {
	Command     []string // Command and arguments to run
	SkipCurrent bool     // Skip the current worktree
	SkipMain    bool     // Skip the main worktree
	Parallel    bool     // Run in parallel (default: sequential)
}

// ForEachResult contains the result of running a command in a worktree
type ForEachResult struct {
	Worktree *WorktreeInfo
	Output   string
	ExitCode int
	Error    error
}

// TemplateContext holds variables for template expansion in for-each commands
type TemplateContext struct {
	Branch          string // Branch name
	BranchSanitized string // Branch name with / → -
	Worktree        string // Absolute path to worktree
	WorktreeName    string // Worktree directory name
	Repo            string // Repository name
	RepoRoot        string // Absolute path to main repo
	Commit          string // Full HEAD commit SHA
	ShortCommit     string // Short HEAD commit SHA (7 chars)
	DefaultBranch   string // Default branch (main/master)
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
func (wm *WorktreeManager) CreateWorktree(ctx context.Context, req CreateWorktreeRequest) (worktreePath string, warning string, err error) {
	logging.Info("CreateWorktree called: name=%s, branch=%s, base=%s, isNew=%v", req.Name, req.Branch, req.BaseBranch, req.IsNewBranch)

	// Check prerequisites
	if err := wm.CheckPrerequisites(); err != nil {
		logging.Error("Prerequisites check failed: %v", err)
		return "", "", err
	}

	// Fetch latest from origin to ensure we have up-to-date remote refs
	wm.FetchOrigin()

	// Load configuration
	cfg, err := wm.configManager.Load()
	if err != nil {
		logging.Error("Failed to load configuration: %v", err)
		return "", "", fmt.Errorf("failed to load configuration: %w", err)
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
				return "", "", fmt.Errorf("failed to get repo info: %w", err)
			}
			worktreeDir = fmt.Sprintf("../%s-worktrees", repoInfo.Name)
			logging.Debug("Using default worktree_dir: %s", worktreeDir)
		}
	}

	// Sanitize worktree name: replace / with - to avoid nested directories
	worktreeName := strings.ReplaceAll(req.Name, "/", "-")
	worktreePath = filepath.Join(worktreeDir, worktreeName)
	logging.Debug("Worktree path: %s", worktreePath)

	// Create worktree directory if it doesn't exist
	if _, err := os.Stat(worktreeDir); os.IsNotExist(err) {
		logging.Debug("Creating worktree directory: %s", worktreeDir)
		if err := os.MkdirAll(worktreeDir, 0755); err != nil {
			logging.Error("Failed to create worktree directory: %v", err)
			return "", "", fmt.Errorf("failed to create worktree directory: %w", err)
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
			return "", "", fmt.Errorf("branch '%s' is already checked out in another worktree", branchName)
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
			// Local is in sync or behind remote
			if req.IsNewBranch {
				// Creating new branch - use remote for latest code
				gitCmd = fmt.Sprintf("git worktree add --track -b %s %s %s", branchName, worktreePath, sourceRef)
				logging.Info("Using remote branch for latest code: %s", sourceRef)
				cmd = exec.Command("git", "worktree", "add", "--track", "-b", branchName, worktreePath, sourceRef)
			} else {
				// Using existing branch (--existing flag) - use local branch directly
				gitCmd = fmt.Sprintf("git worktree add %s %s", worktreePath, branchName)
				logging.Info("Using existing local branch: %s", branchName)
				cmd = exec.Command("git", "worktree", "add", worktreePath, branchName)
			}
		}
	} else if req.IsNewBranch {
		// Branch doesn't exist - create new from base
		baseBranch := req.BaseBranch
		if baseBranch == "" {
			// Get recommended base branch
			baseBranch, err = wm.gitRepo.GetRecommendedBaseBranch(ctx)
			if err != nil {
				logging.Error("Failed to get recommended base branch: %v", err)
				return "", "", fmt.Errorf("failed to get recommended base branch: %w", err)
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
		return "", "", fmt.Errorf("branch '%s' not found locally or on remote", branchName)
	}

	logging.Debug("Running: %s", gitCmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logging.Error("git worktree add failed: %v, output: %s", err, string(output))
		if len(output) == 0 {
			return "", "", fmt.Errorf("git worktree add failed: %w", err)
		}
		return "", "", fmt.Errorf("git worktree add failed: %s", string(output))
	}

	// Initialize submodules in the new worktree
	if _, err := os.Stat(".gitmodules"); err == nil {
		submoduleCmd := exec.Command("git", "-C", worktreePath, "submodule", "update", "--init", "--recursive")
		if err := submoduleCmd.Run(); err != nil {
			logging.Warn("Failed to initialize submodules: %v", err)
		}
	}

	hookBranchName := req.Branch
	if req.IsNewBranch && hookBranchName == "" {
		hookBranchName = req.Name
	}
	hookRepoRoot, _ := wm.getRepoRoot()
	hookWorktreePath := worktreePath
	if !filepath.IsAbs(worktreePath) {
		hookWorktreePath = filepath.Join(hookRepoRoot, worktreePath)
	}

	hookCtx := HookContext{
		WorktreePath: hookWorktreePath,
		BranchName:   hookBranchName,
		BaseBranch:   req.BaseBranch,
		RepoRoot:     hookRepoRoot,
	}
	wm.RunHook(config.HookPostCreate, hookCtx)

	logging.Info("Created worktree '%s' at %s", req.Name, worktreePath)
	return worktreePath, warning, nil
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

	wm.enrichMarkers(ctx, worktrees)

	return worktrees, nil
}

// enrichWorktreeStatus adds detailed status information to a worktree
func (wm *WorktreeManager) enrichWorktreeStatus(wt *WorktreeInfo) {
	// Skip if worktree is missing
	if wt.Status == "missing" {
		return
	}

	// Check for submodules (affects deletion - requires --force)
	if _, err := os.Stat(filepath.Join(wt.Path, ".gitmodules")); err == nil {
		wt.HasSubmodules = true
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

	wt.LastCommit = getLastCommitTime(wt.Path)
}

func (wm *WorktreeManager) enrichMarkers(ctx context.Context, worktrees []WorktreeInfo) {
	mm := NewMarkerManager()
	markers, err := mm.ListMarkers(ctx)
	if err != nil {
		logging.Warn("Failed to list markers: %v", err)
		return
	}

	for i := range worktrees {
		if marker, ok := markers[worktrees[i].Branch]; ok {
			worktrees[i].Marker = marker
		}
	}
}

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
func (wm *WorktreeManager) DeleteWorktree(ctx context.Context, identifier string, force bool) error {
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

	hookResult := wm.RunPreRemoveHook(targetWorktree.Path, targetWorktree.Branch)
	if hookResult.Ran && hookResult.Err != nil {
		return fmt.Errorf("pre-remove hook failed: %w\nOutput: %s", hookResult.Err, hookResult.Output)
	}

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

	// 2. Remove worktree using git
	// Note: --force is required for worktrees with submodules (even after deinit)
	// or when force parameter is true (to ignore uncommitted changes)
	var cmd *exec.Cmd
	if hasSubmodules || force {
		cmd = exec.Command("git", "worktree", "remove", "--force", targetWorktree.Path)
		if hasSubmodules {
			logging.Debug("DeleteWorktree: using --force flag (worktree has submodules)")
		} else {
			logging.Debug("DeleteWorktree: using --force flag (uncommitted changes will be ignored)")
		}
	} else {
		cmd = exec.Command("git", "worktree", "remove", targetWorktree.Path)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)
		var hint string
		if strings.Contains(outputStr, "submodules") {
			hint = "The worktree contains submodules. Try running:\n  git -C " + targetWorktree.Path + " submodule deinit --all --force\nThen try deleting again with force."
		} else if strings.Contains(outputStr, "modified or untracked") {
			hint = "The worktree has uncommitted changes. Commit or stash them first, or use force delete."
		} else {
			hint = "Use force delete to remove anyway."
		}
		return fmt.Errorf("failed to remove worktree '%s': %w\n\nOutput: %s\n\n%s",
			targetWorktree.Name, err, outputStr, hint)
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

type CIInfo struct {
	Status     string
	Conclusion string
	ChecksURL  string
}

func (wm *WorktreeManager) FetchCIStatus(branch string) *CIInfo {
	logging.Debug("FetchCIStatus: checking CI for branch %q", branch)

	cmd := exec.Command("gh", "pr", "checks", branch, "--json", "state,name,conclusion")
	output, err := cmd.Output()
	if err != nil {
		logging.Debug("FetchCIStatus: no checks for branch %q: %v", branch, err)
		return nil
	}

	var checks []struct {
		State      string `json:"state"`
		Name       string `json:"name"`
		Conclusion string `json:"conclusion"`
	}
	if err := json.Unmarshal(output, &checks); err != nil {
		logging.Debug("FetchCIStatus: failed to parse checks: %v", err)
		return nil
	}

	if len(checks) == 0 {
		return nil
	}

	info := &CIInfo{}
	hasFailure := false
	hasPending := false
	allSuccess := true

	for _, check := range checks {
		switch check.State {
		case "FAILURE", "ERROR":
			hasFailure = true
			allSuccess = false
		case "PENDING", "QUEUED", "IN_PROGRESS":
			hasPending = true
			allSuccess = false
		case "SUCCESS":
		default:
			allSuccess = false
		}
	}

	if hasFailure {
		info.Status = "failure"
		info.Conclusion = "Some checks failed"
	} else if hasPending {
		info.Status = "pending"
		info.Conclusion = "Checks in progress"
	} else if allSuccess {
		info.Status = "success"
		info.Conclusion = "All checks passed"
	} else {
		info.Status = "unknown"
	}

	return info
}

func (wm *WorktreeManager) EnrichWithCIStatus(worktrees []WorktreeInfo) {
	logging.Debug("EnrichWithCIStatus: enriching %d worktrees", len(worktrees))

	for i := range worktrees {
		wt := &worktrees[i]

		if wt.IsMain || wt.Branch == "(detached)" || wt.Branch == "(bare)" {
			continue
		}

		if wt.PRNumber == 0 {
			continue
		}

		ci := wm.FetchCIStatus(wt.Branch)
		if ci != nil {
			wt.CIStatus = ci.Status
			wt.CIConclusion = ci.Conclusion
			wt.ChecksURL = ci.ChecksURL
		}
	}
}

func (wm *WorktreeManager) Merge(ctx context.Context, opts MergeOptions) (*MergeResult, error) {
	logging.Info("Merge: starting merge with opts=%+v", opts)

	result := &MergeResult{}

	currentPath, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}
	result.WorktreePath = currentPath

	currentBranch, err := wm.getCurrentBranch()
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}
	result.SourceBranch = currentBranch

	targetBranch := opts.Target
	if targetBranch == "" {
		targetBranch, err = wm.getDefaultBranch()
		if err != nil {
			return nil, fmt.Errorf("failed to determine default branch: %w", err)
		}
	}
	result.TargetBranch = targetBranch

	if currentBranch == targetBranch {
		result.Skipped = true
		result.SkipReason = "already on target branch"
		return result, nil
	}

	worktrees, err := wm.ListWorktrees(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	var currentWorktree *WorktreeInfo
	for i, wt := range worktrees {
		if wt.IsCurrent {
			currentWorktree = &worktrees[i]
			break
		}
	}

	if currentWorktree == nil {
		return nil, fmt.Errorf("could not find current worktree")
	}

	if currentWorktree.IsMain {
		result.Skipped = true
		result.SkipReason = "cannot merge from main worktree"
		return result, nil
	}

	hasChanges := currentWorktree.ModifiedCount > 0 || currentWorktree.UntrackedCount > 0 || currentWorktree.StagedCount > 0
	if hasChanges && !opts.Force {
		if err := wm.stageAndCommitChanges(currentBranch); err != nil {
			return nil, fmt.Errorf("failed to commit changes: %w", err)
		}
	}

	if opts.Squash {
		count, err := wm.getCommitsAhead(currentBranch, targetBranch)
		if err != nil {
			logging.Warn("Merge: could not count commits ahead: %v", err)
		} else if count > 1 {
			if err := wm.squashCommits(targetBranch, count); err != nil {
				return nil, fmt.Errorf("failed to squash commits: %w", err)
			}
			result.CommitsSquashed = count
		}
	}

	if opts.Rebase {
		if err := wm.rebaseOnto(targetBranch); err != nil {
			return nil, fmt.Errorf("rebase failed: %w", err)
		}
	}

	if opts.Verify {
		hookResult := wm.RunPreMergeHook(currentPath, currentBranch, targetBranch)
		if hookResult.Ran && hookResult.Err != nil {
			return nil, fmt.Errorf("pre-merge hook failed: %s\n%s", hookResult.Err, hookResult.Output)
		}
	}

	if err := wm.fastForwardMerge(currentBranch, targetBranch); err != nil {
		return nil, fmt.Errorf("merge failed: %w", err)
	}

	if opts.Remove {
		if opts.Verify {
			wm.RunPreRemoveHook(currentPath, currentBranch)
		}

		repoRoot, _ := wm.getRepoRoot()
		if err := wm.DeleteWorktree(ctx, currentBranch, true); err != nil {
			logging.Warn("Merge: failed to remove worktree: %v", err)
		} else {
			result.WorktreeRemoved = true
		}

		if repoRoot != "" {
			os.Chdir(repoRoot)
		}
	}

	if opts.Verify {
		wm.RunPostMergeHook(currentPath, currentBranch, targetBranch)
	}

	return result, nil
}

func (wm *WorktreeManager) getCurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func (wm *WorktreeManager) getDefaultBranch() (string, error) {
	for _, branch := range []string{"main", "master"} {
		cmd := exec.Command("git", "rev-parse", "--verify", branch)
		if err := cmd.Run(); err == nil {
			return branch, nil
		}
	}

	cmd := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD")
	output, err := cmd.Output()
	if err == nil {
		ref := strings.TrimSpace(string(output))
		if strings.HasPrefix(ref, "refs/remotes/origin/") {
			return strings.TrimPrefix(ref, "refs/remotes/origin/"), nil
		}
	}

	return "", fmt.Errorf("could not determine default branch")
}

func (wm *WorktreeManager) stageAndCommitChanges(branch string) error {
	logging.Info("Merge: staging and committing changes")

	addCmd := exec.Command("git", "add", "-A")
	if output, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add failed: %s", string(output))
	}

	commitCmd := exec.Command("git", "commit", "-m", fmt.Sprintf("WIP: changes on %s", branch))
	if output, err := commitCmd.CombinedOutput(); err != nil {
		if !strings.Contains(string(output), "nothing to commit") {
			return fmt.Errorf("git commit failed: %s", string(output))
		}
	}

	return nil
}

func (wm *WorktreeManager) getCommitsAhead(branch, target string) (int, error) {
	cmd := exec.Command("git", "rev-list", "--count", fmt.Sprintf("%s..%s", target, branch))
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	var count int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &count); err != nil {
		return 0, err
	}

	return count, nil
}

func (wm *WorktreeManager) squashCommits(target string, count int) error {
	logging.Info("Merge: squashing %d commits", count)

	mergeBase := exec.Command("git", "merge-base", "HEAD", target)
	baseOutput, err := mergeBase.Output()
	if err != nil {
		return fmt.Errorf("failed to find merge base: %w", err)
	}
	base := strings.TrimSpace(string(baseOutput))

	resetCmd := exec.Command("git", "reset", "--soft", base)
	if output, err := resetCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git reset failed: %s", string(output))
	}

	branch, _ := wm.getCurrentBranch()
	commitCmd := exec.Command("git", "commit", "-m", fmt.Sprintf("Squashed commits from %s", branch))
	if output, err := commitCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit failed: %s", string(output))
	}

	return nil
}

func (wm *WorktreeManager) rebaseOnto(target string) error {
	logging.Info("Merge: rebasing onto %s", target)

	cmd := exec.Command("git", "rebase", target)
	if output, err := cmd.CombinedOutput(); err != nil {
		exec.Command("git", "rebase", "--abort").Run()
		return fmt.Errorf("rebase failed (conflicts?): %s", string(output))
	}

	return nil
}

func (wm *WorktreeManager) fastForwardMerge(source, target string) error {
	logging.Info("Merge: fast-forward merging %s into %s", source, target)

	sourceRef := exec.Command("git", "rev-parse", source)
	sourceOutput, err := sourceRef.Output()
	if err != nil {
		return fmt.Errorf("failed to get source ref: %w", err)
	}
	sourceCommit := strings.TrimSpace(string(sourceOutput))

	updateRef := exec.Command("git", "update-ref", fmt.Sprintf("refs/heads/%s", target), sourceCommit)
	if output, err := updateRef.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to update target ref: %s", string(output))
	}

	return nil
}

func (wm *WorktreeManager) ForEach(ctx context.Context, opts ForEachOptions) ([]ForEachResult, error) {
	logging.Info("ForEach: running command in all worktrees: %v", opts.Command)

	worktrees, err := wm.ListWorktrees(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	repoRoot, _ := wm.getRepoRoot()
	repoName := filepath.Base(repoRoot)
	defaultBranch, _ := wm.getDefaultBranch()

	var results []ForEachResult

	for _, wt := range worktrees {
		if opts.SkipCurrent && wt.IsCurrent {
			continue
		}
		if opts.SkipMain && wt.IsMain {
			continue
		}
		if wt.Status == "missing" {
			continue
		}

		tmplCtx := wm.buildTemplateContext(&wt, repoRoot, repoName, defaultBranch)
		expandedCmd := wm.expandCommand(opts.Command, tmplCtx)

		result := ForEachResult{Worktree: &wt}

		var cmd *exec.Cmd
		if len(expandedCmd) == 1 {
			cmd = exec.CommandContext(ctx, "sh", "-c", expandedCmd[0])
		} else {
			cmd = exec.CommandContext(ctx, expandedCmd[0], expandedCmd[1:]...)
		}
		cmd.Dir = wt.Path

		output, err := cmd.CombinedOutput()
		result.Output = string(output)

		if err != nil {
			result.Error = err
			if exitErr, ok := err.(*exec.ExitError); ok {
				result.ExitCode = exitErr.ExitCode()
			} else {
				result.ExitCode = 1
			}
		}

		results = append(results, result)
	}

	return results, nil
}

func (wm *WorktreeManager) buildTemplateContext(wt *WorktreeInfo, repoRoot, repoName, defaultBranch string) TemplateContext {
	commit := wm.getCommitSHA(wt.Path)
	shortCommit := commit
	if len(commit) > 7 {
		shortCommit = commit[:7]
	}

	return TemplateContext{
		Branch:          wt.Branch,
		BranchSanitized: sanitizeBranch(wt.Branch),
		Worktree:        wt.Path,
		WorktreeName:    wt.Name,
		Repo:            repoName,
		RepoRoot:        repoRoot,
		Commit:          commit,
		ShortCommit:     shortCommit,
		DefaultBranch:   defaultBranch,
	}
}

func (wm *WorktreeManager) getCommitSHA(worktreePath string) string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = worktreePath
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func sanitizeBranch(branch string) string {
	result := strings.ReplaceAll(branch, "/", "-")
	result = strings.ReplaceAll(result, "\\", "-")
	return result
}

func (wm *WorktreeManager) expandCommand(command []string, ctx TemplateContext) []string {
	expanded := make([]string, len(command))
	for i, arg := range command {
		expanded[i] = expandTemplate(arg, ctx)
	}
	return expanded
}

func expandTemplate(template string, ctx TemplateContext) string {
	replacements := map[string]string{
		"{{ branch }}":            ctx.Branch,
		"{{branch}}":              ctx.Branch,
		"{{ branch | sanitize }}": ctx.BranchSanitized,
		"{{branch|sanitize}}":     ctx.BranchSanitized,
		"{{ worktree }}":          ctx.Worktree,
		"{{worktree}}":            ctx.Worktree,
		"{{ worktree_name }}":     ctx.WorktreeName,
		"{{worktree_name}}":       ctx.WorktreeName,
		"{{ repo }}":              ctx.Repo,
		"{{repo}}":                ctx.Repo,
		"{{ repo_root }}":         ctx.RepoRoot,
		"{{repo_root}}":           ctx.RepoRoot,
		"{{ commit }}":            ctx.Commit,
		"{{commit}}":              ctx.Commit,
		"{{ short_commit }}":      ctx.ShortCommit,
		"{{short_commit}}":        ctx.ShortCommit,
		"{{ default_branch }}":    ctx.DefaultBranch,
		"{{default_branch}}":      ctx.DefaultBranch,
	}

	result := template
	for pattern, value := range replacements {
		result = strings.ReplaceAll(result, pattern, value)
	}
	return result
}

type StepCommitOptions struct {
	Message string
	UseLLM  bool
}

type StepSquashOptions struct {
	Target  string
	Message string
	UseLLM  bool
}

func (wm *WorktreeManager) StepCommit(opts StepCommitOptions) error {
	logging.Info("StepCommit: committing staged changes")

	addCmd := exec.Command("git", "add", "-A")
	if output, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add failed: %s", string(output))
	}

	statusCmd := exec.Command("git", "status", "--porcelain")
	statusOutput, err := statusCmd.Output()
	if err != nil {
		return fmt.Errorf("git status failed: %w", err)
	}
	if len(strings.TrimSpace(string(statusOutput))) == 0 {
		return fmt.Errorf("nothing to commit")
	}

	message := opts.Message
	if opts.UseLLM && message == "" {
		cfg, _ := wm.configManager.Load()
		if cfg != nil && cfg.CommitGenerator.Command != "" {
			generated, err := wm.generateCommitMessage(cfg.CommitGenerator.Command, cfg.CommitGenerator.Args)
			if err != nil {
				logging.Warn("StepCommit: LLM generation failed: %v, using default message", err)
			} else {
				message = generated
			}
		}
	}

	if message == "" {
		branch, _ := wm.getCurrentBranch()
		message = fmt.Sprintf("WIP: changes on %s", branch)
	}

	commitCmd := exec.Command("git", "commit", "-m", message)
	if output, err := commitCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit failed: %s", string(output))
	}

	return nil
}

func (wm *WorktreeManager) StepSquash(opts StepSquashOptions) error {
	logging.Info("StepSquash: squashing commits to %s", opts.Target)

	target := opts.Target
	if target == "" {
		var err error
		target, err = wm.getDefaultBranch()
		if err != nil {
			return fmt.Errorf("could not determine target branch: %w", err)
		}
	}

	currentBranch, err := wm.getCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	count, err := wm.getCommitsAhead(currentBranch, target)
	if err != nil {
		return fmt.Errorf("failed to count commits: %w", err)
	}

	if count <= 1 {
		return fmt.Errorf("nothing to squash (only %d commit ahead of %s)", count, target)
	}

	mergeBase := exec.Command("git", "merge-base", "HEAD", target)
	baseOutput, err := mergeBase.Output()
	if err != nil {
		return fmt.Errorf("failed to find merge base: %w", err)
	}
	base := strings.TrimSpace(string(baseOutput))

	message := opts.Message
	if opts.UseLLM && message == "" {
		cfg, _ := wm.configManager.Load()
		if cfg != nil && cfg.CommitGenerator.Command != "" {
			generated, err := wm.generateSquashMessage(cfg.CommitGenerator.Command, cfg.CommitGenerator.Args, target)
			if err != nil {
				logging.Warn("StepSquash: LLM generation failed: %v, using default message", err)
			} else {
				message = generated
			}
		}
	}

	if message == "" {
		message = fmt.Sprintf("Squashed %d commits from %s", count, currentBranch)
	}

	resetCmd := exec.Command("git", "reset", "--soft", base)
	if output, err := resetCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git reset failed: %s", string(output))
	}

	commitCmd := exec.Command("git", "commit", "-m", message)
	if output, err := commitCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit failed: %s", string(output))
	}

	return nil
}

func (wm *WorktreeManager) generateCommitMessage(command string, args []string) (string, error) {
	diff, err := wm.getStagedDiff()
	if err != nil {
		return "", fmt.Errorf("failed to get diff: %w", err)
	}

	prompt := buildCommitPrompt(diff, "")

	cmd := exec.Command(command, args...)
	cmd.Stdin = strings.NewReader(prompt)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("LLM command failed: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

func (wm *WorktreeManager) generateSquashMessage(command string, args []string, target string) (string, error) {
	branch, _ := wm.getCurrentBranch()

	diffCmd := exec.Command("git", "diff", fmt.Sprintf("%s...HEAD", target))
	diffOutput, err := diffCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get diff: %w", err)
	}

	logCmd := exec.Command("git", "log", "--oneline", fmt.Sprintf("%s..HEAD", target))
	logOutput, _ := logCmd.Output()

	context := fmt.Sprintf("Branch: %s\nCommits being squashed:\n%s", branch, string(logOutput))
	prompt := buildCommitPrompt(string(diffOutput), context)

	cmd := exec.Command(command, args...)
	cmd.Stdin = strings.NewReader(prompt)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("LLM command failed: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

func (wm *WorktreeManager) getStagedDiff() (string, error) {
	cmd := exec.Command("git", "diff", "--cached")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	if len(output) == 0 {
		cmd = exec.Command("git", "diff")
		output, err = cmd.Output()
		if err != nil {
			return "", err
		}
	}
	return string(output), nil
}

func buildCommitPrompt(diff, context string) string {
	var sb strings.Builder
	sb.WriteString("Generate a concise git commit message for the following changes.\n")
	sb.WriteString("Follow conventional commits format (feat:, fix:, docs:, etc.).\n")
	sb.WriteString("Be specific but concise. One line only, no body.\n\n")

	if context != "" {
		sb.WriteString("Context:\n")
		sb.WriteString(context)
		sb.WriteString("\n\n")
	}

	sb.WriteString("Diff:\n")
	if len(diff) > 10000 {
		sb.WriteString(diff[:10000])
		sb.WriteString("\n... (truncated)")
	} else {
		sb.WriteString(diff)
	}

	return sb.String()
}

// StepPush fast-forwards the target branch to include current commits
func (wm *WorktreeManager) StepPush(target string) error {
	logging.Info("StepPush: pushing to local target branch %s", target)

	if target == "" {
		var err error
		target, err = wm.getDefaultBranch()
		if err != nil {
			return fmt.Errorf("could not determine target branch: %w", err)
		}
	}

	currentBranch, err := wm.getCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	if currentBranch == target {
		return fmt.Errorf("already on target branch %s", target)
	}

	// Check if current branch can be fast-forwarded into target
	// This means target must be an ancestor of current HEAD
	mergeBaseCmd := exec.Command("git", "merge-base", target, "HEAD")
	mergeBaseOutput, err := mergeBaseCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to find merge base: %w", err)
	}
	mergeBase := strings.TrimSpace(string(mergeBaseOutput))

	// Get the commit hash of target
	targetCommitCmd := exec.Command("git", "rev-parse", target)
	targetCommitOutput, err := targetCommitCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get target commit: %w", err)
	}
	targetCommit := strings.TrimSpace(string(targetCommitOutput))

	// If merge base equals target commit, we can fast-forward
	if mergeBase != targetCommit {
		return fmt.Errorf("cannot fast-forward: %s has diverged from current branch. Rebase first with 'gren step rebase %s'", target, target)
	}

	// Update the target branch ref to point to current HEAD
	updateRefCmd := exec.Command("git", "update-ref", "refs/heads/"+target, "HEAD")
	if output, err := updateRefCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to update %s: %s", target, string(output))
	}

	logging.Info("StepPush: successfully updated %s to current HEAD", target)
	return nil
}

// StepRebase rebases current branch onto target branch
func (wm *WorktreeManager) StepRebase(target string) error {
	logging.Info("StepRebase: rebasing onto %s", target)

	if target == "" {
		var err error
		target, err = wm.getDefaultBranch()
		if err != nil {
			return fmt.Errorf("could not determine target branch: %w", err)
		}
	}

	currentBranch, err := wm.getCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	if currentBranch == target {
		return fmt.Errorf("already on target branch %s", target)
	}

	// Check if rebase is needed
	behindCount, err := wm.getCommitsBehind(currentBranch, target)
	if err != nil {
		return fmt.Errorf("failed to check if rebase needed: %w", err)
	}

	if behindCount == 0 {
		logging.Info("StepRebase: already up to date with %s", target)
		return nil
	}

	// Perform rebase
	rebaseCmd := exec.Command("git", "rebase", target)
	if output, err := rebaseCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("rebase failed: %s\nUse 'git rebase --abort' to cancel or resolve conflicts manually", string(output))
	}

	logging.Info("StepRebase: successfully rebased onto %s", target)
	return nil
}

// getCommitsBehind returns how many commits the current branch is behind target
func (wm *WorktreeManager) getCommitsBehind(current, target string) (int, error) {
	cmd := exec.Command("git", "rev-list", "--count", fmt.Sprintf("%s..%s", current, target))
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	countStr := strings.TrimSpace(string(output))
	count := 0
	fmt.Sscanf(countStr, "%d", &count)
	return count, nil
}
