package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// getAvailableActions returns a list of available post-create actions
func (m Model) getAvailableActions() []PostCreateAction {
	if m.createState == nil {
		return []PostCreateAction{}
	}

	worktreePath := fmt.Sprintf("../gren-worktrees/%s", m.createState.branchName)
	return m.getActionsForPath(worktreePath, "Return to dashboard")
}

// getActionsForPath returns available actions for a given path
func (m Model) getActionsForPath(worktreePath, backActionName string) []PostCreateAction {

	allActions := []PostCreateAction{
		{
			Name:        "Open in Terminal",
			Icon:        "🖥️",
			Command:     m.getTerminalCommand(),
			Args:        m.getTerminalArgs(worktreePath),
			Description: "Open worktree directory in terminal",
		},
		{
			Name:        "Open in VS Code",
			Icon:        "⚡",
			Command:     "code",
			Args:        []string{worktreePath},
			Description: "Open worktree in Visual Studio Code",
		},
		{
			Name:        "Open in Claude Code",
			Icon:        "🤖",
			Command:     "claude",
			Args:        []string{},
			Description: "Open worktree in Claude Code",
		},
		{
			Name:        "Open in Cursor",
			Icon:        "📂",
			Command:     "cursor",
			Args:        []string{worktreePath},
			Description: "Open worktree in Cursor editor",
		},
		{
			Name:        "Open in Zed",
			Icon:        "⚡",
			Command:     "zed",
			Args:        []string{worktreePath},
			Description: "Open worktree in Zed editor",
		},
		{
			Name:        "Open in Finder",
			Icon:        "🗂️",
			Command:     "open",
			Args:        []string{worktreePath},
			Description: "Open worktree directory in Finder",
		},
	}

	// Check which commands are available and collect them
	var availableActions []PostCreateAction
	for _, action := range allActions {
		if isCommandAvailable(action.Command) {
			action.Available = true
			availableActions = append(availableActions, action)
		}
	}

	// Always add back/return option
	availableActions = append(availableActions, PostCreateAction{
		Name:        backActionName,
		Icon:        "🔙",
		Command:     "",
		Args:        nil,
		Available:   true,
		Description: "Go back to main dashboard",
	})

	return availableActions
}

// getTerminalCommand determines the best way to open the current terminal
func (m Model) getTerminalCommand() string {
	// Check what terminal we're running in by looking at environment variables
	if term := os.Getenv("TERM_PROGRAM"); term != "" {
		switch term {
		case "WarpTerminal":
			return "warp-cli"
		case "iTerm.app":
			return "osascript"
		case "Apple_Terminal":
			return "osascript"
		}
	}

	// Fallback to generic open command
	return "open"
}

// getTerminalArgs returns the appropriate arguments for opening a terminal
func (m Model) getTerminalArgs(worktreePath string) []string {
	termProgram := os.Getenv("TERM_PROGRAM")
	switch termProgram {
	case "WarpTerminal":
		return []string{"open", worktreePath}
	case "iTerm.app":
		return []string{
			"-e",
			fmt.Sprintf(`tell application "iTerm"
				create window with default profile
				tell current session of current window
					write text "cd %s"
				end tell
			end tell`, worktreePath),
		}
	case "Apple_Terminal":
		return []string{
			"-e",
			fmt.Sprintf(`tell application "Terminal"
				do script "cd %s"
			end tell`, worktreePath),
		}
	default:
		return []string{worktreePath}
	}
}

// isCommandAvailable checks if a command is available in PATH
func isCommandAvailable(command string) bool {
	if command == "" {
		return false
	}
	_, err := exec.LookPath(command)
	return err == nil
}

// executeAction executes the selected post-create action
func (m Model) executeAction(action PostCreateAction, worktreePath string) error {
	if action.Command == "" {
		// Special case for "Return to dashboard" - no command to execute
		return nil
	}

	cmd := exec.Command(action.Command, action.Args...)

	// For Claude Code, we need to handle it specially since it's a TUI app
	if action.Command == "claude" {
		// Check if the worktree path exists
		if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
			return fmt.Errorf("worktree path does not exist: %s", worktreePath)
		}

		// Since Warp doesn't support CLI automation yet, use simple open command
		// This will open a new terminal window and run the command
		terminalScript := fmt.Sprintf("cd \"%s\" && claude", worktreePath)
		cmd = exec.Command("osascript", "-e", fmt.Sprintf(`tell application "Terminal"
			do script "%s"
			activate
		end tell`, terminalScript))

		fmt.Printf("DEBUG: Executing Claude via Terminal in: %s\n", worktreePath)
	}

	// Debug: Log command execution
	fmt.Printf("DEBUG: Executing command: %s %v\n", action.Command, action.Args)

	err := cmd.Start()
	if err != nil {
		fmt.Printf("DEBUG: Command failed: %v\n", err)
		return err
	}

	fmt.Printf("DEBUG: Command started successfully\n")
	return nil
}

// isValidBranchName validates git branch names
func isValidBranchName(name string) bool {
	if name == "" {
		return false
	}

	// Basic validation - could be more comprehensive
	for _, char := range name {
		if char == ' ' || char == '~' || char == '^' || char == ':' ||
			char == '?' || char == '*' || char == '[' || char == '\\' {
			return false
		}
	}

	// Can't start with . or -
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "-") {
		return false
	}

	// Can't end with .
	if strings.HasSuffix(name, ".") {
		return false
	}

	// Can't contain consecutive dots
	if strings.Contains(name, "..") {
		return false
	}

	return true
}