package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/langtind/gren/internal/testutil"
)

// TestE2E_WorktreeLifecycle tests the complete lifecycle of a worktree:
// init -> create -> use -> delete
func TestE2E_WorktreeLifecycle(t *testing.T) {
	h := testutil.NewE2EHarness(t)
	defer h.Cleanup()

	// Step 1: Initialize gren
	t.Run("init", func(t *testing.T) {
		result := h.Init()
		result.AssertSuccess(t)

		// Verify .gren directory was created
		if !h.FileExists(".gren") {
			t.Error(".gren directory was not created")
		}

		// Verify config file was created
		if !h.FileExists(".gren/config.toml") {
			t.Error(".gren/config.toml was not created")
		}
	})

	// Step 2: Create a worktree
	var worktreeName = "feature-test"
	t.Run("create worktree", func(t *testing.T) {
		result := h.CreateWorktree(worktreeName)
		result.AssertSuccess(t)

		// Verify worktree was created
		if !h.WorktreeExists(worktreeName) {
			t.Errorf("worktree directory was not created at %s", h.GetWorktreePath(worktreeName))
		}

		// Verify it appears in list
		listResult := h.List()
		listResult.AssertSuccess(t)
		listResult.AssertStdoutContains(t, worktreeName)
	})

	// Step 3: Make changes in the worktree
	t.Run("modify worktree", func(t *testing.T) {
		err := h.WriteFileInWorktree(worktreeName, "new-file.txt", "new content")
		if err != nil {
			t.Fatalf("failed to create file in worktree: %v", err)
		}

		// Verify git detects the change
		worktreePath := h.GetWorktreePath(worktreeName)
		result := h.GitInDir(worktreePath, "status", "--porcelain")
		result.AssertSuccess(t)
		result.AssertStdoutContains(t, "new-file.txt")
	})

	// Step 4: Delete the worktree
	t.Run("delete worktree", func(t *testing.T) {
		// Force delete because we have uncommitted changes
		result := h.DeleteWorktree(worktreeName, true)
		result.AssertSuccess(t)

		// Verify worktree was deleted
		if h.WorktreeExists(worktreeName) {
			t.Error("worktree directory still exists after deletion")
		}

		// Verify it doesn't appear in list
		listResult := h.List()
		listResult.AssertSuccess(t)
		listResult.AssertStdoutNotContains(t, worktreeName)
	})
}

// TestE2E_InitWorkflow tests the init command with various scenarios.
func TestE2E_InitWorkflow(t *testing.T) {
	t.Run("basic init", func(t *testing.T) {
		h := testutil.NewE2EHarness(t)
		defer h.Cleanup()

		result := h.Init()
		result.AssertSuccess(t)

		if !h.FileExists(".gren/config.toml") {
			t.Error("config.toml was not created")
		}
	})

	t.Run("init with custom project name", func(t *testing.T) {
		h := testutil.NewE2EHarness(t)
		defer h.Cleanup()

		result := h.InitWithProject("my-custom-project")
		result.AssertSuccess(t)

		// Read config and verify project name is used
		content, err := h.ReadFile(".gren/config.toml")
		if err != nil {
			t.Fatalf("failed to read config: %v", err)
		}

		if !strings.Contains(content, "my-custom-project") {
			t.Errorf("config should contain custom project name, got:\n%s", content)
		}
	})

	t.Run("re-init preserves config", func(t *testing.T) {
		h := testutil.NewE2EHarness(t)
		defer h.Cleanup()

		// Initial init
		h.Init()

		// Modify config somehow (we'll just verify it still exists after re-init)
		originalContent, _ := h.ReadFile(".gren/config.toml")

		// Re-init
		result := h.Init()
		result.AssertSuccess(t)

		// Config should still exist
		newContent, err := h.ReadFile(".gren/config.toml")
		if err != nil {
			t.Fatalf("failed to read config after re-init: %v", err)
		}

		// Config should not be empty
		if len(newContent) == 0 {
			t.Error("config is empty after re-init")
		}

		// The structure should be preserved (at least not completely different)
		if len(originalContent) > 0 && len(newContent) == 0 {
			t.Error("config was lost during re-init")
		}
	})

	t.Run("init from subdirectory", func(t *testing.T) {
		h := testutil.NewE2EHarness(t)
		defer h.Cleanup()

		// Create a subdirectory
		subdir := filepath.Join(h.RepoPath(), "subdir")
		os.MkdirAll(subdir, 0755)

		// Run init from subdirectory
		result := h.RunInDir(subdir, "init")
		result.AssertSuccess(t)

		// Config should be created at repo root, not in subdir
		if !h.FileExists(".gren/config.toml") {
			t.Error("config should be created at repo root")
		}

		// Config should NOT be in subdir
		subdirConfig := filepath.Join(subdir, ".gren", "config.toml")
		if _, err := os.Stat(subdirConfig); err == nil {
			t.Error("config should not be created in subdirectory")
		}
	})
}

