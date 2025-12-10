package ui

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/langtind/gren/internal/config"
	"github.com/langtind/gren/internal/core"
	"github.com/langtind/gren/internal/logging"
)

// loadProjectInfo loads project information asynchronously
func (m Model) loadProjectInfo() tea.Cmd {
	return func() tea.Msg {
		if m.gitRepo == nil {
			return projectInfoMsg{info: nil, err: fmt.Errorf("git repository not initialized")}
		}
		info, err := m.gitRepo.GetRepoInfo(context.Background())
		return projectInfoMsg{info: info, err: err}
	}
}

// runInitialize initializes create/delete state
func (m Model) runInitialize() tea.Cmd {
	return func() tea.Msg {
		branchStatuses, err := m.getBranchStatuses()
		return initializeMsg{branchStatuses: branchStatuses, err: err}
	}
}

// initializeCreateState initializes the create worktree state
func (m Model) initializeCreateState() tea.Cmd {
	return m.initializeCreateStateWithBase("")
}

func (m Model) initializeCreateStateWithBase(suggestedBase string) tea.Cmd {
	return func() tea.Msg {
		branchStatuses, err := m.getBranchStatuses()
		if err != nil {
			return createInitMsg{err: err}
		}

		// Find recommended base branch
		var recommendedBase string

		// First, try using the suggested base if provided and exists
		if suggestedBase != "" {
			for _, status := range branchStatuses {
				if status.Name == suggestedBase {
					recommendedBase = suggestedBase
					break
				}
			}
		}

		// If no suggested base or suggested base not found, use current branch
		if recommendedBase == "" {
			for _, status := range branchStatuses {
				if status.IsCurrent {
					recommendedBase = status.Name
					break
				}
			}
		}

		// Fall back to main or master
		if recommendedBase == "" {
			for _, status := range branchStatuses {
				if status.Name == "main" || status.Name == "master" {
					recommendedBase = status.Name
					break
				}
			}
		}

		// Last resort: use first branch
		if recommendedBase == "" && len(branchStatuses) > 0 {
			recommendedBase = branchStatuses[0].Name
		}

		return createInitMsg{
			branchStatuses:  branchStatuses,
			recommendedBase: recommendedBase,
		}
	}
}

// initializeDeleteState initializes the delete worktree state
func (m Model) initializeDeleteState() tea.Cmd {
	return func() tea.Msg {
		return deleteInitMsg{}
	}
}

// initializeDeleteStateForWorktree initializes delete state for a specific worktree
func (m Model) initializeDeleteStateForWorktree(worktree Worktree) tea.Cmd {
	return func() tea.Msg {
		return deleteInitMsg{selectedWorktree: &worktree}
	}
}

// loadAvailableBranches loads branches available for existing worktree creation
func (m Model) loadAvailableBranches() tea.Cmd {
	return func() tea.Msg {
		// Get all branches (local and remote)
		branchStatuses, err := m.getAvailableBranchesForWorktree()
		if err != nil {
			return availableBranchesLoadedMsg{err: err}
		}

		return availableBranchesLoadedMsg{
			branches: branchStatuses,
		}
	}
}

// initializeConfigState initializes the config view state
func (m Model) initializeConfigState() tea.Cmd {
	return func() tea.Msg {
		var files []ConfigFile

		// Check for config.json
		if _, err := os.Stat(".gren/config.json"); err == nil {
			files = append(files, ConfigFile{
				Name:        "config.json",
				Path:        ".gren/config.json",
				Icon:        "📄",
				Description: "Main configuration file for gren settings",
			})
		}

		// Check for post-create.sh
		if _, err := os.Stat(".gren/post-create.sh"); err == nil {
			files = append(files, ConfigFile{
				Name:        "post-create.sh",
				Path:        ".gren/post-create.sh",
				Icon:        "📜",
				Description: "Script that runs after creating new worktrees",
			})
		}

		return configInitializedMsg{files: files}
	}
}

