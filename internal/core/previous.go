package core

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

const previousWorktreeConfigKey = "gren.previousWorktree"

// GetPreviousWorktreePath returns the path of the previously active worktree.
// Returns an empty string (and no error) if no previous worktree has been set.
func (wm *WorktreeManager) GetPreviousWorktreePath() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "config", "--local", previousWorktreeConfigKey)
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			// git config exits with code 1 when the key doesn't exist — not an error
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// SetPreviousWorktreePath stores the path of the most recently active worktree
// so it can be recalled with `gren switch -`.
func (wm *WorktreeManager) SetPreviousWorktreePath(path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "config", "--local", previousWorktreeConfigKey, path)
	return cmd.Run()
}