// TestE2E_CreateWorktreeScenarios tests various worktree creation scenarios.
func TestE2E_CreateWorktreeScenarios(t *testing.T) {
	t.Run("create with new branch", func(t *testing.T) {
		h := testutil.NewE2EHarness(t)
		defer h.Cleanup()

		h.Init()

		result := h.CreateWorktree("feature-new-branch")
		result.AssertSuccess(t)

		// Verify branch was created
		gitResult := h.Git("branch", "--list", "feature-new-branch")
		gitResult.AssertSuccess(t)
		gitResult.AssertStdoutContains(t, "feature-new-branch")
	})

	t.Run("create from existing branch", func(t *testing.T) {
		h := testutil.NewE2EHarness(t)
		defer h.Cleanup()

		h.Init()

		// Create a branch first
		h.CreateBranch("existing-branch")

		// Create worktree from existing branch
		result := h.CreateWorktreeFromBranch("wt-existing", "existing-branch")
		result.AssertSuccess(t)

		if !h.WorktreeExists("wt-existing") {
			t.Error("worktree was not created")
		}
	})

	t.Run("create with slashes in name", func(t *testing.T) {
		h := testutil.NewE2EHarness(t)
		defer h.Cleanup()

		h.Init()

		// Branch names with slashes should work but worktree dir names get sanitized
		result := h.CreateWorktree("feature/with/slashes")
		result.AssertSuccess(t)

		// The worktree name should be sanitized (slashes replaced with dashes)
		if !h.WorktreeExists("feature-with-slashes") {
			t.Error("worktree with sanitized name was not created")
		}
	})

	t.Run("fail to create duplicate worktree", func(t *testing.T) {
		h := testutil.NewE2EHarness(t)
		defer h.Cleanup()

		h.Init()

		// Create first worktree
		h.CreateWorktree("duplicate-test")

		// Try to create same worktree again
		result := h.CreateWorktree("duplicate-test")
		result.AssertFailed(t)
	})

	t.Run("fail for nonexistent branch", func(t *testing.T) {
		h := testutil.NewE2EHarness(t)
		defer h.Cleanup()

		h.Init()

		result := h.CreateWorktreeFromBranch("wt-nonexistent", "nonexistent-branch")
		result.AssertFailed(t)
	})
}

// TestE2E_DeleteWorktreeScenarios tests various worktree deletion scenarios.
func TestE2E_DeleteWorktreeScenarios(t *testing.T) {
	t.Run("delete clean worktree with force", func(t *testing.T) {
		h := testutil.NewE2EHarness(t)
		defer h.Cleanup()

		h.Init()
		h.CreateWorktree("to-delete-clean")

		// Use force flag since we're in non-interactive mode
		result := h.DeleteWorktree("to-delete-clean", true)
		result.AssertSuccess(t)

		if h.WorktreeExists("to-delete-clean") {
			t.Error("worktree still exists after deletion")
		}
	})

	t.Run("fail to delete worktree without force in non-TTY", func(t *testing.T) {
		h := testutil.NewE2EHarness(t)
		defer h.Cleanup()

		h.Init()
		h.CreateWorktree("dirty-worktree")

		// Should fail without force in non-TTY mode (can't get confirmation)
		result := h.DeleteWorktree("dirty-worktree", false)
		result.AssertFailed(t)
		result.AssertStdoutContains(t, "non-interactive mode")

		// Worktree should still exist
		if !h.WorktreeExists("dirty-worktree") {
			t.Error("worktree was deleted despite no confirmation possible")
		}
	})

	t.Run("force delete worktree with uncommitted changes", func(t *testing.T) {
		h := testutil.NewE2EHarness(t)
		defer h.Cleanup()

		h.Init()
		h.CreateWorktree("force-delete-test")

		// Create uncommitted changes
		h.WriteFileInWorktree("force-delete-test", "uncommitted.txt", "dirty content")

		// Should succeed with force
		result := h.DeleteWorktree("force-delete-test", true)
		result.AssertSuccess(t)

		if h.WorktreeExists("force-delete-test") {
			t.Error("worktree still exists after force deletion")
		}
	})

	t.Run("fail to delete nonexistent worktree", func(t *testing.T) {
		h := testutil.NewE2EHarness(t)
		defer h.Cleanup()

		h.Init()

		result := h.DeleteWorktree("nonexistent", true)
		result.AssertFailed(t)
	})

	t.Run("fail to delete current worktree", func(t *testing.T) {
		h := testutil.NewE2EHarness(t)
		defer h.Cleanup()

		h.Init()

		// Get the main worktree name (repo directory name)
		repoName := filepath.Base(h.RepoPath())

		// Try to delete the current/main worktree
		result := h.DeleteWorktree(repoName, true)
		result.AssertFailed(t)
	})
}

