package events

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// EventsDir returns the directory where NDJSON event files are stored.
// Linux: $XDG_STATE_HOME/gren/events or ~/.local/state/gren/events
// macOS: ~/Library/Application Support/gren/events
// Other: /tmp/gren/events
func EventsDir() (string, error) {
	switch runtime.GOOS {
	case "linux":
		if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
			return filepath.Join(xdg, "gren", "events"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".local", "state", "gren", "events"), nil
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", "gren", "events"), nil
	default:
		return filepath.Join(os.TempDir(), "gren", "events"), nil
	}
}

// NewEventsFile creates a fresh empty events file for a hook run and
// returns its absolute path. Filename: <ts>-<hookType>-<safeLabel>.ndjson.
func NewEventsFile(hookType, label string) (string, error) {
	dir, err := EventsDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create events dir: %w", err)
	}
	safe := sanitizeLabel(label)
	ts := time.Now().UTC().Format("20060102T150405Z")
	name := fmt.Sprintf("%s-%s-%s.ndjson", ts, hookType, safe)
	path := filepath.Join(dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
	if err != nil {
		return "", err
	}
	_ = f.Close()
	return path, nil
}

func sanitizeLabel(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		case r == '/' || r == ' ' || r == '.':
			b.WriteRune('-')
		}
	}
	out := b.String()
	if out == "" {
		out = "hook"
	}
	if len(out) > 40 {
		out = out[:40]
	}
	return out
}
