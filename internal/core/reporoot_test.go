package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// getRepoRoot must resolve to the MAIN worktree, not the current linked
// worktree, so hook RepoRoot / repo_root / worktree_dir resolve against the
// main checkout even when gren runs from inside a worktree — e.g. `gren
// hook-run` invoked by the herdr plugin from the worktree's own pane. Without
// this, a post-create hook's $REPO_ROOT points at the worktree and shared
// files there (a gitignored .env in the main checkout) can't be found.
func TestGetRepoRoot_FromLinkedWorktree(t *testing.T) {
	main := mkRepo(t)
	wt := filepath.Join(t.TempDir(), "feat-x")

	add := exec.Command("git", "worktree", "add", "-b", "feat/x", wt)
	add.Dir = main
	if out, err := add.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add: %v\n%s", err, out)
	}

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origWd)
	if err := os.Chdir(wt); err != nil {
		t.Fatal(err)
	}

	wm := &WorktreeManager{}
	got, err := wm.getRepoRoot()
	if err != nil {
		t.Fatalf("getRepoRoot: %v", err)
	}

	wantReal, _ := filepath.EvalSymlinks(main)
	gotReal, _ := filepath.EvalSymlinks(got)
	if gotReal != wantReal {
		t.Errorf("getRepoRoot from linked worktree = %q (real %q), want main repo %q (real %q)",
			got, gotReal, main, wantReal)
	}
}
