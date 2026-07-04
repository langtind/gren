package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func gitProjectIDCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
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

// GetProjectID must be stable across all worktrees of the same repo, so a hook
// approved once (e.g. in a herdr bootstrap pane) isn't re-prompted for every new
// worktree. The old implementation returned the current directory, so each
// worktree produced a different ID and approvals never persisted.
func TestGetProjectID_StableAcrossWorktrees(t *testing.T) {
	main := t.TempDir()
	gitProjectIDCmd(t, main, "init", "-b", "main")
	gitProjectIDCmd(t, main, "-c", "commit.gpgsign=false", "commit", "--allow-empty", "-m", "init")

	wt := filepath.Join(t.TempDir(), "wt")
	gitProjectIDCmd(t, main, "worktree", "add", "-b", "feat/x", wt)

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig)

	if err := os.Chdir(main); err != nil {
		t.Fatal(err)
	}
	idMain, err := GetProjectID()
	if err != nil {
		t.Fatal(err)
	}

	if err := os.Chdir(wt); err != nil {
		t.Fatal(err)
	}
	idWt, err := GetProjectID()
	if err != nil {
		t.Fatal(err)
	}

	if idMain != idWt {
		t.Errorf("project ID must be identical from the main checkout and a linked worktree so approvals persist; main=%q worktree=%q", idMain, idWt)
	}
}
