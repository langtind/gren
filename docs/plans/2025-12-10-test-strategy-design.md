# Test Strategy Design for Gren

## Overview

This document describes the testing strategy for gren, a Git worktree manager with TUI and CLI interfaces.

## Goals

1. **Regression prevention** - Catch bugs before they reach users
2. **Behavior documentation** - Tests show how code should be used
3. **Refactoring confidence** - High coverage enables safe code changes

## Strategy: Bottom-up with Hybrid Mocking

- Start with unit tests for each package
- Build up to integration tests for complete flows
- Use mocks for fast unit tests, real git repos for integration tests

## Test Structure

```
internal/
├── testutil/                  # Shared test helpers
│   ├── git.go                 # Temp git repo creation
│   └── mock.go                # Mock implementations
├── git/
│   ├── repo_test.go           # Repository operations
│   └── branches_test.go       # Branch status tests
├── config/
│   ├── config_test.go         # Config loading/saving
│   └── init_test.go           # Package manager detection
├── core/
│   ├── worktree_test.go       # Unit tests with mocks
│   └── worktree_integration_test.go  # Integration with real git
├── cli/
│   └── cli_test.go            # Command parsing
└── ui/
    └── types_test.go          # Helper functions
```

## Test Helpers (testutil package)

### git.go - Real Git Repos
```go
// CreateTempRepo creates a git repo in a temp directory
func CreateTempRepo(t *testing.T) (path string, cleanup func())

// CreateTempRepoWithCommit creates repo with initial commit
func CreateTempRepoWithCommit(t *testing.T) (path string, cleanup func())

// CreateTempRepoWithBranches creates repo with multiple branches
func CreateTempRepoWithBranches(t *testing.T, branches []string) (path string, cleanup func())

// AddWorktree adds a worktree to the test repo
func AddWorktree(t *testing.T, repoPath, worktreePath, branch string)
```

### mock.go - Mock Repository
```go
// MockRepository implements git.Repository interface
type MockRepository struct {
    RepoInfo        *git.RepoInfo
    RepoInfoErr     error
    Worktrees       []git.Worktree
    WorktreesErr    error
    BranchStatuses  []git.BranchStatus
}
```

## Unit Tests by Package

### internal/git
| Function | Tests |
|----------|-------|
| `GetRepoInfo` | ValidRepo, NotARepo, BareRepo |
| `GetBranchStatus` | Clean, WithUncommitted, AheadBehind |
| `BranchExists` | Local, Remote |

### internal/config
| Function | Tests |
|----------|-------|
| `LoadConfig` | Valid, Missing, Invalid |
| `SaveConfig` | Success |
| `DetectPackageManager` | Bun, Yarn, Pnpm, Npm, None |
| `GeneratePostCreateScript` | Success |

### internal/core
| Function | Tests |
|----------|-------|
| `CreateWorktree` | NewBranch, ExistingBranch, InvalidBranchName |
| `DeleteWorktree` | Success, CurrentWorktree |
| `ListWorktrees` | Success |

### internal/cli
| Function | Tests |
|----------|-------|
| `ParseArgs` | Create, Delete, List, Help, NoArgs |

## Integration Tests

Located in `*_integration_test.go` files with build tag:

```go
//go:build integration
```

Run separately: `go test -tags=integration ./...`

### Core Integration Tests
- `TestCreateWorktree_Integration` - Create worktree with real git
- `TestDeleteWorktree_Integration` - Delete worktree with real git
- `TestWorktreeLifecycle_Integration` - Full create -> list -> delete flow

## Test Patterns

### Table-Driven Tests
```go
func TestDetectPackageManager(t *testing.T) {
    tests := []struct {
        name     string
        files    []string
        expected string
    }{
        {"bun lockfile", []string{"bun.lockb"}, "bun"},
        {"yarn lockfile", []string{"yarn.lock"}, "yarn"},
        // ...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Setup, run, assert
        })
    }
}
```

## Running Tests

```bash
# Unit tests only (fast)
go test ./...

# With integration tests (slower)
go test -tags=integration ./...

# With coverage
go test -cover ./...

# Verbose output
go test -v ./...
```

## Estimated Coverage

- **testutil**: Helper package, not tested directly
- **git**: ~80% coverage target
- **config**: ~90% coverage target
- **core**: ~85% coverage target
- **cli**: ~90% coverage target
- **ui**: ~50% coverage (TUI is harder to unit test)

Total: ~25-35 test functions for comprehensive coverage.