// TestE2E_CompareCommand tests the compare functionality.
func TestE2E_CompareCommand(t *testing.T) {
	t.Run("compare with no changes", func(t *testing.T) {
		h := testutil.NewE2EHarness(t)
		defer h.Cleanup()

		h.Init()
		h.CreateWorktree("compare-clean")

		result := h.Compare("compare-clean")
		result.AssertSuccess(t)
		result.AssertStdoutContains(t, "No changes")
	})

	t.Run("compare with new file", func(t *testing.T) {
		h := testutil.NewE2EHarness(t)
		defer h.Cleanup()

		h.Init()
		h.CreateWorktree("compare-new-file")

		// Add a new file to the worktree
		h.WriteFileInWorktree("compare-new-file", "new-feature.txt", "feature content")

		result := h.Compare("compare-new-file")
		result.AssertSuccess(t)
		result.AssertStdoutContains(t, "new-feature.txt")
	})

	t.Run("compare with diff output", func(t *testing.T) {
		h := testutil.NewE2EHarness(t)
		defer h.Cleanup()

		h.Init()
		h.CreateWorktree("compare-diff")

		// Add a new file
		h.WriteFileInWorktree("compare-diff", "diff-file.txt", "diff content")

		result := h.CompareWithDiff("compare-diff")
		result.AssertSuccess(t)
		result.AssertStdoutContains(t, "diff-file.txt")
	})

	t.Run("compare and apply changes", func(t *testing.T) {
		h := testutil.NewE2EHarness(t)
		defer h.Cleanup()

		h.Init()
		h.CreateWorktree("compare-apply")

		// Add a new file to the worktree
		h.WriteFileInWorktree("compare-apply", "applied-file.txt", "applied content")

		result := h.CompareWithApply("compare-apply")
		result.AssertSuccess(t)
		result.AssertStdoutContains(t, "Successfully applied")

		// Verify file was applied to main worktree
		content, err := h.ReadFile("applied-file.txt")
		if err != nil {
			t.Fatalf("applied file not found in main worktree: %v", err)
		}
		if content != "applied content" {
			t.Errorf("applied file content = %q, want 'applied content'", content)
		}
	})

	t.Run("compare nonexistent worktree", func(t *testing.T) {
		h := testutil.NewE2EHarness(t)
		defer h.Cleanup()

		h.Init()

		result := h.Compare("nonexistent")
		result.AssertFailed(t)
	})
}

// TestE2E_ListCommand tests the list command.
func TestE2E_ListCommand(t *testing.T) {
	t.Run("list single worktree", func(t *testing.T) {
		h := testutil.NewE2EHarness(t)
		defer h.Cleanup()

		result := h.List()
		result.AssertSuccess(t)
		// Should show at least one worktree (the main repo)
	})

	t.Run("list multiple worktrees", func(t *testing.T) {
		h := testutil.NewE2EHarness(t)
		defer h.Cleanup()

		h.Init()
		h.CreateWorktree("list-test-1")
		h.CreateWorktree("list-test-2")

		result := h.List()
		result.AssertSuccess(t)
		result.AssertStdoutContains(t, "list-test-1")
		result.AssertStdoutContains(t, "list-test-2")
	})

	t.Run("list verbose", func(t *testing.T) {
		h := testutil.NewE2EHarness(t)
		defer h.Cleanup()

		h.Init()
		h.CreateWorktree("verbose-test")

		result := h.ListVerbose()
		result.AssertSuccess(t)
		// Verbose output should contain paths
		result.AssertStdoutContains(t, "/")
	})
}

