package events

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// EventsDir returns the absolute directory where NDJSON event files are
// stored. Linux: $XDG_STATE_HOME/gren/events or ~/.local/state/gren/events
// (a relative $XDG_STATE_HOME is resolved against cwd so the returned path
// is always absolute — hooks run in their own working directories and a
// relative $GREN_EVENTS_FILE would land elsewhere than the tailer reads).
// macOS: ~/Library/Application Support/gren/events.
// Other: $TMPDIR/gren/events.
func EventsDir() (string, error) {
	switch runtime.GOOS {
	case "linux":
		if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
			abs, err := filepath.Abs(xdg)
			if err != nil {
				return "", err
			}
			return filepath.Join(abs, "gren", "events"), nil
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
// returns its absolute path. Filename includes nanosecond precision plus a
// process-scoped counter so two spawns in the same second don't collide on
// O_EXCL. Perms are user-private (0700 dir, 0600 file): events may contain
// project paths or hook command context.
func NewEventsFile(hookType, label string) (string, error) {
	dir, err := EventsDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create events dir: %w", err)
	}
	safe := sanitizeLabel(label)
	ts := time.Now().UTC().Format("20060102T150405.000000000Z")
	for attempt := 0; attempt < 8; attempt++ {
		suffix := ""
		if attempt > 0 {
			suffix = fmt.Sprintf("-%d", attempt)
		}
		name := fmt.Sprintf("%s-%s-%s%s.ndjson", ts, hookType, safe, suffix)
		path := filepath.Join(dir, name)
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
		if err == nil {
			_ = f.Close()
			return path, nil
		}
		if !os.IsExist(err) {
			return "", err
		}
	}
	return "", fmt.Errorf("could not create unique events file in %s", dir)
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