// getAvailableBranchesForWorktree gets branches that can be used for worktrees
func (m Model) getAvailableBranchesForWorktree() ([]BranchStatus, error) {
	logging.Debug(" Starting getAvailableBranchesForWorktree")

	// Get all local branches
	cmd := exec.Command("git", "branch", "-v")
	output, err := cmd.Output()
	if err != nil {
		logging.Debug(" git branch -v failed: %v", err)
		return nil, fmt.Errorf("failed to run 'git branch -v': %w", err)
	}

	outputStr := string(output)
	logging.Debug(" git branch -v output: %q", outputStr)

	if strings.TrimSpace(outputStr) == "" {
		return nil, fmt.Errorf("git branch command returned empty output")
	}

	var branches []BranchStatus
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	// Get existing worktree branches for reference (but don't filter them out completely)
	existingWorktreeBranches := make(map[string]bool)
	for _, wt := range m.worktrees {
		existingWorktreeBranches[wt.Branch] = true
	}
	logging.Debug(" Existing worktree branches: %v", existingWorktreeBranches)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse branch line format: "* main abc123 commit message" or "  feature def456 commit message"
		isCurrent := strings.HasPrefix(line, "* ")

		// Remove prefix and extract branch name safely
		var branchName string
		if isCurrent {
			// Remove "* " and get first field
			parts := strings.Fields(line[2:])
			if len(parts) < 2 {
				continue
			}
			branchName = parts[0]
		} else {
			// For non-current branches, skip leading spaces and get first field
			parts := strings.Fields(line)
			if len(parts) < 2 {
				continue
			}
			branchName = parts[0]
		}
		logging.Debug(" Processing branch: %s, isCurrent: %v", branchName, isCurrent)

		// Skip if this branch already has a worktree
		if existingWorktreeBranches[branchName] {
			logging.Debug(" Skipping branch %s - already has worktree", branchName)
			continue
		}

		// Validate that branch exists and is a valid reference
		validateCmd := exec.Command("git", "rev-parse", "--verify", branchName)
		if err := validateCmd.Run(); err != nil {
			logging.Debug(" Branch %s failed validation: %v", branchName, err)
			// Skip invalid branches
			continue
		}
		logging.Debug(" Branch %s passed validation", branchName)

		// Get detailed status for this branch
		branchStatus := BranchStatus{
			Name:             branchName,
			IsClean:          true, // Simplified for now
			UncommittedFiles: 0,
			UntrackedFiles:   0,
			IsCurrent:        isCurrent,
			AheadCount:       0,
			BehindCount:      0,
		}

		branches = append(branches, branchStatus)
		logging.Debug(" Added branch: %s", branchName)
	}

	logging.Debug(" Final branches count: %d", len(branches))
	for i, b := range branches {
		logging.Debug(" Branch %d: %s", i, b.Name)
	}

	// Debug: return info about what we found
	if len(branches) == 0 {
		totalLines := len(lines)
		existingCount := len(existingWorktreeBranches)
		return nil, fmt.Errorf("no available branches found - processed %d lines, %d existing worktrees", totalLines, existingCount)
	}

	return branches, nil
}

// getBranchStatuses gets the status of all branches
func (m Model) getBranchStatuses() ([]BranchStatus, error) {
	if m.gitRepo == nil {
		return nil, fmt.Errorf("git repository not initialized")
	}

	ctx := context.Background()
	gitStatuses, err := m.gitRepo.GetBranchStatuses(ctx)
	if err != nil {
		return nil, err
	}

	// Convert git.BranchStatus to ui.BranchStatus
	var statuses []BranchStatus
	for _, gs := range gitStatuses {
		statuses = append(statuses, BranchStatus{
			Name:             gs.Name,
			IsClean:          gs.IsClean,
			UncommittedFiles: gs.UncommittedFiles,
			UntrackedFiles:   gs.UntrackedFiles,
			IsCurrent:        gs.IsCurrent,
			AheadCount:       gs.AheadCount,
			BehindCount:      gs.BehindCount,
		})
	}

	return statuses, nil
}

// runProjectAnalysis analyzes the project for initialization
func (m Model) runProjectAnalysis() tea.Cmd {
	return func() tea.Msg {
		// This would run actual analysis
		return projectAnalysisCompleteMsg{}
	}
}

