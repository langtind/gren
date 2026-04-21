package core

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/langtind/gren/internal/config"
	"github.com/langtind/gren/internal/events"
)

// TestExecuteHook_EventObserverReceivesEventsInOrder verifies that events
// delivered to the observer match, in order, what ends up in HookResult.Events.
// This is the core contract: observer sees everything the result sees.
func TestExecuteHook_EventObserverReceivesEventsInOrder(t *testing.T) {
	repo := mkRepo(t)
	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	t.Setenv("HOME", stateDir)

	script := filepath.Join(repo, "hook.sh")
	body := `#!/bin/sh
set -e
emit() { printf '{"ts":"%s","phase":"%s","status":"%s"}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$1" "$2" >> "$GREN_EVENTS_FILE"; }
emit install start
sleep 0.05
emit install ok
emit migrate start
sleep 0.05
emit migrate ok
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	wm := &WorktreeManager{}
	var mu sync.Mutex
	var seen []events.Event
	wm.SetEventObserver(func(e events.Event) {
		mu.Lock()
		seen = append(seen, e)
		mu.Unlock()
	})

	ctx := HookContext{WorktreePath: repo, BranchName: "main", RepoRoot: repo}
	result := wm.executeHook(config.HookPostCreate, "./hook.sh", ctx, "", false)
	if result.Err != nil {
		t.Fatalf("hook failed: %v, output: %s", result.Err, result.Output)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(seen) != len(result.Events) {
		t.Fatalf("observer saw %d events, result has %d", len(seen), len(result.Events))
	}
	for i := range seen {
		if seen[i].Phase != result.Events[i].Phase || seen[i].Status != result.Events[i].Status {
			t.Errorf("event %d mismatch: observer=%s/%s result=%s/%s",
				i, seen[i].Phase, seen[i].Status, result.Events[i].Phase, result.Events[i].Status)
		}
	}
}

// TestExecuteHook_EventObserverReceivesSyntheticInterrupted: when the hook
// exits non-zero with an open phase, the synthetic interrupted event must
// reach the observer too — otherwise live UIs can't mark the failed phase.
func TestExecuteHook_EventObserverReceivesSyntheticInterrupted(t *testing.T) {
	repo := mkRepo(t)
	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	t.Setenv("HOME", stateDir)

	script := filepath.Join(repo, "hook.sh")
	body := `#!/bin/sh
emit() { printf '{"ts":"%s","phase":"%s","status":"%s"}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$1" "$2" >> "$GREN_EVENTS_FILE"; }
emit install start
emit install ok
emit migrate start
exit 1
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	wm := &WorktreeManager{}
	var mu sync.Mutex
	var seen []events.Event
	wm.SetEventObserver(func(e events.Event) {
		mu.Lock()
		seen = append(seen, e)
		mu.Unlock()
	})

	ctx := HookContext{WorktreePath: repo, BranchName: "main", RepoRoot: repo}
	result := wm.executeHook(config.HookPostCreate, "./hook.sh", ctx, "", false)
	if result.Err == nil {
		t.Fatal("expected non-nil Err")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(seen) == 0 {
		t.Fatal("observer saw no events")
	}
	last := seen[len(seen)-1]
	if last.Status != events.StatusInterrupted || last.Phase != "migrate" {
		t.Errorf("expected synthetic migrate/interrupted as last observed event, got %+v", last)
	}
}

// TestSetEventObserver_NilClears: after clearing, subsequent hook runs
// must not call the old observer.
func TestSetEventObserver_NilClears(t *testing.T) {
	repo := mkRepo(t)
	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	t.Setenv("HOME", stateDir)

	script := filepath.Join(repo, "hook.sh")
	body := `#!/bin/sh
printf '{"ts":"2026-01-01T00:00:00Z","phase":"p","status":"ok"}\n' >> "$GREN_EVENTS_FILE"
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	wm := &WorktreeManager{}
	// Atomic because the observer callback runs on the consumer goroutine;
	// a non-atomic counter + unsynchronized read would be a race (flagged by
	// -race) if the "nil clears" contract ever regressed — exactly the
	// behaviour this test is meant to catch.
	var called atomic.Int32
	wm.SetEventObserver(func(e events.Event) {
		called.Add(1)
	})
	wm.SetEventObserver(nil)

	ctx := HookContext{WorktreePath: repo, BranchName: "main", RepoRoot: repo}
	result := wm.executeHook(config.HookPostCreate, "./hook.sh", ctx, "", false)
	if result.Err != nil {
		t.Fatalf("hook failed: %v", result.Err)
	}
	if n := called.Load(); n != 0 {
		t.Errorf("expected observer calls=0 after nil clear, got %d", n)
	}
}

// TestEventObserver_DoesNotBlockHookOnSlowObserver: if the observer is
// moderately slow, the hook must still complete (events.Tail has a bounded
// buffer but the observer is on a separate path). This is a smoke test
// that a 20ms-per-event observer does not deadlock a small hook.
func TestEventObserver_DoesNotBlockHookOnSlowObserver(t *testing.T) {
	repo := mkRepo(t)
	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	t.Setenv("HOME", stateDir)

	script := filepath.Join(repo, "hook.sh")
	body := `#!/bin/sh
emit() { printf '{"ts":"%s","phase":"%s","status":"%s"}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$1" "$2" >> "$GREN_EVENTS_FILE"; }
emit p1 start
emit p1 ok
emit p2 start
emit p2 ok
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	wm := &WorktreeManager{}
	wm.SetEventObserver(func(e events.Event) {
		time.Sleep(20 * time.Millisecond)
	})

	done := make(chan struct{})
	var result HookResult
	go func() {
		defer close(done)
		ctx := HookContext{WorktreePath: repo, BranchName: "main", RepoRoot: repo}
		result = wm.executeHook(config.HookPostCreate, "./hook.sh", ctx, "", false)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("hook did not complete within 5s with slow observer")
	}
	if result.Err != nil {
		t.Fatalf("hook failed: %v", result.Err)
	}
}
