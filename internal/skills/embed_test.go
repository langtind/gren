package skills

import (
	"strings"
	"testing"
)

func TestGrenSetupSkillEmbedded(t *testing.T) {
	if GrenSetupSkill == "" {
		t.Fatal("GrenSetupSkill should not be empty")
	}
	if !strings.HasPrefix(GrenSetupSkill, "---") {
		t.Error("GrenSetupSkill should start with YAML frontmatter")
	}
	if !strings.Contains(GrenSetupSkill, "name: gren-setup") {
		t.Error("GrenSetupSkill should contain 'name: gren-setup' in frontmatter")
	}
}

func TestGetGrenSetupPrompt(t *testing.T) {
	prompt := GetGrenSetupPrompt()
	if prompt == "" {
		t.Fatal("GetGrenSetupPrompt should not return empty string")
	}
	if strings.Contains(prompt, "name: gren-setup") {
		t.Error("GetGrenSetupPrompt should strip frontmatter")
	}
	if !strings.Contains(prompt, "Generate gren post-create hook script") {
		t.Error("GetGrenSetupPrompt should contain the skill body")
	}
}

func TestGetSkillBody(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "with frontmatter",
			input:    "---\nname: test\n---\nBody content",
			expected: "Body content",
		},
		{
			name:     "without frontmatter",
			input:    "Just plain content",
			expected: "Just plain content",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "frontmatter only no closing",
			input:    "---\nname: test\nNo closing delimiter",
			expected: "---\nname: test\nNo closing delimiter",
		},
		{
			name:     "with leading newlines after frontmatter",
			input:    "---\nname: test\n---\n\n\nBody",
			expected: "Body",
		},
		{
			name:     "complex frontmatter",
			input:    "---\nname: skill\ndescription: A test skill\nallowed-tools: Read, Glob\n---\n\n# Title\n\nContent here.",
			expected: "# Title\n\nContent here.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetSkillBody(tt.input)
			if result != tt.expected {
				t.Errorf("GetSkillBody(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
