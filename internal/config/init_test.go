package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDetectPackageManager(t *testing.T) {
	tests := []struct {
		name     string
		files    []string
		expected string
	}{
		{
			name:     "bun lockb file",
			files:    []string{"bun.lockb"},
			expected: "bun",
		},
		{
			name:     "bun lock file",
			files:    []string{"bun.lock"},
			expected: "bun",
		},
		{
			name:     "pnpm lockfile",
			files:    []string{"pnpm-lock.yaml"},
			expected: "pnpm",
		},
		{
			name:     "yarn lockfile",
			files:    []string{"yarn.lock"},
			expected: "yarn",
		},
		{
			name:     "npm fallback with package.json",
			files:    []string{"package.json"},
			expected: "npm",
		},
		{
			name:     "bun takes priority over yarn",
			files:    []string{"bun.lockb", "yarn.lock", "package.json"},
			expected: "bun",
		},
		{
			name:     "pnpm takes priority over yarn",
			files:    []string{"pnpm-lock.yaml", "yarn.lock"},
			expected: "pnpm",
		},
		{
			name:     "no package manager",
			files:    []string{},
			expected: "auto",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory
			tempDir, err := os.MkdirTemp("", "gren-pm-test-*")
			if err != nil {
				t.Fatalf("failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tempDir)

			// Save original dir and change to temp
			originalDir, _ := os.Getwd()
			defer os.Chdir(originalDir)
			os.Chdir(tempDir)

			// Create test files
			for _, file := range tt.files {
				if err := os.WriteFile(file, []byte(""), 0644); err != nil {
					t.Fatalf("failed to create file %s: %v", file, err)
				}
			}

			// Create default config and detect
			config, _ := NewDefaultConfig("test-project", tempDir)
			config, _ = detectProjectSettings(config)

			if config.PackageManager != tt.expected {
				t.Errorf("PackageManager = %q, want %q", config.PackageManager, tt.expected)
			}
		})
	}
}

func TestDetectEnvFiles(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "gren-env-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	// Create .gitignore
	gitignore := ".env.local\n.env.*.local\n"
	os.WriteFile(".gitignore", []byte(gitignore), 0644)

	// Create env files
	os.WriteFile(".env.local", []byte("SECRET=value"), 0644)
	os.WriteFile(".env.test.local", []byte("TEST=value"), 0644)

	config, _ := NewDefaultConfig("test", tempDir)
	_, detected := detectProjectSettings(config)

	// Should detect .env.local (other patterns may or may not match depending on glob)
	found := false
	for _, f := range detected.EnvFiles {
		if f == ".env.local" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected .env.local in detected files, got %v", detected.EnvFiles)
	}
}

func TestDetectClaudeDir(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gren-claude-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	t.Run("claude dir exists and gitignored", func(t *testing.T) {
		os.Mkdir(".claude", 0755)
		os.WriteFile(".gitignore", []byte(".claude\n"), 0644)

		config, _ := NewDefaultConfig("test", tempDir)
		_, detected := detectProjectSettings(config)

		if !detected.ClaudeDir {
			t.Error("Expected ClaudeDir to be true")
		}
	})

	t.Run("claude dir exists but not gitignored", func(t *testing.T) {
		os.RemoveAll(".claude")
		os.Remove(".gitignore")
		os.Mkdir(".claude", 0755)
		// No .gitignore

		config, _ := NewDefaultConfig("test", tempDir)
		_, detected := detectProjectSettings(config)

		if detected.ClaudeDir {
			t.Error("Expected ClaudeDir to be false when not gitignored")
		}
	})
}

func TestInitialize(t *testing.T) {
	t.Run("successful initialization", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "gren-init-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)
		os.Chdir(tempDir)

		// Initialize git repo (required for getRepoRoot)
		exec.Command("git", "init").Run()
		exec.Command("git", "config", "user.email", "test@test.com").Run()
		exec.Command("git", "config", "user.name", "Test User").Run()

		result := Initialize("test-project")

		if !result.Success {
			t.Errorf("Initialize() failed: %v", result.Error)
		}

		if !result.ConfigCreated {
			t.Error("Expected ConfigCreated to be true")
		}

		if !result.HookCreated {
			t.Error("Expected HookCreated to be true")
		}

		// Verify .gren directory exists
		if _, err := os.Stat(".gren"); err != nil {
			t.Errorf(".gren directory not created: %v", err)
		}

		// Verify config file exists
		if _, err := os.Stat(filepath.Join(".gren", "config.json")); err != nil {
			t.Errorf("config.json not created: %v", err)
		}

		// Verify hook file exists
		if _, err := os.Stat(filepath.Join(".gren", "post-create.sh")); err != nil {
			t.Errorf("post-create.sh not created: %v", err)
		}
	})

	t.Run("empty project name fails", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "gren-init-empty-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)
		os.Chdir(tempDir)

		result := Initialize("")

		if result.Success {
			t.Error("Initialize() should fail with empty project name")
		}

		if result.Error == nil {
			t.Error("Expected error for empty project name")
		}
	})

	t.Run("hook not recreated if exists", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "gren-init-hook-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		originalDir, _ := os.Getwd()
		defer os.Chdir(originalDir)
		os.Chdir(tempDir)

		// Initialize git repo (required for getRepoRoot)
		exec.Command("git", "init").Run()
		exec.Command("git", "config", "user.email", "test@test.com").Run()
		exec.Command("git", "config", "user.name", "Test User").Run()

		// Pre-create hook with custom content
		os.MkdirAll(".gren", 0755)
		customHook := "#!/bin/bash\n# Custom hook\n"
		hookPath := filepath.Join(".gren", "post-create.sh")
		os.WriteFile(hookPath, []byte(customHook), 0755)

		result := Initialize("test-project")

		if !result.Success {
			t.Errorf("Initialize() failed: %v", result.Error)
		}

		// HookCreated should be false since hook already existed
		if result.HookCreated {
			t.Error("HookCreated should be false when hook already exists")
		}

		// Hook content should be unchanged
		content, _ := os.ReadFile(hookPath)
		if string(content) != customHook {
			t.Error("Existing hook was overwritten")
		}
	})
}

func TestDetectConfigFiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gren-config-detect-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	// Create gitignored config files
	os.WriteFile(".gitignore", []byte(".envrc\n.nvmrc\n"), 0644)
	os.WriteFile(".envrc", []byte("export FOO=bar"), 0644)
	os.WriteFile(".nvmrc", []byte("18"), 0644)

	config, _ := NewDefaultConfig("test", tempDir)
	_, detected := detectProjectSettings(config)

	if len(detected.ConfigFiles) != 2 {
		t.Errorf("Expected 2 config files, got %d: %v", len(detected.ConfigFiles), detected.ConfigFiles)
	}
}

func TestDetectClaudeMd(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gren-claudemd-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	// Create gitignored CLAUDE.md
	os.WriteFile(".gitignore", []byte("CLAUDE.md\n"), 0644)
	os.WriteFile("CLAUDE.md", []byte("# Instructions"), 0644)

	config, _ := NewDefaultConfig("test", tempDir)
	_, detected := detectProjectSettings(config)

	if !detected.ClaudeMd {
		t.Error("Expected ClaudeMd to be true")
	}
}

func TestGenerateHookContent(t *testing.T) {
	t.Run("with bun and detected files", func(t *testing.T) {
		config := &Config{
			WorktreeDir:    "../worktrees",
			PackageManager: "bun",
			Version:        "1.0.0",
		}

		detected := DetectedFiles{
			EnvFiles:  []string{".env.local"},
			ClaudeDir: true,
		}

		content := generateHookContentWithSymlinks(config, detected)

		// Check for shebang
		if content[:2] != "#!" {
			t.Error("Hook should start with shebang")
		}

		// Check for env file symlink
		if !contains(content, ".env.local") {
			t.Error("Hook should contain .env.local symlink")
		}

		// Check for claude dir symlink
		if !contains(content, ".claude") {
			t.Error("Hook should contain .claude symlink")
		}

		// Check for bun in comments
		if !contains(content, "bun") {
			t.Error("Hook should mention bun package manager")
		}
	})

	t.Run("with auto package manager", func(t *testing.T) {
		config := &Config{
			WorktreeDir:    "../worktrees",
			PackageManager: "auto",
			Version:        "1.0.0",
		}

		detected := DetectedFiles{}

		content := generateHookContentWithSymlinks(config, detected)

		// Should have npm as fallback suggestion
		if !contains(content, "npm install") {
			t.Error("Hook should mention npm install as fallback")
		}
	})

	t.Run("with config files", func(t *testing.T) {
		config := &Config{
			WorktreeDir:    "../worktrees",
			PackageManager: "npm",
			Version:        "1.0.0",
		}

		detected := DetectedFiles{
			ConfigFiles: []string{".envrc", ".nvmrc"},
		}

		content := generateHookContentWithSymlinks(config, detected)

		if !contains(content, ".envrc") {
			t.Error("Hook should contain .envrc symlink")
		}
		if !contains(content, ".nvmrc") {
			t.Error("Hook should contain .nvmrc symlink")
		}
	})

	t.Run("with CLAUDE.md", func(t *testing.T) {
		config := &Config{
			WorktreeDir:    "../worktrees",
			PackageManager: "npm",
			Version:        "1.0.0",
		}

		detected := DetectedFiles{
			ClaudeMd: true,
		}

		content := generateHookContentWithSymlinks(config, detected)

		if !contains(content, "CLAUDE.md") {
			t.Error("Hook should contain CLAUDE.md symlink")
		}
	})

	t.Run("with no detected files", func(t *testing.T) {
		config := &Config{
			WorktreeDir:    "../worktrees",
			PackageManager: "npm",
			Version:        "1.0.0",
		}

		detected := DetectedFiles{}

		content := generateHookContentWithSymlinks(config, detected)

		// Should still have basic structure
		if !contains(content, "#!/usr/bin/env bash") {
			t.Error("Hook should have bash shebang")
		}
		if !contains(content, "Post-create setup complete") {
			t.Error("Hook should have completion message")
		}
	})
}

