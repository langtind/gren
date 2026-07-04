package core

import (
	"strconv"
	"strings"
	"testing"

	"github.com/langtind/gren/internal/config"
)

// Inline hook commands (the `sh -c` path) must have their template variables
// expanded before execution, so hooks can derive per-worktree ports/DB names
// from the branch. Script-file hooks already get context via args/env; this
// closes the gap for inline commands configured in .gren/config.toml.
func TestExecuteHook_InlineTemplating(t *testing.T) {
	repo := mkRepo(t)
	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	t.Setenv("HOME", stateDir)

	wm := &WorktreeManager{}
	ctx := HookContext{WorktreePath: repo, BranchName: "feat/my-thing", RepoRoot: repo}

	hook := `echo "port={{ branch | hash_port }} db={{ branch | sanitize_db }} b={{ branch }} wt={{ worktree_name }}"`
	result := wm.executeHook(config.HookPostCreate, hook, ctx, "", false)
	if result.Err != nil {
		t.Fatalf("hook failed: %v (stderr: %s)", result.Err, result.Stderr)
	}

	wantPort := strconv.Itoa(hashPort("feat/my-thing"))
	wantDB := sanitizeDB("feat/my-thing")

	for _, want := range []string{
		"port=" + wantPort,
		"db=" + wantDB,
		"b=feat/my-thing",
	} {
		if !strings.Contains(result.Output, want) {
			t.Errorf("expected output to contain %q, got %q", want, result.Output)
		}
	}
}
