package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorktreeBlockingContent(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	git("add", "tracked.txt")
	git("-c", "commit.gpgsign=false", "commit", "-m", "init")

	if got := worktreeBlockingContent(dir); len(got) != 0 {
		t.Errorf("clean worktree should have no blocking content, got %v", got)
	}

	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("node_modules/\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "node_modules"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "node_modules", "pkg"), []byte("y"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("z"), 0644); err != nil {
		t.Fatal(err)
	}

	joined := strings.Join(worktreeBlockingContent(dir), "\n")
	if !strings.Contains(joined, "node_modules/") {
		t.Errorf("blocking content should list ignored node_modules/, got %q", joined)
	}
	if !strings.Contains(joined, "untracked.txt") {
		t.Errorf("blocking content should list untracked.txt, got %q", joined)
	}
}

func TestSplitBlockingContent(t *testing.T) {
	real, ignored := splitBlockingContent([]string{
		"!! node_modules/",
		"?? untracked.txt",
		" M modified.txt",
		"!! .venv/",
	})
	if len(real) != 2 || real[0] != "?? untracked.txt" || real[1] != " M modified.txt" {
		t.Errorf("real = %v, want untracked + modified entries", real)
	}
	if len(ignored) != 2 || ignored[0] != "!! node_modules/" || ignored[1] != "!! .venv/" {
		t.Errorf("ignored = %v, want the two !! entries", ignored)
	}

	real, ignored = splitBlockingContent([]string{"!! .env", "!! dist/"})
	if len(real) != 0 || len(ignored) != 2 {
		t.Errorf("ignored-only input: real = %v, ignored = %v", real, ignored)
	}
}

func TestCapList(t *testing.T) {
	if got := capList([]string{"a", "b"}, 5); len(got) != 2 {
		t.Errorf("capList within limit = %v, want unchanged", got)
	}
	got := capList([]string{"a", "b", "c", "d"}, 2)
	if len(got) != 3 || got[2] != "… and 2 more" {
		t.Errorf("capList over limit = %v, want [a b '… and 2 more']", got)
	}
}
