package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewDefaultConfig(t *testing.T) {
	tests := []struct {
		name        string
		projectName string
		repoRoot    string
		wantErr     bool
		wantDir     string
		wantMain    string
	}{
		{
			name:        "valid project name",
			projectName: "my-project",
			repoRoot:    "/home/user/repos/my-project",
			wantErr:     false,
			wantDir:     "/home/user/repos/my-project-worktrees",
			wantMain:    "/home/user/repos/my-project",
		},
		{
			name:        "project with spaces preserved",
			projectName: "  spaced-project  ",
			repoRoot:    "/home/user/repos/spaced",
			wantErr:     false,
			wantDir:     "/home/user/repos/  spaced-project  -worktrees",
			wantMain:    "/home/user/repos/spaced",
		},
		{
			name:        "empty project name",
			projectName: "",
			repoRoot:    "/home/user/repos/test",
			wantErr:     true,
		},
		{
			name:        "whitespace only project name",
			projectName: "   ",
			repoRoot:    "/home/user/repos/test",
			wantErr:     true,
		},
		{
			name:        "empty repo root",
			projectName: "my-project",
			repoRoot:    "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := NewDefaultConfig(tt.projectName, tt.repoRoot)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewDefaultConfig() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("NewDefaultConfig() unexpected error: %v", err)
				return
			}

			if config.WorktreeDir != tt.wantDir {
				t.Errorf("WorktreeDir = %q, want %q", config.WorktreeDir, tt.wantDir)
			}

			if config.MainWorktree != tt.wantMain {
				t.Errorf("MainWorktree = %q, want %q", config.MainWorktree, tt.wantMain)
			}

			if config.Version != DefaultVersion {
				t.Errorf("Version = %q, want %q", config.Version, DefaultVersion)
			}

			if config.PackageManager != "auto" {
				t.Errorf("PackageManager = %q, want %q", config.PackageManager, "auto")
			}
		})
	}
}

func TestManagerLoadSave(t *testing.T) {
	// Create temp directory for test
	tempDir, err := os.MkdirTemp("", "gren-config-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current dir: %v", err)
	}
	defer os.Chdir(originalDir)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change to temp dir: %v", err)
	}

	manager := NewManager()

	t.Run("save and load config", func(t *testing.T) {
		config := &Config{
			MainWorktree:   tempDir,
			WorktreeDir:    "../test-worktrees",
			PackageManager: "bun",
			PostCreateHook: ".gren/post-create.sh",
			Version:        "1.0.0",
		}

		// Save
		if err := manager.Save(config); err != nil {
			t.Fatalf("Save() error: %v", err)
		}

		// Verify file exists
		configPath := filepath.Join(ConfigDir, ConfigFile)
		if _, err := os.Stat(configPath); err != nil {
			t.Fatalf("config file not created: %v", err)
		}

		// Load
		loaded, err := manager.Load()
		if err != nil {
			t.Fatalf("Load() error: %v", err)
		}

		// Verify loaded matches saved
		if loaded.WorktreeDir != config.WorktreeDir {
			t.Errorf("WorktreeDir = %q, want %q", loaded.WorktreeDir, config.WorktreeDir)
		}
		if loaded.PackageManager != config.PackageManager {
			t.Errorf("PackageManager = %q, want %q", loaded.PackageManager, config.PackageManager)
		}
		if loaded.Version != config.Version {
			t.Errorf("Version = %q, want %q", loaded.Version, config.Version)
		}
	})

	t.Run("load missing config", func(t *testing.T) {
		// Create a new temp dir without config
		emptyDir, err := os.MkdirTemp("", "gren-empty-*")
		if err != nil {
			t.Fatalf("failed to create empty dir: %v", err)
		}
		defer os.RemoveAll(emptyDir)

		if err := os.Chdir(emptyDir); err != nil {
			t.Fatalf("failed to change dir: %v", err)
		}
		defer os.Chdir(tempDir)

		newManager := NewManager()
		_, err = newManager.Load()
		if err == nil {
			t.Error("Load() expected error for missing config, got nil")
		}
	})

	t.Run("save nil config", func(t *testing.T) {
		err := manager.Save(nil)
		if err == nil {
			t.Error("Save(nil) expected error, got nil")
		}
	})
}

