package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewApprovalManager(t *testing.T) {
	manager := NewApprovalManager()
	if manager == nil {
		t.Fatal("NewApprovalManager() returned nil")
	}
	if manager.approvedCommands == nil {
		t.Error("approvedCommands map is nil")
	}
}

func TestApprovalManagerIsApproved(t *testing.T) {
	// Create temp directory for test
	tempDir, err := os.MkdirTemp("", "gren-approval-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Override config path for testing
	manager := &ApprovalManager{
		approvedCommands: make(map[string]map[string]bool),
		configPath:       filepath.Join(tempDir, "approved-commands.json"),
	}

	projectID := "/test/project"

	tests := []struct {
		name     string
		command  string
		approved bool
	}{
		{
			name:     "unapproved command",
			command:  "npm install",
			approved: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := manager.IsApproved(projectID, tt.command)
			if got != tt.approved {
				t.Errorf("IsApproved() = %v, want %v", got, tt.approved)
			}
		})
	}
}

func TestApprovalManagerApprove(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gren-approval-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	manager := &ApprovalManager{
		approvedCommands: make(map[string]map[string]bool),
		configPath:       filepath.Join(tempDir, "approved-commands.json"),
	}

	projectID := "/test/project"
	command := "npm install"

	// Initially not approved
	if manager.IsApproved(projectID, command) {
		t.Error("command should not be approved initially")
	}

	// Approve the command
	if err := manager.Approve(projectID, command); err != nil {
		t.Fatalf("Approve() error: %v", err)
	}

	// Now should be approved
	if !manager.IsApproved(projectID, command) {
		t.Error("command should be approved after Approve()")
	}

	// Verify file was created
	if _, err := os.Stat(manager.configPath); os.IsNotExist(err) {
		t.Error("config file was not created")
	}
}

func TestApprovalManagerApproveAll(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gren-approval-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	manager := &ApprovalManager{
		approvedCommands: make(map[string]map[string]bool),
		configPath:       filepath.Join(tempDir, "approved-commands.json"),
	}

	projectID := "/test/project"
	commands := []string{"npm install", "npm test", "npm run build"}

	// Approve all commands
	if err := manager.ApproveAll(projectID, commands); err != nil {
		t.Fatalf("ApproveAll() error: %v", err)
	}

	// Verify all are approved
	for _, cmd := range commands {
		if !manager.IsApproved(projectID, cmd) {
			t.Errorf("command %q should be approved", cmd)
		}
	}

	// Verify AreAllApproved returns true
	if !manager.AreAllApproved(projectID, commands) {
		t.Error("AreAllApproved() should return true")
	}
}

func TestApprovalManagerAreAllApproved(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gren-approval-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	manager := &ApprovalManager{
		approvedCommands: make(map[string]map[string]bool),
		configPath:       filepath.Join(tempDir, "approved-commands.json"),
	}

	projectID := "/test/project"
	commands := []string{"npm install", "npm test"}

	// Initially none approved
	if manager.AreAllApproved(projectID, commands) {
		t.Error("AreAllApproved() should return false when none approved")
	}

	// Approve only one
	manager.Approve(projectID, "npm install")
	if manager.AreAllApproved(projectID, commands) {
		t.Error("AreAllApproved() should return false when only some approved")
	}

	// Approve the second
	manager.Approve(projectID, "npm test")
	if !manager.AreAllApproved(projectID, commands) {
		t.Error("AreAllApproved() should return true when all approved")
	}

	// Empty commands should return true
	if !manager.AreAllApproved(projectID, []string{}) {
		t.Error("AreAllApproved() should return true for empty commands")
	}
}

func TestApprovalManagerRevoke(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gren-approval-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	manager := &ApprovalManager{
		approvedCommands: make(map[string]map[string]bool),
		configPath:       filepath.Join(tempDir, "approved-commands.json"),
	}

	projectID := "/test/project"
	command := "npm install"

	// Approve then revoke
	manager.Approve(projectID, command)
	if !manager.IsApproved(projectID, command) {
		t.Error("command should be approved")
	}

	if err := manager.Revoke(projectID, command); err != nil {
		t.Fatalf("Revoke() error: %v", err)
	}

	if manager.IsApproved(projectID, command) {
		t.Error("command should not be approved after Revoke()")
	}
}

