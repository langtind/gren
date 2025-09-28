package ui

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/langtind/gren/internal/config"
	"github.com/langtind/gren/internal/core"
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
	log.Printf("DEBUG: Starting getAvailableBranchesForWorktree")

	// Get all local branches
	cmd := exec.Command("git", "branch", "-v")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("DEBUG: git branch -v failed: %v", err)
		return nil, fmt.Errorf("failed to run 'git branch -v': %w", err)
	}

	outputStr := string(output)
	log.Printf("DEBUG: git branch -v output: %q", outputStr)

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
	log.Printf("DEBUG: Existing worktree branches: %v", existingWorktreeBranches)

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
		log.Printf("DEBUG: Processing branch: %s, isCurrent: %v", branchName, isCurrent)

		// Skip if this branch already has a worktree
		if existingWorktreeBranches[branchName] {
			log.Printf("DEBUG: Skipping branch %s - already has worktree", branchName)
			continue
		}

		// Validate that branch exists and is a valid reference
		validateCmd := exec.Command("git", "rev-parse", "--verify", branchName)
		if err := validateCmd.Run(); err != nil {
			log.Printf("DEBUG: Branch %s failed validation: %v", branchName, err)
			// Skip invalid branches
			continue
		}
		log.Printf("DEBUG: Branch %s passed validation", branchName)

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
		log.Printf("DEBUG: Added branch: %s", branchName)
	}

	log.Printf("DEBUG: Final branches count: %d", len(branches))
	for i, b := range branches {
		log.Printf("DEBUG: Branch %d: %s", i, b.Name)
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
			return worktreeCreatedMsg{err: fmt.Errorf("create state is nil")}
		}

		branchName := m.createState.branchName
		baseBranch := m.createState.baseBranch

		// Use WorktreeManager to create worktree (same logic as CLI)
		req := core.CreateWorktreeRequest{
			Name:        branchName,
			Branch:      branchName,
			BaseBranch:  baseBranch,
			IsNewBranch: m.createState.createMode == CreateModeNewBranch,
			WorktreeDir: "", // Let WorktreeManager determine from config
		}

		ctx := context.Background()
		worktreeManager := core.NewWorktreeManager(m.gitRepo, m.configManager)
		if err := worktreeManager.CreateWorktree(ctx, req); err != nil {
			return worktreeCreatedMsg{err: err}
		}

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

		// Handle single worktree deletion
		if m.deleteState.targetWorktree != nil {
			worktree := *m.deleteState.targetWorktree

			// Remove git worktree
			cmd := exec.Command("git", "worktree", "remove", worktree.Path, "--force")
			if err := cmd.Run(); err != nil {
				return worktreeDeletedMsg{err: fmt.Errorf("failed to remove worktree %s: %w", worktree.Name, err)}
			}

			// Delete local branch if it exists
			cmd = exec.Command("git", "branch", "-D", worktree.Branch)
			cmd.Run() // Ignore errors for branch deletion

			deletedCount = 1
		} else {
			// Handle multi-select deletion
			for _, selectedIdx := range m.deleteState.selectedWorktrees {
				if selectedIdx < len(m.worktrees) {
					worktree := m.worktrees[selectedIdx]

					// Remove git worktree
					cmd := exec.Command("git", "worktree", "remove", worktree.Path, "--force")
					if err := cmd.Run(); err != nil {
						return worktreeDeletedMsg{err: fmt.Errorf("failed to remove worktree %s: %w", worktree.Name, err)}
					}

					// Delete local branch if it exists
					cmd = exec.Command("git", "branch", "-D", worktree.Branch)
					cmd.Run() // Ignore errors for branch deletion

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
	return func() tea.Msg {
		scriptPath := ".gren/post-create.sh"

		// Try different editors in order of preference
		editors := []string{"code", "cursor", "nano", "vi"}
		var cmd *exec.Cmd

		for _, editor := range editors {
			if isCommandAvailable(editor) {
				cmd = exec.Command(editor, scriptPath)
				break
			}
		}

		if cmd == nil {
			return scriptEditCompleteMsg{err: fmt.Errorf("no suitable editor found")}
		}

		err := cmd.Run()
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

// generateCopyPatterns generates copy patterns from detected files
func (m Model) generateCopyPatterns(detectedFiles []DetectedFile) []CopyPattern {
	var patterns []CopyPattern

	for _, file := range detectedFiles {
		patterns = append(patterns, CopyPattern{
			Pattern:      file.Path,
			Type:         file.Type,
			IsGitIgnored: file.IsGitIgnored,
			Description:  file.Description,
			Enabled:      true, // Enable by default
			Detected:     true, // This was automatically detected
		})
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
	if m.height > 0 && height > m.height - 6 {
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
	return func() tea.Msg {
		// Try to detect available editors in order of preference
		editors := []struct {
			name    string
			command string
		}{
			{"Visual Studio Code", "code"},
			{"Zed", "zed"},
			{"Vim", "vim"},
			{"Nano", "nano"},
		}

		var foundEditor string
		for _, editor := range editors {
			if _, err := exec.LookPath(editor.command); err == nil {
				foundEditor = editor.command
				break
			}
		}

		if foundEditor == "" {
			return configFileOpenedMsg{err: fmt.Errorf("no suitable editor found (tried: code, zed, vim, nano)")}
		}

		// Check if file exists
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return configFileOpenedMsg{err: fmt.Errorf("config file does not exist: %s", filePath)}
		}

		// Execute the editor
		cmd := exec.Command(foundEditor, filePath)
		if err := cmd.Start(); err != nil {
			return configFileOpenedMsg{err: fmt.Errorf("failed to open %s: %w", foundEditor, err)}
		}

		// For GUI editors like code/zed, we don't wait
		// For terminal editors like vim/nano, we should wait but that would block the UI
		// For now, we'll treat all editors the same way
		return configFileOpenedMsg{err: nil} // Success
	}
}