package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/langtind/gren/internal/config"
)

// TestExecuteHook_CapturesStdoutAndStderrSeparately verifies the split
// introduced for the hook error protocol: previously CombinedOutput merged
// the two, which meant a failure trace on stderr was buried inside normal
// progress on stdout. They must land in distinct fields now.
func TestExecuteHook_CapturesStdoutAndStderrSeparately(t *testing.T) {
	repo := mkRepo(t)
	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	t.Setenv("HOME", stateDir)

	script := filepath.Join(repo, "hook.sh")
	body := `#!/bin/sh
echo "progress line"
echo "failure trace" >&2
exit 1
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	wm := &WorktreeManager{}
	ctx := HookContext{WorktreePath: repo, BranchName: "main", RepoRoot: repo}
	result := wm.executeHook(config.HookPostCreate, "./hook.sh", ctx, "", false)

	if result.Err == nil {
		t.Fatal("expected non-zero exit to produce Err")
	}
	if !strings.Contains(result.Output, "progress line") {
		t.Errorf("stdout not captured in Output: %q", result.Output)
	}
	if strings.Contains(result.Output, "failure trace") {
		t.Errorf("stderr leaked into Output (should be in Stderr): %q", result.Output)
	}
	if !strings.Contains(result.Stderr, "failure trace") {
		t.Errorf("stderr not captured in Stderr: %q", result.Stderr)
	}
	if strings.Contains(result.Stderr, "progress line") {
		t.Errorf("stdout leaked into Stderr: %q", result.Stderr)
	}
}

// TestExecuteHook_BashErrorLandsInStderr mirrors the flyt repro: a bash
// parse-time failure (bad substitution) must land in Stderr so the UI can
// show it as the failure cause instead of silently swallowing it.
func TestExecuteHook_BashErrorLandsInStderr(t *testing.T) {
	repo := mkRepo(t)
	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	t.Setenv("HOME", stateDir)

	script := filepath.Join(repo, "hook.sh")
	// ${var^^} is bash-only; sh shebang treats it as a syntax error.
	body := `#!/bin/sh
app=jobb
echo "${app^^}_DB"
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	wm := &WorktreeManager{}
	ctx := HookContext{WorktreePath: repo, BranchName: "main", RepoRoot: repo}
	result := wm.executeHook(config.HookPostCreate, "./hook.sh", ctx, "", false)

	if result.Err == nil {
		t.Fatal("expected hook to fail on bash-ism")
	}
	if !strings.Contains(result.Stderr, "bad substitution") {
		t.Errorf("expected 'bad substitution' trace in Stderr, got stdout=%q stderr=%q",
			result.Output, result.Stderr)
	}
}
