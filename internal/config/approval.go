package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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

// GetProjectID returns a unique identifier for the current project.
func GetProjectID() (string, error) {
	// Use the git remote URL as a stable project identifier
	// Fall back to the repository root path if no remote is configured
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Try to get the canonical path
	absPath, err := filepath.Abs(cwd)
	if err != nil {
		absPath = cwd
	}

	return absPath, nil
}