// createWorktree creates the actual worktree using WorktreeManager
func (m Model) createWorktree() tea.Cmd {
	return func() tea.Msg {
		if m.createState == nil {
			logging.Error("Create worktree failed: create state is nil")
			return worktreeCreatedMsg{err: fmt.Errorf("create state is nil")}
		}

		branchName := m.createState.branchName
		baseBranch := m.createState.baseBranch
		isNewBranch := m.createState.createMode == CreateModeNewBranch

		logging.Info("Creating worktree: branch=%s, base=%s, isNew=%v", branchName, baseBranch, isNewBranch)

		// Use WorktreeManager to create worktree (same logic as CLI)
		req := core.CreateWorktreeRequest{
			Name:        branchName,
			Branch:      branchName,
			BaseBranch:  baseBranch,
			IsNewBranch: isNewBranch,
			WorktreeDir: "", // Let WorktreeManager determine from config
		}

		ctx := context.Background()
		worktreeManager := core.NewWorktreeManager(m.gitRepo, m.configManager)
		if err := worktreeManager.CreateWorktree(ctx, req); err != nil {
			logging.Error("Create worktree failed: %v", err)
			return worktreeCreatedMsg{err: err}
		}

		logging.Info("Successfully created worktree: %s", branchName)
		return worktreeCreatedMsg{branchName: branchName}
	}
}

// switchToWorktree opens a new terminal in the worktree directory
func (m Model) switchToWorktree(worktreePath string) tea.Cmd {
	return func() tea.Msg {
		// Detect terminal and open new window/tab in worktree directory
		termProgram := os.Getenv("TERM_PROGRAM")
		var cmd *exec.Cmd

		switch termProgram {
		case "WarpTerminal":
			// Warp terminal
			cmd = exec.Command("warp-cli", "open", worktreePath)
		case "iTerm.app":
			// iTerm2
			script := fmt.Sprintf(`tell application "iTerm"
				create window with default profile
				tell current session of current window
					write text "cd %s"
				end tell
			end tell`, worktreePath)
			cmd = exec.Command("osascript", "-e", script)
		case "Apple_Terminal":
			// macOS Terminal
			script := fmt.Sprintf(`tell application "Terminal"
				do script "cd %s"
			end tell`, worktreePath)
			cmd = exec.Command("osascript", "-e", script)
		default:
			// Fallback: just open the directory
			cmd = exec.Command("open", worktreePath)
		}

		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to open terminal: %w", err)
		}

		return nil
	}
}

