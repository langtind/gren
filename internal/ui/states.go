package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/langtind/gren/internal/config"
	"github.com/langtind/gren/internal/git"
)

// NewModel creates a new Model with the given dependencies
func NewModel(gitRepo git.Repository, configManager *config.Manager, version string) Model {
	return Model{
		currentView:   DashboardView,
		gitRepo:       gitRepo,
		configManager: configManager,
		keys:          DefaultKeyMap(),
		selected:      0,
		version:       version,
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
		} else if strings.HasPrefix(line, "prunable") && currentWorktree != nil {
			// Mark worktree as missing/prunable
			currentWorktree.Status = "missing"
		}
	}

	// Add the last worktree if exists
	if currentWorktree != nil {
		worktrees = append(worktrees, *currentWorktree)
	}

	// Mark current worktree and get status for all worktrees
	if len(worktrees) > 0 {
		cwd, _ := os.Getwd()
		for i := range worktrees {
			isCurrent := worktrees[i].Path == cwd
			worktrees[i].IsCurrent = isCurrent
			// Get status for all worktrees (not just current)
			if status, err := m.getWorktreeStatus(worktrees[i].Path, isCurrent); err == nil {
				worktrees[i].Status = status
			}
		}
	}

	return worktrees
}

// WorktreeStatusInfo contains detailed status information for a worktree
type WorktreeStatusInfo struct {
	Status              string // "clean", "modified", "untracked", "mixed", "unpushed", "missing"
	HasModified         bool
	HasUntracked        bool
	UnpushedCount       int
	IsNotPushedToRemote bool // Branch doesn't exist on remote at all
}

// getWorktreeStatus gets the git status for a worktree
func (m Model) getWorktreeStatus(worktreePath string, isCurrent bool) (string, error) {
	info := m.getWorktreeStatusInfo(worktreePath, isCurrent)
	return info.Status, nil
}

// getWorktreeStatusInfo gets detailed status information for a worktree
func (m Model) getWorktreeStatusInfo(worktreePath string, isCurrent bool) WorktreeStatusInfo {
	info := WorktreeStatusInfo{Status: "clean"}

	// Get git status for uncommitted/untracked changes
	var cmd *exec.Cmd
	if isCurrent {
		cmd = exec.Command("git", "status", "--porcelain")
	} else {
		cmd = exec.Command("git", "-C", worktreePath, "status", "--porcelain")
	}

	output, err := cmd.Output()
	if err == nil {
		statusLines := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, line := range statusLines {
			if len(line) < 2 {
				continue
			}
			// Check staged/modified status
			if line[0] != ' ' && line[0] != '?' {
				info.HasModified = true
			}
			if line[1] != ' ' && line[1] != '?' {
				info.HasModified = true
			}
			// Check untracked status
			if line[0] == '?' && line[1] == '?' {
				info.HasUntracked = true
			}
		}
	}

	// Check for unpushed commits
	var unpushedCmd *exec.Cmd
	if isCurrent {
		unpushedCmd = exec.Command("git", "log", "@{u}..HEAD", "--oneline")
	} else {
		unpushedCmd = exec.Command("git", "-C", worktreePath, "log", "@{u}..HEAD", "--oneline")
	}

	unpushedOutput, err := unpushedCmd.Output()
	if err != nil {
		// No upstream - check if branch exists on remote
		var branchCmd *exec.Cmd
		if isCurrent {
			branchCmd = exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
		} else {
			branchCmd = exec.Command("git", "-C", worktreePath, "rev-parse", "--abbrev-ref", "HEAD")
		}
		branchOutput, _ := branchCmd.Output()
		branch := strings.TrimSpace(string(branchOutput))

		if branch != "" && branch != "HEAD" {
			// Check if branch exists on remote
			var remoteCheckCmd *exec.Cmd
			if isCurrent {
				remoteCheckCmd = exec.Command("git", "rev-parse", "--verify", "origin/"+branch)
			} else {
				remoteCheckCmd = exec.Command("git", "-C", worktreePath, "rev-parse", "--verify", "origin/"+branch)
			}
			if remoteCheckCmd.Run() != nil {
				info.IsNotPushedToRemote = true
			}
		}
	} else {
		unpushedLines := strings.Split(strings.TrimSpace(string(unpushedOutput)), "\n")
		if len(unpushedLines) > 0 && unpushedLines[0] != "" {
			info.UnpushedCount = len(unpushedLines)
		}
	}

	// Determine overall status
	if info.HasModified && info.HasUntracked {
		info.Status = "mixed"
	} else if info.HasModified {
		info.Status = "modified"
	} else if info.HasUntracked {
		info.Status = "untracked"
	} else if info.UnpushedCount > 0 || info.IsNotPushedToRemote {
		info.Status = "unpushed"
	}

	return info
}

// setupCreateState initializes create state from message
func (m *Model) setupCreateState(msg createInitMsg) {
	m.createState = &CreateState{
		currentStep:       CreateStepBranchMode,
		createMode:        CreateModeNewBranch,
		branchName:        "",
		baseBranch:        msg.recommendedBase,
		branchStatuses:    msg.branchStatuses,
		filteredBranches:  msg.branchStatuses, // Initialize with all branches
		availableBranches: []BranchStatus{},   // Will be populated when needed
		selectedBranch:    0,
		scrollOffset:      0,
		searchQuery:       "",
		selectedMode:      0, // Default to "Create new branch"
		showWarning:       false,
		warningAccepted:   false,
		selectedAction:    0,
	}

	// Find the index of recommended base branch
	for i, status := range msg.branchStatuses {
		if status.Name == msg.recommendedBase {
			m.createState.selectedBranch = i
			break
		}
	}

	// Calculate scroll offset to center the selected branch in the visible window
	m.centerScrollOnSelectedBranch()
}