func TestFileExists(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gren-file-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	existingFile := filepath.Join(tempDir, "exists.txt")
	os.WriteFile(existingFile, []byte("test"), 0644)

	if !fileExists(existingFile) {
		t.Error("fileExists() = false for existing file")
	}

	if fileExists(filepath.Join(tempDir, "not-exists.txt")) {
		t.Error("fileExists() = true for non-existing file")
	}
}

func TestDirExists(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gren-dir-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	subDir := filepath.Join(tempDir, "subdir")
	os.Mkdir(subDir, 0755)

	if !dirExists(subDir) {
		t.Error("dirExists() = false for existing directory")
	}

	// Create a file, not a directory
	file := filepath.Join(tempDir, "file.txt")
	os.WriteFile(file, []byte("test"), 0644)

	if dirExists(file) {
		t.Error("dirExists() = true for file (not directory)")
	}

	if dirExists(filepath.Join(tempDir, "not-exists")) {
		t.Error("dirExists() = true for non-existing path")
	}
}

func TestIsGitIgnored(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gren-gitignore-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	t.Run("no gitignore file", func(t *testing.T) {
		if isGitIgnored(".env") {
			t.Error("isGitIgnored() = true when no .gitignore exists")
		}
	})

	t.Run("file is gitignored", func(t *testing.T) {
		os.WriteFile(".gitignore", []byte(".env\n.claude/\n"), 0644)

		if !isGitIgnored(".env") {
			t.Error("isGitIgnored() = false for .env which is in .gitignore")
		}

		if !isGitIgnored(".claude") {
			t.Error("isGitIgnored() = false for .claude which is in .gitignore")
		}
	})

	t.Run("file is not gitignored", func(t *testing.T) {
		if isGitIgnored("README.md") {
			t.Error("isGitIgnored() = true for README.md which is not in .gitignore")
		}
	})
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