// deleteSelectedWorktrees deletes the selected worktrees
func (m Model) deleteSelectedWorktrees() tea.Cmd {
	return func() tea.Msg {
		if m.deleteState == nil {
			return worktreeDeletedMsg{err: fmt.Errorf("delete state is nil")}
		}

		deletedCount := 0

		// Helper function to delete a single worktree
		deleteWorktree := func(worktree Worktree) error {
			logging.Info("Deleting worktree: %s (path: %s)", worktree.Name, worktree.Path)

			// 1. Remove symlinks that point outside the worktree (they cause issues with git worktree remove)
			// This includes .gren, .env files, etc. that may have been symlinked by post-create hooks
			entries, err := os.ReadDir(worktree.Path)
			if err == nil {
				for _, entry := range entries {
					entryPath := filepath.Join(worktree.Path, entry.Name())
					if info, err := os.Lstat(entryPath); err == nil && info.Mode()&os.ModeSymlink != 0 {
						// It's a symlink - check if it points outside the worktree
						target, err := os.Readlink(entryPath)
						if err == nil {
							// Resolve to absolute path
							if !filepath.IsAbs(target) {
								target = filepath.Join(worktree.Path, target)
							}
							absTarget, _ := filepath.Abs(target)
							absWorktree, _ := filepath.Abs(worktree.Path)
							// If symlink points outside worktree, remove it
							if !strings.HasPrefix(absTarget, absWorktree) {
								logging.Debug("Removing external symlink: %s -> %s", entryPath, absTarget)
								if err := os.Remove(entryPath); err != nil {
									logging.Warn("Failed to remove symlink %s: %v", entryPath, err)
								}
							}
						}
					}
				}
			}

			// 2. Deinit submodules and track if worktree has submodules
			hasSubmodules := false
			if _, err := os.Stat(filepath.Join(worktree.Path, ".gitmodules")); err == nil {
				hasSubmodules = true
				logging.Debug("Worktree has submodules, running deinit")
				cmd := exec.Command("git", "-C", worktree.Path, "submodule", "deinit", "--all", "--force")
				output, err := cmd.CombinedOutput()
				if err != nil {
					logging.Error("Failed to deinit submodules: %v, output: %s", err, string(output))
					return fmt.Errorf("failed to deinit submodules in worktree '%s': %w\n\nThis can happen if submodules have uncommitted changes.\nTry running manually:\n  cd %s\n  git submodule deinit --all --force\n\nOutput: %s",
						worktree.Name, err, worktree.Path, string(output))
				}
				logging.Debug("Submodules deinited successfully")
			}

			// 3. Remove worktree using git
			// Note: --force is required for worktrees with submodules (even after deinit)
			var cmd *exec.Cmd
			if hasSubmodules {
				logging.Debug("Running: git worktree remove --force %s", worktree.Path)
				cmd = exec.Command("git", "worktree", "remove", "--force", worktree.Path)
			} else {
				logging.Debug("Running: git worktree remove %s", worktree.Path)
				cmd = exec.Command("git", "worktree", "remove", worktree.Path)
			}
			output, err := cmd.CombinedOutput()
			if err != nil {
				logging.Error("Failed to remove worktree: %v, output: %s", err, string(output))
				return fmt.Errorf("failed to remove worktree '%s': %w\n\nOutput: %s\n\nIf the worktree has uncommitted changes, commit or stash them first.",
					worktree.Name, err, string(output))
			}

			logging.Info("Successfully deleted worktree: %s", worktree.Name)
			return nil
		}

		// Handle single worktree deletion
		if m.deleteState.targetWorktree != nil {
			worktree := *m.deleteState.targetWorktree

			if err := deleteWorktree(worktree); err != nil {
				return worktreeDeletedMsg{err: fmt.Errorf("failed to remove worktree %s: %w", worktree.Name, err)}
			}

			deletedCount = 1
		} else {
			// Handle multi-select deletion
			for _, selectedIdx := range m.deleteState.selectedWorktrees {
				if selectedIdx < len(m.worktrees) {
					worktree := m.worktrees[selectedIdx]

					if err := deleteWorktree(worktree); err != nil {
						return worktreeDeletedMsg{err: fmt.Errorf("failed to remove worktree %s: %w", worktree.Name, err)}
					}

					deletedCount++
				}
			}
		}

		return worktreeDeletedMsg{deletedCount: deletedCount}
	}
}

// initializeOpenInState creates and initializes the "Open in..." state
func (m Model) initializeOpenInState(worktreePath string) tea.Cmd {
	return func() tea.Msg {
		// Use the consolidated action generation logic
		availableActions := m.getActionsForPath(worktreePath, "Back to dashboard")

		return openInInitializedMsg{
			worktreePath: worktreePath,
			actions:      availableActions,
		}
	}
}

// openPostCreateScript opens the post-create script in an external editor
func (m Model) openPostCreateScript() tea.Cmd {
	scriptPath := ".gren/post-create.sh"

	// First check EDITOR environment variable
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}

	// If no env var set, try common editors
	if editor == "" {
		fallbackEditors := []string{"code", "zed", "vim", "nano"}
		for _, e := range fallbackEditors {
			if isCommandAvailable(e) {
				editor = e
				break
			}
		}
	}

	if editor == "" {
		return func() tea.Msg {
			return scriptEditCompleteMsg{err: fmt.Errorf("no editor found. Set EDITOR environment variable")}
		}
	}

	// Check if it's a terminal editor (vim, nvim, nano, etc.)
	terminalEditors := map[string]bool{
		"vim": true, "nvim": true, "vi": true, "nano": true, "emacs": true, "helix": true, "hx": true,
	}

	// Get the base command name (in case EDITOR contains a path)
	editorBase := filepath.Base(editor)

	if terminalEditors[editorBase] {
		// Use tea.ExecProcess for terminal editors - this suspends the TUI
		cmd := exec.Command(editor, scriptPath)
		return tea.ExecProcess(cmd, func(err error) tea.Msg {
			return scriptEditCompleteMsg{err: err}
		})
	}

	// For GUI editors, just start them in background
	return func() tea.Msg {
		cmd := exec.Command(editor, scriptPath)
		err := cmd.Start()
		return scriptEditCompleteMsg{err: err}
	}
}

