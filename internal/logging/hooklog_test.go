package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewHookLogCreatesFile(t *testing.T) {
	t.Setenv("GREN_LOG_DIR", t.TempDir())
	f, path, err := NewHookLog("post-create", "feat/x")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("hello"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Errorf("hook log content = %q, want hello", data)
	}
	if !strings.Contains(filepath.Base(path), "post-create-feat-x-") {
		t.Errorf("unexpected hook log name %q", filepath.Base(path))
	}
}

func TestPruneHookLogsKeepsNewest(t *testing.T) {
	t.Setenv("GREN_LOG_DIR", t.TempDir())
	dir := HookLogDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		p := filepath.Join(dir, fmt.Sprintf("post-create-b-%d.log", i))
		if err := os.WriteFile(p, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
		mt := time.Now().Add(time.Duration(i) * time.Minute)
		if err := os.Chtimes(p, mt, mt); err != nil {
			t.Fatal(err)
		}
	}
	PruneHookLogs(2, 24*time.Hour)
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Errorf("expected 2 newest files kept, got %d", len(entries))
	}
}
