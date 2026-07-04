package config

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// ApprovalManager handles persistent storage of approved hook commands.
// Commands are approved per-project to prevent malicious configs from
// automatically executing untrusted code.
type ApprovalManager struct {
	approvedCommands map[string]map[string]bool // projectID -> command -> approved
	configPath       string
}

// ApprovalData represents the persisted approval data.
type ApprovalData struct {
	Version  string                     `json:"version"`
	Projects map[string]map[string]bool `json:"projects"` // projectID -> command -> approved
}

// NewApprovalManager creates a new approval manager.
func NewApprovalManager() *ApprovalManager {
	am := &ApprovalManager{
		approvedCommands: make(map[string]map[string]bool),
		configPath:       getApprovalConfigPath(),
	}
	am.load()
	return am
}

// getApprovalConfigPath returns the path to the approval config file.
func getApprovalConfigPath() string {
	var configDir string
	switch runtime.GOOS {
	case "darwin":
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, "Library", "Application Support", "gren")
	case "windows":
		configDir = filepath.Join(os.Getenv("APPDATA"), "gren")
	default: // linux and others
		if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
			configDir = filepath.Join(xdgConfig, "gren")
		} else {
			home, _ := os.UserHomeDir()
			configDir = filepath.Join(home, ".config", "gren")
		}
	}
	return filepath.Join(configDir, "approved-commands.json")
}

// load reads the approval data from disk.
func (am *ApprovalManager) load() {
	data, err := os.ReadFile(am.configPath)
	if err != nil {
		return // File doesn't exist yet, that's OK
	}

	var approvalData ApprovalData
	if err := json.Unmarshal(data, &approvalData); err != nil {
		return // Corrupt file, start fresh
	}

	if approvalData.Projects != nil {
		am.approvedCommands = approvalData.Projects
	}
}

// save writes the approval data to disk.
func (am *ApprovalManager) save() error {
	// Ensure directory exists
	dir := filepath.Dir(am.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data := ApprovalData{
		Version:  "1.0.0",
		Projects: am.approvedCommands,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal approval data: %w", err)
	}

	return os.WriteFile(am.configPath, jsonData, 0644)
}

// IsApproved checks if a command is approved for a project.
func (am *ApprovalManager) IsApproved(projectID, command string) bool {
	if projectCommands, ok := am.approvedCommands[projectID]; ok {
		return projectCommands[command]
	}
	return false
}

// AreAllApproved checks if all commands are approved for a project.
func (am *ApprovalManager) AreAllApproved(projectID string, commands []string) bool {
	for _, cmd := range commands {
		if !am.IsApproved(projectID, cmd) {
			return false
		}
	}
	return true
}

// GetUnapprovedCommands returns the commands that are not yet approved.
func (am *ApprovalManager) GetUnapprovedCommands(projectID string, commands []string) []string {
	var unapproved []string
	for _, cmd := range commands {
		if !am.IsApproved(projectID, cmd) {
			unapproved = append(unapproved, cmd)
		}
	}
	return unapproved
}

// Approve marks a command as approved for a project.
func (am *ApprovalManager) Approve(projectID, command string) error {
	if am.approvedCommands[projectID] == nil {
		am.approvedCommands[projectID] = make(map[string]bool)
	}
	am.approvedCommands[projectID][command] = true
	return am.save()
}

// ApproveAll marks multiple commands as approved for a project.
func (am *ApprovalManager) ApproveAll(projectID string, commands []string) error {
	if am.approvedCommands[projectID] == nil {
		am.approvedCommands[projectID] = make(map[string]bool)
	}
	for _, cmd := range commands {
		am.approvedCommands[projectID][cmd] = true
	}
	return am.save()
}

// Revoke removes approval for a command.
func (am *ApprovalManager) Revoke(projectID, command string) error {
	if projectCommands, ok := am.approvedCommands[projectID]; ok {
		delete(projectCommands, command)
	}
	return am.save()
}

// RevokeAll removes all approvals for a project.
func (am *ApprovalManager) RevokeAll(projectID string) error {
	delete(am.approvedCommands, projectID)
	return am.save()
}

// ListApproved returns all approved commands for a project.
func (am *ApprovalManager) ListApproved(projectID string) []string {
	var commands []string
	if projectCommands, ok := am.approvedCommands[projectID]; ok {
		for cmd := range projectCommands {
			commands = append(commands, cmd)
		}
	}
	return commands
}

// GetProjectID returns a stable identifier for the whole project, shared by all
// of its worktrees, so hook approvals persist across worktrees instead of being
// re-prompted for each one.
//
// It prefers the git remote URL; failing that, the main worktree root (the
// shared git common dir's parent, which is identical from every linked
// worktree); and only as a last resort the current directory. The previous
// implementation always used the current directory, so running from each
// worktree produced a different ID and approvals never carried over.
func GetProjectID() (string, error) {
	// 1. Git remote URL — stable across worktrees and machines.
	if out, err := exec.Command("git", "remote", "get-url", "origin").Output(); err == nil {
		if url := strings.TrimSpace(string(out)); url != "" {
			return url, nil
		}
	}

	// 2. Main worktree root (shared git common dir's parent). Identical from the
	//    main checkout and every linked worktree, so approvals persist without a
	//    remote.
	if out, err := exec.Command("git", "rev-parse", "--path-format=absolute", "--git-common-dir").Output(); err == nil {
		if commonDir := strings.TrimSpace(string(out)); commonDir != "" {
			return filepath.Dir(filepath.Clean(commonDir)), nil
		}
	}

	// 3. Last resort: the current directory (not a git repo).
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Clean(cwd), nil
}
