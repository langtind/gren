package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// InitResult contains the result of initialization
type InitResult struct {
	Success         bool
	ConfigCreated   bool
	HookCreated     bool
	Message         string
	Error           error
}

// Initialize sets up gren configuration for the current repository
func Initialize(projectName string) InitResult {
	result := InitResult{}

	// Create .gren directory
	err := os.MkdirAll(".gren", 0755)
	if err != nil {
		result.Error = fmt.Errorf("failed to create .gren directory: %w", err)
		return result
	}

	// Create default configuration
	config, err := NewDefaultConfig(projectName)
	if err != nil {
		result.Error = fmt.Errorf("failed to create default config: %w", err)
		return result
	}

	// Detect package manager and files to copy
	config = detectProjectSettings(config)

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
		err = createPostCreateHook(hookPath, config)
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

// detectProjectSettings analyzes the project and adjusts configuration
func detectProjectSettings(config *Config) *Config {
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

	// Detect additional files to copy
	copyPatterns := []string{".env*"}

	// Check for common config files
	checkFiles := []string{".envrc", ".nvmrc", ".node-version"}
	for _, file := range checkFiles {
		if fileExists(file) {
			copyPatterns = append(copyPatterns, file)
		}
	}

	// Check for .claude directory (if gitignored)
	if dirExists(".claude") && isGitIgnored(".claude") {
		copyPatterns = append(copyPatterns, ".claude/**/*")
	}

	config.CopyPatterns = copyPatterns
	return config
}

// createPostCreateHook creates a default post-create hook script
func createPostCreateHook(hookPath string, config *Config) error {
	// Ensure directory exists
	dir := filepath.Dir(hookPath)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}

	// Generate hook content
	content := generateHookContent(config)

	// Write hook file
	err = os.WriteFile(hookPath, []byte(content), 0755)
	if err != nil {
		return err
	}

	return nil
}

// generateHookContent creates the content for the post-create hook
func generateHookContent(config *Config) string {
	var builder strings.Builder

	builder.WriteString("#!/bin/bash\n")
	builder.WriteString("# gren post-create hook\n")
	builder.WriteString("# This script runs after creating a new worktree\n\n")
	builder.WriteString("set -e\n\n")
	builder.WriteString("WORKTREE_PATH=\"$1\"\n")
	builder.WriteString("BRANCH_NAME=\"$2\"\n")
	builder.WriteString("BASE_BRANCH=\"$3\"\n")
	builder.WriteString("REPO_ROOT=\"$4\"\n\n")
	builder.WriteString("cd \"$WORKTREE_PATH\"\n\n")
	builder.WriteString("echo \"🔧 Running post-create setup for $BRANCH_NAME...\"\n\n")

	// Copy files
	builder.WriteString("# Copy environment and config files\n")
	for _, pattern := range config.CopyPatterns {
		if pattern == ".env*" {
			builder.WriteString("echo \"📁 Copying .env files...\"\n")
			builder.WriteString("cp \"$REPO_ROOT\"/.env* . 2>/dev/null || echo \"  No .env files found\"\n\n")
		} else if pattern == ".claude/**/*" {
			builder.WriteString("# Copy .claude directory if gitignored\n")
			builder.WriteString("if [[ -d \"$REPO_ROOT/.claude\" ]] && grep -q \"\\.claude\" \"$REPO_ROOT/.gitignore\" 2>/dev/null; then\n")
			builder.WriteString("    echo \"📁 Copying .claude config...\"\n")
			builder.WriteString("    cp -r \"$REPO_ROOT/.claude\" . 2>/dev/null || true\n")
			builder.WriteString("fi\n\n")
		} else {
			builder.WriteString(fmt.Sprintf("# Copy %s\n", pattern))
			builder.WriteString(fmt.Sprintf("if [[ -f \"$REPO_ROOT/%s\" ]]; then\n", pattern))
			builder.WriteString(fmt.Sprintf("    echo \"📁 Copying %s...\"\n", pattern))
			builder.WriteString(fmt.Sprintf("    cp \"$REPO_ROOT/%s\" .\n", pattern))
			builder.WriteString("fi\n\n")
		}
	}

	// Package manager installation
	if config.PackageManager != "auto" && config.PackageManager != "" {
		builder.WriteString("# Install dependencies\n")
		builder.WriteString("if [[ -f \"package.json\" ]]; then\n")
		builder.WriteString(fmt.Sprintf("    echo \"📦 Installing dependencies with %s...\"\n", config.PackageManager))
		builder.WriteString(fmt.Sprintf("    %s install\n", config.PackageManager))
		builder.WriteString("fi\n\n")
	} else {
		builder.WriteString("# Auto-detect and install dependencies\n")
		builder.WriteString("if [[ -f \"package.json\" ]]; then\n")
		builder.WriteString("    if [[ -f \"$REPO_ROOT/bun.lockb\" ]] || [[ -f \"$REPO_ROOT/bun.lock\" ]]; then\n")
		builder.WriteString("        echo \"📦 Installing dependencies with bun...\"\n")
		builder.WriteString("        bun install\n")
		builder.WriteString("    elif [[ -f \"$REPO_ROOT/pnpm-lock.yaml\" ]]; then\n")
		builder.WriteString("        echo \"📦 Installing dependencies with pnpm...\"\n")
		builder.WriteString("        pnpm install\n")
		builder.WriteString("    elif [[ -f \"$REPO_ROOT/yarn.lock\" ]]; then\n")
		builder.WriteString("        echo \"📦 Installing dependencies with yarn...\"\n")
		builder.WriteString("        yarn install\n")
		builder.WriteString("    else\n")
		builder.WriteString("        echo \"📦 Installing dependencies with npm...\"\n")
		builder.WriteString("        npm install\n")
		builder.WriteString("    fi\n")
		builder.WriteString("fi\n\n")
	}

	// Direnv setup
	builder.WriteString("# Setup direnv if .envrc exists\n")
	builder.WriteString("if [[ -f \".envrc\" ]] && command -v direnv >/dev/null 2>&1; then\n")
	builder.WriteString("    echo \"🔧 Running direnv allow...\"\n")
	builder.WriteString("    direnv allow\n")
	builder.WriteString("fi\n\n")

	builder.WriteString("echo \"✅ Post-create setup complete!\"\n")

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

	return strings.Contains(string(data), path)
}