package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// HookLogDir is where per-run hook output logs live.
func HookLogDir() string {
	return filepath.Join(getLogDir(), "hooks")
}

// NewHookLog creates and opens a per-run hook output log, returning the open
// file (caller closes it) and its path.
func NewHookLog(hookType, branch string) (*os.File, string, error) {
	dir := HookLogDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, "", fmt.Errorf("create hook log dir: %w", err)
	}
	name := fmt.Sprintf("%s-%s-%d.log", sanitizeLabel(hookType), sanitizeLabel(branch), time.Now().UnixNano())
	path := filepath.Join(dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, "", err
	}
	return f, path, nil
}

// PruneHookLogs keeps the newest keepCount hook logs and deletes any older than
// maxAge. Best-effort; errors are ignored.
func PruneHookLogs(keepCount int, maxAge time.Duration) {
	entries, err := os.ReadDir(HookLogDir())
	if err != nil {
		return
	}
	type fileInfo struct {
		path string
		mod  time.Time
	}
	var files []fileInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, fileInfo{filepath.Join(HookLogDir(), e.Name()), info.ModTime()})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod.After(files[j].mod) })
	cutoff := time.Now().Add(-maxAge)
	for i, f := range files {
		if i >= keepCount || f.mod.Before(cutoff) {
			_ = os.Remove(f.path)
		}
	}
}

// sanitizeLabel makes s safe as one filename segment.
func sanitizeLabel(s string) string {
	if s == "" {
		return "none"
	}
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, s)
}
