package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const (
	// ConfigDir is the directory where gren configuration is stored.
	ConfigDir = ".gren"
	// ConfigFileTOML is the TOML configuration file name (preferred).
	ConfigFileTOML = "config.toml"
	// ConfigFileJSON is the legacy JSON configuration file name.
	ConfigFileJSON = "config.json"
	// ConfigFile is kept for backward compatibility, points to JSON.
	ConfigFile = ConfigFileJSON
	// DefaultVersion is the current configuration version.
	DefaultVersion = "1.0.0"
	// DefaultHookFile is the default post-create hook script.
	DefaultHookFile = "post-create.sh"
)

// Config represents the gren configuration.
type Config struct {
	// MainWorktree is deprecated - main worktree is now detected dynamically
	// Kept for backwards compatibility with old configs, but not used or saved
	MainWorktree   string `json:"main_worktree,omitempty" toml:"main_worktree,omitempty"`
	WorktreeDir    string `json:"worktree_dir" toml:"worktree_dir"`
	PackageManager string `json:"package_manager" toml:"package_manager"`
	PostCreateHook string `json:"post_create_hook" toml:"post_create_hook"`
	Version        string `json:"version" toml:"version"`
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
		// MainWorktree intentionally not set - detected dynamically now
		WorktreeDir:    worktreeDir,
		PackageManager: "auto",
		PostCreateHook: filepath.Join(ConfigDir, DefaultHookFile),
		Version:        DefaultVersion,
	}, nil
}

// Load reads the configuration from the config file.
// Tries TOML first (config.toml), then falls back to JSON (config.json).
func (m *Manager) Load() (*Config, error) {
	var config Config
	var data []byte
	var err error
	var usedTOML bool

	// Try TOML first (preferred format)
	tomlPath := filepath.Join(m.configDir, ConfigFileTOML)
	data, err = os.ReadFile(tomlPath)
	if err == nil {
		usedTOML = true
		if err := toml.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse TOML config file: %w", err)
		}
	} else if os.IsNotExist(err) {
		// Fall back to JSON
		jsonPath := filepath.Join(m.configDir, ConfigFileJSON)
		data, err = os.ReadFile(jsonPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("configuration not found: run 'gren init' first")
			}
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config file: %w", err)
		}
	} else {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Note: MainWorktree from old configs is ignored - now detected dynamically
	_ = usedTOML // Can be used for logging/debugging

	if err := m.validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// Save writes the configuration to the config file in TOML format.
// If a JSON config exists, it will be removed after successfully saving TOML.
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

	// Save as TOML (preferred format)
	configPath := filepath.Join(m.configDir, ConfigFileTOML)

	data, err := toml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Add a header comment
	header := "# gren configuration\n# See https://github.com/langtind/gren for documentation\n\n"
	data = append([]byte(header), data...)

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Remove legacy JSON config if it exists
	jsonPath := filepath.Join(m.configDir, ConfigFileJSON)
	if _, err := os.Stat(jsonPath); err == nil {
		os.Remove(jsonPath) // Ignore errors - not critical
	}

	return nil
}

// SaveJSON writes the configuration to JSON format (for backward compatibility).
func (m *Manager) SaveJSON(config *Config) error {
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

	configPath := filepath.Join(m.configDir, ConfigFileJSON)

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Exists checks if any configuration file exists (TOML or JSON).
func (m *Manager) Exists() bool {
	// Check TOML first (preferred)
	tomlPath := filepath.Join(m.configDir, ConfigFileTOML)
	if _, err := os.Stat(tomlPath); err == nil {
		return true
	}
	// Fall back to JSON
	jsonPath := filepath.Join(m.configDir, ConfigFileJSON)
	_, err := os.Stat(jsonPath)
	return err == nil
}

// ExistsTOML checks if TOML configuration exists.
func (m *Manager) ExistsTOML() bool {
	tomlPath := filepath.Join(m.configDir, ConfigFileTOML)
	_, err := os.Stat(tomlPath)
	return err == nil
}

// ExistsJSON checks if legacy JSON configuration exists.
func (m *Manager) ExistsJSON() bool {
	jsonPath := filepath.Join(m.configDir, ConfigFileJSON)
	_, err := os.Stat(jsonPath)
	return err == nil
}

// validateConfig validates the configuration fields.
func (m *Manager) validateConfig(config *Config) error {
	// Note: MainWorktree validation removed - now detected dynamically

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
