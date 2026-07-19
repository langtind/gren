package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/langtind/gren/internal/config"
	"github.com/langtind/gren/internal/events"
)

// Verifies the on-disk NDJSON events file contains every line the hook
// emitted and is parseable back to the same events collected in-memory.
// Tasks 5+6 already cover executeHook's collection and interrupted synthesis
// at the in-memory level; this locks the post-mortem artifact contract.
func TestExecuteHook_EventsFileMatchesCollected(t *testing.T) {
	repo := mkRepo(t)
	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	t.Setenv("HOME", stateDir)

	script := filepath.Join(repo, "hook.sh")
	body := `#!/bin/sh
emit() { printf '{"ts":"%s","phase":"%s","status":"%s"%s}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$1" "$2" "$3" >> "$GREN_EVENTS_FILE"; }
emit install start ""
emit install ok ',"detail":"bun install done"'
emit migrate start ',"app":"web"'
exit 2
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	wm := &WorktreeManager{}
	ctx := HookContext{WorktreePath: repo, BranchName: "main", RepoRoot: repo}
	result := wm.executeHook(config.HookPostCreate, "./hook.sh", ctx, "", false)

	if result.Err == nil {
		t.Fatal("expected hook to fail with exit 2")
	}
	if result.EventsFile == "" {
		t.Fatal("expected EventsFile path set")
	}

	// Read the file back and parse each line — must match what we collected
	// in-memory (minus the synthetic interrupted, which isn't written to disk).
	raw, err := os.ReadFile(result.EventsFile)
	if err != nil {
		t.Fatalf("read events file: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines on disk, got %d: %q", len(lines), raw)
	}
	parsed := make([]events.Event, 0, len(lines))
	for _, line := range lines {
		ev, err := events.ParseLine(line)
		if err != nil {
			t.Fatalf("parse disk line %q: %v", line, err)
		}
		parsed = append(parsed, ev)
	}

	// In-memory should have 3 disk events + 1 synthetic interrupted.
	if len(result.Events) != 4 {
		t.Fatalf("expected 4 in-memory events, got %d", len(result.Events))
	}
	for i := 0; i < 3; i++ {
		if result.Events[i].Phase != parsed[i].Phase || result.Events[i].Status != parsed[i].Status {
			t.Errorf("mismatch at %d: disk=%+v mem=%+v", i, parsed[i], result.Events[i])
		}
	}
	if result.Events[3].Status != events.StatusInterrupted || result.Events[3].Phase != "migrate" {
		t.Errorf("expected synthetic migrate/interrupted, got %+v", result.Events[3])
	}

	// Verify detail + app fields round-tripped correctly.
	if parsed[1].Detail != "bun install done" {
		t.Errorf("detail not preserved: %q", parsed[1].Detail)
	}
	if parsed[2].App != "web" {
		t.Errorf("app not preserved: %q", parsed[2].App)
	}
}
