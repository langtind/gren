package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseMarkerType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected MarkerType
		wantErr  bool
	}{
		{
			name:     "working keyword",
			input:    "working",
			expected: MarkerWorking,
		},
		{
			name:     "work shorthand",
			input:    "work",
			expected: MarkerWorking,
		},
		{
			name:     "working emoji",
			input:    "🤖",
			expected: MarkerWorking,
		},
		{
			name:     "waiting keyword",
			input:    "waiting",
			expected: MarkerWaiting,
		},
		{
			name:     "wait shorthand",
			input:    "wait",
			expected: MarkerWaiting,
		},
		{
			name:     "waiting emoji",
			input:    "💬",
			expected: MarkerWaiting,
		},
		{
			name:     "idle keyword",
			input:    "idle",
			expected: MarkerIdle,
		},
		{
			name:     "idle emoji",
			input:    "💤",
			expected: MarkerIdle,
		},
		{
			name:     "custom emoji",
			input:    "🔥",
			expected: MarkerType("🔥"),
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMarkerType(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMarkerType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("ParseMarkerType() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSanitizeBranchForConfig(t *testing.T) {
	tests := []struct {
		branch   string
		expected string
	}{
		{"main", "main"},
		{"feature/auth", "feature%2Fauth"},                 // URL encoding for slashes
		{"feat/user/login", "feat%2Fuser%2Flogin"},         // Multiple slashes
		{"simple-branch", "simple-branch"},                 // Hyphens unchanged
		{"feat_underscore", "feat_underscore"},             // Underscores preserved
		{"feat/with_underscore", "feat%2Fwith_underscore"}, // Mixed: slash encoded, underscore preserved
	}

	for _, tt := range tests {
		t.Run(tt.branch, func(t *testing.T) {
			got := sanitizeBranchForConfig(tt.branch)
			if got != tt.expected {
				t.Errorf("sanitizeBranchForConfig(%q) = %q, want %q", tt.branch, got, tt.expected)
			}
		})
	}
}

func TestRestoreBranchFromConfig(t *testing.T) {
	tests := []struct {
		key      string
		expected string
	}{
		{"main", "main"},
		{"feature%2Fauth", "feature/auth"},                 // URL decoding for slashes
		{"feat%2Fuser%2Flogin", "feat/user/login"},         // Multiple slashes
		{"simple-branch", "simple-branch"},                 // Hyphens unchanged
		{"feat_underscore", "feat_underscore"},             // Underscores preserved (lossless)
		{"feat%2Fwith_underscore", "feat/with_underscore"}, // Mixed: slash decoded, underscore preserved
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := restoreBranchFromConfig(tt.key)
			if got != tt.expected {
				t.Errorf("restoreBranchFromConfig(%q) = %q, want %q", tt.key, got, tt.expected)
			}
		})
	}
}

func TestSetupClaudePlugin(t *testing.T) {
	// Create a temp directory to work in
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to chdir to temp dir: %v", err)
	}
	defer os.Chdir(origDir)

	// Run setup
	err = SetupClaudePlugin(false)
	if err != nil {
		t.Fatalf("SetupClaudePlugin() error = %v", err)
	}

	// Verify directory structure
	expectedFiles := []string{
		".claude-plugin/plugin.json",
		".claude-plugin/hooks/hooks.json",
		".claude-plugin/skills/gren-setup/SKILL.md",
	}

	for _, f := range expectedFiles {
		path := filepath.Join(tmpDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Expected file %s to exist", f)
		}
	}

	// Verify SKILL.md contains frontmatter
	skillContent, err := os.ReadFile(filepath.Join(tmpDir, ".claude-plugin/skills/gren-setup/SKILL.md"))
	if err != nil {
		t.Fatalf("Failed to read SKILL.md: %v", err)
	}
	if !strings.HasPrefix(string(skillContent), "---") {
		t.Error("SKILL.md should start with YAML frontmatter")
	}
	if !strings.Contains(string(skillContent), "name: gren-setup") {
		t.Error("SKILL.md should contain 'name: gren-setup'")
	}
	if !strings.Contains(string(skillContent), "post-create hook") {
		t.Error("SKILL.md should contain post-create hook instructions")
	}

	// Verify plugin.json content
	pluginContent, err := os.ReadFile(filepath.Join(tmpDir, ".claude-plugin/plugin.json"))
	if err != nil {
		t.Fatalf("Failed to read plugin.json: %v", err)
	}
	if !strings.Contains(string(pluginContent), `"name": "gren"`) {
		t.Error("plugin.json should contain plugin name")
	}

	// Verify hooks.json content
	hooksContent, err := os.ReadFile(filepath.Join(tmpDir, ".claude-plugin/hooks/hooks.json"))
	if err != nil {
		t.Fatalf("Failed to read hooks.json: %v", err)
	}
	if !strings.Contains(string(hooksContent), "gren marker set working") {
		t.Error("hooks.json should contain marker hooks")
	}
}

func TestSetupClaudePluginAlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to chdir to temp dir: %v", err)
	}
	defer os.Chdir(origDir)

	// Create the plugin directory
	os.MkdirAll(".claude-plugin", 0755)

	// Should fail without force
	err = SetupClaudePlugin(false)
	if err == nil {
		t.Fatal("SetupClaudePlugin() should fail when directory exists without force")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("Error should mention 'already exists', got: %v", err)
	}

	// Should succeed with force
	err = SetupClaudePlugin(true)
	if err != nil {
		t.Fatalf("SetupClaudePlugin(force=true) error = %v", err)
	}

	// Verify files were created
	if _, err := os.Stat(filepath.Join(tmpDir, ".claude-plugin/skills/gren-setup/SKILL.md")); os.IsNotExist(err) {
		t.Error("SKILL.md should exist after force overwrite")
	}
}

func TestBranchEncodingRoundTrip(t *testing.T) {
	// Test that encode->decode is lossless for various branch names
	branches := []string{
		"main",
		"feature/auth",
		"feat/user/login",
		"simple-branch",
		"feat_underscore",
		"feat/with_underscore",
		"fix/bug_123",
		"release/v1.0.0",
	}

	for _, branch := range branches {
		t.Run(branch, func(t *testing.T) {
			encoded := sanitizeBranchForConfig(branch)
			decoded := restoreBranchFromConfig(encoded)
			if decoded != branch {
				t.Errorf("Round-trip failed: %q -> %q -> %q", branch, encoded, decoded)
			}
		})
	}
}
