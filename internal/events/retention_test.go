package events

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPruneOldFiles_KeepsNewestAgeCapTrumpsCount(t *testing.T) {
	// 25 files staggered 1 day apart. Age cap (7d) is stricter than count
	// cap (20) for this arrangement — files older than 7 days are pruned
	// even though they'd fit under the count cap. File i has mtime
	// (25-i) days ago; i=19..24 are strictly within 7 days → 6 survivors.
	dir := t.TempDir()
	for i := 0; i < 25; i++ {
		p := filepath.Join(dir, "20260101T000000Z-post-create-"+itoa(i)+".ndjson")
		if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		mtime := time.Now().Add(-time.Duration(25-i) * 24 * time.Hour)
		_ = os.Chtimes(p, mtime, mtime)
	}
	if err := Prune(dir, 20, 7*24*time.Hour); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 6 {
		t.Errorf("expected 6 files strictly within 7-day window, got %d", len(entries))
	}
}

func TestPruneOldFiles_HardCapAppliesEvenWhenAllRecent(t *testing.T) {
	// Both caps apply independently: hard keepCount cap still trims even
	// when every file is within the age window.
	dir := t.TempDir()
	for i := 0; i < 25; i++ {
		p := filepath.Join(dir, "20260101T000000Z-post-create-"+itoa(i)+".ndjson")
		if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		// Stagger mtimes so newest-first order is deterministic.
		mtime := time.Now().Add(-time.Duration(25-i) * time.Second)
		_ = os.Chtimes(p, mtime, mtime)
	}
	if err := Prune(dir, 20, 7*24*time.Hour); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 20 {
		t.Errorf("expected 20 files after hard cap, got %d", len(entries))
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
