package ui

import (
	"github.com/langtind/gren/internal/core"
	"github.com/langtind/gren/internal/git"
)

// Message types for Tea commands
type projectInfoMsg struct {
	info *git.RepoInfo
	err  error
}

type initializeMsg struct {
	branchStatuses []BranchStatus
	err            error
}

type createInitMsg struct {
	branchStatuses  []BranchStatus
	recommendedBase string
	err             error
}

type deleteInitMsg struct {
	selectedWorktree *Worktree // Specific worktree to delete, nil for multi-select
	err              error
}

type projectAnalysisCompleteMsg struct{}

type initExecutionCompleteMsg struct {
	configCreated bool
	hookCreated   bool
	message       string
	err           error
}

type worktreeCreatedMsg struct {
	branchName string
	warning    string // Warning message (e.g., "main has 2 unpushed commits")
	err        error
}

type worktreeDeletedMsg struct {
	deletedCount int
	err          error
}

type openInInitializedMsg struct {
	worktreePath string
	actions      []PostCreateAction
}

type scriptEditCompleteMsg struct {
	err error
}

type commitCompleteMsg struct {
	err error
}

type scriptCreateCompleteMsg struct {
	err error
}

type availableBranchesLoadedMsg struct {
	branches []BranchStatus
	err      error
}

type configInitializedMsg struct {
	files []ConfigFile
}

type configFileOpenedMsg struct {
	err error
}

type pruneCompleteMsg struct {
	err         error
	prunedCount int
	prunedPaths []string
}

// New cleanup progress messages for incremental feedback
type cleanupStartedMsg struct {
	totalCount int
}

type cleanupItemStartMsg struct {
	worktreeIndex int    // Index in staleWorktrees array
	worktreeName  string // Branch name for logging
}

type cleanupItemCompleteMsg struct {
	worktreeIndex int
	worktreeName  string
	success       bool
	errorMsg      string // Human-readable error (e.g., "has uncommitted changes")
}

type cleanupFinishedMsg struct {
	totalCleaned int
	totalFailed  int
}

type aiScriptGeneratedMsg struct {
	script string
	err    error
}

type githubRefreshCompleteMsg struct {
	worktrees []Worktree
	ghStatus  core.GitHubStatus
}

type openPRCompleteMsg struct {
	err error
}

type compareInitMsg struct {
	sourceWorktree string
	sourcePath     string
	files          []CompareFileItem
	err            error
}

type compareApplyCompleteMsg struct {
	appliedCount int
	err          error
}

type compareDiffViewedMsg struct {
	err error
}

type compareDiffLoadedMsg struct {
	content string
	err     error
}

type mergeProgressMsg struct {
	message string
}

type mergeCompleteMsg struct {
	result string
	err    error
}

type forEachItemCompleteMsg struct {
	worktree string
	output   string
	success  bool
}

type forEachCompleteMsg struct{}

type stepCommitCompleteMsg struct {
	result string
	err    error
}

type llmMessageGeneratedMsg struct {
	message string
	err     error
}
