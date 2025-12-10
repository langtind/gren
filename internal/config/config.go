package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// ConfigDir is the directory where gren configuration is stored.
	ConfigDir = ".gren"
	// ConfigFile is the main configuration file name.
	ConfigFile = "config.json"
	// DefaultVersion is the current configuration version.
	DefaultVersion = "1.0.0"
	// DefaultHookFile is the default post-create hook script.
	DefaultHookFile = "post-create.sh"
)

// Config represents the gren configuration.
type Config struct {
	MainWorktree   string `json:"main_worktree"`
	WorktreeDir    string `json:"worktree_dir"`
	PackageManager string `json:"package_manager"`
	PostCreateHook string `json:"post_create_hook"`
	Version        string `json:"version"`
}

// Manager handles configuration operations.
type Manager struct {
	configDir string
}

// NewManager creates a new configuration manager.
func NewManager() *Manager {
	return &Manager{
		configDir: ConfigDir,
	}
}

// NewDefaultConfig returns a default configuration for the given project.
// repoRoot should be the absolute path to the main worktree (where .git directory lives).
func NewDefaultConfig(projectName, repoRoot string) (*Config, error) {
	if strings.TrimSpace(projectName) == "" {
		return nil, fmt.Errorf("project name cannot be empty")
	}
	if strings.TrimSpace(repoRoot) == "" {
		return nil, fmt.Errorf("repo root cannot be empty")
	}

	// Use absolute path for worktree directory, sibling to main worktree
	worktreeDir := filepath.Join(filepath.Dir(repoRoot), projectName+"-worktrees")

	return &Config{
		MainWorktree:   repoRoot,
		WorktreeDir:    worktreeDir,
		PackageManager: "auto",
		PostCreateHook: filepath.Join(ConfigDir, DefaultHookFile),
		Version:        DefaultVersion,
	}, nil
}

// Load reads the configuration from the config file.
func (m *Manager) Load() (*Config, error) {
	configPath := filepath.Join(m.configDir, ConfigFile)

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("configuration not found: run 'gren init' first")
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := m.validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// Save writes the configuration to the config file.
func (m *Manager) Save(config *Config) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	if err := m.validateConfig(config); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Ensure config directory exists
	if err := os.MkdirAll(m.configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath := filepath.Join(m.configDir, ConfigFile)

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Exists checks if the configuration file exists.
func (m *Manager) Exists() bool {
	configPath := filepath.Join(m.configDir, ConfigFile)
	_, err := os.Stat(configPath)
	return err == nil
}

// validateConfig validates the configuration fields.
func (m *Manager) validateConfig(config *Config) error {
	if strings.TrimSpace(config.MainWorktree) == "" {
		return fmt.Errorf("main_worktree cannot be empty")
	}

	if strings.TrimSpace(config.WorktreeDir) == "" {
		return fmt.Errorf("worktree_dir cannot be empty")
	}

	if strings.TrimSpace(config.Version) == "" {
		return fmt.Errorf("version cannot be empty")
	}

	// Validate package manager if specified
	if config.PackageManager != "" && config.PackageManager != "auto" {
		validManagers := []string{"npm", "yarn", "pnpm", "bun"}
		valid := false
		for _, manager := range validManagers {
			if config.PackageManager == manager {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid package_manager: %s (must be one of: %s, auto)",
				config.PackageManager, strings.Join(validManagers, ", "))
		}
	}

	return nil
}
