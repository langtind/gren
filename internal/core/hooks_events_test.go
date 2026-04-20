package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/langtind/gren/internal/config"
)

// mkRepo initializes a git repo in a fresh tempdir and returns its path.
func mkRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"commit", "--allow-empty", "-m", "initial"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func TestExecuteHook_CollectsEvents(t *testing.T) {
	repo := mkRepo(t)
	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	// Also override HOME for darwin where EventsDir uses ~/Library/...
	t.Setenv("HOME", stateDir)

	script := filepath.Join(repo, "hook.sh")
	body := `#!/bin/sh
set -e
emit() { printf '{"ts":"%s","phase":"%s","status":"%s"}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$1" "$2" >> "$GREN_EVENTS_FILE"; }
emit install start
sleep 0.05
emit install ok
emit migrate start
sleep 0.05
emit migrate ok
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	wm := &WorktreeManager{}
	ctx := HookContext{
		WorktreePath: repo,
		BranchName:   "main",
		RepoRoot:     repo,
	}
	result := wm.executeHook(config.HookPostCreate, "./hook.sh", ctx, "", false)

	if result.Err != nil {
		t.Fatalf("hook failed: %v, output: %s", result.Err, result.Output)
	}
	if len(result.Events) != 4 {
		t.Fatalf("expected 4 events, got %d: %+v", len(result.Events), result.Events)
	}
	phases := []string{}
	for _, e := range result.Events {
		phases = append(phases, e.Phase+"/"+string(e.Status))
	}
	want := "install/start,install/ok,migrate/start,migrate/ok"
	if strings.Join(phases, ",") != want {
		t.Errorf("unexpected phase order: %s", strings.Join(phases, ","))
	}
	if result.EventsFile == "" {
		t.Errorf("expected EventsFile to be set on result")
	}
	if _, err := os.Stat(result.EventsFile); err != nil {
		t.Errorf("events file missing: %v", err)
	}
}

func TestExecuteHook_NoEventsForSilentHook(t *testing.T) {
	// A hook that writes no events should still succeed; Events should be empty
	// but EventsFile should be created (can be empty).
	repo := mkRepo(t)
	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	// Also override HOME for darwin where EventsDir uses ~/Library/...
	t.Setenv("HOME", stateDir)

	script := filepath.Join(repo, "hook.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho hello\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	wm := &WorktreeManager{}
	ctx := HookContext{WorktreePath: repo, BranchName: "main", RepoRoot: repo}
	result := wm.executeHook(config.HookPostCreate, "./hook.sh", ctx, "", false)

	if result.Err != nil {
		t.Fatalf("unexpected hook failure: %v", result.Err)
	}
	if !strings.Contains(result.Output, "hello") {
		t.Errorf("expected stdout preserved, got: %s", result.Output)
	}
	if len(result.Events) != 0 {
		t.Errorf("expected 0 events for silent hook, got %d", len(result.Events))
	}
	if result.EventsFile == "" {
		t.Errorf("expected EventsFile path set even without events")
	}
}
