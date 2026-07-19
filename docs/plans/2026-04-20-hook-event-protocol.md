# Hook Event Protocol Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Give gren a structured event protocol so post-create / pre-remove hook scripts can report phase-level progress (start / ok / error), let the TUI render live progress, and make silently-interrupted hooks impossible to mistake for success.

**Architecture:** Gren writes a per-run NDJSON events file under a state dir, exports its path via `GREN_EVENTS_FILE`, and tails it in a goroutine while the hook runs. The tailer buffers partial lines until `\n`. Events feed a `HookResult.Events` slice and a `tea.Msg` stream for the TUI. On hook exit, post-analysis marks the last unfinished `start` as `interrupted` if exit code ≠ 0. Protocol is additive — hooks that don't write events still work as today.

**Tech Stack:** Go, `bufio.Scanner` with custom split func / manual read-loop for live tailing, Bubble Tea messages for streaming to TUI, `encoding/json` for NDJSON.

**Explicit decisions already made (do not re-debate):**
- Transport: file path via `GREN_EVENTS_FILE` env var (not FD 3). Cross-platform, debuggable, survives hook crash as a post-mortem artifact.
- v1 event fields: `ts`, `phase`, `status` required; `app`, `detail` optional. No `progress`. No `warn` status — only `start`, `ok`, `error`.
- Phase names are free-form strings in v1, but documented convention is `lowercase-kebab`.
- No resume feature in this release.
- State dir: honor `$XDG_STATE_HOME` first on Linux; default to `~/.local/state/gren/events/` (Linux) or `~/Library/Application Support/gren/events/` (macOS). Outside the repo so `.gren/` stays clean.
- Retention: on each hook spawn, prune to newest 20 files OR files newer than 7 days (whichever keeps more). Documented in README.
- Tailer MUST handle partial lines live: buffer until `\n` arrives, then parse. Not just skip-last-on-exit.

**Backwards compatibility:** Hooks that don't write events are unaffected. Event file is written on every hook spawn but is empty if the hook doesn't emit. `HookResult.Output` semantics unchanged.

---

## Task 1: NDJSON event type + tolerant parser

**Files:**
- Create: `internal/events/event.go`
- Create: `internal/events/event_test.go`

**Step 1: Write the failing test**

```go
// internal/events/event_test.go
package events

import (
	"testing"
	"time"
)

func TestParseLine_Valid(t *testing.T) {
	line := `{"ts":"2026-04-20T22:51:52Z","phase":"migrate","status":"start","app":"web","detail":"alembic upgrade head"}`
	ev, err := ParseLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.Phase != "migrate" || ev.Status != StatusStart || ev.App != "web" {
		t.Errorf("unexpected event: %+v", ev)
	}
	if ev.Detail != "alembic upgrade head" {
		t.Errorf("unexpected detail: %q", ev.Detail)
	}
	wantTS, _ := time.Parse(time.RFC3339, "2026-04-20T22:51:52Z")
	if !ev.TS.Equal(wantTS) {
		t.Errorf("ts mismatch: got %v want %v", ev.TS, wantTS)
	}
}

func TestParseLine_MinimalFields(t *testing.T) {
	line := `{"ts":"2026-04-20T22:51:52Z","phase":"install","status":"ok"}`
	ev, err := ParseLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev.App != "" || ev.Detail != "" {
		t.Errorf("expected empty optional fields, got %+v", ev)
	}
}

func TestParseLine_MissingRequired(t *testing.T) {
	cases := []string{
		`{"phase":"x","status":"ok"}`,        // missing ts
		`{"ts":"2026-04-20T22:51:52Z","status":"ok"}`, // missing phase
		`{"ts":"2026-04-20T22:51:52Z","phase":"x"}`,   // missing status
	}
	for _, line := range cases {
		if _, err := ParseLine(line); err == nil {
			t.Errorf("expected error for %q", line)
		}
	}
}

func TestParseLine_UnknownStatus(t *testing.T) {
	line := `{"ts":"2026-04-20T22:51:52Z","phase":"x","status":"warn"}`
	if _, err := ParseLine(line); err == nil {
		t.Errorf("expected error for unknown status")
	}
}

func TestParseLine_GarbageJSON(t *testing.T) {
	cases := []string{"not-json", "", "   ", "{", `{"ts":"bad-timestamp","phase":"x","status":"ok"}`}
	for _, line := range cases {
		if _, err := ParseLine(line); err == nil {
			t.Errorf("expected error for %q", line)
		}
	}
}

func TestParseLine_HugeDetail(t *testing.T) {
	big := make([]byte, 128*1024)
	for i := range big {
		big[i] = 'x'
	}
	line := `{"ts":"2026-04-20T22:51:52Z","phase":"x","status":"ok","detail":"` + string(big) + `"}`
	ev, err := ParseLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ev.Detail) != len(big) {
		t.Errorf("detail truncated")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -p 1 -v ./internal/events/...`
