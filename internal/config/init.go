package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// InitResult contains the result of initialization
type InitResult struct {
	Success       bool
	ConfigCreated bool
	HookCreated   bool
	Message       string
	Error         error
}

// Initialize sets up gren configuration for the current repository
func Initialize(projectName string) InitResult {
	result := InitResult{}

	// Get the repository root (main worktree path)
	repoRoot, err := getRepoRoot()
	if err != nil {
		result.Error = fmt.Errorf("failed to get repository root: %w", err)
		return result
	}

	// Create .gren directory
	err = os.MkdirAll(".gren", 0755)
	if err != nil {
		result.Error = fmt.Errorf("failed to create .gren directory: %w", err)
		return result
	}

	// Create default configuration
	config, err := NewDefaultConfig(projectName, repoRoot)
	if err != nil {
		result.Error = fmt.Errorf("failed to create default config: %w", err)
		return result
	}

	// Detect package manager and files to symlink
	config, detected := detectProjectSettings(config)

	// Save configuration
	manager := NewManager()
	err = manager.Save(config)
	if err != nil {
		result.Error = fmt.Errorf("failed to save configuration: %w", err)
		return result
	}
	result.ConfigCreated = true

	// Create post-create hook script if it doesn't exist
	hookPath := config.PostCreateHook
	if !fileExists(hookPath) {
		err = createPostCreateHookWithSymlinks(hookPath, config, detected)
		if err != nil {
			result.Error = fmt.Errorf("failed to create post-create hook: %w", err)
			return result
		}
		result.HookCreated = true
	}

	result.Success = true
	result.Message = fmt.Sprintf("Initialized gren for project '%s'", projectName)

	return result
}

// DetectedFiles holds files detected during project analysis
type DetectedFiles struct {
	EnvFiles    []string // e.g. .env.local, .env.llm.local
	ConfigFiles []string // e.g. .envrc, .nvmrc
	ClaudeDir   bool     // .claude directory exists and is gitignored
	ClaudeMd    bool     // CLAUDE.md exists and is gitignored
}

// detectProjectSettings analyzes the project and adjusts configuration
func detectProjectSettings(config *Config) (*Config, DetectedFiles) {
	detected := DetectedFiles{}

	// Detect package manager
	if fileExists("bun.lockb") || fileExists("bun.lock") {
		config.PackageManager = "bun"
	} else if fileExists("pnpm-lock.yaml") {
		config.PackageManager = "pnpm"
	} else if fileExists("yarn.lock") {
		config.PackageManager = "yarn"
	} else if fileExists("package.json") {
		config.PackageManager = "npm"
	}

	// Detect .env files that are gitignored
	envPatterns := []string{".env.local", ".env.*.local", ".env.llm.local"}
	for _, pattern := range envPatterns {
		matches, _ := filepath.Glob(pattern)
		for _, match := range matches {
			if isGitIgnored(match) {
				detected.EnvFiles = append(detected.EnvFiles, match)
			}
		}
	}

	// Check for common config files that are gitignored
	checkFiles := []string{".envrc", ".nvmrc", ".node-version"}
	for _, file := range checkFiles {
		if fileExists(file) && isGitIgnored(file) {
			detected.ConfigFiles = append(detected.ConfigFiles, file)
		}
	}

	// Check for .claude directory (if gitignored)
	if dirExists(".claude") && isGitIgnored(".claude") {
		detected.ClaudeDir = true
	}

	// Check for CLAUDE.md (if gitignored)
	if fileExists("CLAUDE.md") && isGitIgnored("CLAUDE.md") {
		detected.ClaudeMd = true
	}

	return config, detected
}

// createPostCreateHookWithSymlinks creates a post-create hook script using symlinks
func createPostCreateHookWithSymlinks(hookPath string, config *Config, detected DetectedFiles) error {
	// Ensure directory exists
	dir := filepath.Dir(hookPath)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}

	// Generate hook content with symlinks
	content := generateHookContentWithSymlinks(config, detected)

	// Write hook file
	err = os.WriteFile(hookPath, []byte(content), 0755)
	if err != nil {
		return err
	}

	return nil
}

