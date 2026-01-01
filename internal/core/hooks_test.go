package core

import (
	"encoding/json"
	"testing"

	"github.com/langtind/gren/internal/config"
)

func TestHookJSONContext(t *testing.T) {
	ctx := HookJSONContext{
		HookType:      "post-create",
		Branch:        "feature/test",
		Worktree:      "/path/to/worktree",
		WorktreeName:  "test-worktree",
		Repo:          "my-repo",
		RepoRoot:      "/path/to/repo",
		Commit:        "abc123def456",
		ShortCommit:   "abc123d",
		DefaultBranch: "main",
		TargetBranch:  "main",
		BaseBranch:    "main",
		ExecuteCmd:    "npm test",
	}

	// Test JSON marshaling
	data, err := json.Marshal(ctx)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}

	// Verify JSON structure
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}

	tests := []struct {
		key   string
		value string
	}{
		{"hook_type", "post-create"},
		{"branch", "feature/test"},
		{"worktree", "/path/to/worktree"},
		{"worktree_name", "test-worktree"},
		{"repo", "my-repo"},
		{"repo_root", "/path/to/repo"},
		{"commit", "abc123def456"},
		{"short_commit", "abc123d"},
		{"default_branch", "main"},
		{"target_branch", "main"},
		{"base_branch", "main"},
		{"execute_cmd", "npm test"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if val, ok := result[tt.key]; !ok {
				t.Errorf("JSON missing key %q", tt.key)
			} else if val != tt.value {
				t.Errorf("JSON key %q = %q, want %q", tt.key, val, tt.value)
			}
		})
	}
}

func TestHookJSONContextOmitEmpty(t *testing.T) {
	ctx := HookJSONContext{
		HookType: "post-switch",
		Branch:   "main",
	}

	data, err := json.Marshal(ctx)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}

	// Optional fields with omitempty should not be present when empty
	optionalFields := []string{"target_branch", "base_branch", "execute_cmd"}
	for _, field := range optionalFields {
		if _, ok := result[field]; ok {
			t.Errorf("JSON should omit empty field %q", field)
		}
	}
}

func TestHookTypes(t *testing.T) {
	// Test that all hook types are defined
	hookTypes := []config.HookType{
		config.HookPostCreate,
		config.HookPreRemove,
		config.HookPreMerge,
		config.HookPostMerge,
		config.HookPostSwitch,
		config.HookPostStart,
	}

	for _, ht := range hookTypes {
		if ht == "" {
			t.Error("HookType should not be empty")
		}
	}
}

func TestHooksGet(t *testing.T) {
	hooks := config.Hooks{
		PostCreate: "npm install",
		PreRemove:  "npm run cleanup",
		PreMerge:   "npm test",
		PostMerge:  "npm run deploy",
		PostSwitch: "npm run switch",
		PostStart:  "npm start",
	}

	tests := []struct {
		hookType config.HookType
		want     string
	}{
		{config.HookPostCreate, "npm install"},
		{config.HookPreRemove, "npm run cleanup"},
		{config.HookPreMerge, "npm test"},
		{config.HookPostMerge, "npm run deploy"},
		{config.HookPostSwitch, "npm run switch"},
		{config.HookPostStart, "npm start"},
		{config.HookType("unknown"), ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.hookType), func(t *testing.T) {
			got := hooks.Get(tt.hookType)
			if got != tt.want {
				t.Errorf("Hooks.Get(%s) = %q, want %q", tt.hookType, got, tt.want)
			}
		})
	}
}

func TestNamedHook(t *testing.T) {
	hook := config.NamedHook{
		Name:     "my-hook",
		Command:  "npm test",
		Branches: []string{"main", "develop"},
		Disabled: false,
	}

	if hook.Name != "my-hook" {
		t.Errorf("NamedHook.Name = %q, want %q", hook.Name, "my-hook")
	}
	if hook.Command != "npm test" {
		t.Errorf("NamedHook.Command = %q, want %q", hook.Command, "npm test")
	}
	if len(hook.Branches) != 2 {
		t.Errorf("NamedHook.Branches length = %d, want 2", len(hook.Branches))
	}
	if hook.Disabled {
		t.Error("NamedHook.Disabled should be false")
	}
}

func TestNamedHookDisabled(t *testing.T) {
	hook := config.NamedHook{
		Name:     "disabled-hook",
		Command:  "echo disabled",
		Disabled: true,
	}

	if !hook.Disabled {
		t.Error("NamedHook.Disabled should be true")
	}
}

func TestNamedHookBranchFilter(t *testing.T) {
	hook := config.NamedHook{
		Name:     "branch-filter-hook",
		Command:  "npm test",
		Branches: []string{"main", "release/*"},
	}

	tests := []struct {
		branch  string
		matches bool
	}{
		{"main", true},
		{"develop", false},
		{"release/*", true}, // Exact match (would need glob matching for wildcards)
	}

	for _, tt := range tests {
		t.Run(tt.branch, func(t *testing.T) {
			matches := false
			for _, b := range hook.Branches {
				if b == tt.branch {
					matches = true
					break
				}
			}
			if matches != tt.matches {
				t.Errorf("branch %q matches = %v, want %v", tt.branch, matches, tt.matches)
			}
		})
	}
}