// TestE2E_NavigateCommand tests navigation between worktrees.
func TestE2E_NavigateCommand(t *testing.T) {
	t.Run("navigate to existing worktree", func(t *testing.T) {
		h := testutil.NewE2EHarness(t)
		defer h.Cleanup()

		h.Init()
		h.CreateWorktree("nav-target")

		// Create a temp directive file
		directiveFile, err := os.CreateTemp("", "gren-directive-*")
		if err != nil {
			t.Fatalf("failed to create directive file: %v", err)
		}
		directiveFile.Close()
		defer os.Remove(directiveFile.Name())

		h.SetEnv("GREN_DIRECTIVE_FILE", directiveFile.Name())

		result := h.Navigate("nav-target")
		result.AssertSuccess(t)

		// Verify directive file was written
		content, err := os.ReadFile(directiveFile.Name())
		if err != nil {
			t.Fatalf("failed to read directive file: %v", err)
		}
		if !strings.Contains(string(content), "cd ") {
			t.Errorf("directive file should contain cd command, got: %s", content)
		}
	})

	t.Run("navigate to nonexistent worktree", func(t *testing.T) {
		h := testutil.NewE2EHarness(t)
		defer h.Cleanup()

		result := h.Navigate("nonexistent")
		result.AssertFailed(t)
	})

	t.Run("navigate without argument", func(t *testing.T) {
		h := testutil.NewE2EHarness(t)
		defer h.Cleanup()

		result := h.Run("navigate")
		result.AssertFailed(t)
	})
}

// TestE2E_HelpAndVersion tests help and version commands.
func TestE2E_HelpAndVersion(t *testing.T) {
	t.Run("help flag", func(t *testing.T) {
		h := testutil.NewE2EHarness(t)
		defer h.Cleanup()

		result := h.Run("--help")
		result.AssertSuccess(t)
		result.AssertStdoutContains(t, "gren")
		result.AssertStdoutContains(t, "create")
		result.AssertStdoutContains(t, "list")
		result.AssertStdoutContains(t, "delete")
	})

	t.Run("version flag", func(t *testing.T) {
		h := testutil.NewE2EHarness(t)
		defer h.Cleanup()

		result := h.Run("--version")
		result.AssertSuccess(t)
		result.AssertStdoutContains(t, "gren version")
	})
}

// TestE2E_ShellInit tests shell initialization scripts.
func TestE2E_ShellInit(t *testing.T) {
	shells := []string{"bash", "zsh", "fish"}

	for _, shell := range shells {
		t.Run(shell, func(t *testing.T) {
			h := testutil.NewE2EHarness(t)
			defer h.Cleanup()

			result := h.Run("shell-init", shell)
			result.AssertSuccess(t)

			// All shells should define a gren function
			if shell == "fish" {
				result.AssertStdoutContains(t, "function gren")
			} else {
				result.AssertStdoutContains(t, "gren()")
			}
		})
	}

	t.Run("unsupported shell", func(t *testing.T) {
		h := testutil.NewE2EHarness(t)
		defer h.Cleanup()

		result := h.Run("shell-init", "powershell")
		result.AssertFailed(t)
	})
}

