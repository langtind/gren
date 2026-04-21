package events

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEventsDir_LinuxXDG(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only")
	}
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	got, err := EventsDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(tmp, "gren", "events")
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestEventsDir_LinuxDefault(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only")
	}
	t.Setenv("XDG_STATE_HOME", "")
	got, err := EventsDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(got, filepath.Join(".local", "state", "gren", "events")) {
		t.Errorf("unexpected path: %s", got)
	}
}

func TestEventsDir_MacOS(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin-only")
	}
	got, err := EventsDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, filepath.Join("Library", "Application Support", "gren", "events")) {
		t.Errorf("unexpected path: %s", got)
	}
}

func TestNewEventsFile_CreatesDirAndFile(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path, err := NewEventsFile("post-create", "mybranch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer os.Remove(path)
	if _, err := os.Stat(path); err != nil {
		t.Errorf("events file not created: %v", err)
	}
	if !strings.Contains(filepath.Base(path), "post-create") {
		t.Errorf("filename should include hook type, got: %s", path)
	}
}
