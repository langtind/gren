package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/langtind/gren/internal/config"
	"github.com/langtind/gren/internal/core"
	"github.com/langtind/gren/internal/events"
	"github.com/langtind/gren/internal/git"
)

func TestParseFormat(t *testing.T) {
	tests := []struct {
		format   string
		wantJSON bool
		wantErr  bool
	}{
		{"", false, false},
		{"json", true, false},
		{"JSON", false, true},  // case-sensitive on purpose: no silent near-misses
		{"yaml", false, true},  // unsupported must fail loudly, not fall back to prose
		{"table", false, true}, // ditto
	}
	for _, tt := range tests {
		got, err := parseFormat(tt.format)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseFormat(%q) error = %v, wantErr %v", tt.format, err, tt.wantErr)
		}
		if got != tt.wantJSON {
			t.Errorf("parseFormat(%q) = %v, want %v", tt.format, got, tt.wantJSON)
		}
	}
}

// TestPrintHookEventsWritesToHumanOut guards the mechanism that keeps JSON
// stdout clean. Hook phase summaries are the largest chunk of prose gren emits
// during a delete, and they used to go to stdout unconditionally.
func TestPrintHookEventsWritesToHumanOut(t *testing.T) {
	var buf bytes.Buffer
	prev := humanOutOverride
	humanOutOverride = &buf
	defer func() { humanOutOverride = prev }()

	results := []core.HookResult{{
		Name:   "setup",
		Ran:    true,
		Events: []events.Event{{Phase: "install", Status: events.StatusOK}},
	}}

	stdout := captureStdout(t, func() { printHookEvents(results) })

	if stdout != "" {
		t.Errorf("printHookEvents wrote to stdout: %q — this corrupts --format=json payloads", stdout)
	}
	if !bytes.Contains(buf.Bytes(), []byte("install")) {
		t.Errorf("printHookEvents did not write the phase to humanOut, got %q", buf.String())
	}
}

// deleteJSONRepo builds a repo with one worktree and returns the repo dir, the
// worktree name, and its path.
func deleteJSONRepo(t *testing.T, name string) (repoDir, worktreePath string) {
	t.Helper()
	dir, cleanup := setupTempGitRepoWithCleanWorktrees(t)
	t.Cleanup(cleanup)

	originalDir, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(originalDir) })
	os.Chdir(dir)

	config.Initialize(filepath.Base(dir), true)
	cli := NewCLI(git.NewLocalRepository(), config.NewManager())

	out := captureStdout(t, func() {
		if err := cli.ParseAndExecute([]string{"gren", "create", "-n", name, "--no-hooks", "-y", "--format=json"}); err != nil {
			t.Fatalf("create %s failed: %v", name, err)
		}
	})
	var created CreateJSON
	if err := json.Unmarshal([]byte(out), &created); err != nil {
		t.Fatalf("parse create JSON %q: %v", out, err)
	}
	return dir, created.Path
}

// runDeleteJSON runs `gren delete` with the given args and returns the parsed
// payload plus whether the command errored. It fails the test if stdout is not
// pure JSON — the invariant every one of these cases has to hold.
func runDeleteJSON(t *testing.T, args ...string) (DeleteJSON, bool) {
	t.Helper()
	cli := NewCLI(git.NewLocalRepository(), config.NewManager())

	var cmdErr error
	stdout := captureStdout(t, func() {
		captureStderr(t, func() {
			cmdErr = cli.ParseAndExecute(append([]string{"gren", "delete"}, args...))
		})
	})

	var result DeleteJSON
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("delete --format=json stdout must be pure JSON, got parse error %v\nstdout: %q", err, stdout)
	}
	return result, cmdErr != nil
}

