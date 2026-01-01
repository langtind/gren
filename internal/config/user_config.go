package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/pelletier/go-toml/v2"
)

// UserConfig represents global user configuration that applies across all projects.
type UserConfig struct {
	// Defaults section
	Defaults UserDefaults `toml:"defaults,omitempty"`

	// LLM configuration for commit message generation
	CommitGenerator CommitGenerator `toml:"commit-generation,omitempty"`

	// Global hooks (run for all projects)
	Hooks Hooks `toml:"hooks,omitempty"`

	// Extended hooks with names
	NamedHooks NamedHooksConfig `toml:"named-hooks,omitempty"`
}

// UserDefaults contains default settings for worktree operations.
type UserDefaults struct {
	// WorktreeDir template for worktree directory names
	// Supports {{ repo }} template variable
	WorktreeDir string `toml:"worktree-dir,omitempty"`

	// RemoveAfterMerge controls whether worktrees are removed after merge
	RemoveAfterMerge bool `toml:"remove-after-merge,omitempty"`

	// SquashOnMerge controls whether commits are squashed on merge
	SquashOnMerge bool `toml:"squash-on-merge,omitempty"`

	// RebaseOnMerge controls whether to rebase before merge
	RebaseOnMerge bool `toml:"rebase-on-merge,omitempty"`
}

// NamedHooksConfig holds named hooks organized by lifecycle event.
type NamedHooksConfig struct {
	PostCreate []NamedHook `toml:"post-create,omitempty"`
	PreRemove  []NamedHook `toml:"pre-remove,omitempty"`
	PreMerge   []NamedHook `toml:"pre-merge,omitempty"`
	PostMerge  []NamedHook `toml:"post-merge,omitempty"`
	PostSwitch []NamedHook `toml:"post-switch,omitempty"`
	PostStart  []NamedHook `toml:"post-start,omitempty"`
}

// NamedHook represents a hook with a name for identification and approval.
type NamedHook struct {
	Name    string `toml:"name"`
	Command string `toml:"command"`
	// Optional: only run on specific branches (glob patterns)
	Branches []string `toml:"branches,omitempty"`
	// Optional: skip this hook if set to true
	Disabled bool `toml:"disabled,omitempty"`
}

// UserConfigManager handles user-level configuration.
type UserConfigManager struct {
	configPath string
}

// NewUserConfigManager creates a new user config manager.
func NewUserConfigManager() *UserConfigManager {
	return &UserConfigManager{
		configPath: getUserConfigPath(),
	}
}

// getUserConfigPath returns the platform-specific path for user config.
func getUserConfigPath() string {
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
	return filepath.Join(configDir, "config.toml")
}

// Load reads the user configuration from disk.
func (ucm *UserConfigManager) Load() (*UserConfig, error) {
	data, err := os.ReadFile(ucm.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty config if file doesn't exist
			return &UserConfig{}, nil
		}
		return nil, fmt.Errorf("failed to read user config: %w", err)
	}

	var config UserConfig
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse user config: %w", err)
	}

	return &config, nil
}

// Save writes the user configuration to disk.
func (ucm *UserConfigManager) Save(config *UserConfig) error {
	// Ensure directory exists
	dir := filepath.Dir(ucm.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := toml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal user config: %w", err)
	}

	// Add header comment
	header := []byte("# gren global user configuration\n# See https://github.com/langtind/gren for documentation\n\n")
	data = append(header, data...)

	return os.WriteFile(ucm.configPath, data, 0644)
}

// Exists checks if user config file exists.
func (ucm *UserConfigManager) Exists() bool {
	_, err := os.Stat(ucm.configPath)
	return err == nil
}

// ConfigPath returns the path to the user config file.
func (ucm *UserConfigManager) ConfigPath() string {
	return ucm.configPath
}

// MergeWithProject merges user config with project config.
// Project config takes precedence over user config.
func MergeConfigs(user *UserConfig, project *Config) *Config {
	if project == nil {
		project = &Config{}
	}

	if user == nil {
		return project
	}

	// Apply user defaults if project doesn't specify
	if project.WorktreeDir == "" && user.Defaults.WorktreeDir != "" {
		project.WorktreeDir = user.Defaults.WorktreeDir
	}

	// Merge commit generator (project takes precedence)
	if project.CommitGenerator.Command == "" && user.CommitGenerator.Command != "" {
		project.CommitGenerator = user.CommitGenerator
	}

	// Merge hooks - user hooks run first, then project hooks
	if user.Hooks.PostCreate != "" && project.Hooks.PostCreate == "" {
		project.Hooks.PostCreate = user.Hooks.PostCreate
	}
	if user.Hooks.PreRemove != "" && project.Hooks.PreRemove == "" {
		project.Hooks.PreRemove = user.Hooks.PreRemove
	}
	if user.Hooks.PreMerge != "" && project.Hooks.PreMerge == "" {
		project.Hooks.PreMerge = user.Hooks.PreMerge
	}
	if user.Hooks.PostMerge != "" && project.Hooks.PostMerge == "" {
		project.Hooks.PostMerge = user.Hooks.PostMerge
	}

	return project
}

// GetNamedHooks returns named hooks for a specific hook type from user config.
func (uc *UserConfig) GetNamedHooks(hookType HookType) []NamedHook {
	switch hookType {
	case HookPostCreate:
		return uc.NamedHooks.PostCreate
	case HookPreRemove:
		return uc.NamedHooks.PreRemove
	case HookPreMerge:
		return uc.NamedHooks.PreMerge
	case HookPostMerge:
		return uc.NamedHooks.PostMerge
	case HookPostSwitch:
		return uc.NamedHooks.PostSwitch
	case HookPostStart:
		return uc.NamedHooks.PostStart
	default:
		return nil
	}
}
