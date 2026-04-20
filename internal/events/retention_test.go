package events

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPruneOldFiles_KeepsNewest(t *testing.T) {
	dir := t.TempDir()
	// Create 25 files, stagger mtimes backwards so oldest are beyond "keep 20" window.
	for i := 0; i < 25; i++ {
		p := filepath.Join(dir, "20260101T000000Z-post-create-"+itoa(i)+".ndjson")
		if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		// Oldest gets mtime 25 days ago; each newer one is 1 day newer.
		// So files 0-17 are older than 7 days, files 18-24 are within 7 days.
		// With keepCount=20 rule: keep newest 20 (indices 5..24).
		// With maxAge=7d rule: keep 7 newest (indices 18..24).
		// Union: keep newest 20 (wins). Expect 20 files.
		mtime := time.Now().Add(-time.Duration(25-i) * 24 * time.Hour)
		_ = os.Chtimes(p, mtime, mtime)
	}
	if err := Prune(dir, 20, 7*24*time.Hour); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 20 {
		t.Errorf("expected 20 files after prune, got %d", len(entries))
	}
}

func TestPruneOldFiles_KeepsRecentEvenIfOverCount(t *testing.T) {
	// If all 25 files are within retention window, keep all — rule is
	// "keep newest N OR files newer than age, whichever keeps more".
	dir := t.TempDir()
	for i := 0; i < 25; i++ {
		p := filepath.Join(dir, "20260101T000000Z-post-create-"+itoa(i)+".ndjson")
		if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := Prune(dir, 20, 7*24*time.Hour); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 25 {
		t.Errorf("expected 25 files (all within window), got %d", len(entries))
	}
}

func TestPruneOldFiles_RemovesAncient(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "old.ndjson")
	if err := os.WriteFile(old, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	ancient := time.Now().Add(-30 * 24 * time.Hour)
	_ = os.Chtimes(old, ancient, ancient)

	fresh := filepath.Join(dir, "fresh.ndjson")
	if err := os.WriteFile(fresh, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Prune(dir, 20, 7*24*time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf("expected old file removed")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("expected fresh file kept")
	}
}

func TestPruneOldFiles_NonexistentDir(t *testing.T) {
	// Prune on a nonexistent dir should not error — normal startup case.
	if err := Prune(filepath.Join(t.TempDir(), "nope"), 20, 7*24*time.Hour); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func itoa(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return string(rune('0'+i/10)) + string(rune('0'+i%10))
}
