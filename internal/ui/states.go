package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"gren/internal/config"
	"gren/internal/git"
)

// NewModel creates a new Model with the given dependencies
func NewModel(gitRepo git.Repository, configManager *config.Manager) Model {
	return Model{
		currentView:   DashboardView,
		gitRepo:       gitRepo,
		configManager: configManager,
		keys:          DefaultKeyMap(),
		selected:      0,
	}
}

// Init initializes the model when the program starts
func (m Model) Init() tea.Cmd {
	return m.loadProjectInfo()
}

// updateProjectInfo updates the project info and refreshes worktrees
func (m Model) updateProjectInfo(info *git.RepoInfo, err error) Model {
	m.repoInfo = info
	m.err = err

	if info != nil {
		// Load config
		if m.configManager != nil {
			if cfg, err := m.configManager.Load(); err == nil {
				m.config = cfg
			}
		}

		// Refresh worktrees list
		if err := m.refreshWorktrees(); err != nil {
			m.err = err
		}
	}

	return m
}

// refreshWorktrees refreshes the list of worktrees
func (m *Model) refreshWorktrees() error {
	if m.repoInfo == nil || !m.repoInfo.IsGitRepo {
		m.worktrees = nil
		return nil
	}

	// Get list of worktrees using git command
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		// If git worktree fails, probably no worktrees
		m.worktrees = nil
		return nil
	}

	m.worktrees = m.parseWorktreeList(string(output))
	return nil
}

// parseWorktreeList parses git worktree list --porcelain output
func (m Model) parseWorktreeList(output string) []Worktree {
	var worktrees []Worktree
	lines := strings.Split(strings.TrimSpace(output), "\n")

	var currentWorktree *Worktree
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if currentWorktree != nil {
				worktrees = append(worktrees, *currentWorktree)
				currentWorktree = nil
			}
			continue
		}

		if strings.HasPrefix(line, "worktree ") {
			if currentWorktree != nil {
				worktrees = append(worktrees, *currentWorktree)
			}
			path := strings.TrimPrefix(line, "worktree ")
			currentWorktree = &Worktree{
				Name:      filepath.Base(path),
				Path:      path,
				Status:    "clean",
				IsCurrent: false,
			}
		} else if strings.HasPrefix(line, "HEAD ") && currentWorktree != nil {
			// Skip HEAD line - we'll use the branch line instead
		} else if strings.HasPrefix(line, "branch ") && currentWorktree != nil {
			branchRef := strings.TrimPrefix(line, "branch ")
			// Extract branch name from refs/heads/branchname
			if strings.HasPrefix(branchRef, "refs/heads/") {
				currentWorktree.Branch = strings.TrimPrefix(branchRef, "refs/heads/")
			} else {
				currentWorktree.Branch = branchRef
			}
		} else if line == "bare" && currentWorktree != nil {
			// Skip bare worktrees for now
		} else if line == "detached" && currentWorktree != nil {
			currentWorktree.Branch = "detached"
		}
	}

	// Add the last worktree if exists
	if currentWorktree != nil {
		worktrees = append(worktrees, *currentWorktree)
	}

	// Mark the current worktree
	if len(worktrees) > 0 {
		cwd, _ := os.Getwd()
		for i := range worktrees {
			if worktrees[i].Path == cwd {
				worktrees[i].IsCurrent = true
				// Get status for current worktree
				if status, err := m.getWorktreeStatus(worktrees[i].Path, true); err == nil {
					worktrees[i].Status = status
				}
				break
			}
		}
	}

	return worktrees
}

// getWorktreeStatus gets the git status for a worktree
func (m Model) getWorktreeStatus(worktreePath string, isCurrent bool) (string, error) {
	if !isCurrent {
		// For non-current worktrees, we need to check status in that directory
		// This is simplified - real implementation would be more sophisticated
		return "clean", nil
	}

	// For current worktree, get actual status using git command
	cmd := exec.Command("git", "status", "--porcelain")
	if !isCurrent {
		cmd.Dir = worktreePath
	}

	output, err := cmd.Output()
	if err != nil {
		return "unknown", err
	}

	statusLines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(statusLines) == 1 && statusLines[0] == "" {
		return "clean", nil
	}

	hasModified := false
	hasUntracked := false

	for _, line := range statusLines {
		if len(line) < 2 {
			continue
		}

		// Check staged/modified status
		if line[0] != ' ' && line[0] != '?' {
			hasModified = true
		}
		if line[1] != ' ' && line[1] != '?' {
			hasModified = true
		}

		// Check untracked status
		if line[0] == '?' && line[1] == '?' {
			hasUntracked = true
		}
	}

	if hasModified && hasUntracked {
		return "mixed", nil
	} else if hasModified {
		return "modified", nil
	} else if hasUntracked {
		return "untracked", nil
	}

	return "clean", nil
}

// setupCreateState initializes create state from message
func (m *Model) setupCreateState(msg createInitMsg) {
	m.createState = &CreateState{
		currentStep:      CreateStepBranchMode,
		createMode:       CreateModeNewBranch,
		branchName:       "",
		baseBranch:       msg.recommendedBase,
		branchStatuses:   msg.branchStatuses,
		availableBranches: []BranchStatus{}, // Will be populated when needed
		selectedBranch:   0,
		selectedMode:     0, // Default to "Create new branch"
		showWarning:      false,
		warningAccepted:  false,
		selectedAction:   0,
	}

	// Find the index of recommended base branch
	for i, status := range msg.branchStatuses {
		if status.Name == msg.recommendedBase {
			m.createState.selectedBranch = i
			break
		}
	}
}

// setupDeleteState initializes delete state
func (m *Model) setupDeleteState() {
	m.deleteState = &DeleteState{
		currentStep:       DeleteStepSelection,
		selectedWorktrees: make([]int, 0),
		warnings:          make([]string, 0),
	}
}