// commitConfiguration commits the .gren configuration to git
func (m Model) commitConfiguration() tea.Cmd {
	return func() tea.Msg {
		// Add .gren directory to git
		cmd := exec.Command("git", "add", ".gren/")
		if err := cmd.Run(); err != nil {
			return commitCompleteMsg{err: fmt.Errorf("failed to add .gren to git: %w", err)}
		}

		// Commit the changes
		cmd = exec.Command("git", "commit", "-m", "Add gren worktree configuration")
		err := cmd.Run()
		return commitCompleteMsg{err: err}
	}
}

// runInitialization runs the actual initialization process
func (m Model) runInitialization() tea.Cmd {
	return func() tea.Msg {
		// Get project name from repo info
		projectName := "project"
		if m.repoInfo != nil {
			projectName = m.repoInfo.Name
		}

		// Use the same initialization logic as CLI
		result := config.Initialize(projectName)
		if result.Error != nil {
			return initExecutionCompleteMsg{err: result.Error}
		}

		return initExecutionCompleteMsg{
			configCreated: result.ConfigCreated,
			hookCreated:   result.HookCreated,
			message:       result.Message,
		}
	}
}

// Additional helper functions for initialization and project analysis

// analyzeProject analyzes the current project structure
func (m Model) analyzeProject() []DetectedFile {
	var files []DetectedFile

	// Detect all .env files using glob pattern
	envFiles, err := filepath.Glob(".env*")
	if err == nil {
		for _, envFile := range envFiles {
			files = append(files, DetectedFile{
				Path:         envFile,
				Type:         "env",
				IsGitIgnored: m.isGitIgnored(envFile),
				Description:  getFileDescription(envFile, "env"),
			})
		}
	}

	// Common patterns to look for (excluding env files which we handle above)
	patterns := map[string]string{
		"package.json":  "config",
		"go.mod":        "config",
		"Cargo.toml":    "config",
		".nvmrc":        "tool",
		".node-version": "tool",
		".envrc":        "tool",
	}

	for pattern, fileType := range patterns {
		if _, err := os.Stat(pattern); err == nil {
			files = append(files, DetectedFile{
				Path:         pattern,
				Type:         fileType,
				IsGitIgnored: m.isGitIgnored(pattern),
				Description:  getFileDescription(pattern, fileType),
			})
		}
	}

	// Sort by type and name
	sort.Slice(files, func(i, j int) bool {
		if files[i].Type != files[j].Type {
			return files[i].Type < files[j].Type
		}
		return files[i].Path < files[j].Path
	})

	return files
}

// getFileDescription returns a human-readable description for a file
func getFileDescription(path, fileType string) string {
	switch path {
	case ".env":
		return "Environment variables"
	case ".env.local":
		return "Local environment variables"
	case ".env.example":
		return "Environment variables template"
	case ".env.prod":
		return "Production environment variables"
	case ".env.preview":
		return "Preview environment variables"
	case ".envrc":
		return "Direnv configuration"
	case "package.json":
		return "Node.js dependencies"
	case "go.mod":
		return "Go module definition"
	case "Cargo.toml":
		return "Rust dependencies"
	case ".nvmrc":
		return "Node.js version"
	case ".node-version":
		return "Node.js version"
	default:
		if fileType == "env" {
			return "Environment variables"
		}
		return fmt.Sprintf("%s file", strings.Title(fileType))
	}
}

// isGitIgnored checks if a file is git ignored
func (m Model) isGitIgnored(filename string) bool {
	cmd := exec.Command("git", "check-ignore", filename)
	err := cmd.Run()
	return err == nil // If command succeeds, file is ignored
}

// parseGitIgnore parses .gitignore file and returns patterns
func (m Model) parseGitIgnore() []string {
	file, err := os.Open(".gitignore")
	if err != nil {
		return nil
	}
	defer file.Close()

	var patterns []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			patterns = append(patterns, line)
		}
	}

	return patterns
}

