package core

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// MarkerType represents the type of Claude activity marker
type MarkerType string

const (
	MarkerWorking MarkerType = "🤖" // Claude is actively working
	MarkerWaiting MarkerType = "💬" // Claude is waiting for input
	MarkerIdle    MarkerType = "💤" // Claude session is idle
)

// Marker represents a Claude activity marker for a worktree
type Marker struct {
	Branch    string     // Branch name this marker is for
	Type      MarkerType // Marker type (emoji)
	Timestamp time.Time  // When the marker was set
}

// MarkerManager handles Claude activity markers via git config
type MarkerManager struct {
	timeout time.Duration
}

// NewMarkerManager creates a new MarkerManager
func NewMarkerManager() *MarkerManager {
	return &MarkerManager{
		timeout: 5 * time.Second,
	}
}

// SetMarker sets a marker for a branch
func (mm *MarkerManager) SetMarker(ctx context.Context, branch string, markerType MarkerType) error {
	if branch == "" {
		return fmt.Errorf("branch name is required")
	}

	ctx, cancel := context.WithTimeout(ctx, mm.timeout)
	defer cancel()

	key := fmt.Sprintf("gren.marker.%s", sanitizeBranchForConfig(branch))
	cmd := exec.CommandContext(ctx, "git", "config", "--local", key, string(markerType))
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("git command timed out")
		}
		return fmt.Errorf("failed to set marker: %w", err)
	}

	return nil
}

// ClearMarker clears the marker for a branch
func (mm *MarkerManager) ClearMarker(ctx context.Context, branch string) error {
	if branch == "" {
		return fmt.Errorf("branch name is required")
	}

	ctx, cancel := context.WithTimeout(ctx, mm.timeout)
	defer cancel()

	key := fmt.Sprintf("gren.marker.%s", sanitizeBranchForConfig(branch))
	cmd := exec.CommandContext(ctx, "git", "config", "--local", "--unset", key)
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("git command timed out")
		}
		return nil
	}

	return nil
}

// GetMarker gets the marker for a branch
func (mm *MarkerManager) GetMarker(ctx context.Context, branch string) (MarkerType, error) {
	if branch == "" {
		return "", fmt.Errorf("branch name is required")
	}

	ctx, cancel := context.WithTimeout(ctx, mm.timeout)
	defer cancel()

	key := fmt.Sprintf("gren.marker.%s", sanitizeBranchForConfig(branch))
	cmd := exec.CommandContext(ctx, "git", "config", "--local", "--get", key)
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("git command timed out")
		}
		return "", nil
	}

	return MarkerType(strings.TrimSpace(string(output))), nil
}

// ListMarkers lists all markers in the repository
func (mm *MarkerManager) ListMarkers(ctx context.Context) (map[string]MarkerType, error) {
	ctx, cancel := context.WithTimeout(ctx, mm.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "config", "--local", "--get-regexp", "^gren\\.marker\\.")
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("git command timed out")
		}
		return make(map[string]MarkerType), nil
	}

	markers := make(map[string]MarkerType)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		value := parts[1]
		branch := strings.TrimPrefix(key, "gren.marker.")
		branch = restoreBranchFromConfig(branch)
		markers[branch] = MarkerType(value)
	}

	return markers, nil
}

// ClearAllMarkers clears all markers in the repository
func (mm *MarkerManager) ClearAllMarkers(ctx context.Context) error {
	markers, err := mm.ListMarkers(ctx)
	if err != nil {
		return err
	}

	for branch := range markers {
		if err := mm.ClearMarker(ctx, branch); err != nil {
			return fmt.Errorf("failed to clear marker for %s: %w", branch, err)
		}
	}

	return nil
}

// ParseMarkerType parses a string into a MarkerType
func ParseMarkerType(s string) (MarkerType, error) {
	switch strings.ToLower(s) {
	case "working", "work", "🤖":
		return MarkerWorking, nil
	case "waiting", "wait", "💬":
		return MarkerWaiting, nil
	case "idle", "💤":
		return MarkerIdle, nil
	default:
		// Allow custom emoji markers
		if len(s) > 0 {
			return MarkerType(s), nil
		}
		return "", fmt.Errorf("invalid marker type: %s (use: working, waiting, idle, or custom emoji)", s)
	}
}

func sanitizeBranchForConfig(branch string) string {
	// Use URL encoding to safely encode branch names with special characters
	// This ensures lossless round-tripping for branches with underscores
	return url.PathEscape(branch)
}

func restoreBranchFromConfig(key string) string {
	unescaped, err := url.PathUnescape(key)
	if err != nil {
		return key // Fallback to original if unescape fails
	}
	return unescaped
}

func SetupClaudePlugin(force bool) error {
	pluginDir := ".claude-plugin"
	hooksDir := filepath.Join(pluginDir, "hooks")

	if _, err := os.Stat(pluginDir); err == nil && !force {
		return fmt.Errorf(".claude-plugin directory already exists (use -f to overwrite)")
	}

	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}

	hooksJSON := `{
  "hooks": {
    "UserPromptSubmit": [
      { "command": "gren marker set working" }
    ],
    "Notification": [
      { "command": "gren marker set waiting" }
    ],
    "SessionEnd": [
      { "command": "gren marker clear" }
    ]
  }
}
`
	hooksPath := filepath.Join(hooksDir, "hooks.json")
	if err := os.WriteFile(hooksPath, []byte(hooksJSON), 0644); err != nil {
		return fmt.Errorf("failed to write hooks.json: %w", err)
	}

	pluginJSON := `{
  "name": "gren",
  "description": "Git worktree manager with Claude activity tracking",
  "version": "1.0.0"
}
`
	pluginPath := filepath.Join(pluginDir, "plugin.json")
	if err := os.WriteFile(pluginPath, []byte(pluginJSON), 0644); err != nil {
		return fmt.Errorf("failed to write plugin.json: %w", err)
	}

	fmt.Println("✅ Created .claude-plugin directory")
	fmt.Println("📁 .claude-plugin/")
	fmt.Println("   ├── plugin.json")
	fmt.Println("   └── hooks/")
	fmt.Println("       └── hooks.json")
	fmt.Println("")
	fmt.Println("Claude will now set markers when working in this repo:")
	fmt.Println("  🤖 working - when processing your request")
	fmt.Println("  💬 waiting - when waiting for input")
	fmt.Println("")
	fmt.Println("View markers with: gren marker list")
	fmt.Println("See in TUI: markers appear next to branch names")

	return nil
}