func TestProjectNamedHooks(t *testing.T) {
	namedHooks := config.ProjectNamedHooks{
		PostCreate: []config.NamedHook{
			{Name: "install", Command: "npm install"},
			{Name: "setup", Command: "npm run setup"},
		},
		PreMerge: []config.NamedHook{
			{Name: "test", Command: "npm test"},
		},
	}

	if len(namedHooks.PostCreate) != 2 {
		t.Errorf("PostCreate hooks length = %d, want 2", len(namedHooks.PostCreate))
	}
	if len(namedHooks.PreMerge) != 1 {
		t.Errorf("PreMerge hooks length = %d, want 1", len(namedHooks.PreMerge))
	}
	if len(namedHooks.PostMerge) != 0 {
		t.Errorf("PostMerge hooks length = %d, want 0", len(namedHooks.PostMerge))
	}
}

func TestProjectNamedHooksGetNamedHooks(t *testing.T) {
	namedHooks := config.ProjectNamedHooks{
		PostCreate: []config.NamedHook{
			{Name: "install", Command: "npm install"},
		},
		PreMerge: []config.NamedHook{
			{Name: "test", Command: "npm test"},
		},
		PostMerge: []config.NamedHook{
			{Name: "deploy", Command: "npm run deploy"},
		},
		PreRemove: []config.NamedHook{
			{Name: "cleanup", Command: "npm run cleanup"},
		},
		PostSwitch: []config.NamedHook{
			{Name: "switch", Command: "npm run switch"},
		},
		PostStart: []config.NamedHook{
			{Name: "start", Command: "npm start"},
		},
	}

	tests := []struct {
		hookType config.HookType
		wantLen  int
	}{
		{config.HookPostCreate, 1},
		{config.HookPreMerge, 1},
		{config.HookPostMerge, 1},
		{config.HookPreRemove, 1},
		{config.HookPostSwitch, 1},
		{config.HookPostStart, 1},
		{config.HookType("unknown"), 0},
	}

	for _, tt := range tests {
		t.Run(string(tt.hookType), func(t *testing.T) {
			hooks := namedHooks.GetNamedHooks(tt.hookType)
			if len(hooks) != tt.wantLen {
				t.Errorf("GetNamedHooks(%s) length = %d, want %d", tt.hookType, len(hooks), tt.wantLen)
			}
		})
	}
}

func TestConfigGetAllHooks(t *testing.T) {
	cfg := &config.Config{
		Hooks: config.Hooks{
			PostCreate: "simple hook",
		},
		NamedHooks: config.ProjectNamedHooks{
			PostCreate: []config.NamedHook{
				{Name: "named1", Command: "cmd1"},
				{Name: "named2", Command: "cmd2"},
			},
		},
	}

	hooks := cfg.GetAllHooks(config.HookPostCreate)

	// Should have simple hook + 2 named hooks = 3 total
	if len(hooks) != 3 {
		t.Errorf("GetAllHooks() length = %d, want 3", len(hooks))
	}

	// First should be the simple hook (converted to NamedHook)
	if hooks[0].Command != "simple hook" {
		t.Errorf("First hook command = %q, want %q", hooks[0].Command, "simple hook")
	}
}

func TestConfigGetAllHooksNoSimple(t *testing.T) {
	cfg := &config.Config{
		NamedHooks: config.ProjectNamedHooks{
			PostCreate: []config.NamedHook{
				{Name: "named1", Command: "cmd1"},
			},
		},
	}

	hooks := cfg.GetAllHooks(config.HookPostCreate)

	if len(hooks) != 1 {
		t.Errorf("GetAllHooks() length = %d, want 1", len(hooks))
	}
}

func TestConfigGetAllHooksEmpty(t *testing.T) {
	cfg := &config.Config{}

	hooks := cfg.GetAllHooks(config.HookPostCreate)

	if len(hooks) != 0 {
		t.Errorf("GetAllHooks() length = %d, want 0", len(hooks))
	}
}

func TestConfigGetAllHooksIncludesDisabled(t *testing.T) {
	// Note: GetAllHooks returns all hooks, filtering disabled is done elsewhere
	cfg := &config.Config{
		NamedHooks: config.ProjectNamedHooks{
			PostCreate: []config.NamedHook{
				{Name: "enabled", Command: "cmd1", Disabled: false},
				{Name: "disabled", Command: "cmd2", Disabled: true},
			},
		},
	}

	hooks := cfg.GetAllHooks(config.HookPostCreate)

	// GetAllHooks includes all hooks, disabled filtering is done during execution
	if len(hooks) != 2 {
		t.Errorf("GetAllHooks() length = %d, want 2", len(hooks))
	}

	// Verify the hooks have correct commands
	if hooks[0].Command != "cmd1" {
		t.Errorf("First hook command = %q, want %q", hooks[0].Command, "cmd1")
	}
	if hooks[1].Command != "cmd2" {
		t.Errorf("Second hook command = %q, want %q", hooks[1].Command, "cmd2")
	}

	// Verify disabled flag is preserved
	if hooks[0].Disabled {
		t.Error("First hook should not be disabled")
	}
	if !hooks[1].Disabled {
		t.Error("Second hook should be disabled")
	}
}
