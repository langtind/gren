package core

import (
	"os"
	"path/filepath"
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

	// Template variables are pre-quoted by the hook engine, so hook commands
	// must NOT wrap them in their own quotes.
	hook := `echo port={{ branch | hash_port }} db={{ branch | sanitize_db }} b={{ branch }}`
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

// A branch name with shell metacharacters must NOT inject commands into an
// inline hook. Git permits `$`, `(`, `)`, backticks, and `;` in branch names,
// so an inline hook that interpolates {{ branch }} would otherwise execute
// arbitrary code when run against an untrusted branch (e.g. a PR checkout).
func TestExecuteHook_InlineTemplating_NoInjection(t *testing.T) {
	repo := mkRepo(t)
	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	t.Setenv("HOME", stateDir)

	wm := &WorktreeManager{}
	// ${IFS} avoids the literal space git branch names forbid; the payload
	// would create ./pwned if the substitution executed.
	evil := "main$(touch${IFS}pwned)"
	ctx := HookContext{WorktreePath: repo, BranchName: evil, RepoRoot: repo}

	hook := `echo branch={{ branch }}`
	result := wm.executeHook(config.HookPostCreate, hook, ctx, "", false)
	if result.Err != nil {
		t.Fatalf("hook failed: %v (stderr: %s)", result.Err, result.Stderr)
	}

	if _, err := os.Stat(filepath.Join(repo, "pwned")); err == nil {
		t.Fatal("command injection: the payload created ./pwned")
	}
	if !strings.Contains(result.Output, "branch="+evil) {
		t.Errorf("expected branch to appear literally, got %q", result.Output)
	}
}