Expected: FAIL, `ParseLine` undefined.

**Step 3: Write minimal implementation**

```go
// internal/events/event.go
package events

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Status string

const (
	StatusStart Status = "start"
	StatusOK    Status = "ok"
	StatusError Status = "error"
	// Added post-hoc by gren when a start has no matching ok/error and the hook exited non-zero.
	StatusInterrupted Status = "interrupted"
)

// Event is a single NDJSON event emitted by a hook.
type Event struct {
	TS     time.Time `json:"ts"`
	Phase  string    `json:"phase"`
	Status Status    `json:"status"`
	App    string    `json:"app,omitempty"`
	Detail string    `json:"detail,omitempty"`
}

// ParseLine parses one NDJSON line. Returns error for empty, malformed,
// missing-required, or unknown-status lines. Callers should log and skip.
func ParseLine(line string) (Event, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return Event{}, fmt.Errorf("empty line")
	}
	var raw struct {
		TS     string `json:"ts"`
		Phase  string `json:"phase"`
		Status string `json:"status"`
		App    string `json:"app"`
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return Event{}, fmt.Errorf("invalid json: %w", err)
	}
	if raw.TS == "" {
		return Event{}, fmt.Errorf("missing ts")
	}
	if raw.Phase == "" {
		return Event{}, fmt.Errorf("missing phase")
	}
	if raw.Status == "" {
		return Event{}, fmt.Errorf("missing status")
	}
	ts, err := time.Parse(time.RFC3339, raw.TS)
	if err != nil {
		return Event{}, fmt.Errorf("invalid ts: %w", err)
	}
	switch Status(raw.Status) {
	case StatusStart, StatusOK, StatusError:
		// accepted
	default:
		return Event{}, fmt.Errorf("unknown status: %q", raw.Status)
	}
	return Event{
		TS:     ts,
		Phase:  raw.Phase,
		Status: Status(raw.Status),
		App:    raw.App,
		Detail: raw.Detail,
	}, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test -p 1 -v ./internal/events/...`
Expected: PASS all 6.

**Step 5: Commit**

```bash
git add internal/events/event.go internal/events/event_test.go
git commit -m "feat(events): add NDJSON event type and tolerant parser"
```

---

## Task 2: State dir resolver with XDG_STATE_HOME support

**Files:**
- Create: `internal/events/statedir.go`
- Create: `internal/events/statedir_test.go`

**Step 1: Write the failing test**

```go
// internal/events/statedir_test.go
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
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	// Force Linux path resolution by overriding only where we can — on macOS
	// this test uses the real app-support dir; accept that as a write test.
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
```

**Step 2: Run test to verify it fails**

Run: `go test -p 1 -v ./internal/events/...`
Expected: FAIL, `EventsDir` / `NewEventsFile` undefined.

**Step 3: Write minimal implementation**

```go
// internal/events/statedir.go
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
```

**Step 4: Run test to verify it passes**

Run: `go test -p 1 -v ./internal/events/...`
Expected: PASS (or SKIP on wrong OS).

**Step 5: Commit**

```bash
git add internal/events/statedir.go internal/events/statedir_test.go
git commit -m "feat(events): add state dir resolver with XDG_STATE_HOME"
```

---

## Task 3: Retention cleanup

**Files:**
- Create: `internal/events/retention.go`
- Create: `internal/events/retention_test.go`

**Step 1: Write the failing test**

```go
// internal/events/retention_test.go
package events

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPruneOldFiles_KeepsNewest(t *testing.T) {
	dir := t.TempDir()
	// Create 25 files, all fresh.
	for i := 0; i < 25; i++ {
		p := filepath.Join(dir, "20260101T000000Z-post-create-"+itoa(i)+".ndjson")
		if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		// Stagger mtimes 1s apart so order is deterministic.
		mtime := time.Now().Add(-time.Duration(25-i) * time.Second)
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
	// (Per plan explicit decision.)
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

func itoa(i int) string {
	// avoid strconv import chatter in test helper
	if i < 10 {
		return string(rune('0' + i))
	}
	return string(rune('0'+i/10)) + string(rune('0'+i%10))
}
```

