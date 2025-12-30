package core

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/langtind/gren/internal/logging"
)

// FileStatus represents the type of change for a file
type FileStatus int

const (
	FileAdded    FileStatus = 1
	FileModified FileStatus = 2
	FileDeleted  FileStatus = 3
)

// String returns a human-readable status
func (s FileStatus) String() string {
	switch s {
	case FileAdded:
		return "added"
	case FileModified:
		return "modified"
	case FileDeleted:
		return "deleted"
	default:
		return "unknown"
	}
}

// FileChange represents a changed file between worktrees
type FileChange struct {
	Path        string     // Relative path from worktree root
	Status      FileStatus // Type of change
	IsCommitted bool       // True if change is committed, false if uncommitted
}

// CompareResult contains the result of comparing two worktrees
type CompareResult struct {
	SourceWorktree string       // Name of the source worktree (with changes)
	TargetWorktree string       // Name of the target worktree (current)
	Files          []FileChange // List of changed files
}

// CompareWorktrees compares changes between a source worktree and the current worktree
// Returns files that exist in source but differ from (or don't exist in) the current worktree
func (wm *WorktreeManager) CompareWorktrees(ctx context.Context, sourceWorktree string) (*CompareResult, error) {
	logging.Debug("CompareWorktrees: comparing %s to current worktree", sourceWorktree)

	// Get all worktrees
	worktrees, err := wm.ListWorktrees(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	// Find source and current worktree
	var sourcePath, currentPath, currentName string
	for _, wt := range worktrees {
		if wt.Name == sourceWorktree || wt.Path == sourceWorktree {
			sourcePath = wt.Path
		}
		if wt.IsCurrent {
			currentPath = wt.Path
			currentName = wt.Name
		}
	}

	if sourcePath == "" {
		return nil, fmt.Errorf("worktree '%s' not found", sourceWorktree)
	}

	if sourcePath == currentPath {
		return nil, fmt.Errorf("cannot compare worktree to itself")
	}

	result := &CompareResult{
		SourceWorktree: sourceWorktree,
		TargetWorktree: currentName,
		Files:          []FileChange{},
	}

	// Get uncommitted changes in source worktree
	uncommittedChanges, err := wm.getUncommittedChanges(sourcePath)
	if err != nil {
		logging.Warn("failed to get uncommitted changes: %v", err)
	}
	result.Files = append(result.Files, uncommittedChanges...)

	// Get committed changes (diff between branches)
	committedChanges, err := wm.getCommittedChanges(sourcePath, currentPath)
	if err != nil {
		logging.Warn("failed to get committed changes: %v", err)
	}
	result.Files = append(result.Files, committedChanges...)

	// Deduplicate files (uncommitted changes take precedence)
	result.Files = deduplicateFiles(result.Files)

	logging.Info("CompareWorktrees: found %d changed files", len(result.Files))
	return result, nil
}

// getUncommittedChanges returns uncommitted changes in a worktree
func (wm *WorktreeManager) getUncommittedChanges(worktreePath string) ([]FileChange, error) {
	cmd := exec.Command("git", "-C", worktreePath, "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git status failed: %w", err)
	}

	var changes []FileChange
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if len(line) < 4 {
			continue
		}

		// Git status --porcelain format: "XY PATH" or "XY ORIG -> PATH"
		// XY is 2 chars, then a space, then the path
		status := line[:2]
		// The path starts after "XY " (3 chars)
		path := line[3:]

		// Handle renamed files (R status) - format: "R  old -> new"
		if strings.Contains(path, " -> ") {
			parts := strings.Split(path, " -> ")
			if len(parts) == 2 {
				path = parts[1] // Use the new name
			}
		}

		var fileStatus FileStatus
		switch {
		case status[0] == '?' || status[1] == '?':
			fileStatus = FileAdded
		case status[0] == 'A' || status[1] == 'A':
			fileStatus = FileAdded
		case status[0] == 'D' || status[1] == 'D':
			fileStatus = FileDeleted
		default:
			fileStatus = FileModified
		}

		changes = append(changes, FileChange{
			Path:        path,
			Status:      fileStatus,
			IsCommitted: false,
		})
	}

	return changes, nil
}

