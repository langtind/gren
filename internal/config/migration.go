package config

import (
	"fmt"
	"strconv"
	"strings"
)

// CurrentConfigVersion is the latest config version.
// Bump this when adding new config fields that require migration.
const CurrentConfigVersion = "1.1.0"

// MigrationResult contains information about a config migration.
type MigrationResult struct {
	OldVersion     string
	NewVersion     string
	WasJSON        bool
	FieldsAdded    []string
	FieldsMigrated []string
}

// NeedsMigration checks if the config needs to be migrated.
// Returns true if the config version is older than CurrentConfigVersion,
// or if the config is in JSON format (needs TOML conversion).
func (m *Manager) NeedsMigration() (bool, *MigrationResult, error) {
	result := &MigrationResult{
		NewVersion: CurrentConfigVersion,
	}

	// Check if using JSON format
	result.WasJSON = m.ExistsJSON() && !m.ExistsTOML()

	// If no config exists, no migration needed
	if !m.Exists() {
		return false, nil, nil
	}

	// Load current config
	config, err := m.Load()
	if err != nil {
		return false, nil, fmt.Errorf("failed to load config for migration check: %w", err)
	}

	result.OldVersion = config.Version
	if result.OldVersion == "" {
		result.OldVersion = "0.0.0" // Pre-versioned config
	}

	// Check if version is older
	needsMigration := compareVersions(result.OldVersion, CurrentConfigVersion) < 0

	// Also need migration if still on JSON
	if result.WasJSON {
		needsMigration = true
	}

	return needsMigration, result, nil
}

// Migrate updates the config to the latest version.
// This adds any missing fields with sensible defaults and updates the version.
func (m *Manager) Migrate() (*MigrationResult, error) {
	needsMigration, result, err := m.NeedsMigration()
	if err != nil {
		return nil, err
	}

	if !needsMigration {
		return nil, nil // Nothing to do
	}

	// Load current config
	config, err := m.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config for migration: %w", err)
	}

	// Track what we're migrating
	if config.Version == "" {
		result.FieldsMigrated = append(result.FieldsMigrated, "version (was empty)")
	}

	// Migrate legacy post_create_hook to Hooks.PostCreate
	if config.PostCreateHook != "" && config.Hooks.PostCreate == "" {
		config.Hooks.PostCreate = config.PostCreateHook
		config.PostCreateHook = "" // Clear legacy field
		result.FieldsMigrated = append(result.FieldsMigrated, "post_create_hook → hooks.post-create")
	}

	// Update version
	config.Version = CurrentConfigVersion

	// Save config (will convert JSON to TOML if needed)
	if err := m.Save(config); err != nil {
		return nil, fmt.Errorf("failed to save migrated config: %w", err)
	}

	return result, nil
}

// GetMigrationMessage returns a user-friendly message about pending migration.
func (m *Manager) GetMigrationMessage() (string, error) {
	needsMigration, result, err := m.NeedsMigration()
	if err != nil {
		return "", err
	}

	if !needsMigration {
		return "", nil
	}

	var parts []string

	if result.WasJSON {
		parts = append(parts, "JSON → TOML format")
	}

	if result.OldVersion != CurrentConfigVersion {
		parts = append(parts, fmt.Sprintf("v%s → v%s", result.OldVersion, CurrentConfigVersion))
	}

	return fmt.Sprintf("Config update available (%s)", strings.Join(parts, ", ")), nil
}

// compareVersions compares two semantic version strings.
// Returns -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2.
func compareVersions(v1, v2 string) int {
	// Remove 'v' prefix if present
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	// Pad with zeros to make same length
	for len(parts1) < 3 {
		parts1 = append(parts1, "0")
	}
	for len(parts2) < 3 {
		parts2 = append(parts2, "0")
	}

	for i := 0; i < 3; i++ {
		n1, _ := strconv.Atoi(parts1[i])
		n2, _ := strconv.Atoi(parts2[i])

		if n1 < n2 {
			return -1
		}
		if n1 > n2 {
			return 1
		}
	}

	return 0
}