// generateHookContentWithSymlinks creates the content for the post-create hook using symlinks
func generateHookContentWithSymlinks(config *Config, detected DetectedFiles) string {
	var builder strings.Builder

	builder.WriteString("#!/usr/bin/env bash\n")
	builder.WriteString("# gren post-create hook\n")
	builder.WriteString("# This script runs after creating a new worktree\n")
	builder.WriteString("# Edit this file to customize your worktree setup\n\n")
	builder.WriteString("set -euo pipefail\n\n")
	builder.WriteString("WORKTREE_PATH=\"$1\"\n")
	builder.WriteString("BRANCH_NAME=\"${2:-}\"\n")
	builder.WriteString("BASE_BRANCH=\"${3:-}\"\n")
	builder.WriteString("REPO_ROOT=\"$4\"\n\n")
	builder.WriteString("cd \"$WORKTREE_PATH\"\n\n")
	builder.WriteString("echo \"🔧 Running post-create setup for $BRANCH_NAME...\"\n")
	builder.WriteString("echo \"\"\n\n")

	// Symlink env files
	if len(detected.EnvFiles) > 0 {
		builder.WriteString("# Symlink environment files\n")
		builder.WriteString("echo \"🔗 Symlinking env files...\"\n")
		for _, envFile := range detected.EnvFiles {
			builder.WriteString(fmt.Sprintf("[ -f \"$REPO_ROOT/%s\" ] && ln -sf \"$REPO_ROOT/%s\" \"$WORKTREE_PATH/%s\" && echo \"   ✓ %s\"\n", envFile, envFile, envFile, envFile))
		}
		builder.WriteString("echo \"\"\n\n")
	}

	// Symlink config files
	if len(detected.ConfigFiles) > 0 {
		builder.WriteString("# Symlink config files\n")
		builder.WriteString("echo \"🔗 Symlinking config files...\"\n")
		for _, configFile := range detected.ConfigFiles {
			builder.WriteString(fmt.Sprintf("[ -f \"$REPO_ROOT/%s\" ] && ln -sf \"$REPO_ROOT/%s\" \"$WORKTREE_PATH/%s\" && echo \"   ✓ %s\"\n", configFile, configFile, configFile, configFile))
		}
		builder.WriteString("echo \"\"\n\n")
	}

	// Symlink .claude directory
	if detected.ClaudeDir {
		builder.WriteString("# Symlink .claude directory\n")
		builder.WriteString("if [ -d \"$REPO_ROOT/.claude\" ]; then\n")
		builder.WriteString("    echo \"🔗 Symlinking .claude...\"\n")
		builder.WriteString("    ln -sf \"$REPO_ROOT/.claude\" \"$WORKTREE_PATH/.claude\"\n")
		builder.WriteString("    echo \"   ✓ .claude\"\n")
		builder.WriteString("fi\n")
		builder.WriteString("echo \"\"\n\n")
	}

	// Symlink CLAUDE.md
	if detected.ClaudeMd {
		builder.WriteString("# Symlink CLAUDE.md\n")
		builder.WriteString("if [ -f \"$REPO_ROOT/CLAUDE.md\" ]; then\n")
		builder.WriteString("    echo \"🔗 Symlinking CLAUDE.md...\"\n")
		builder.WriteString("    ln -sf \"$REPO_ROOT/CLAUDE.md\" \"$WORKTREE_PATH/CLAUDE.md\"\n")
		builder.WriteString("    echo \"   ✓ CLAUDE.md\"\n")
		builder.WriteString("fi\n")
		builder.WriteString("echo \"\"\n\n")
	}

	// Direnv setup
	builder.WriteString("# Auto-allow direnv if available\n")
	builder.WriteString("if command -v direnv &> /dev/null && [ -f \".envrc\" ]; then\n")
	builder.WriteString("    echo \"⚙️  Running direnv allow...\"\n")
	builder.WriteString("    direnv allow\n")
	builder.WriteString("    echo \"\"\n")
	builder.WriteString("fi\n\n")

	// Package manager installation (commented out by default - user can enable)
	builder.WriteString("# Uncomment below to auto-install dependencies\n")
	builder.WriteString("# if [ -f \"package.json\" ]; then\n")
	if config.PackageManager != "auto" && config.PackageManager != "" {
		builder.WriteString(fmt.Sprintf("#     echo \"📦 Installing dependencies with %s...\"\n", config.PackageManager))
		builder.WriteString(fmt.Sprintf("#     %s install\n", config.PackageManager))
	} else {
		builder.WriteString("#     echo \"📦 Installing dependencies...\"\n")
		builder.WriteString("#     npm install  # or: yarn, pnpm, bun\n")
	}
	builder.WriteString("# fi\n\n")

	builder.WriteString("echo \"✅ Post-create setup complete!\"\n")
	builder.WriteString("echo \"\"\n")

	return builder.String()
}

// Helper functions
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func isGitIgnored(path string) bool {
	if !fileExists(".gitignore") {
		return false
	}

	data, err := os.ReadFile(".gitignore")
	if err != nil {
		return false
	}

	gitignoreContent := string(data)

	// Check for various gitignore patterns for the path
	patterns := []string{
		path,             // .claude
		"/" + path,       // /.claude
		path + "/",       // .claude/
		"/" + path + "/", // /.claude/
	}

	for _, pattern := range patterns {
		if strings.Contains(gitignoreContent, pattern) {
			return true
		}
	}

	return false
}

// getRepoRoot returns the absolute path to the repository root (main worktree)
func getRepoRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}
