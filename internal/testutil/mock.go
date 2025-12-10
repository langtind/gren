package testutil

import (
	"context"

	"github.com/langtind/gren/internal/git"
)

// MockRepository implements git.Repository for testing.
type MockRepository struct {
	// RepoInfo to return from GetRepoInfo
	RepoInfo    *git.RepoInfo
	RepoInfoErr error

	// IsGitRepo result
	IsGitRepoResult bool
	IsGitRepoErr    error

	// RepoName to return
	RepoName    string
	RepoNameErr error

	// CurrentBranch to return
	CurrentBranch    string
	CurrentBranchErr error

	// BranchStatuses to return
	BranchStatuses    []git.BranchStatus
	BranchStatusesErr error

	// RecommendedBaseBranch to return
	RecommendedBaseBranch    string
	RecommendedBaseBranchErr error
}

// GetRepoInfo returns the mocked RepoInfo.
func (m *MockRepository) GetRepoInfo(ctx context.Context) (*git.RepoInfo, error) {
	return m.RepoInfo, m.RepoInfoErr
}

// IsGitRepo returns the mocked result.
func (m *MockRepository) IsGitRepo(ctx context.Context) (bool, error) {
	return m.IsGitRepoResult, m.IsGitRepoErr
}

// GetRepoName returns the mocked repo name.
func (m *MockRepository) GetRepoName(ctx context.Context) (string, error) {
	return m.RepoName, m.RepoNameErr
}

// GetCurrentBranch returns the mocked current branch.
func (m *MockRepository) GetCurrentBranch(ctx context.Context) (string, error) {
	return m.CurrentBranch, m.CurrentBranchErr
}

// GetBranchStatuses returns the mocked branch statuses.
func (m *MockRepository) GetBranchStatuses(ctx context.Context) ([]git.BranchStatus, error) {
	return m.BranchStatuses, m.BranchStatusesErr
}

// GetRecommendedBaseBranch returns the mocked recommended base branch.
func (m *MockRepository) GetRecommendedBaseBranch(ctx context.Context) (string, error) {
	return m.RecommendedBaseBranch, m.RecommendedBaseBranchErr
}

// NewMockRepository creates a MockRepository with sensible defaults.
func NewMockRepository() *MockRepository {
	return &MockRepository{
		RepoInfo: &git.RepoInfo{
			Name:          "test-repo",
			Path:          "/tmp/test-repo",
			IsGitRepo:     true,
			IsInitialized: true,
			CurrentBranch: "main",
		},
		IsGitRepoResult: true,
		RepoName:        "test-repo",
		CurrentBranch:   "main",
		BranchStatuses: []git.BranchStatus{
			{Name: "main", IsClean: true, IsCurrent: true},
		},
		RecommendedBaseBranch: "main",
	}
}