// TestDeleteJSONDryRunReportsBlockingContent is the case that makes this flag
// worth having: an agent asking "is this worktree safe to remove?" gets a
// structured answer instead of having to reimplement the check with
// `git status --porcelain`, and nothing is touched.
func TestDeleteJSONDryRunReportsBlockingContent(t *testing.T) {
	_, worktreePath := deleteJSONRepo(t, "dry-run-blocked")

	// An untracked file is exactly what should stop an unattended delete.
	if err := os.WriteFile(filepath.Join(worktreePath, "scratch.txt"), []byte("work in progress\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result, errored := runDeleteJSON(t, "--dry-run", "--format=json", "dry-run-blocked")

	if errored {
		t.Errorf("a dry run must not fail; it is an inspection")
	}
	if result.Deleted {
		t.Errorf("dry run reported deleted=true")
	}
	if result.Reason != DeleteReasonDryRun {
		t.Errorf("reason = %q, want %q", result.Reason, DeleteReasonDryRun)
	}
	if result.Blocking == nil || len(result.Blocking.Tracked) == 0 {
		t.Fatalf("untracked file was not reported as blocking: %+v", result.Blocking)
	}
	var found bool
	for _, e := range result.Blocking.Tracked {
		if e.Path == "scratch.txt" {
			found = true
			if e.Status != "??" {
				t.Errorf("status = %q, want %q for an untracked file", e.Status, "??")
			}
		}
		if strings.HasPrefix(e.Path, "?") {
			t.Errorf("path %q still carries the porcelain prefix; status belongs in .status", e.Path)
		}
	}
	if !found {
		t.Errorf("scratch.txt missing from blocking.tracked: %+v", result.Blocking.Tracked)
	}
	if !result.WouldForce {
		t.Errorf("would_force should be true when content blocks removal")
	}
	if _, err := os.Stat(worktreePath); err != nil {
		t.Errorf("dry run removed the worktree: %v", err)
	}
}

// TestDeleteJSONWithoutForceRefuses locks in that JSON mode never prompts.
// Its callers — plugins, agents, CI — cannot answer a y/N, and a prompt that
// nobody answers is a hang, not a safety feature.
func TestDeleteJSONWithoutForceRefuses(t *testing.T) {
	_, worktreePath := deleteJSONRepo(t, "no-force")

	result, errored := runDeleteJSON(t, "--format=json", "no-force")

	if !errored {
		t.Errorf("delete --format=json without -f must exit non-zero")
	}
	if result.Deleted {
		t.Errorf("deleted=true without -f")
	}
	if result.Reason != DeleteReasonConfirmationRequired {
		t.Errorf("reason = %q, want %q", result.Reason, DeleteReasonConfirmationRequired)
	}
	if _, err := os.Stat(worktreePath); err != nil {
		t.Errorf("worktree was removed despite the refusal: %v", err)
	}
}

// TestDeleteJSONForceDeletes covers the happy path and the branch_kept promise.
func TestDeleteJSONForceDeletes(t *testing.T) {
	_, worktreePath := deleteJSONRepo(t, "force-delete")

	result, errored := runDeleteJSON(t, "-f", "--format=json", "force-delete")

	if errored {
		t.Fatalf("delete -f --format=json failed")
	}
	if !result.Deleted {
		t.Errorf("deleted=false after a forced delete: %+v", result)
	}
	if !result.BranchKept {
		t.Errorf("branch_kept must be true — gren preserves the branch by design")
	}
	if result.Path == "" {
		t.Errorf("path is empty; callers need it to close the matching herdr workspace")
	}
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Errorf("worktree still on disk after deleted=true: %v", err)
	}
}

// TestDeleteJSONNotFound checks the closed reason vocabulary holds for the
// case a caller hits most often after a manual cleanup.
func TestDeleteJSONNotFound(t *testing.T) {
	deleteJSONRepo(t, "exists")

	result, errored := runDeleteJSON(t, "-f", "--format=json", "no-such-worktree")

	if !errored {
		t.Errorf("deleting a missing worktree must exit non-zero")
	}
	if result.Reason != DeleteReasonNotFound {
		t.Errorf("reason = %q, want %q", result.Reason, DeleteReasonNotFound)
	}
}

// hookRunJSONRepo builds a repo whose post-create hook is the given shell
// command, then returns the repo dir and a created worktree path.
func hookRunJSONRepo(t *testing.T, name, hookCmd string) (repoDir, worktreePath string) {
	t.Helper()
	dir, worktreePath := deleteJSONRepo(t, name)

	cfg := "version = \"" + config.CurrentConfigVersion + "\"\n" +
		"worktree_dir = \"" + filepath.Join(filepath.Dir(dir), filepath.Base(dir)+"-worktrees") + "\"\n" +
		"[hooks]\npost-create = \"" + hookCmd + "\"\n"
	if err := os.WriteFile(filepath.Join(dir, ".gren", "config.toml"), []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	return dir, worktreePath
}

// runHookRunJSON runs `gren hook-run --format=json` and asserts stdout parses.
func runHookRunJSON(t *testing.T, args ...string) (HookRunJSON, bool) {
	t.Helper()
	cli := NewCLI(git.NewLocalRepository(), config.NewManager())

	var cmdErr error
	stdout := captureStdout(t, func() {
		captureStderr(t, func() {
			cmdErr = cli.ParseAndExecute(append([]string{"gren", "hook-run"}, args...))
		})
	})

	var result HookRunJSON
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("hook-run --format=json stdout must be pure JSON, got parse error %v\nstdout: %q", err, stdout)
	}
	return result, cmdErr != nil
}

// TestHookRunJSONReportsSuccess covers the call herdr's bootstrap pane makes.
// Today it learns only an exit code; with --format=json it can report which
// hook ran and what it printed.
func TestHookRunJSONReportsSuccess(t *testing.T) {
	_, worktreePath := hookRunJSONRepo(t, "hook-ok", "echo setup-done")

	result, errored := runHookRunJSON(t, "--type", "post-create", "--path", worktreePath,
		"--branch", "hook-ok", "--format=json")

	if errored {
		t.Fatalf("a succeeding hook must exit zero: %+v", result)
	}
	if !result.Ok {
		t.Errorf("ok=false for a succeeding hook: %+v", result)
	}
	if !result.Ran {
		t.Errorf("ran=false although a hook is configured")
	}
	if result.Type != "post-create" {
		t.Errorf("type = %q, want post-create", result.Type)
	}
}

// TestHookRunJSONReportsFailure is the case that matters in a pane nobody is
// watching: the payload has to carry the reason, and the exit code still has
// to be non-zero so shell callers behave unchanged.
func TestHookRunJSONReportsFailure(t *testing.T) {
	_, worktreePath := hookRunJSONRepo(t, "hook-fail", "exit 3")

	result, errored := runHookRunJSON(t, "--type", "post-create", "--path", worktreePath,
		"--branch", "hook-fail", "--format=json")

	if !errored {
		t.Errorf("a failing hook must still exit non-zero in JSON mode")
	}
	if result.Ok {
		t.Errorf("ok=true for a failing hook: %+v", result)
	}
	if result.Error == "" {
		t.Errorf("error is empty; the payload must say why the hook failed")
	}
	if len(result.Hooks) == 0 {
		t.Errorf("hooks[] is empty; per-hook detail is the point of the flag")
	}
}

// TestHookRunJSONUnknownTypeIsStructured keeps the error path parseable too —
// a caller should never have to switch between "parse JSON" and "read prose"
// depending on whether its arguments were right.
func TestHookRunJSONUnknownTypeIsStructured(t *testing.T) {
	_, worktreePath := deleteJSONRepo(t, "hook-unknown")

	result, errored := runHookRunJSON(t, "--type", "post-nonsense", "--path", worktreePath,
		"--branch", "hook-unknown", "--format=json")

	if !errored {
		t.Errorf("an unknown hook type must exit non-zero")
	}
	if result.Error == "" {
		t.Errorf("error is empty for an unknown hook type")
	}
}
