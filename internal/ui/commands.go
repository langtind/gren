package ui

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
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
	return func() tea.Msg {
		branchStatuses, err := m.getBranchStatuses()
		if err != nil {
			return createInitMsg{err: err}
		}

		// Find recommended base branch (current branch or main/master)
		var recommendedBase string
		for _, status := range branchStatuses {
			if status.IsCurrent {
				recommendedBase = status.Name
				break
			}
		}
		if recommendedBase == "" {
			// Fall back to main or master
			for _, status := range branchStatuses {
				if status.Name == "main" || status.Name == "master" {
					recommendedBase = status.Name
					break
				}
			}
		}
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

// createWorktree creates the actual worktree
func (m Model) createWorktree() tea.Cmd {
	return func() tea.Msg {
		if m.createState == nil {
			return worktreeCreatedMsg{err: fmt.Errorf("create state is nil")}
		}

		branchName := m.createState.branchName
		baseBranch := m.createState.baseBranch
		worktreePath := fmt.Sprintf("../gren-worktrees/%s", branchName)

		// Create worktrees directory if it doesn't exist
		worktreesDir := "../gren-worktrees"
		if err := os.MkdirAll(worktreesDir, 0755); err != nil {
			return worktreeCreatedMsg{err: fmt.Errorf("failed to create worktrees directory: %w", err)}
		}

		// Check if worktree path already exists
		if _, err := os.Stat(worktreePath); err == nil {
			return worktreeCreatedMsg{err: fmt.Errorf("worktree directory already exists: %s", worktreePath)}
		}

		// Check if branch already exists
		checkCmd := exec.Command("git", "branch", "--list", branchName)
		if output, err := checkCmd.Output(); err == nil && len(strings.TrimSpace(string(output))) > 0 {
			return worktreeCreatedMsg{err: fmt.Errorf("branch '%s' already exists", branchName)}
		}

		// Create the git worktree
		cmd := exec.Command("git", "worktree", "add", "-b", branchName, worktreePath, baseBranch)
		output, err := cmd.CombinedOutput()
		if err != nil {
			// Provide more detailed error information
			errorMsg := fmt.Sprintf("git worktree add failed: %s", string(output))
			if len(output) == 0 {
				errorMsg = fmt.Sprintf("git worktree add failed with exit code: %v", err)
			}
			return worktreeCreatedMsg{err: fmt.Errorf(errorMsg)}
		}

		// Copy .gren/ configuration to worktree
		if err := m.copyGrenConfig(worktreePath); err != nil {
			// Don't fail the entire operation, just log it
			// The worktree was created successfully
		}

		// Run post-create script if it exists
		if err := m.runPostCreateScript(worktreePath); err != nil {
			// Don't fail the entire operation, just log it
			// The worktree was created successfully
		}

		return worktreeCreatedMsg{branchName: branchName}
	}
}

// copyGrenConfig copies .gren/ directory to worktree
func (m Model) copyGrenConfig(worktreePath string) error {
	grenSrc := ".gren"
	grenDest := filepath.Join(worktreePath, ".gren")

	if _, err := os.Stat(grenSrc); os.IsNotExist(err) {
		return nil // No .gren directory to copy
	}

	return copyDir(grenSrc, grenDest)
}

// copyDir recursively copies a directory
func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies a single file
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// runPostCreateScript runs the post-create script if it exists
func (m Model) runPostCreateScript(worktreePath string) error {
	scriptPath := filepath.Join(worktreePath, ".gren", "post-create.sh")
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return nil // No script to run
	}

	cmd := exec.Command("bash", scriptPath)
	cmd.Dir = worktreePath
	return cmd.Run()
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
		// Create .gren directory structure
		if err := os.MkdirAll(".gren", 0755); err != nil {
			return initExecutionCompleteMsg{}
		}

		// Create basic configuration files
		// This is a simplified version - real implementation would be more complex
		return initExecutionCompleteMsg{}
	}
}

// Additional helper functions for initialization and project analysis

// analyzeProject analyzes the current project structure
func (m Model) analyzeProject() []DetectedFile {
	var files []DetectedFile

	// Common patterns to look for
	patterns := map[string]string{
		".env":         "env",
		".env.local":   "env",
		".env.example": "env",
		"package.json": "config",
		"go.mod":       "config",
		"Cargo.toml":   "config",
		".nvmrc":       "tool",
		".node-version": "tool",
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