// setupDeleteStateForWorktree initializes delete state for specific worktree
func (m *Model) setupDeleteStateForWorktree(worktree Worktree) {
	m.deleteState = &DeleteState{
		currentStep:       DeleteStepConfirm,
		selectedWorktrees: []int{}, // Will be handled differently for single worktree
		warnings:          make([]string, 0),
		targetWorktree:    &worktree, // Store the specific worktree
	}
}

// initializeOpenInState initializes the OpenIn state with actions
func (m *Model) initializeOpenInStateFromMsg(msg openInInitializedMsg) {
	m.openInState = &OpenInState{
		worktreePath:    msg.worktreePath,
		actions:         msg.actions,
		selectedIndex:   0,
	}
}

// detectPackageManager detects the package manager used in the project
func (m Model) detectPackageManager() string {
	if _, err := os.Stat("package.json"); err == nil {
		if _, err := os.Stat("yarn.lock"); err == nil {
			return "yarn"
		} else if _, err := os.Stat("pnpm-lock.yaml"); err == nil {
			return "pnpm"
		}
		return "npm"
	}

	if _, err := os.Stat("go.mod"); err == nil {
		return "go"
	}

	if _, err := os.Stat("Cargo.toml"); err == nil {
		return "cargo"
	}

	if _, err := os.Stat("requirements.txt"); err == nil {
		return "python"
	}
	if _, err := os.Stat("pyproject.toml"); err == nil {
		return "python"
	}

	return "unknown"
}

// detectPostCreateCommand detects appropriate post-create command
func (m Model) detectPostCreateCommand() string {
	packageManager := m.detectPackageManager()

	switch packageManager {
	case "npm":
		return "npm install"
	case "yarn":
		return "yarn install"
	case "pnpm":
		return "pnpm install"
	case "go":
		return "go mod download"
	case "cargo":
		return "cargo build"
	case "python":
		if _, err := os.Stat("requirements.txt"); err == nil {
			return "pip install -r requirements.txt"
		}
		return "pip install -e ."
	default:
		return "echo 'No package manager detected'"
	}
}

// generateSetupScript generates the post-create setup script
func (m Model) generateSetupScript() string {
	var script strings.Builder

	script.WriteString("#!/bin/bash\n")
	script.WriteString("# Auto-generated post-create script for gren worktrees\n")
	script.WriteString("# This script runs after creating a new worktree\n\n")
	script.WriteString("set -e  # Exit on any error\n\n")

	script.WriteString("echo 'Setting up worktree...'\n\n")

	// Copy environment files
	if m.initState != nil && len(m.initState.copyPatterns) > 0 {
		script.WriteString("# Copy configuration files\n")
		copyCommands := m.generateCopyCommands([]string{})
		for _, cmd := range copyCommands {
			script.WriteString(fmt.Sprintf("%s\n", cmd))
		}
		script.WriteString("\n")
	}

	// Install dependencies
	installCmd := m.detectPostCreateCommand()
	script.WriteString("# Install dependencies\n")
	script.WriteString(fmt.Sprintf("echo 'Running: %s'\n", installCmd))
	script.WriteString(fmt.Sprintf("%s\n\n", installCmd))

	script.WriteString("echo 'Worktree setup complete!'\n")

	return script.String()
}

// generateCopyCommands generates copy commands for patterns
func (m Model) generateCopyCommands(patterns []string) []string {
	var commands []string

	// Default patterns if none provided
	if len(patterns) == 0 {
		patterns = []string{".env*", ".nvmrc", ".node-version"}
	}

	for _, pattern := range patterns {
		// Simple copy command - real implementation would be more sophisticated
		commands = append(commands, fmt.Sprintf("cp -f ../%s . 2>/dev/null || true", pattern))
	}

	return commands
}

// generateConfigFile generates the gren configuration file
func (m Model) generateConfigFile() string {
	config := `# Gren Worktree Manager Configuration
worktree_dir: ../gren-worktrees
copy_patterns:
  - .env*
  - .nvmrc
  - .node-version
post_create_script: .gren/post-create.sh
`
	return config
}

// createAndOpenScript creates script files and optionally opens them in editor
func (m Model) createAndOpenScript() tea.Cmd {
	return func() tea.Msg {
		// Generate scripts
		postCreateScript := m.generateSetupScript()
		configFile := m.generateConfigFile()

		// Create .gren directory
		if err := os.MkdirAll(".gren", 0755); err != nil {
			return scriptCreateCompleteMsg{err: err}
		}

		// Write post-create script
		scriptPath := ".gren/post-create.sh"
		if err := os.WriteFile(scriptPath, []byte(postCreateScript), 0755); err != nil {
			return scriptCreateCompleteMsg{err: err}
		}

		// Write config file
		configPath := ".gren/config.yml"
		if err := os.WriteFile(configPath, []byte(configFile), 0644); err != nil {
			return scriptCreateCompleteMsg{err: err}
		}

		return scriptCreateCompleteMsg{}
	}
}

// createScriptFiles creates the script files without opening editor
func (m Model) createScriptFiles() tea.Cmd {
	return m.createAndOpenScript()
}

// openScriptInEditor opens the post-create script in an external editor
func (m Model) openScriptInEditor() tea.Cmd {
	return m.openPostCreateScript()
}

// needsGitignoreUpdate checks if .gitignore needs to be updated
func needsGitignoreUpdate() bool {
	gitignoreContent, err := os.ReadFile(".gitignore")
	if err != nil {
		return true // If .gitignore doesn't exist or can't be read
	}

	content := string(gitignoreContent)
	return !strings.Contains(content, ".gren/")
}