// initializeActionsList creates and configures the list.Model for post-create actions
func (m *Model) initializeActionsList() {
	if m.createState == nil {
		return
	}

	actions := m.getAvailableActions()
	items := make([]list.Item, len(actions))
	for i, action := range actions {
		items[i] = action
	}

	// Create the list with dynamic sizing
	width := 50
	if m.width > 0 {
		width = m.width - 4
		if width < 40 {
			width = 40
		}
	}
	height := len(items) + 3 // Items + title + padding
	if m.height > 0 && height > m.height-6 {
		height = m.height - 6
	}
	if height < 5 {
		height = 5
	}

	// Create custom delegate for better item rendering
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = WorktreeSelectedStyle
	delegate.Styles.SelectedDesc = WorktreePathStyle
	delegate.Styles.NormalTitle = WorktreeNameStyle
	delegate.Styles.NormalDesc = WorktreePathStyle

	l := list.New(items, delegate, width, height)
	l.Title = "What would you like to do next?"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)

	// Custom styling to match gren design
	l.Styles.Title = TitleStyle
	l.Styles.PaginationStyle = HelpStyle
	l.Styles.HelpStyle = HelpStyle

	m.createState.actionsList = l
}

// openConfigFile opens a configuration file in an available editor
func (m Model) openConfigFile(filePath string) tea.Cmd {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return func() tea.Msg {
			return configFileOpenedMsg{err: fmt.Errorf("config file does not exist: %s", filePath)}
		}
	}

	// First check EDITOR environment variable
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}

	// If no env var set, try common editors
	if editor == "" {
		fallbackEditors := []string{"code", "zed", "vim", "nano"}
		for _, e := range fallbackEditors {
			if _, err := exec.LookPath(e); err == nil {
				editor = e
				break
			}
		}
	}

	if editor == "" {
		return func() tea.Msg {
			return configFileOpenedMsg{err: fmt.Errorf("no editor found. Set EDITOR environment variable or install code/vim/nano")}
		}
	}

	// Check if it's a terminal editor (vim, nvim, nano, etc.)
	terminalEditors := map[string]bool{
		"vim": true, "nvim": true, "vi": true, "nano": true, "emacs": true, "helix": true, "hx": true,
	}

	editorBase := filepath.Base(editor)

	if terminalEditors[editorBase] {
		// Use tea.ExecProcess for terminal editors - this suspends the TUI
		cmd := exec.Command(editor, filePath)
		return tea.ExecProcess(cmd, func(err error) tea.Msg {
			return configFileOpenedMsg{err: err}
		})
	}

	// For GUI editors, just start them in background
	return func() tea.Msg {
		cmd := exec.Command(editor, filePath)
		if err := cmd.Start(); err != nil {
			return configFileOpenedMsg{err: fmt.Errorf("failed to open %s: %w", editor, err)}
		}
		return configFileOpenedMsg{err: nil}
	}
}

// pruneWorktrees removes missing/prunable worktrees from git tracking
func (m Model) pruneWorktrees() tea.Cmd {
	return func() tea.Msg {
		// Run git worktree prune to remove missing worktrees
		cmd := exec.Command("git", "worktree", "prune", "--verbose")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return pruneCompleteMsg{err: fmt.Errorf("failed to prune worktrees: %w", err)}
		}

		// Parse the output to count and list pruned worktrees
		var prunedPaths []string
		outputStr := string(output)
		lines := strings.Split(strings.TrimSpace(outputStr), "\n")

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && strings.Contains(line, "Removing") {
				// Extract path from "Removing worktrees/path: gitdir file points to non-existent location"
				parts := strings.SplitN(line, ":", 2)
				if len(parts) > 0 {
					path := strings.TrimSpace(strings.TrimPrefix(parts[0], "Removing"))
					if path != "" {
						prunedPaths = append(prunedPaths, path)
					}
				}
			}
		}

		return pruneCompleteMsg{
			err:         nil,
			prunedCount: len(prunedPaths),
			prunedPaths: prunedPaths,
		}
	}
}