// getCommittedChanges returns files that differ between worktree branches
func (wm *WorktreeManager) getCommittedChanges(sourcePath, targetPath string) ([]FileChange, error) {
	// Get source branch
	sourceBranchCmd := exec.Command("git", "-C", sourcePath, "rev-parse", "--abbrev-ref", "HEAD")
	sourceBranchOut, err := sourceBranchCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get source branch: %w", err)
	}
	sourceBranch := strings.TrimSpace(string(sourceBranchOut))

	// Get target branch
	targetBranchCmd := exec.Command("git", "-C", targetPath, "rev-parse", "--abbrev-ref", "HEAD")
	targetBranchOut, err := targetBranchCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get target branch: %w", err)
	}
	targetBranch := strings.TrimSpace(string(targetBranchOut))

	// Get diff between branches (commits in source not in target)
	cmd := exec.Command("git", "-C", sourcePath, "diff", "--name-status", targetBranch+".."+sourceBranch)
	output, err := cmd.Output()
	if err != nil {
		// Branches might not have common ancestor or other issue
		logging.Debug("git diff failed (might be expected): %v", err)
		return nil, nil
	}

	var changes []FileChange
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if len(line) < 2 {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		statusChar := parts[0]
		path := parts[len(parts)-1] // Last part is the filename (handles renames)

		var fileStatus FileStatus
		switch statusChar[0] {
		case 'A':
			fileStatus = FileAdded
		case 'D':
			fileStatus = FileDeleted
		default:
			fileStatus = FileModified
		}

		changes = append(changes, FileChange{
			Path:        path,
			Status:      fileStatus,
			IsCommitted: true,
		})
	}

	return changes, nil
}

// deduplicateFiles removes duplicate file entries, preferring uncommitted over committed
func deduplicateFiles(files []FileChange) []FileChange {
	seen := make(map[string]int) // path -> index in result
	var result []FileChange

	for _, f := range files {
		if idx, exists := seen[f.Path]; exists {
			// Uncommitted changes take precedence
			if !f.IsCommitted && result[idx].IsCommitted {
				result[idx] = f
			}
		} else {
			seen[f.Path] = len(result)
			result = append(result, f)
		}
	}

	return result
}

// validatePath checks if a path is safe (no path traversal)
func validatePath(path string) error {
	// Check for path traversal attempts
	if strings.Contains(path, "..") {
		return fmt.Errorf("invalid path (contains '..'): %s", path)
	}
	// Check for absolute paths
	if filepath.IsAbs(path) {
		return fmt.Errorf("invalid path (absolute path not allowed): %s", path)
	}
	return nil
}

// ApplyChanges applies selected file changes from source worktree to current worktree
func (wm *WorktreeManager) ApplyChanges(ctx context.Context, sourceWorktree string, files []FileChange) error {
	if len(files) == 0 {
		return nil
	}

	// Validate all paths before applying any changes
	for _, file := range files {
		if err := validatePath(file.Path); err != nil {
			return fmt.Errorf("security error: %w", err)
		}
	}

	logging.Info("ApplyChanges: applying %d files from %s", len(files), sourceWorktree)

	// Get all worktrees
	worktrees, err := wm.ListWorktrees(ctx)
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	// Find source and current worktree paths
	var sourcePath, currentPath string
	for _, wt := range worktrees {
		if wt.Name == sourceWorktree || wt.Path == sourceWorktree {
			sourcePath = wt.Path
		}
		if wt.IsCurrent {
			currentPath = wt.Path
		}
	}

	if sourcePath == "" {
		return fmt.Errorf("worktree '%s' not found", sourceWorktree)
	}

	// Apply each file change
	for _, file := range files {
		srcFile := filepath.Join(sourcePath, file.Path)
		dstFile := filepath.Join(currentPath, file.Path)

		switch file.Status {
		case FileAdded, FileModified:
			// Copy file from source to destination
			if err := copyFile(srcFile, dstFile); err != nil {
				return fmt.Errorf("failed to apply %s: %w", file.Path, err)
			}
			logging.Debug("Applied file: %s", file.Path)

		case FileDeleted:
			// Delete file in destination
			if err := os.Remove(dstFile); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to delete %s: %w", file.Path, err)
			}
			logging.Debug("Deleted file: %s", file.Path)
		}
	}

	logging.Info("ApplyChanges: successfully applied %d files", len(files))
	return nil
}

// copyFile copies a file from src to dst, creating directories as needed
func copyFile(src, dst string) error {
	// Create destination directory if needed
	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source: %w", err)
	}
	defer srcFile.Close()

	// Get source file info for permissions
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat source: %w", err)
	}

	// Create destination file
	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("failed to create destination: %w", err)
	}
	defer dstFile.Close()

	// Copy content
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy content: %w", err)
	}

	return nil
}