func TestManagerExists(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "gren-exists-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	manager := NewManager()

	t.Run("config does not exist", func(t *testing.T) {
		if manager.Exists() {
			t.Error("Exists() = true, want false for missing config")
		}
	})

	t.Run("config exists after save", func(t *testing.T) {
		config := &Config{
			MainWorktree: tempDir,
			WorktreeDir:  "../test",
			Version:      "1.0.0",
		}
		if err := manager.Save(config); err != nil {
			t.Fatalf("Save() error: %v", err)
		}

		if !manager.Exists() {
			t.Error("Exists() = false, want true after save")
		}
	})
}

func TestLoadInvalidJSON(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gren-invalid-json-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	// Create .gren directory with invalid JSON
	os.MkdirAll(ConfigDir, 0755)
	configPath := filepath.Join(ConfigDir, ConfigFile)
	os.WriteFile(configPath, []byte("{ invalid json }"), 0644)

	manager := NewManager()
	_, err = manager.Load()
	if err == nil {
		t.Error("Load() expected error for invalid JSON, got nil")
	}
}

func TestLoadInvalidConfig(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gren-invalid-config-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	// Create .gren directory with valid JSON but invalid config (empty main_worktree)
	os.MkdirAll(ConfigDir, 0755)
	configPath := filepath.Join(ConfigDir, ConfigFile)
	os.WriteFile(configPath, []byte(`{"main_worktree": "", "worktree_dir": "../test", "version": "1.0.0"}`), 0644)

	manager := NewManager()
	_, err = manager.Load()
	if err == nil {
		t.Error("Load() expected error for invalid config, got nil")
	}
}

func TestSaveInvalidConfig(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gren-save-invalid-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)
	os.Chdir(tempDir)

	manager := NewManager()

	// Try to save config with empty main_worktree
	config := &Config{
		MainWorktree: "",
		WorktreeDir:  "../test",
		Version:      "1.0.0",
	}
	err = manager.Save(config)
	if err == nil {
		t.Error("Save() expected error for invalid config, got nil")
	}
}

func TestValidateConfig(t *testing.T) {
	manager := NewManager()

	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				MainWorktree:   "/home/user/repo",
				WorktreeDir:    "../worktrees",
				PackageManager: "npm",
				Version:        "1.0.0",
			},
			wantErr: false,
		},
		{
			name: "valid config with auto package manager",
			config: &Config{
				MainWorktree:   "/home/user/repo",
				WorktreeDir:    "../worktrees",
				PackageManager: "auto",
				Version:        "1.0.0",
			},
			wantErr: false,
		},
		{
			name: "valid config with bun",
			config: &Config{
				MainWorktree:   "/home/user/repo",
				WorktreeDir:    "../worktrees",
				PackageManager: "bun",
				Version:        "1.0.0",
			},
			wantErr: false,
		},
		{
			name: "empty main worktree",
			config: &Config{
				MainWorktree: "",
				WorktreeDir:  "../worktrees",
				Version:      "1.0.0",
			},
			wantErr: true,
		},
		{
			name: "empty worktree dir",
			config: &Config{
				MainWorktree: "/home/user/repo",
				WorktreeDir:  "",
				Version:      "1.0.0",
			},
			wantErr: true,
		},
		{
			name: "empty version",
			config: &Config{
				MainWorktree: "/home/user/repo",
				WorktreeDir:  "../worktrees",
				Version:      "",
			},
			wantErr: true,
		},
		{
			name: "invalid package manager",
			config: &Config{
				MainWorktree:   "/home/user/repo",
				WorktreeDir:    "../worktrees",
				PackageManager: "invalid",
				Version:        "1.0.0",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.validateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
