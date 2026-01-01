package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestNewUserConfigManager(t *testing.T) {
	manager := NewUserConfigManager()
	if manager == nil {
		t.Fatal("NewUserConfigManager() returned nil")
	}
	if manager.configPath == "" {
		t.Error("configPath is empty")
	}
}

func TestUserConfigManagerConfigPath(t *testing.T) {
	manager := NewUserConfigManager()
	path := manager.ConfigPath()
	if path == "" {
		t.Error("ConfigPath() returned empty string")
	}
	// Should end with config.toml
	if filepath.Base(path) != "config.toml" {
		t.Errorf("ConfigPath() = %q, should end with 'config.toml'", path)
	}
}

func TestUserConfigManagerLoad_MissingFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gren-user-config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	manager := &UserConfigManager{
		configPath: filepath.Join(tempDir, "nonexistent.toml"),
	}

	config, err := manager.Load()
	if err != nil {
		t.Fatalf("Load() should not error for missing file: %v", err)
	}
	if config == nil {
		t.Fatal("Load() returned nil for missing file")
	}
}

func TestUserConfigManagerLoad_ValidFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gren-user-config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.toml")
	configContent := `
[defaults]
worktree-dir = "../my-worktrees"
remove-after-merge = true
squash-on-merge = true

[commit-generation]
command = "llm"
args = ["-m", "gpt-4"]

[hooks]
post-create = "npm install"
pre-merge = "npm test"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	manager := &UserConfigManager{configPath: configPath}
	config, err := manager.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if config.Defaults.WorktreeDir != "../my-worktrees" {
		t.Errorf("Defaults.WorktreeDir = %q, want %q", config.Defaults.WorktreeDir, "../my-worktrees")
	}
	if !config.Defaults.RemoveAfterMerge {
		t.Error("Defaults.RemoveAfterMerge should be true")
	}
	if !config.Defaults.SquashOnMerge {
		t.Error("Defaults.SquashOnMerge should be true")
	}
	if config.CommitGenerator.Command != "llm" {
		t.Errorf("CommitGenerator.Command = %q, want %q", config.CommitGenerator.Command, "llm")
	}
	if len(config.CommitGenerator.Args) != 2 {
		t.Errorf("CommitGenerator.Args length = %d, want 2", len(config.CommitGenerator.Args))
	}
	if config.Hooks.PostCreate != "npm install" {
		t.Errorf("Hooks.PostCreate = %q, want %q", config.Hooks.PostCreate, "npm install")
	}
	if config.Hooks.PreMerge != "npm test" {
		t.Errorf("Hooks.PreMerge = %q, want %q", config.Hooks.PreMerge, "npm test")
	}
}

func TestUserConfigManagerLoad_InvalidTOML(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gren-user-config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.toml")
	if err := os.WriteFile(configPath, []byte("invalid toml { }"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	manager := &UserConfigManager{configPath: configPath}
	_, err = manager.Load()
	if err == nil {
		t.Error("Load() should error for invalid TOML")
	}
}

func TestUserConfigManagerSave(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gren-user-config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "subdir", "config.toml")
	manager := &UserConfigManager{configPath: configPath}

	config := &UserConfig{
		Defaults: UserDefaults{
			WorktreeDir:      "../test-worktrees",
			RemoveAfterMerge: true,
		},
		CommitGenerator: CommitGenerator{
			Command: "test-cmd",
			Args:    []string{"arg1"},
		},
	}

	if err := manager.Save(config); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("config file was not created")
	}

	// Verify content can be loaded back
	loaded, err := manager.Load()
	if err != nil {
		t.Fatalf("Load() error after save: %v", err)
	}

	if loaded.Defaults.WorktreeDir != config.Defaults.WorktreeDir {
		t.Errorf("WorktreeDir = %q, want %q", loaded.Defaults.WorktreeDir, config.Defaults.WorktreeDir)
	}
}

func TestUserConfigManagerExists(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gren-user-config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.toml")
	manager := &UserConfigManager{configPath: configPath}

	if manager.Exists() {
		t.Error("Exists() should return false for nonexistent file")
	}

	// Create the file
	if err := os.WriteFile(configPath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	if !manager.Exists() {
		t.Error("Exists() should return true after file creation")
	}
}

func TestMergeConfigs(t *testing.T) {
	userConfig := &UserConfig{
		Defaults: UserDefaults{
			WorktreeDir: "../user-worktrees",
		},
		CommitGenerator: CommitGenerator{
			Command: "llm",
			Args:    []string{"-m", "gpt-4"},
		},
		Hooks: Hooks{
			PostCreate: "user-post-create",
			PreMerge:   "user-pre-merge",
		},
	}

	projectConfig := &Config{
		WorktreeDir: "", // Empty, should use user default
		Hooks: Hooks{
			PostCreate: "project-post-create", // Overrides user
			PreRemove:  "project-pre-remove",  // Project-only
		},
	}

	merged := MergeConfigs(userConfig, projectConfig)

	// User defaults should apply when project is empty
	if merged.WorktreeDir != "../user-worktrees" {
		t.Errorf("WorktreeDir = %q, want %q", merged.WorktreeDir, "../user-worktrees")
	}

	// Project hooks should override user hooks
	if merged.Hooks.PostCreate != "project-post-create" {
		t.Errorf("Hooks.PostCreate = %q, want %q", merged.Hooks.PostCreate, "project-post-create")
	}

	// User hooks should be used when project doesn't override
	if merged.Hooks.PreMerge != "user-pre-merge" {
		t.Errorf("Hooks.PreMerge = %q, want %q", merged.Hooks.PreMerge, "user-pre-merge")
	}

	// Project-only hooks should remain
	if merged.Hooks.PreRemove != "project-pre-remove" {
		t.Errorf("Hooks.PreRemove = %q, want %q", merged.Hooks.PreRemove, "project-pre-remove")
	}

	// CommitGenerator should be merged
	if merged.CommitGenerator.Command != "llm" {
		t.Errorf("CommitGenerator.Command = %q, want %q", merged.CommitGenerator.Command, "llm")
	}
}

func TestMergeConfigs_NilUserConfig(t *testing.T) {
	projectConfig := &Config{
		WorktreeDir: "../project-worktrees",
	}

	// nil user config should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("MergeConfigs panicked with nil user config: %v", r)
		}
	}()

	merged := MergeConfigs(nil, projectConfig)

	// Should return project config unchanged
	if merged.WorktreeDir != "../project-worktrees" {
		t.Errorf("WorktreeDir = %q, want %q", merged.WorktreeDir, "../project-worktrees")
	}
}

func TestMergeConfigs_NilProjectConfig(t *testing.T) {
	userConfig := &UserConfig{
		Defaults: UserDefaults{
			WorktreeDir: "../user-worktrees",
		},
	}

	merged := MergeConfigs(userConfig, nil)

	// Should return config with user defaults
	if merged.WorktreeDir != "../user-worktrees" {
		t.Errorf("WorktreeDir = %q, want %q", merged.WorktreeDir, "../user-worktrees")
	}
}

func TestMergeConfigs_BothNil(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("MergeConfigs panicked with both nil: %v", r)
		}
	}()

	merged := MergeConfigs(nil, nil)

	// Should return empty config
	if merged == nil {
		t.Fatal("MergeConfigs(nil, nil) should not return nil")
	}
}

func TestMergeConfigs_ProjectOverrides(t *testing.T) {
	userConfig := &UserConfig{
		Defaults: UserDefaults{
			WorktreeDir: "../user-worktrees",
		},
	}

	projectConfig := &Config{
		WorktreeDir: "../project-worktrees",
	}

	merged := MergeConfigs(userConfig, projectConfig)

	// Project should override user when both are set
	if merged.WorktreeDir != "../project-worktrees" {
		t.Errorf("WorktreeDir = %q, want %q (project should override)", merged.WorktreeDir, "../project-worktrees")
	}
}

func TestUserDefaults(t *testing.T) {
	defaults := UserDefaults{
		WorktreeDir:      "../worktrees",
		RemoveAfterMerge: true,
		SquashOnMerge:    true,
		RebaseOnMerge:    false,
	}

	if defaults.WorktreeDir != "../worktrees" {
		t.Errorf("WorktreeDir = %q, want %q", defaults.WorktreeDir, "../worktrees")
	}
	if !defaults.RemoveAfterMerge {
		t.Error("RemoveAfterMerge should be true")
	}
	if !defaults.SquashOnMerge {
		t.Error("SquashOnMerge should be true")
	}
	if defaults.RebaseOnMerge {
		t.Error("RebaseOnMerge should be false")
	}
}

func TestNamedHooksConfig(t *testing.T) {
	config := NamedHooksConfig{
		PostCreate: []NamedHook{
			{Name: "install", Command: "npm install"},
		},
		PreMerge: []NamedHook{
			{Name: "test", Command: "npm test"},
		},
	}

	if len(config.PostCreate) != 1 {
		t.Errorf("PostCreate length = %d, want 1", len(config.PostCreate))
	}
	if len(config.PreMerge) != 1 {
		t.Errorf("PreMerge length = %d, want 1", len(config.PreMerge))
	}
}

func TestUserConfigGetNamedHooks(t *testing.T) {
	config := &UserConfig{
		NamedHooks: NamedHooksConfig{
			PostCreate: []NamedHook{
				{Name: "install", Command: "npm install"},
			},
			PreRemove: []NamedHook{
				{Name: "cleanup", Command: "rm -rf node_modules"},
			},
			PreMerge: []NamedHook{
				{Name: "test", Command: "npm test"},
			},
			PostMerge: []NamedHook{
				{Name: "deploy", Command: "npm run deploy"},
			},
			PostSwitch: []NamedHook{
				{Name: "start", Command: "npm run dev"},
			},
			PostStart: []NamedHook{
				{Name: "log", Command: "echo started"},
			},
		},
	}

	tests := []struct {
		hookType HookType
		wantLen  int
	}{
		{HookPostCreate, 1},
		{HookPreRemove, 1},
		{HookPreMerge, 1},
		{HookPostMerge, 1},
		{HookPostSwitch, 1},
		{HookPostStart, 1},
		{HookType("unknown"), 0},
	}

	for _, tt := range tests {
		t.Run(string(tt.hookType), func(t *testing.T) {
			hooks := config.GetNamedHooks(tt.hookType)
			if len(hooks) != tt.wantLen {
				t.Errorf("GetNamedHooks(%s) length = %d, want %d", tt.hookType, len(hooks), tt.wantLen)
			}
		})
	}
}

func TestUserConfigTOMLRoundTrip(t *testing.T) {
	original := &UserConfig{
		Defaults: UserDefaults{
			WorktreeDir:      "../worktrees",
			RemoveAfterMerge: true,
		},
		CommitGenerator: CommitGenerator{
			Command: "llm",
			Args:    []string{"-m", "claude"},
		},
		Hooks: Hooks{
			PostCreate: "npm install",
		},
	}

	// Marshal
	data, err := toml.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	// Unmarshal
	var loaded UserConfig
	if err := toml.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	// Verify
	if loaded.Defaults.WorktreeDir != original.Defaults.WorktreeDir {
		t.Errorf("WorktreeDir = %q, want %q", loaded.Defaults.WorktreeDir, original.Defaults.WorktreeDir)
	}
	if loaded.CommitGenerator.Command != original.CommitGenerator.Command {
		t.Errorf("CommitGenerator.Command = %q, want %q", loaded.CommitGenerator.Command, original.CommitGenerator.Command)
	}
}
