package output

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout captures stdout during function execution
func captureStdout(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// captureStderr captures stderr during function execution
func captureStderr(f func()) string {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	f()

	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestSymbols(t *testing.T) {
	symbols := map[string]string{
		"SymbolSuccess":  SymbolSuccess,
		"SymbolError":    SymbolError,
		"SymbolWarning":  SymbolWarning,
		"SymbolProgress": SymbolProgress,
		"SymbolHint":     SymbolHint,
		"SymbolInfo":     SymbolInfo,
		"SymbolBranch":   SymbolBranch,
		"SymbolFolder":   SymbolFolder,
		"SymbolTree":     SymbolTree,
	}

	for name, symbol := range symbols {
		if symbol == "" {
			t.Errorf("%s should not be empty", name)
		}
	}
}

func TestSuccess(t *testing.T) {
	output := captureStdout(func() {
		Success("test success message")
	})

	if !strings.Contains(output, "test success message") {
		t.Errorf("Success() output should contain message, got: %s", output)
	}
}

func TestSuccessf(t *testing.T) {
	output := captureStdout(func() {
		Successf("formatted %s %d", "test", 42)
	})

	if !strings.Contains(output, "formatted test 42") {
		t.Errorf("Successf() output should contain formatted message, got: %s", output)
	}
}

func TestError(t *testing.T) {
	output := captureStderr(func() {
		Error("test error message")
	})

	if !strings.Contains(output, "test error message") {
		t.Errorf("Error() output should contain message, got: %s", output)
	}
}

func TestErrorf(t *testing.T) {
	output := captureStderr(func() {
		Errorf("error: %s", "something went wrong")
	})

	if !strings.Contains(output, "error: something went wrong") {
		t.Errorf("Errorf() output should contain formatted message, got: %s", output)
	}
}

func TestWarning(t *testing.T) {
	output := captureStdout(func() {
		Warning("test warning message")
	})

	if !strings.Contains(output, "test warning message") {
		t.Errorf("Warning() output should contain message, got: %s", output)
	}
}

func TestWarningf(t *testing.T) {
	output := captureStdout(func() {
		Warningf("warning: %s", "be careful")
	})

	if !strings.Contains(output, "warning: be careful") {
		t.Errorf("Warningf() output should contain formatted message, got: %s", output)
	}
}

func TestProgress(t *testing.T) {
	output := captureStdout(func() {
		Progress("loading...")
	})

	if !strings.Contains(output, "loading...") {
		t.Errorf("Progress() output should contain message, got: %s", output)
	}
}

func TestProgressf(t *testing.T) {
	output := captureStdout(func() {
		Progressf("step %d of %d", 1, 3)
	})

	if !strings.Contains(output, "step 1 of 3") {
		t.Errorf("Progressf() output should contain formatted message, got: %s", output)
	}
}

func TestHint(t *testing.T) {
	output := captureStdout(func() {
		Hint("try using --help")
	})

	if !strings.Contains(output, "try using --help") {
		t.Errorf("Hint() output should contain message, got: %s", output)
	}
}

func TestHintf(t *testing.T) {
	output := captureStdout(func() {
		Hintf("run %s to continue", "gren init")
	})

	if !strings.Contains(output, "run gren init to continue") {
		t.Errorf("Hintf() output should contain formatted message, got: %s", output)
	}
}

func TestInfo(t *testing.T) {
	output := captureStdout(func() {
		Info("general info")
	})

	if !strings.Contains(output, "general info") {
		t.Errorf("Info() output should contain message, got: %s", output)
	}
}

func TestInfof(t *testing.T) {
	output := captureStdout(func() {
		Infof("count: %d", 10)
	})

	if !strings.Contains(output, "count: 10") {
		t.Errorf("Infof() output should contain formatted message, got: %s", output)
	}
}

func TestHeader(t *testing.T) {
	output := captureStdout(func() {
		Header("Section Title")
	})

	if !strings.Contains(output, "Section Title") {
		t.Errorf("Header() output should contain title, got: %s", output)
	}
}

func TestTextStyles(t *testing.T) {
	tests := []struct {
		name     string
		fn       func(string) string
		input    string
		expected string
	}{
		{"Bold", Bold, "test", "test"},
		{"Dim", Dim, "test", "test"},
		{"Cyan", Cyan, "test", "test"},
		{"Green", Green, "test", "test"},
		{"Yellow", Yellow, "test", "test"},
		{"Red", Red, "test", "test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.fn(tt.input)
			// The styled output should contain the original text
			if !strings.Contains(result, tt.expected) {
				t.Errorf("%s() result should contain %q, got: %s", tt.name, tt.expected, result)
			}
		})
	}
}

func TestPath(t *testing.T) {
	// Test with a non-home path
	result := Path("/tmp/test/path")
	if !strings.Contains(result, "/tmp/test/path") {
		t.Errorf("Path() should contain the path, got: %s", result)
	}

	// Test with home path (if available)
	home, err := os.UserHomeDir()
	if err == nil {
		homePath := home + "/Documents/test"
		result := Path(homePath)
		if !strings.Contains(result, "~") {
			t.Errorf("Path() should shorten home dir to ~, got: %s", result)
		}
	}
}

func TestBranch(t *testing.T) {
	result := Branch("feature/test")
	if !strings.Contains(result, "feature/test") {
		t.Errorf("Branch() should contain branch name, got: %s", result)
	}
}

func TestKeyValue(t *testing.T) {
	output := captureStdout(func() {
		KeyValue("Name", "test-value")
	})

	if !strings.Contains(output, "Name") {
		t.Errorf("KeyValue() should contain key, got: %s", output)
	}
	if !strings.Contains(output, "test-value") {
		t.Errorf("KeyValue() should contain value, got: %s", output)
	}
}

func TestListItem(t *testing.T) {
	t.Run("non-current item", func(t *testing.T) {
		output := captureStdout(func() {
			ListItem("regular item", false)
		})
		if !strings.Contains(output, "regular item") {
			t.Errorf("ListItem() should contain text, got: %s", output)
		}
	})

	t.Run("current item", func(t *testing.T) {
		output := captureStdout(func() {
			ListItem("current item", true)
		})
		if !strings.Contains(output, "current item") {
			t.Errorf("ListItem() should contain text, got: %s", output)
		}
	})
}

func TestBlank(t *testing.T) {
	output := captureStdout(func() {
		Blank()
	})
	if output != "\n" {
		t.Errorf("Blank() should output newline, got: %q", output)
	}
}

func TestWorktreeHeader(t *testing.T) {
	output := captureStdout(func() {
		WorktreeHeader("my-project")
	})

	if !strings.Contains(output, "Git Worktree Manager") {
		t.Errorf("WorktreeHeader() should contain title, got: %s", output)
	}
	if !strings.Contains(output, "my-project") {
		t.Errorf("WorktreeHeader() should contain project name, got: %s", output)
	}
}

func TestWorktreeCreated(t *testing.T) {
	output := captureStdout(func() {
		WorktreeCreated("feature-test", "feature-test", "/path/to/worktree")
	})

	if !strings.Contains(output, "feature-test") {
		t.Errorf("WorktreeCreated() should contain branch name, got: %s", output)
	}
}

func TestWorktreeSwitched(t *testing.T) {
	output := captureStdout(func() {
		WorktreeSwitched("feature-test", "/path/to/worktree")
	})

	if !strings.Contains(output, "feature-test") {
		t.Errorf("WorktreeSwitched() should contain name, got: %s", output)
	}
}

func TestWorktreeRemoved(t *testing.T) {
	output := captureStdout(func() {
		WorktreeRemoved("old-feature")
	})

	if !strings.Contains(output, "old-feature") {
		t.Errorf("WorktreeRemoved() should contain name, got: %s", output)
	}
}

func TestPrintWorktreeList(t *testing.T) {
	items := []WorktreeListItem{
		{
			Name:      "main",
			Branch:    "main",
			Path:      "/path/to/main",
			IsCurrent: true,
			IsMain:    true,
		},
		{
			Name:      "feature-test",
			Branch:    "feature/test",
			Path:      "/path/to/feature",
			IsCurrent: false,
			IsMain:    false,
			Status:    "modified",
		},
	}

	output := captureStdout(func() {
		PrintWorktreeList(items, "test-repo")
	})

	if !strings.Contains(output, "main") {
		t.Errorf("PrintWorktreeList() should contain main worktree, got: %s", output)
	}
	if !strings.Contains(output, "feature-test") {
		t.Errorf("PrintWorktreeList() should contain feature worktree, got: %s", output)
	}
}

func TestPrintSimpleWorktreeList(t *testing.T) {
	items := []WorktreeListItem{
		{
			Name:      "main",
			IsCurrent: true,
		},
		{
			Name:      "feature",
			IsCurrent: false,
			StaleInfo: "merged",
		},
		{
			Name:      "ci-pass",
			IsCurrent: false,
			CIStatus:  "success",
		},
		{
			Name:      "ci-fail",
			IsCurrent: false,
			CIStatus:  "failure",
		},
		{
			Name:      "ci-pending",
			IsCurrent: false,
			CIStatus:  "pending",
		},
	}

	output := captureStdout(func() {
		PrintSimpleWorktreeList(items)
	})

	if !strings.Contains(output, "main") {
		t.Errorf("PrintSimpleWorktreeList() should contain main, got: %s", output)
	}
	if !strings.Contains(output, "feature") {
		t.Errorf("PrintSimpleWorktreeList() should contain feature, got: %s", output)
	}
	if !strings.Contains(output, "merged") {
		t.Errorf("PrintSimpleWorktreeList() should contain stale info, got: %s", output)
	}
}

func TestRepoName(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/home/user/projects/my-repo", "my-repo"},
		{"/var/git/awesome-project", "awesome-project"},
		{"/tmp/test", "test"},
		{"simple", "simple"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := RepoName(tt.path)
			if result != tt.expected {
				t.Errorf("RepoName(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}
