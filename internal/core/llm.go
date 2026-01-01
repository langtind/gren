package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/langtind/gren/internal/config"
	"github.com/langtind/gren/internal/logging"
)

// DiffSizeThreshold is the maximum diff size in characters before truncation
const DiffSizeThreshold = 400000

// LockFiles are files to filter from diffs to save tokens
var LockFiles = []string{
	"package-lock.json",
	"yarn.lock",
	"pnpm-lock.yaml",
	"bun.lockb",
	"Cargo.lock",
	"go.sum",
	"Gemfile.lock",
	"poetry.lock",
	"Pipfile.lock",
	"composer.lock",
	"flake.lock",
}

// LLMGenerator handles commit message generation using external LLM commands
type LLMGenerator struct {
	Command  string
	Args     []string
	Template string
}

// NewLLMGenerator creates an LLM generator from config
func NewLLMGenerator(cfg *config.Config) *LLMGenerator {
	if cfg == nil || cfg.CommitGenerator.Command == "" {
		return nil
	}
	return &LLMGenerator{
		Command: cfg.CommitGenerator.Command,
		Args:    cfg.CommitGenerator.Args,
	}
}

// GenerateCommitMessage generates a commit message for staged changes
func (g *LLMGenerator) GenerateCommitMessage(diff, context string) (string, error) {
	if g == nil {
		return "", fmt.Errorf("LLM generator not configured")
	}

	// Filter lock files from diff
	filteredDiff := filterLockFiles(diff)

	// Truncate if too large
	filteredDiff, truncated := truncateDiff(filteredDiff)
	if truncated {
		logging.Debug("LLM: diff truncated to %d characters", DiffSizeThreshold)
	}

	// Build prompt
	prompt := g.buildPrompt(filteredDiff, context)

	// Execute LLM command
	cmd := exec.Command(g.Command, g.Args...)
	cmd.Stdin = strings.NewReader(prompt)

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("LLM command failed: %w", err)
	}

	return cleanCommitMessage(string(output)), nil
}

// GenerateSquashMessage generates a squash commit message
func (g *LLMGenerator) GenerateSquashMessage(diff, commitLog, branch, target string) (string, error) {
	if g == nil {
		return "", fmt.Errorf("LLM generator not configured")
	}

	// Filter lock files from diff
	filteredDiff := filterLockFiles(diff)

	// Truncate if too large
	filteredDiff, truncated := truncateDiff(filteredDiff)
	if truncated {
		logging.Debug("LLM: diff truncated to %d characters", DiffSizeThreshold)
	}

	// Build context with commit log
	context := fmt.Sprintf("Branch: %s\nTarget: %s\n\nCommits being squashed:\n%s", branch, target, commitLog)

	// Build prompt
	prompt := g.buildPrompt(filteredDiff, context)

	// Execute LLM command
	cmd := exec.Command(g.Command, g.Args...)
	cmd.Stdin = strings.NewReader(prompt)

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("LLM command failed: %w", err)
	}

	return cleanCommitMessage(string(output)), nil
}

// buildPrompt creates the prompt for the LLM
func (g *LLMGenerator) buildPrompt(diff, context string) string {
	// Check for custom template
	if g.Template != "" {
		return expandPromptTemplate(g.Template, diff, context)
	}

	// Default prompt
	var sb strings.Builder
	sb.WriteString("Generate a concise git commit message for the following changes.\n")
	sb.WriteString("Follow conventional commits format (feat:, fix:, docs:, refactor:, test:, chore:, etc.).\n")
	sb.WriteString("Be specific but concise. One line only, maximum 72 characters.\n")
	sb.WriteString("Focus on WHAT changed and WHY, not HOW.\n")
	sb.WriteString("Do not include any extra text, just the commit message.\n\n")

	if context != "" {
		sb.WriteString("Context:\n")
		sb.WriteString(context)
		sb.WriteString("\n\n")
	}

	sb.WriteString("Diff:\n")
	sb.WriteString(diff)

	return sb.String()
}

// expandPromptTemplate expands template variables in a custom prompt
func expandPromptTemplate(template, diff, context string) string {
	replacements := map[string]string{
		"{{ diff }}":    diff,
		"{{diff}}":      diff,
		"{{ context }}": context,
		"{{context}}":   context,
	}

	result := template
	for pattern, value := range replacements {
		result = strings.ReplaceAll(result, pattern, value)
	}
	return result
}

