package core

import (
	"strings"
	"testing"

	"github.com/langtind/gren/internal/config"
)

// SetForceInteractive makes ALL hooks run with inherited stdio (a real TTY),
// regardless of each hook's own `interactive` setting. This is what powers
// `gren hook-run --interactive`, so a caller (e.g. the herdr bootstrap pane)
// can run the repo's normal, non-interactive hooks in a terminal — needed for
// interactive setup like `op` TouchID or `make seed`.
//
// Observable contract: in captured (non-interactive) mode, executeHook records
// the hook's stdout in result.Output; when stdio is inherited it goes straight
// to the terminal, so result.Output is empty.
func TestExecuteHook_ForceInteractive(t *testing.T) {
	repo := mkRepo(t)
	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	t.Setenv("HOME", stateDir)

	wm := &WorktreeManager{}
	ctx := HookContext{WorktreePath: repo, BranchName: "main", RepoRoot: repo}

	// Baseline: a non-interactive hook has its stdout captured.
	base := wm.executeHook(config.HookPostCreate, "echo captured", ctx, "", false)
	if base.Err != nil {
		t.Fatalf("baseline hook failed: %v", base.Err)
	}
	if !strings.Contains(base.Output, "captured") {
		t.Fatalf("expected captured stdout in non-interactive mode, got %q", base.Output)
	}

	// With force-interactive on, the same non-interactive hook inherits stdio,
	// so nothing is captured into result.Output.
	wm.SetForceInteractive(true)
	forced := wm.executeHook(config.HookPostCreate, "true", ctx, "", false)
	if forced.Err != nil {
		t.Fatalf("forced hook failed: %v", forced.Err)
	}
	if forced.Output != "" {
		t.Errorf("expected empty captured output when force-interactive, got %q", forced.Output)
	}

	// Clearing it restores capture.
	wm.SetForceInteractive(false)
	restored := wm.executeHook(config.HookPostCreate, "echo again", ctx, "", false)
	if !strings.Contains(restored.Output, "again") {
		t.Errorf("expected capture restored after clearing force-interactive, got %q", restored.Output)
	}
}
