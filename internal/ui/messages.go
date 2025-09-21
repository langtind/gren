package ui

import "gren/internal/git"

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
	branchStatuses    []BranchStatus
	recommendedBase   string
	err               error
}

type deleteInitMsg struct {
	err error
}

type projectAnalysisCompleteMsg struct{}

type initExecutionCompleteMsg struct{}

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