package events

import (
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Prune deletes old event files. Both caps apply independently: no file
// older than `maxAge` is kept, and at most `keepCount` files are kept
// overall (newest wins). Best-effort: errors on individual removes are
// ignored. Returns nil for a nonexistent dir (normal startup case).
func Prune(dir string, keepCount int, maxAge time.Duration) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	type fileInfo struct {
		path  string
		mtime time.Time
	}
	files := make([]fileInfo, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if filepath.Ext(e.Name()) != ".ndjson" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, fileInfo{filepath.Join(dir, e.Name()), info.ModTime()})
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].mtime.After(files[j].mtime) // newest first
	})

	cutoff := time.Now().Add(-maxAge)
	keep := make(map[string]bool)
	for i, f := range files {
		if i >= keepCount {
			continue // exceeds hard count cap
		}
		if !f.mtime.After(cutoff) {
			continue // older than age cap
		}
		keep[f.path] = true
	}
	for _, f := range files {
		if !keep[f.path] {
			_ = os.Remove(f.path)
		}
	}
	return nil
}
