package core

import (
	"strings"
	"testing"
	"time"

	"github.com/langtind/gren/internal/config"
)

// promptingHook guards its own prompt the way hook authors are told to: ask
// only when stdin is a terminal. Under HookInteractivityNever gren must not
// hand it a pty, or the guard passes and `read` blocks on a terminal nobody is
// attached to. That is the `gren delete -f` hang.
const promptingHook = `if [ ! -t 0 ]; then echo skipped-no-tty; exit 0; fi; printf 'Drop databases? (y/N): '; read ans; echo "answered:$ans"`

func runHookWithDeadline(t *testing.T, wm *WorktreeManager, ctx HookContext, cmd string, interactive bool) HookResult {
	t.Helper()
	done := make(chan HookResult, 1)
	go func() { done <- wm.executeHook(config.HookPreRemove, cmd, ctx, "", interactive) }()
	select {
	case r := <-done:
		return r
	case <-time.After(20 * time.Second):
		t.Fatal("hook did not finish within 20s: gren is blocked on a prompt no one can answer")
		return HookResult{}
	}
}

func TestExecuteHook_NeverDeniesPtyToInteractiveHook(t *testing.T) {
	repo := mkRepo(t)
	t.Setenv("GREN_LOG_DIR", t.TempDir())
	ctx := HookContext{WorktreePath: repo, BranchName: "main", RepoRoot: repo}

	wm := &WorktreeManager{}
	wm.SetHookInteractivity(HookInteractivityNever)

	got := runHookWithDeadline(t, wm, ctx, promptingHook, true)
	if got.Err != nil {
		t.Fatalf("hook failed: %v (output %q)", got.Err, got.Output)
	}
	if !strings.Contains(got.Output, "skipped-no-tty") {
		t.Errorf("expected the hook to take its non-interactive branch, got %q", got.Output)
	}
	if strings.Contains(got.Output, "Drop databases?") {
		t.Errorf("hook was given a pty despite HookInteractivityNever: %q", got.Output)
	}
}

func TestExecuteHook_NeverSetsNonInteractiveEnv(t *testing.T) {
	repo := mkRepo(t)
	t.Setenv("GREN_LOG_DIR", t.TempDir())
	ctx := HookContext{WorktreePath: repo, BranchName: "main", RepoRoot: repo}

	wm := &WorktreeManager{}
	if got := wm.executeHook(config.HookPreRemove, `echo "flag=${GREN_NONINTERACTIVE:-unset}"`, ctx, "", false); !strings.Contains(got.Output, "flag=unset") {
		t.Errorf("auto mode should not set GREN_NONINTERACTIVE, got %q", got.Output)
	}

	wm.SetHookInteractivity(HookInteractivityNever)
	if got := wm.executeHook(config.HookPreRemove, `echo "flag=${GREN_NONINTERACTIVE:-unset}"`, ctx, "", false); !strings.Contains(got.Output, "flag=1") {
		t.Errorf("never mode should set GREN_NONINTERACTIVE=1, got %q", got.Output)
	}
}

// A caller that explicitly asked for a TTY (`gren hook-run --interactive`) but
// has no terminal on stdin still gets a pty, since that is what makes tools
// like `op` and colored output work. It must not hang: with no input source,
// the child's read sees EOF instead of waiting forever.
func TestExecuteHook_ForcedPtyWithoutTerminalDoesNotHang(t *testing.T) {
	repo := mkRepo(t)
	t.Setenv("GREN_LOG_DIR", t.TempDir())
	ctx := HookContext{WorktreePath: repo, BranchName: "main", RepoRoot: repo}

	wm := &WorktreeManager{}
	wm.SetHookInteractivity(HookInteractivityForce)

	got := runHookWithDeadline(t, wm, ctx, promptingHook, false)
	if !strings.Contains(got.Output, "Drop databases?") {
		t.Fatalf("expected the forced pty to make the hook prompt, got %q", got.Output)
	}
	if !strings.Contains(got.Output, "answered:") {
		t.Errorf("expected the prompt to read EOF and continue, got %q", got.Output)
	}
}

// An inherited GREN_NONINTERACTIVE belongs to whoever exported it. A hook that
// shells back into gren must not pass its own answer down to a hook that does
// get a terminal, so the variable is rebuilt per run rather than inherited.
func TestExecuteHook_InheritedNonInteractiveEnvIsNotPassedThrough(t *testing.T) {
	repo := mkRepo(t)
	t.Setenv("GREN_LOG_DIR", t.TempDir())
	t.Setenv("GREN_NONINTERACTIVE", "1")
	ctx := HookContext{WorktreePath: repo, BranchName: "main", RepoRoot: repo}

	for _, tc := range []struct {
		mode HookInteractivity
		want string
	}{
		{HookInteractivityAuto, "flag=unset"},
		{HookInteractivityForce, "flag=unset"},
		{HookInteractivityNever, "flag=1"},
	} {
		wm := &WorktreeManager{}
		wm.SetHookInteractivity(tc.mode)
		got := runHookWithDeadline(t, wm, ctx, `echo "flag=${GREN_NONINTERACTIVE:-unset}"`, false)
		if !strings.Contains(got.Output, tc.want) {
			t.Errorf("mode %s: want %q, got %q", tc.mode, tc.want, got.Output)
		}
	}
}