// navigateToWorktree writes navigation command to temp file and quits TUI
func (m Model) navigateToWorktree(worktreePath string) tea.Cmd {
	return func() tea.Msg {
		// Write navigation command to temp file
		tempFile := "/tmp/gren_navigate"
		command := fmt.Sprintf("cd \"%s\"", worktreePath)

		if err := os.WriteFile(tempFile, []byte(command), 0644); err != nil {
			return fmt.Errorf("failed to write navigation command: %w", err)
		}

		// Quit the TUI to allow wrapper script to execute the navigation
		return tea.Quit()
	}
}

// generateAISetupScript generates a setup script using Claude CLI
func (m Model) generateAISetupScript() tea.Cmd {
	return func() tea.Msg {
		logging.Info("Starting AI setup script generation")

		// Check if Claude CLI is available
		claudePath, err := exec.LookPath("claude")
		if err != nil {
			// Try common locations
			possiblePaths := []string{
				"/usr/local/bin/claude",
				os.ExpandEnv("$HOME/.local/bin/claude"),
				"/opt/homebrew/bin/claude",
			}
			for _, p := range possiblePaths {
				if _, err := os.Stat(p); err == nil {
					claudePath = p
					break
				}
			}
			if claudePath == "" {
				return aiScriptGeneratedMsg{err: fmt.Errorf("Claude CLI not found. Install it with: npm install -g @anthropic-ai/claude-cli")}
			}
		}

		logging.Debug("Found Claude CLI at: %s", claudePath)

		// Gather project context
		var contextBuilder strings.Builder
		contextBuilder.WriteString("Analyze this project and generate a bash setup script for a new git worktree.\n\n")
		contextBuilder.WriteString("Project context:\n")

		// Detected files
		if m.initState != nil && len(m.initState.detectedFiles) > 0 {
			contextBuilder.WriteString("\nDetected files to consider:\n")
			for _, f := range m.initState.detectedFiles {
				gitIgnored := ""
				if f.IsGitIgnored {
					gitIgnored = " (gitignored)"
				}
				contextBuilder.WriteString(fmt.Sprintf("- %s%s\n", f.Path, gitIgnored))
			}
		}

		// Package manager
		if m.initState != nil && m.initState.packageManager != "" {
			contextBuilder.WriteString(fmt.Sprintf("\nDetected package manager: %s\n", m.initState.packageManager))
		}

		// Check for common project files
		projectFiles := []string{"package.json", "go.mod", "Cargo.toml", "requirements.txt", "pyproject.toml", "Makefile", ".envrc"}
		var foundFiles []string
		for _, f := range projectFiles {
			if _, err := os.Stat(f); err == nil {
				foundFiles = append(foundFiles, f)
			}
		}
		if len(foundFiles) > 0 {
			contextBuilder.WriteString(fmt.Sprintf("\nProject files found: %s\n", strings.Join(foundFiles, ", ")))
		}

		contextBuilder.WriteString(`
Requirements for the script:
1. Copy gitignored environment files (like .env*) from the main worktree using symlinks
2. Install dependencies using the detected package manager
3. Run any necessary build or setup commands
4. Handle direnv if .envrc exists
5. Be idempotent (safe to run multiple times)

Output ONLY the bash script content, no explanations. Start with #!/bin/bash
`)

		// Run Claude CLI with the prompt
		cmd := exec.Command(claudePath, "-p", contextBuilder.String())
		cmd.Dir, _ = os.Getwd()

		output, err := cmd.CombinedOutput()
		if err != nil {
			logging.Error("Claude CLI failed: %v, output: %s", err, string(output))
			return aiScriptGeneratedMsg{err: fmt.Errorf("Claude CLI failed: %s", string(output))}
		}

		script := strings.TrimSpace(string(output))

		// Basic validation - should start with shebang
		if !strings.HasPrefix(script, "#!/") {
			// Try to extract script from response if Claude added explanation
			lines := strings.Split(script, "\n")
			inScript := false
			var scriptLines []string
			for _, line := range lines {
				if strings.HasPrefix(line, "#!/") {
					inScript = true
				}
				if inScript {
					scriptLines = append(scriptLines, line)
				}
			}
			if len(scriptLines) > 0 {
				script = strings.Join(scriptLines, "\n")
			} else {
				script = "#!/bin/bash\n\n# AI-generated script\n" + script
			}
		}

		logging.Info("AI script generated successfully")
		return aiScriptGeneratedMsg{script: script}
	}
}