// TestE2E_ErrorHandling tests various error scenarios.
func TestE2E_ErrorHandling(t *testing.T) {
	t.Run("run in non-git directory", func(t *testing.T) {
		// Create a non-git temp directory
		tmpDir, err := os.MkdirTemp("", "gren-non-git-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		h := testutil.NewE2EHarness(t)
		defer h.Cleanup()

		// Run list in non-git directory
		result := h.RunInDir(tmpDir, "list")
		result.AssertFailed(t)
	})

	t.Run("unknown command", func(t *testing.T) {
		h := testutil.NewE2EHarness(t)
		defer h.Cleanup()

		result := h.Run("unknown-command")
		result.AssertFailed(t)
		result.AssertStdoutContains(t, "unknown command")
	})
}

// TestE2E_CommandAliases tests command aliases.
func TestE2E_CommandAliases(t *testing.T) {
	// These are the documented aliases
	aliases := map[string]string{
		"nav": "navigate",
		"cd":  "navigate",
	}

	for alias, command := range aliases {
		t.Run(alias+" is alias for "+command, func(t *testing.T) {
			h := testutil.NewE2EHarness(t)
			defer h.Cleanup()

			// Both should give the same type of error (missing required arg)
			// rather than "unknown command"
			result := h.Run(alias)
			if strings.Contains(result.Stdout, "unknown command") {
				t.Errorf("%s should be recognized as an alias for %s", alias, command)
			}
		})
	}
}

// TestE2E_HookExecution tests hook execution during various operations.
func TestE2E_HookExecution(t *testing.T) {
	t.Run("post-create hook runs on worktree creation", func(t *testing.T) {
		h := testutil.NewE2EHarness(t)
		defer h.Cleanup()

		h.Init()

		// Create a config with a post-create hook that creates a marker file
		// Using the simple hooks format
		configContent := `worktree_dir = "` + h.WorktreeDir + `"
version = "2.0.0"
package_manager = "auto"

[hooks]
post-create = "touch hook-executed.txt"
`
		h.WriteFile(".gren/config.toml", configContent)

		// Create a worktree with -y flag to auto-approve hooks
		result := h.CreateWorktreeWithHooks("hook-test")
		result.AssertSuccess(t)

		// Verify the hook created the marker file in the worktree
		worktreePath := h.GetWorktreePath("hook-test")
		markerPath := filepath.Join(worktreePath, "hook-executed.txt")
		if _, err := os.Stat(markerPath); os.IsNotExist(err) {
			t.Errorf("post-create hook did not run (marker file not created)\nStdout: %s\nStderr: %s", result.Stdout, result.Stderr)
		}
	})

	t.Run("hook receives context via environment", func(t *testing.T) {
		h := testutil.NewE2EHarness(t)
		defer h.Cleanup()

		h.Init()

		// Create a hook that writes GREN_JSON_CONTEXT to a file
		// Using the simple hooks format
		configContent := `worktree_dir = "` + h.WorktreeDir + `"
version = "2.0.0"
package_manager = "auto"

[hooks]
post-create = "echo $GREN_JSON_CONTEXT > context.json"
`
		h.WriteFile(".gren/config.toml", configContent)

		result := h.CreateWorktreeWithHooks("context-test")
		result.AssertSuccess(t)

		// Read the context file
		worktreePath := h.GetWorktreePath("context-test")
		contextPath := filepath.Join(worktreePath, "context.json")
		content, err := os.ReadFile(contextPath)
		if err != nil {
			t.Fatalf("failed to read context file: %v\nStdout: %s\nStderr: %s", err, result.Stdout, result.Stderr)
		}

		// Verify context contains expected fields
		contextStr := string(content)
		expectedFields := []string{"hook_type", "branch", "worktree"}
		for _, field := range expectedFields {
			if !strings.Contains(contextStr, field) {
				t.Errorf("context should contain %q, got: %s", field, contextStr)
			}
		}
	})
}

// TestE2E_MultipleWorktreeOperations tests complex workflows with multiple worktrees.
func TestE2E_MultipleWorktreeOperations(t *testing.T) {
	t.Run("create multiple worktrees and manage them", func(t *testing.T) {
		h := testutil.NewE2EHarness(t)
		defer h.Cleanup()

		h.Init()

		// Create multiple worktrees
		worktrees := []string{"feature-1", "feature-2", "bugfix-1"}
		for _, wt := range worktrees {
			result := h.CreateWorktree(wt)
			result.AssertSuccess(t)
		}

		// Verify all exist
		for _, wt := range worktrees {
			if !h.WorktreeExists(wt) {
				t.Errorf("worktree %s was not created", wt)
			}
		}

		// List should show all
		listResult := h.List()
		for _, wt := range worktrees {
			listResult.AssertStdoutContains(t, wt)
		}

		// Delete one (use force since non-interactive)
		h.DeleteWorktree("feature-2", true)

		// Verify it's gone
		if h.WorktreeExists("feature-2") {
			t.Error("feature-2 should be deleted")
		}

		// Others should still exist
		if !h.WorktreeExists("feature-1") {
			t.Error("feature-1 should still exist")
		}
		if !h.WorktreeExists("bugfix-1") {
			t.Error("bugfix-1 should still exist")
		}
	})
}