func TestApprovalManagerRevokeAll(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gren-approval-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	manager := &ApprovalManager{
		approvedCommands: make(map[string]map[string]bool),
		configPath:       filepath.Join(tempDir, "approved-commands.json"),
	}

	projectID := "/test/project"
	commands := []string{"npm install", "npm test", "npm run build"}

	// Approve all then revoke all
	manager.ApproveAll(projectID, commands)

	if err := manager.RevokeAll(projectID); err != nil {
		t.Fatalf("RevokeAll() error: %v", err)
	}

	for _, cmd := range commands {
		if manager.IsApproved(projectID, cmd) {
			t.Errorf("command %q should not be approved after RevokeAll()", cmd)
		}
	}
}

func TestApprovalManagerPersistence(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gren-approval-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "approved-commands.json")
	projectID := "/test/project"
	command := "npm install"

	// Create manager and approve
	manager1 := &ApprovalManager{
		approvedCommands: make(map[string]map[string]bool),
		configPath:       configPath,
	}
	manager1.Approve(projectID, command)

	// Create new manager and load (load is called internally, we test via IsApproved)
	manager2 := &ApprovalManager{
		approvedCommands: make(map[string]map[string]bool),
		configPath:       configPath,
	}
	manager2.load() // load doesn't return error

	// Should be approved in new manager
	if !manager2.IsApproved(projectID, command) {
		t.Error("approval should persist across manager instances")
	}
}

func TestApprovalManagerListApproved(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gren-approval-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	manager := &ApprovalManager{
		approvedCommands: make(map[string]map[string]bool),
		configPath:       filepath.Join(tempDir, "approved-commands.json"),
	}

	projectID := "/test/project"
	commands := []string{"npm install", "npm test"}
	manager.ApproveAll(projectID, commands)

	approved := manager.ListApproved(projectID)
	if len(approved) != 2 {
		t.Errorf("ListApproved() returned %d commands, want 2", len(approved))
	}

	// Check for nonexistent project
	approved = manager.ListApproved("/nonexistent")
	if len(approved) != 0 {
		t.Errorf("ListApproved() for nonexistent project should return empty, got %d", len(approved))
	}
}

func TestApprovalManagerGetUnapprovedCommands(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gren-approval-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	manager := &ApprovalManager{
		approvedCommands: make(map[string]map[string]bool),
		configPath:       filepath.Join(tempDir, "approved-commands.json"),
	}

	projectID := "/test/project"
	commands := []string{"npm install", "npm test", "npm run build"}

	// Initially all should be unapproved
	unapproved := manager.GetUnapprovedCommands(projectID, commands)
	if len(unapproved) != 3 {
		t.Errorf("GetUnapprovedCommands() returned %d, want 3", len(unapproved))
	}

	// Approve some commands
	manager.Approve(projectID, "npm install")
	manager.Approve(projectID, "npm test")

	// Now only one should be unapproved
	unapproved = manager.GetUnapprovedCommands(projectID, commands)
	if len(unapproved) != 1 {
		t.Errorf("GetUnapprovedCommands() returned %d, want 1", len(unapproved))
	}
	if unapproved[0] != "npm run build" {
		t.Errorf("GetUnapprovedCommands() = %v, want [npm run build]", unapproved)
	}

	// Approve all
	manager.Approve(projectID, "npm run build")
	unapproved = manager.GetUnapprovedCommands(projectID, commands)
	if len(unapproved) != 0 {
		t.Errorf("GetUnapprovedCommands() returned %d, want 0", len(unapproved))
	}
}

func TestGetProjectID(t *testing.T) {
	// GetProjectID uses cwd, so we test it returns something
	id, err := GetProjectID()
	if err != nil {
		t.Fatalf("GetProjectID() error: %v", err)
	}
	if id == "" {
		t.Error("GetProjectID() returned empty string")
	}

	// IDs should be consistent
	id2, err := GetProjectID()
	if err != nil {
		t.Fatalf("GetProjectID() second call error: %v", err)
	}
	if id != id2 {
		t.Errorf("GetProjectID() not consistent: %q != %q", id, id2)
	}
}
