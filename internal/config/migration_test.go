package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "1.1.0", -1},
		{"1.1.0", "1.0.0", 1},
		{"1.0.0", "2.0.0", -1},
		{"2.0.0", "1.0.0", 1},
		{"1.0.0", "1.0.1", -1},
		{"1.0.1", "1.0.0", 1},
		{"v1.0.0", "1.0.0", 0},  // Handle v prefix
		{"1.0.0", "v1.1.0", -1}, // Handle v prefix
		{"0.0.0", "1.0.0", -1},  // Pre-versioned
		{"1.0", "1.0.0", 0},     // Short version
		{"1", "1.0.0", 0},       // Very short version
	}

	for _, tt := range tests {
		result := compareVersions(tt.v1, tt.v2)
		if result != tt.expected {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", tt.v1, tt.v2, result, tt.expected)
		}
	}
}

func TestNeedsMigration_NoConfig(t *testing.T) {
	tmpDir := t.TempDir()
	manager := &Manager{configDir: filepath.Join(tmpDir, ".gren")}

	needsMigration, result, err := manager.NeedsMigration()
	if err != nil {
		t.Fatalf("NeedsMigration() error = %v", err)
	}
	if needsMigration {
		t.Error("NeedsMigration() = true, want false for non-existent config")
	}
	if result != nil {
		t.Error("NeedsMigration() result should be nil for non-existent config")
	}
}

func TestNeedsMigration_CurrentVersion(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".gren")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	manager := &Manager{configDir: configDir}

	// Create config with current version
	config := &Config{
		WorktreeDir: "/tmp/worktrees",
		Version:     CurrentConfigVersion,
	}
	if err := manager.Save(config); err != nil {
		t.Fatal(err)
	}

	needsMigration, _, err := manager.NeedsMigration()
	if err != nil {
		t.Fatalf("NeedsMigration() error = %v", err)
	}
	if needsMigration {
		t.Error("NeedsMigration() = true, want false for current version")
	}
}

func TestNeedsMigration_OldVersion(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".gren")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	manager := &Manager{configDir: configDir}

	// Create config with old version
	config := &Config{
		WorktreeDir: "/tmp/worktrees",
		Version:     "1.0.0",
	}
	if err := manager.Save(config); err != nil {
		t.Fatal(err)
	}

	needsMigration, result, err := manager.NeedsMigration()
	if err != nil {
		t.Fatalf("NeedsMigration() error = %v", err)
	}
	if !needsMigration {
		t.Error("NeedsMigration() = false, want true for old version")
	}
	if result.OldVersion != "1.0.0" {
		t.Errorf("OldVersion = %q, want %q", result.OldVersion, "1.0.0")
	}
	if result.NewVersion != CurrentConfigVersion {
		t.Errorf("NewVersion = %q, want %q", result.NewVersion, CurrentConfigVersion)
	}
}

func TestNeedsMigration_JSONFormat(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".gren")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	manager := &Manager{configDir: configDir}

	// Create JSON config (simulating old format)
	config := Config{
		WorktreeDir: "/tmp/worktrees",
		Version:     CurrentConfigVersion, // Even with current version
	}
	data, _ := json.MarshalIndent(config, "", "  ")
	if err := os.WriteFile(filepath.Join(configDir, ConfigFileJSON), data, 0644); err != nil {
		t.Fatal(err)
	}

	needsMigration, result, err := manager.NeedsMigration()
	if err != nil {
		t.Fatalf("NeedsMigration() error = %v", err)
	}
	if !needsMigration {
		t.Error("NeedsMigration() = false, want true for JSON format")
	}
	if !result.WasJSON {
		t.Error("WasJSON = false, want true")
	}
}

func TestMigrate_JSONToTOML(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".gren")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	manager := &Manager{configDir: configDir}

	// Create JSON config with old settings
	config := Config{
		WorktreeDir:    "/tmp/worktrees",
		PackageManager: "npm",
		PostCreateHook: "echo hello",
		Version:        "1.0.0",
	}
	data, _ := json.MarshalIndent(config, "", "  ")
	jsonPath := filepath.Join(configDir, ConfigFileJSON)
	if err := os.WriteFile(jsonPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	// Run migration
	result, err := manager.Migrate()
	if err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	// Check result
	if !result.WasJSON {
		t.Error("WasJSON = false, want true")
	}
	if result.OldVersion != "1.0.0" {
		t.Errorf("OldVersion = %q, want %q", result.OldVersion, "1.0.0")
	}

	// Check that TOML file exists
	if !manager.ExistsTOML() {
		t.Error("TOML config should exist after migration")
	}

	// Check that JSON file was removed
	if _, err := os.Stat(jsonPath); !os.IsNotExist(err) {
		t.Error("JSON config should be removed after migration")
	}

	// Load and verify migrated config
	migratedConfig, err := manager.Load()
	if err != nil {
		t.Fatalf("Load() after migration error = %v", err)
	}
	if migratedConfig.Version != CurrentConfigVersion {
		t.Errorf("Version = %q, want %q", migratedConfig.Version, CurrentConfigVersion)
	}
	if migratedConfig.Hooks.PostCreate != "echo hello" {
		t.Errorf("Hooks.PostCreate = %q, want %q", migratedConfig.Hooks.PostCreate, "echo hello")
	}
	if migratedConfig.WorktreeDir != "/tmp/worktrees" {
		t.Errorf("WorktreeDir = %q, want %q", migratedConfig.WorktreeDir, "/tmp/worktrees")
	}
}

func TestMigrate_VersionBumpOnly(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".gren")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	manager := &Manager{configDir: configDir}

	// Create TOML config with old version
	config := &Config{
		WorktreeDir: "/tmp/worktrees",
		Version:     "1.0.0",
	}
	if err := manager.Save(config); err != nil {
		t.Fatal(err)
	}

	// Run migration
	result, err := manager.Migrate()
	if err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	if result.WasJSON {
		t.Error("WasJSON = true, want false for TOML config")
	}

	// Load and verify
	migratedConfig, err := manager.Load()
	if err != nil {
		t.Fatalf("Load() after migration error = %v", err)
	}
	if migratedConfig.Version != CurrentConfigVersion {
		t.Errorf("Version = %q, want %q", migratedConfig.Version, CurrentConfigVersion)
	}
}

func TestMigrate_NoMigrationNeeded(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".gren")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}

	manager := &Manager{configDir: configDir}

	// Create config with current version
	config := &Config{
		WorktreeDir: "/tmp/worktrees",
		Version:     CurrentConfigVersion,
	}
	if err := manager.Save(config); err != nil {
		t.Fatal(err)
	}

	// Run migration
	result, err := manager.Migrate()
	if err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if result != nil {
		t.Error("Migrate() should return nil when no migration needed")
	}
}
