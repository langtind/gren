package core

import (
	"os"
	"path/filepath"
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
// the hook's stdout in result.Output and stderr separately. In interactive mode
// the hook runs against a real pty, but its combined output is tee'd to a capped
// tail in result.Output (and a per-run disk log) so it is still captured.
func TestExecuteHook_ForceInteractive(t *testing.T) {
	repo := mkRepo(t)
	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	t.Setenv("HOME", stateDir)
	t.Setenv("GREN_LOG_DIR", t.TempDir())

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

	// Force-interactive runs against a real pty, but the combined output is now
	// tee'd to a capped tail in Output — captured even though the child saw a TTY.
	wm.SetForceInteractive(true)
	forced := wm.executeHook(config.HookPostCreate, "echo interactive-captured", ctx, "", false)
	if forced.Err != nil {
		t.Fatalf("forced hook failed: %v", forced.Err)
	}
	if !strings.Contains(forced.Output, "interactive-captured") {
		t.Errorf("expected interactive output tee'd to Output, got %q", forced.Output)
	}

	// Clearing it keeps normal capture.
	wm.SetForceInteractive(false)
	restored := wm.executeHook(config.HookPostCreate, "echo again", ctx, "", false)
	if !strings.Contains(restored.Output, "again") {
		t.Errorf("expected capture after clearing force-interactive, got %q", restored.Output)
	}
}

// TestExecuteHook_InteractiveWritesHookLogFileOnFailure is the regression guard
// for the core observability fix: an interactive hook that fails must leave its
// full (merged stdout+stderr) output in a per-run disk file, so the trace
// survives even if the pane closes on failure.
func TestExecuteHook_InteractiveWritesHookLogFileOnFailure(t *testing.T) {
	repo := mkRepo(t)
	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	t.Setenv("HOME", stateDir)
	logDir := t.TempDir()
	t.Setenv("GREN_LOG_DIR", logDir)

	wm := &WorktreeManager{}
	wm.SetForceInteractive(true)
	ctx := HookContext{WorktreePath: repo, BranchName: "feat/x", RepoRoot: repo}

	result := wm.executeHook(config.HookPostCreate,
		"echo SETUP-BANNER; echo BOOM >&2; exit 1", ctx, "", false)

	if result.Err == nil {
		t.Fatal("expected the failing hook to return an error")
	}
	if !strings.Contains(result.Output, "SETUP-BANNER") {
		t.Errorf("expected captured stdout in Output, got %q", result.Output)
	}

	hookDir := filepath.Join(logDir, "hooks")
	entries, err := os.ReadDir(hookDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected a per-run hook log in %s (err=%v, entries=%d)", hookDir, err, len(entries))
	}
	data, err := os.ReadFile(filepath.Join(hookDir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "SETUP-BANNER") || !strings.Contains(content, "BOOM") {
		t.Errorf("hook log should hold both stdout and stderr (pty merges them), got: %q", content)
	}
	if !strings.Contains(entries[0].Name(), "post-create-feat-x-") {
		t.Errorf("unexpected hook log name %q", entries[0].Name())
	}
}