**Step 2: Run test to verify it fails**

Run: `go test -p 1 -v -run Prune ./internal/events/...`
Expected: FAIL, `Prune` undefined.

**Step 3: Write minimal implementation**

```go
// internal/events/retention.go
package events

import (
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Prune deletes old event files. Keeps the newest `keepCount` files OR any
// file newer than `maxAge` — whichever rule keeps more. Best-effort: errors
// on individual removes are ignored (logged by caller if it cares).
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
		if i < keepCount || f.mtime.After(cutoff) {
			keep[f.path] = true
		}
	}
	for _, f := range files {
		if !keep[f.path] {
			_ = os.Remove(f.path)
		}
	}
	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test -p 1 -v -run Prune ./internal/events/...`
Expected: PASS all 3.

**Step 5: Commit**

```bash
git add internal/events/retention.go internal/events/retention_test.go
git commit -m "feat(events): prune old event files (keep newest 20 or 7 days)"
```

---

## Task 4: Live tailing reader with partial-line buffering

**Files:**
- Create: `internal/events/tailer.go`
- Create: `internal/events/tailer_test.go`

**Step 1: Write the failing test**

```go
// internal/events/tailer_test.go
package events

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"
)

func TestTailer_ReadsCompleteLines(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/events.ndjson"
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan Event, 8)
	invalid := make(chan string, 8)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		Tail(ctx, path, ch, invalid)
	}()

	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	defer f.Close()

	// Write one complete event.
	f.WriteString(`{"ts":"2026-04-20T22:51:52Z","phase":"p1","status":"start"}` + "\n")
	got := <-ch
	if got.Phase != "p1" {
		t.Errorf("unexpected phase: %s", got.Phase)
	}

	// Write partial then completion — tailer must buffer.
	f.WriteString(`{"ts":"2026-04-20T22:51:53Z","phase":"p2",`)
	// Give tailer a chance to attempt a read.
	time.Sleep(50 * time.Millisecond)
	select {
	case ev := <-ch:
		t.Errorf("unexpected event from partial line: %+v", ev)
	default:
	}
	f.WriteString(`"status":"ok"}` + "\n")
	got = <-ch
	if got.Phase != "p2" || got.Status != StatusOK {
		t.Errorf("unexpected event after completion: %+v", got)
	}

	cancel()
	wg.Wait()
}

func TestTailer_ReportsInvalidLinesAndContinues(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/events.ndjson"
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := make(chan Event, 8)
	invalid := make(chan string, 8)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		Tail(ctx, path, ch, invalid)
	}()

	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	defer f.Close()
	f.WriteString("not-json\n")
	f.WriteString(`{"ts":"2026-04-20T22:51:53Z","phase":"ok","status":"ok"}` + "\n")

	// Must get the valid one even though we also got garbage.
	select {
	case ev := <-ch:
		if ev.Phase != "ok" {
			t.Errorf("unexpected event: %+v", ev)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for valid event")
	}
	// And the garbage should be reported.
	select {
	case line := <-invalid:
		if line == "" {
			t.Errorf("expected non-empty invalid line")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for invalid report")
	}

	cancel()
	wg.Wait()
}
```

**Step 2: Run test to verify it fails**

Run: `go test -p 1 -v -run Tailer ./internal/events/...`
Expected: FAIL, `Tail` undefined.

**Step 3: Write minimal implementation**