// filterLockFiles removes lock file changes from the diff
func filterLockFiles(diff string) string {
	lines := strings.Split(diff, "\n")
	var result []string
	inLockFile := false
	currentFile := ""

	for _, line := range lines {
		// Check for file header
		if strings.HasPrefix(line, "diff --git") {
			// Extract file path from diff header
			parts := strings.Split(line, " ")
			if len(parts) >= 4 {
				// Format: diff --git a/path b/path
				currentFile = strings.TrimPrefix(parts[2], "a/")
			}
			inLockFile = isLockFile(currentFile)
			if inLockFile {
				logging.Debug("LLM: filtering lock file from diff: %s", currentFile)
				continue
			}
		}

		if inLockFile {
			// Skip lines until we see a new file
			if strings.HasPrefix(line, "diff --git") {
				parts := strings.Split(line, " ")
				if len(parts) >= 4 {
					currentFile = strings.TrimPrefix(parts[2], "a/")
				}
				inLockFile = isLockFile(currentFile)
				if inLockFile {
					continue
				}
			} else {
				continue
			}
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// isLockFile checks if a file path is a lock file
func isLockFile(path string) bool {
	filename := filepath.Base(path)
	for _, lockFile := range LockFiles {
		if filename == lockFile {
			return true
		}
	}
	return false
}

// truncateDiff truncates a diff if it exceeds the threshold
func truncateDiff(diff string) (string, bool) {
	if len(diff) <= DiffSizeThreshold {
		return diff, false
	}
	return diff[:DiffSizeThreshold] + "\n\n... (diff truncated for LLM processing)", true
}

// cleanCommitMessage cleans up the LLM output to be a valid commit message
func cleanCommitMessage(output string) string {
	// Trim whitespace
	msg := strings.TrimSpace(output)

	// Remove markdown code blocks if present
	if strings.HasPrefix(msg, "```") {
		msg = strings.TrimPrefix(msg, "```")
		if idx := strings.Index(msg, "```"); idx != -1 {
			msg = msg[:idx]
		}
		msg = strings.TrimSpace(msg)
	}

	// Remove common prefixes
	prefixes := []string{
		"Commit message:",
		"commit message:",
		"Here's the commit message:",
		"Here is the commit message:",
	}
	for _, prefix := range prefixes {
		msg = strings.TrimPrefix(msg, prefix)
	}
	msg = strings.TrimSpace(msg)

	// Remove quotes if wrapped
	if (strings.HasPrefix(msg, "\"") && strings.HasSuffix(msg, "\"")) ||
		(strings.HasPrefix(msg, "'") && strings.HasSuffix(msg, "'")) {
		msg = msg[1 : len(msg)-1]
	}

	// Take only the first line if multiple lines
	if idx := strings.Index(msg, "\n"); idx != -1 {
		msg = msg[:idx]
	}

	// Truncate to 72 characters if too long
	if len(msg) > 72 {
		msg = msg[:69] + "..."
	}

	return msg
}

// CreateBackupRef creates a backup reference before squash
func CreateBackupRef(branch string) error {
	refName := fmt.Sprintf("refs/backup/%s", strings.ReplaceAll(branch, "/", "-"))

	cmd := exec.Command("git", "update-ref", refName, "HEAD")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create backup ref: %w, output: %s", err, string(output))
	}

	logging.Info("Created backup ref: %s", refName)
	return nil
}

// RestoreFromBackup restores from a backup reference
func RestoreFromBackup(branch string) error {
	refName := fmt.Sprintf("refs/backup/%s", strings.ReplaceAll(branch, "/", "-"))

	// Check if backup exists
	checkCmd := exec.Command("git", "rev-parse", "--verify", refName)
	if err := checkCmd.Run(); err != nil {
		return fmt.Errorf("no backup found for branch %s", branch)
	}

	// Reset to backup
	resetCmd := exec.Command("git", "reset", "--hard", refName)
	output, err := resetCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to restore from backup: %w, output: %s", err, string(output))
	}

	logging.Info("Restored from backup ref: %s", refName)
	return nil
}

// LoadPromptTemplate loads a custom prompt template from file
func LoadPromptTemplate(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	// Resolve relative path
	if !filepath.IsAbs(path) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		path = filepath.Join(cwd, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read template file: %w", err)
	}

	return string(data), nil
}

// ValidateConventionalCommit checks if a message follows conventional commits format
func ValidateConventionalCommit(message string) bool {
	pattern := regexp.MustCompile(`^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)(\(.+\))?: .+`)
	return pattern.MatchString(message)
}
