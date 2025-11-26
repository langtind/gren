package ui

import "github.com/langtind/gren/internal/git"

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