```go
// internal/events/tailer.go
package events

import (
	"bytes"
	"context"
	"io"
	"os"
	"time"
)

// Tail reads NDJSON events from path as they are appended. Valid events are
// sent to events; lines that fail ParseLine are sent to invalid (non-blocking
// drop if the channel is full). Exits when ctx is cancelled. Designed to be
// run in a goroutine.
//
// Partial lines (no trailing newline) are buffered until the newline arrives,
// both during the live run and across reads. This matters: a hook killed mid-
// write leaves a half-line, but a slow writer may also flush a partial line
// that the next write completes. Only emit on newline.
func Tail(ctx context.Context, path string, events chan<- Event, invalid chan<- string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	var buf bytes.Buffer
	chunk := make([]byte, 4096)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		for {
			n, err := f.Read(chunk)
			if n > 0 {
				buf.Write(chunk[:n])
				drainLines(&buf, events, invalid)
			}
			if err == io.EOF || n == 0 {
				break
			}
			if err != nil {
				return
			}
		}
		select {
		case <-ctx.Done():
			// Final drain — in case anything was written after last read.
			for {
				n, _ := f.Read(chunk)
				if n == 0 {
					break
				}
				buf.Write(chunk[:n])
			}
			drainLines(&buf, events, invalid)
			return
		case <-ticker.C:
		}
	}
}

// drainLines splits buf on \n, parses complete lines, leaves any trailing
// partial line in buf for later.
func drainLines(buf *bytes.Buffer, events chan<- Event, invalid chan<- string) {
	for {
		data := buf.Bytes()
		i := bytes.IndexByte(data, '\n')
		if i < 0 {
			return
		}
		line := string(data[:i])
		buf.Next(i + 1)
		if line == "" {
			continue
		}
		ev, err := ParseLine(line)
		if err != nil {
			select {
			case invalid <- line:
			default:
			}
			continue
		}
		select {
		case events <- ev:
		default:
			// Caller's buffer is full; drop to avoid blocking hook execution.
		}
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test -p 1 -v -run Tailer ./internal/events/...`
Expected: PASS both.

**Step 5: Commit**

```bash
git add internal/events/tailer.go internal/events/tailer_test.go
git commit -m "feat(events): add live NDJSON tailer with partial-line buffering"
```

---

## Task 5: Hook execution wiring — export env var, tail, attach events

**Files:**
- Modify: `internal/core/hooks.go` (HookResult struct + executeHook)
- Create: `internal/core/hooks_events_test.go`

**Step 1: Write the failing test**

A self-contained test that spawns a shell hook writing NDJSON events, then asserts `HookResult.Events` contains them. This is the integration point — verifies env var, tailer, and collection all wire up.

```go
// internal/core/hooks_events_test.go
package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/langtind/gren/internal/config"
	"github.com/langtind/gren/internal/events"
)

// mkRepo initializes a git repo in a fresh tempdir and returns its path.
func mkRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"commit", "--allow-empty", "-m", "initial"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func TestExecuteHook_CollectsEvents(t *testing.T) {
	repo := mkRepo(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

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
	ctx := HookContext{
		WorktreePath: repo,
		BranchName:   "main",
		RepoRoot:     repo,
	}
	result := wm.executeHook(config.HookPostCreate, "./hook.sh", ctx, "", false)

	if result.Err != nil {
		t.Fatalf("hook failed: %v, output: %s", result.Err, result.Output)
	}
	if len(result.Events) != 4 {
		t.Fatalf("expected 4 events, got %d: %+v", len(result.Events), result.Events)
	}
	phases := []string{}
	for _, e := range result.Events {
		phases = append(phases, e.Phase+"/"+string(e.Status))
	}
	want := "install/start,install/ok,migrate/start,migrate/ok"
	if strings.Join(phases, ",") != want {
		t.Errorf("unexpected phase order: %s", strings.Join(phases, ","))
	}
	// Verify the events file was recorded on the result so the TUI can link
	// to it for post-mortem debugging.
	if result.EventsFile == "" {
		t.Errorf("expected EventsFile to be set on result")
	}
	if _, err := os.Stat(result.EventsFile); err != nil {
		t.Errorf("events file missing: %v", err)
	}
	_ = events.StatusOK // silence unused import when trimming
}
```

**Step 2: Run test to verify it fails**

Run: `go test -p 1 -v -run TestExecuteHook_CollectsEvents ./internal/core/...`
Expected: FAIL — `Events` / `EventsFile` fields don't exist on `HookResult`.

**Step 3: Modify `internal/core/hooks.go`**

Edit the `HookResult` struct:

```go
type HookResult struct {
	Ran        bool
	Output     string
	Err        error
	Command    string
	Name       string
	Events     []events.Event // structured phase events from the hook
	EventsFile string         // absolute path to NDJSON file for post-mortem
}
```

Add import: `"github.com/langtind/gren/internal/events"` and `"context"` and `"sync"`.

Replace the body of `executeHook` (after `cmd.Env = append(os.Environ(), …)` and before the Run/CombinedOutput block) with:

```go
	// Create a per-run NDJSON events file and export its path so hooks can emit
	// structured progress events. Protocol is additive — hooks that don't write
	// to it are unaffected.
	eventsPath, evErr := events.NewEventsFile(string(hookType), ctx.BranchName)
	if evErr != nil {
		logging.Warn("events: failed to create file, disabling for this run: %v", evErr)
		eventsPath = ""
	} else {
		cmd.Env = append(cmd.Env, "GREN_EVENTS_FILE="+eventsPath)
		// Best-effort retention sweep.
		if dir, err := events.EventsDir(); err == nil {
			_ = events.Prune(dir, 20, 7*24*time.Hour)
		}
	}

	// Start live tailer in goroutine if we have a file.
	var (
		collected    []events.Event
		invalidCount int
		tailerDone   = make(chan struct{})
		tailCtx      context.Context
		tailCancel   context.CancelFunc
		evCh         chan events.Event
		invCh        chan string
		mu           sync.Mutex
	)
	if eventsPath != "" {
		tailCtx, tailCancel = context.WithCancel(context.Background())
		evCh = make(chan events.Event, 256)
		invCh = make(chan string, 64)
		go func() {
			events.Tail(tailCtx, eventsPath, evCh, invCh)
			close(tailerDone)
		}()
		// Consumer drains channels; keeps goroutine alive until Tail returns.
		go func() {
			for {
				select {
				case ev, ok := <-evCh:
					if !ok {
						return
					}
					mu.Lock()
					collected = append(collected, ev)
					mu.Unlock()
				case line, ok := <-invCh:
					if !ok {
						return
					}
					invalidCount++
					logging.Warn("events: skipped invalid line: %s", line)
				case <-tailerDone:
					// Drain any remaining buffered events.
					for {
						select {
						case ev := <-evCh:
							mu.Lock()
							collected = append(collected, ev)
							mu.Unlock()
						case <-invCh:
							invalidCount++
						default:
							return
						}
					}
				}
			}
		}()
	}
```

Then after the `cmd.Run()` / `CombinedOutput()` call:

```go
	// Stop tailer and wait for final drain before reading events.
	if eventsPath != "" {
		tailCancel()
		<-tailerDone
		// Small grace so consumer goroutine drains remaining buffered channel items.
		// This is bounded by the buffer size, not wall-clock; a second-drain loop
		// would be safer, but the consumer already drains on tailerDone close.
	}
	mu.Lock()
	eventsCopy := append([]events.Event(nil), collected...)
	mu.Unlock()
```

And set on result:

```go
	result := HookResult{
		Ran:        true,
		Output:     string(output),
		Command:    cmdDesc,
		Name:       hookName,
		Err:        err,
		Events:     eventsCopy,
		EventsFile: eventsPath,
	}
```

(Also add `"time"` import if not already present.)

**Step 4: Run test to verify it passes**

Run: `go test -p 1 -v -run TestExecuteHook_CollectsEvents ./internal/core/...`
Expected: PASS.

Also run full suite to catch regressions:

```
go test -p 1 ./...
```

Expected: all previously-passing tests still pass.

**Step 5: Commit**

```bash
git add internal/core/hooks.go internal/core/hooks_events_test.go
git commit -m "feat(core): wire hook event file and live tailer into executeHook"
```

---

## Task 6: Interrupted detection on non-zero exit

**Files:**
- Modify: `internal/core/hooks.go` (post-process events after hook exit)
- Modify/Add: `internal/core/hooks_events_test.go`

**Step 1: Write the failing test**

```go
func TestExecuteHook_MarksInterruptedOnNonzeroExit(t *testing.T) {
	repo := mkRepo(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	// Hook starts 2 phases, completes the first, starts the second but exits
	// non-zero before an ok/error for it. Expect an interrupted synthetic event.
	script := filepath.Join(repo, "hook.sh")
	body := `#!/bin/sh
emit() { printf '{"ts":"%s","phase":"%s","status":"%s"}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$1" "$2" >> "$GREN_EVENTS_FILE"; }
emit install start
emit install ok
emit migrate start
exit 130
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	wm := &WorktreeManager{}
	ctx := HookContext{WorktreePath: repo, BranchName: "main", RepoRoot: repo}
	result := wm.executeHook(config.HookPostCreate, "./hook.sh", ctx, "", false)

	if result.Err == nil {
		t.Fatal("expected non-nil Err for exit 130")
	}

	// Expect: install/start, install/ok, migrate/start, migrate/interrupted (synthetic)
	if len(result.Events) != 4 {
		t.Fatalf("expected 4 events (incl synthetic interrupted), got %d: %+v", len(result.Events), result.Events)
	}
	last := result.Events[3]
	if last.Phase != "migrate" || last.Status != events.StatusInterrupted {
		t.Errorf("expected last event migrate/interrupted, got %s/%s", last.Phase, last.Status)
	}
}