// filterBranches filters the branch list based on the search query (fzf-like)
func (m *Model) filterBranches() {
	if m.createState == nil {
		return
	}

	query := strings.ToLower(m.createState.searchQuery)
	if query == "" {
		m.createState.filteredBranches = m.createState.branchStatuses
		m.createState.selectedBranch = 0
		m.centerScrollOnSelectedBranch()
		return
	}

	// Filter branches that contain the search query (case-insensitive)
	var filtered []BranchStatus
	for _, status := range m.createState.branchStatuses {
		if strings.Contains(strings.ToLower(status.Name), query) {
			filtered = append(filtered, status)
		}
	}

	m.createState.filteredBranches = filtered
	m.createState.selectedBranch = 0
	m.createState.scrollOffset = 0
}

// filterAvailableBranches filters the available branches list based on the search query (fzf-like)
func (m *Model) filterAvailableBranches() {
	if m.createState == nil {
		return
	}

	query := strings.ToLower(m.createState.searchQuery)
	if query == "" {
		m.createState.filteredAvailableBranches = m.createState.availableBranches
		m.createState.selectedBranch = 0
		m.centerScrollOnSelectedBranch()
		return
	}

	// Filter branches that contain the search query (case-insensitive)
	var filtered []BranchStatus
	for _, status := range m.createState.availableBranches {
		if strings.Contains(strings.ToLower(status.Name), query) {
			filtered = append(filtered, status)
		}
	}

	m.createState.filteredAvailableBranches = filtered
	m.createState.selectedBranch = 0
	m.createState.scrollOffset = 0
}

// centerScrollOnSelectedBranch calculates scroll offset to center the selected branch
func (m *Model) centerScrollOnSelectedBranch() {
	if m.createState == nil {
		return
	}

	// Calculate visible window size (same calculation as in renderBaseBranchStep)
	maxVisible := m.height - 15
	if maxVisible < 5 {
		maxVisible = 5
	}
	if maxVisible > 20 {
		maxVisible = 20
	}

	branches := m.createState.filteredBranches
	if len(branches) == 0 {
		branches = m.createState.branchStatuses
	}
	totalBranches := len(branches)

	// Center the selected branch in the visible window
	selectedIdx := m.createState.selectedBranch
	halfVisible := maxVisible / 2

	// Calculate offset to put selected branch in the middle
	offset := selectedIdx - halfVisible
	if offset < 0 {
		offset = 0
	}
	// Don't scroll past the end
	if offset > totalBranches-maxVisible {
		offset = totalBranches - maxVisible
	}
	if offset < 0 {
		offset = 0
	}

	m.createState.scrollOffset = offset
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
		worktreePath:  msg.worktreePath,
		actions:       msg.actions,
		selectedIndex: 0,
	}
}

// detectPackageManager detects the package manager used in the project
func (m Model) detectPackageManager() string {
	if _, err := os.Stat("package.json"); err == nil {
		// Check for bun lock files first
		if _, err := os.Stat("bun.lockb"); err == nil {
			return "bun"
		} else if _, err := os.Stat("bun.lock"); err == nil {
			return "bun"
		} else if _, err := os.Stat("yarn.lock"); err == nil {
			return "yarn"
		} else if _, err := os.Stat("pnpm-lock.yaml"); err == nil {
			return "pnpm"
		}

		// Check for packageManager field in package.json as a fallback
		if data, err := os.ReadFile("package.json"); err == nil {
			if strings.Contains(string(data), "\"packageManager\": \"bun@") {
				return "bun"
			}
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
		return "python (pip)"
	}
	if _, err := os.Stat("pyproject.toml"); err == nil {
		return "python (pip)"
	}

	if _, err := os.Stat("Makefile"); err == nil {
		return "make"
	}

	// Check for common project types
	if _, err := os.Stat("README.md"); err == nil {
		return "generic project"
	}

	return "no package manager detected"
}

// detectPostCreateCommand detects appropriate post-create command
func (m Model) detectPostCreateCommand() string {
	packageManager := m.detectPackageManager()

	switch packageManager {
	case "bun":
		return "bun install"
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
	case "python (pip)":
		if _, err := os.Stat("requirements.txt"); err == nil {
			return "pip install -r requirements.txt"
		}
		return "pip install -e ."
	case "make":
		return "make install"
	case "generic project":
		return "" // No setup command needed
	default:
		return "" // No setup command needed
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

	// Symlink environment files
	if m.initState != nil && len(m.initState.detectedFiles) > 0 {
		script.WriteString("# Symlink configuration files\n")
		for _, file := range m.initState.detectedFiles {
			script.WriteString(fmt.Sprintf("ln -sf \"$REPO_ROOT/%s\" . 2>/dev/null || true\n", file.Path))
		}
		script.WriteString("\n")
	}

	// Install dependencies
	installCmd := m.detectPostCreateCommand()
	if installCmd != "" {
		script.WriteString("# Install dependencies\n")
		script.WriteString(fmt.Sprintf("echo 'Running: %s'\n", installCmd))
		script.WriteString(fmt.Sprintf("%s\n\n", installCmd))
	}

	script.WriteString("echo 'Worktree setup complete!'\n")

	return script.String()
}

// generateConfigFile generates the gren configuration file
func (m Model) generateConfigFile() string {
	config := `# Gren Worktree Manager Configuration
worktree_dir: ../gren-worktrees
post_create_hook: .gren/post-create.sh
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
		configPath := ".gren/config.json"
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
