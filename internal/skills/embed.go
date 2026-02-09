package skills

import (
	_ "embed"
	"strings"
)

//go:embed gren_setup.md
var GrenSetupSkill string

// GetGrenSetupPrompt returns the gren-setup skill body without YAML frontmatter.
func GetGrenSetupPrompt() string {
	return GetSkillBody(GrenSetupSkill)
}

// GetSkillBody strips YAML frontmatter from a skill markdown file.
// Frontmatter is delimited by "---" at the start and end.
// If no frontmatter is found, the content is returned as-is.
func GetSkillBody(content string) string {
	// Must start with "---"
	if !strings.HasPrefix(content, "---") {
		return content
	}

	// Find the closing "---"
	rest := content[3:]
	idx := strings.Index(rest, "\n---")
	if idx == -1 {
		return content
	}

	// Skip past the closing "---" and any trailing newline
	body := rest[idx+4:]
	body = strings.TrimLeft(body, "\n")
	return body
}