func TestExecuteHook_NoSyntheticInterruptedWhenClean(t *testing.T) {
	repo := mkRepo(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	script := filepath.Join(repo, "hook.sh")
	body := `#!/bin/sh
emit() { printf '{"ts":"%s","phase":"%s","status":"%s"}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$1" "$2" >> "$GREN_EVENTS_FILE"; }
emit install start
emit install ok
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	wm := &WorktreeManager{}
	ctx := HookContext{WorktreePath: repo, BranchName: "main", RepoRoot: repo}
	result := wm.executeHook(config.HookPostCreate, "./hook.sh", ctx, "", false)
	if result.Err != nil {
		t.Fatalf("unexpected err: %v", result.Err)
	}
	if len(result.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(result.Events))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -p 1 -v -run TestExecuteHook_Marks ./internal/core/...`
Expected: FAIL — no synthetic event appended.

**Step 3: Add post-process helper in `internal/core/hooks.go`**

```go
// finalizeEvents appends a synthetic interrupted event if the hook exited
// non-zero and the last start phase has no matching ok/error. This is how
// we surface SIGINT/SIGKILL deaths that the hook itself couldn't observe.
func finalizeEvents(evs []events.Event, hookErr error) []events.Event {
	if hookErr == nil {
		return evs
	}
	// Find last phase with status start that has no later ok/error for the
	// same phase+app pair.
	type key struct{ phase, app string }
	closed := map[key]bool{}
	for _, e := range evs {
		if e.Status == events.StatusOK || e.Status == events.StatusError {
			closed[key{e.Phase, e.App}] = true
		}
	}
	for i := len(evs) - 1; i >= 0; i-- {
		e := evs[i]
		if e.Status != events.StatusStart {
			continue
		}
		if closed[key{e.Phase, e.App}] {
			continue
		}
		evs = append(evs, events.Event{
			TS:     time.Now().UTC(),
			Phase:  e.Phase,
			App:    e.App,
			Status: events.StatusInterrupted,
			Detail: "hook exited before phase completed",
		})
		break
	}
	return evs
}
```

Call it inside `executeHook`, after the tailer drain, before constructing `result`:

```go
	eventsCopy = finalizeEvents(eventsCopy, err)
```

**Step 4: Run test to verify it passes**

Run: `go test -p 1 -v -run TestExecuteHook ./internal/core/...`
Expected: both new tests PASS, all existing still PASS.

**Step 5: Commit**

```bash
git add internal/core/hooks.go internal/core/hooks_events_test.go
git commit -m "feat(core): mark unfinished phase interrupted when hook exits nonzero"
```

---

## Task 7: TUI progress view — render phase list after hook completes (minimal v1)

**Files:**
- Modify: `internal/ui/hook_approval.go` (or wherever hook output is rendered today)
- Modify: `internal/ui/types.go` if needed for state storage

**Scope for v1:** Show the collected phase list in the existing post-hook output view — one line per event with a status glyph and phase/app/detail. Live-streaming into the TUI during hook execution is a nice-to-have but not required for v1; the events are already captured and surfaced. If wiring live `tea.Msg` is trivial given the current hook-run path, do it; if it requires refactoring `runHookByType` into a streaming `tea.Cmd`, defer.

**Step 1: Explore current hook-completion rendering**

Read the relevant files:

```
internal/ui/hook_approval.go
internal/ui/model.go (search for "hook" message handling)
internal/ui/commands.go (search for PostCreate)
```

Identify where `HookResult.Output` is currently rendered to the user after a hook completes. That's where we add an event summary.

**Step 2: Write the failing test (render function only — avoid full TUI integration)**

Create `internal/ui/hook_events_render_test.go`:

```go
package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/langtind/gren/internal/events"
)

func TestRenderHookEvents_ShowsGlyphsAndPhases(t *testing.T) {
	evs := []events.Event{
		{TS: time.Now(), Phase: "install", Status: events.StatusStart},
		{TS: time.Now(), Phase: "install", Status: events.StatusOK, Detail: "bun install done"},
		{TS: time.Now(), Phase: "migrate", Status: events.StatusInterrupted, Detail: "hook exited before phase completed"},
	}
	out := RenderHookEvents(evs)
	for _, want := range []string{"install", "migrate", "✓", "⊘", "interrupted"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got: %s", want, out)
		}
	}
}
```

**Step 3: Run test to verify it fails**

Run: `go test -p 1 -v -run TestRenderHookEvents ./internal/ui/...`
Expected: FAIL, `RenderHookEvents` undefined.

**Step 4: Implement `RenderHookEvents`**

Create `internal/ui/hook_events_render.go`:

```go
package ui

import (
	"fmt"
	"strings"

	"github.com/langtind/gren/internal/events"
)

// RenderHookEvents collapses a flat event list into per-phase status lines.
// Lives alongside the existing hook output rendering.
func RenderHookEvents(evs []events.Event) string {
	if len(evs) == 0 {
		return ""
	}
	type row struct {
		phase, app, detail string
		status             events.Status
	}
	// Key by phase+app so "migrate/web" and "migrate/api" stay distinct.
	order := []string{}
	rows := map[string]*row{}
	for _, e := range evs {
		k := e.Phase + "\x00" + e.App
		r, ok := rows[k]
		if !ok {
			r = &row{phase: e.Phase, app: e.App}
			rows[k] = r
			order = append(order, k)
		}
		r.status = e.Status
		if e.Detail != "" {
			r.detail = e.Detail
		}
	}
	var b strings.Builder
	for _, k := range order {
		r := rows[k]
		glyph := glyphFor(r.status)
		name := r.phase
		if r.app != "" {
			name = r.app + " / " + r.phase
		}
		line := fmt.Sprintf("  %s %s", glyph, name)
		if r.detail != "" {
			line += "  — " + r.detail
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

func glyphFor(s events.Status) string {
	switch s {
	case events.StatusStart:
		return "…"
	case events.StatusOK:
		return "✓"
	case events.StatusError:
		return "✗"
	case events.StatusInterrupted:
		return "⊘"
	default:
		return "?"
	}
}
```

**Step 5: Run test to verify it passes**

Run: `go test -p 1 -v -run TestRenderHookEvents ./internal/ui/...`
Expected: PASS.

**Step 6: Wire `RenderHookEvents` into the place where hook output is shown today**

Find the render path (likely in `internal/ui/create_view.go` or wherever post-create hook output appears). Prepend the event summary to the raw output block. Keep stdout/stderr visible below — events don't replace them.

**Step 7: Commit**

```bash
git add internal/ui/hook_events_render.go internal/ui/hook_events_render_test.go internal/ui/<view-file-modified>
git commit -m "feat(ui): render hook phase events in post-hook output"
```

---

## Task 8: Template — emit_event helper in generated post-create script

**Files:**
- Modify: template string for generated `post-create.sh` (grep for `#!/usr/bin/env bash` and existing template — likely in `internal/ui/states.go` around the `generateSetupScript` call)

**Step 1: Locate the template**

```
Grep: internal/ui/states.go for "generateSetupScript" / "postCreateScript"
Grep: internal/config/ for "post-create" template strings
```

**Step 2: Add `emit_event` to the top of the generated template**

Append near the top, after `set -euo pipefail`:

```bash
# gren structured-event helper. No-op if GREN_EVENTS_FILE is unset (e.g.
# when script is run manually outside of gren).
emit_event() {
  local phase="$1"
  local status="$2"
  local detail="${3:-}"
  local app="${4:-}"
  [ -z "${GREN_EVENTS_FILE:-}" ] && return 0
  local ts
  ts=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  # Minimal JSON escape — handle backslashes and double quotes in detail only.
  detail=${detail//\\/\\\\}
  detail=${detail//\"/\\\"}
  printf '{"ts":"%s","phase":"%s","status":"%s"%s%s}\n' \
    "$ts" "$phase" "$status" \
    "${app:+,\"app\":\"$app\"}" \
    "${detail:+,\"detail\":\"$detail\"}" \
    >> "$GREN_EVENTS_FILE"
}

# Auto-emit interrupt/error on signals. The Go side also synthesizes an
# 'interrupted' event when exit code ≠ 0 and a phase is still in 'start',
# but emitting on trap gives cleaner output when the hook catches the signal.
trap 'emit_event "${GREN_CURRENT_PHASE:-unknown}" error "trapped signal"; exit 130' INT TERM
```

Wrap existing template steps with `emit_event`. Example for a `install` step in the template:

```bash
GREN_CURRENT_PHASE=install
emit_event install start
if command -v bun >/dev/null; then
  bun install
elif command -v pnpm >/dev/null; then
  pnpm install
else
  npm install
fi
emit_event install ok
```

**Step 3: Verify generated template includes the helper**

Manual check: run `gren init` in a scratch repo, open generated `.gren/post-create.sh`, confirm `emit_event` is present.

No unit test required here (template is a string; the integration happens in Task 11).

**Step 4: Commit**

```bash
git add <template files>
git commit -m "feat(template): add emit_event helper to post-create script template"
```

---

## Task 9: Documentation — README protocol section

**Files:**
- Modify: `README.md`

**Step 1: Add a "Hook Event Protocol" section**

Content to include:

1. What it is, one sentence.
2. Where event files are stored (per-OS paths, XDG override).
3. Retention rule: newest 20 files OR files within 7 days.
4. The `GREN_EVENTS_FILE` env var.
5. Event schema with required/optional fields.
6. Allowed `status` values: `start`, `ok`, `error`. Note that `interrupted` is synthesized by gren.
7. Example `emit_event` shell helper (copy from template).
8. Backwards-compatibility guarantee: hooks that don't emit events still work.
9. Short troubleshooting note: "If your worktree setup seems to have finished silently but something's broken, check `~/.local/state/gren/events/` for the last `<hook>.ndjson`".

**Step 2: Commit**

```bash
git add README.md
git commit -m "docs: document hook event protocol (GREN_EVENTS_FILE)"
```

---

## Task 10: End-to-end integration test with real worktree + mock hook

**Files:**
- Create: `internal/core/hooks_events_integration_test.go`

**Step 1: Write the test**

Spawns an actual worktree creation (using the existing `WorktreeManager` API) with a project config pointing at a mock `post-create.sh` that emits a full NDJSON event sequence including a mid-run `exit 1`. Asserts:

- The hook fails (Err != nil).
- `HookResult.Events` contains the expected sequence plus a synthetic `interrupted` for any open phase.
- `HookResult.EventsFile` exists on disk and its content matches the emitted lines.

Use existing test patterns — check `internal/core/` for similar integration tests to mirror style.

**Step 2: Run**

```
go test -p 1 -v -run Integration ./internal/core/...
```

Expected: PASS.

**Step 3: Commit**

```bash
git add internal/core/hooks_events_integration_test.go
git commit -m "test(core): integration test for hook event protocol end-to-end"
```

---

## Task 11: CHANGELOG + release notes stub

**Files:**
- Modify: `CHANGELOG.md` (or create if missing)

**Step 1: Add entry under `Unreleased`**

```markdown
## [Unreleased]

### Added
- Structured event protocol for hook scripts. Hooks can now write NDJSON
  events to `$GREN_EVENTS_FILE` to report phase-level progress (`start`,
  `ok`, `error`). Events are displayed in the TUI after hook completion.
  If a hook exits non-zero with an open phase, gren marks it as
  `interrupted` — silent SIGINT/SIGKILL deaths can no longer be mistaken
  for success. See README § Hook Event Protocol. Hooks that don't emit
  events are unaffected.
- Retention: gren keeps the newest 20 event files or any from the last
  7 days under `$XDG_STATE_HOME/gren/events/` (Linux) or
  `~/Library/Application Support/gren/events/` (macOS). Pruned on each
  hook run.
```

**Step 2: Commit**

```bash
git add CHANGELOG.md
git commit -m "docs: changelog entry for hook event protocol"
```

---

## Final verification

Before handing off:

```bash
go test -p 1 ./...
go vet ./...
go build -o /tmp/gren-test .
```

All must pass. Binary builds clean.

Manual smoke test:

```bash
cd <some repo with .gren/post-create.sh using emit_event>
/tmp/gren-test create -n test-events
# observe TUI shows phase list
ls ~/.local/state/gren/events/    # (linux) or ~/Library/Application Support/gren/events/ (macOS)
```

---

## Out of scope (explicit)

- Live streaming of events into the TUI during hook execution. Events are collected during execution but rendered at the end in v1. Streaming is a follow-up.
- `progress` (0–1) field.
- `warn` status.
- Resume-from-phase support (`GREN_RESUME_FROM`). Hook authors can design around it by making phases idempotent, but gren does not drive it in v1.
- Windows support for the state directory — use `os.TempDir()` fallback; tighten later if/when Windows builds get tested.
