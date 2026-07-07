# Hook Logging & Observability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make gren hook failures diagnosable after the fact — even when a herdr pane closes or the process is killed — by capturing interactive hook output to disk, logging abnormal deaths, and adding a `gren logs` command.

**Architecture:** Interactive hooks currently attach straight to the pane TTY, so their output is never captured (`internal/core/hooks.go:314-319`). We route interactive runs through a pseudo-terminal and tee the combined stream to both the terminal and a per-run disk file, keeping full TTY fidelity for `op`/`make seed`/`read`. A signal + panic handler records *how* gren died. A new `gren logs` command plus size-based rotation and a `GREN_LOG_DIR` override make the log reachable and clean.

**Tech Stack:** Go, `github.com/creack/pty` (new, Unix-only), `golang.org/x/term` (existing), standard library.

## Global Constraints

- **Run tests with `-p 1`** (repo convention — tests use `os.Chdir`): `go test -p 1 ./...`.
- **Design source of truth:** `docs/superpowers/specs/2026-07-07-hook-logging-observability-design.md`.
- **New dependency limited to `github.com/creack/pty`**, Unix only; Windows keeps current behaviour via build tags.
- **Per-run hook logs** live at `<logdir>/hooks/<hooktype>-<branch>-<unixnano>.log`; `<logdir>` = `getLogDir()`.
- **Log rotation:** rotate `gren.log` at 5 MiB, keep 3 generations.
- **Commits:** Conventional Commits, lowercase subject, signed (`git commit -S`). Never `--no-verify`.
- **Branch:** all work on `feat/hook-logging-observability`.
- **Interactive detection:** `interactive || wm.forceInteractive.Load()` (set by `--interactive`/`--tty`, `internal/cli/cli.go:2638`).

---

## File Structure

- `internal/logging/logging.go` — add `GREN_LOG_DIR` override + `rotateIfLarge`; add `LogPanic`, `LogTermination`.
- `internal/logging/hooklog.go` *(new)* — `HookLogDir`, `NewHookLog`, `PruneHookLogs`, `sanitizeLabel`.
- `internal/core/hookpty_unix.go` *(new, `//go:build !windows`)* — `runInteractiveCaptured` via PTY.
- `internal/core/hookpty_windows.go` *(new, `//go:build windows`)* — passthrough fallback.
- `internal/core/hooks.go` — interactive branch tees to disk; `cappedWriter`; failure-log pointer.
- `main.go` — signal handler + panic recovery.
- `internal/cli/logs.go` *(new)* — `handleLogs` + pure helpers `tailLines`, `lastErrorBlock`, `followFile`, `listHookLogs`.
- `internal/cli/cli.go` — dispatch `case "logs"`.
- `internal/cli/help.go`, `internal/cli/completion.go` — surface the command.

---

## Task 1: Log-dir override + size rotation

**Files:**
- Modify: `internal/logging/logging.go` (`getLogDir`, `Init`)
- Test: `internal/logging/logging_test.go`

**Interfaces:**
- Produces: `getLogDir()` honours `GREN_LOG_DIR`; `rotateIfLarge(path string, maxBytes int64, backups int)`.

- [ ] **Step 1: Write failing tests**

Append to `internal/logging/logging_test.go`:

```go
func TestGetLogDirHonorsEnvOverride(t *testing.T) {
	t.Setenv("GREN_LOG_DIR", "/tmp/custom-gren-logs")
	if got := getLogDir(); got != "/tmp/custom-gren-logs" {
		t.Errorf("getLogDir() = %q, want /tmp/custom-gren-logs", got)
	}
}

func TestRotateIfLargeRotatesWhenOverLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gren.log")
	if err := os.WriteFile(path, bytes.Repeat([]byte("x"), 20), 0644); err != nil {
		t.Fatal(err)
	}
	rotateIfLarge(path, 10, 3) // 20 bytes > 10 → rotate
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected %s to be rotated away", path)
	}
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Errorf("expected %s.1 to exist: %v", path, err)
	}
}

func TestRotateIfLargeKeepsSmallFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gren.log")
	if err := os.WriteFile(path, []byte("small"), 0644); err != nil {
		t.Fatal(err)
	}
	rotateIfLarge(path, 1024, 3)
	if _, err := os.Stat(path); err != nil {
		t.Errorf("small file should be left in place: %v", err)
	}
}
```

- [ ] **Step 2: Run tests, verify they fail**

Run: `go test -p 1 ./internal/logging/ -run 'TestGetLogDirHonorsEnvOverride|TestRotateIfLarge'`
Expected: FAIL — `rotateIfLarge` undefined; override not honoured.

- [ ] **Step 3: Implement**

In `internal/logging/logging.go`, add the override as the first lines of `getLogDir`:

```go
func getLogDir() string {
	if dir := os.Getenv("GREN_LOG_DIR"); dir != "" {
		return dir
	}
	switch runtime.GOOS {
	// ... existing cases unchanged ...
	}
}
```

Add near the top (after the `var (...)` block) the constants and rotation helper:

```go
const (
	maxLogBytes = 5 * 1024 * 1024 // rotate gren.log past 5 MiB
	logBackups  = 3               // keep gren.log.1 .. .3
)

// rotateIfLarge rotates path -> path.1 -> path.2 -> path.3 when it exceeds
// maxBytes, keeping `backups` generations. Best-effort: on any error the
// existing file is left in place and Init just appends to it.
func rotateIfLarge(path string, maxBytes int64, backups int) {
	info, err := os.Stat(path)
	if err != nil || info.Size() <= maxBytes {
		return
	}
	_ = os.Remove(fmt.Sprintf("%s.%d", path, backups))
	for i := backups - 1; i >= 1; i-- {
		_ = os.Rename(fmt.Sprintf("%s.%d", path, i), fmt.Sprintf("%s.%d", path, i+1))
	}
	_ = os.Rename(path, path+".1")
}
```

In `Init`, call it right after computing `logPath`, before `os.OpenFile`:

```go
	logPath = filepath.Join(logDir, "gren.log")
	rotateIfLarge(logPath, maxLogBytes, logBackups)
```

- [ ] **Step 4: Run tests, verify pass**

Run: `go test -p 1 ./internal/logging/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/logging/logging.go internal/logging/logging_test.go
git commit -S -m "feat(logging): add GREN_LOG_DIR override and size-based rotation"
```

---

## Task 2: Per-run hook log files

**Files:**
- Create: `internal/logging/hooklog.go`
- Test: `internal/logging/hooklog_test.go`

**Interfaces:**
- Consumes: `getLogDir()` (Task 1).
- Produces: `HookLogDir() string`; `NewHookLog(hookType, branch string) (*os.File, string, error)`; `PruneHookLogs(keepCount int, maxAge time.Duration)`.

- [ ] **Step 1: Write failing tests**

Create `internal/logging/hooklog_test.go`:

```go
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
```

- [ ] **Step 2: Run tests, verify they fail**

Run: `go test -p 1 ./internal/logging/ -run 'HookLog'`
Expected: FAIL — `NewHookLog`/`HookLogDir`/`PruneHookLogs` undefined.

- [ ] **Step 3: Implement**

Create `internal/logging/hooklog.go`:

```go
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
```

- [ ] **Step 4: Run tests, verify pass**

Run: `go test -p 1 ./internal/logging/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/logging/hooklog.go internal/logging/hooklog_test.go
git commit -S -m "feat(logging): add per-run hook output log helpers"
```

---

## Task 3: PTY tee capture for interactive hooks

**Files:**
- Create: `internal/core/hookpty_unix.go` (`//go:build !windows`)
- Create: `internal/core/hookpty_windows.go` (`//go:build windows`)
- Create: `internal/core/hookpty_unix_test.go` (`//go:build !windows`)
- Create: `internal/core/hooks_capture_test.go`
- Modify: `internal/core/hooks.go` (interactive branch ~313-325, failure log ~355; add `cappedWriter`, constants, `io` import)
- Modify: `go.mod`, `go.sum` (add `github.com/creack/pty`)

**Interfaces:**
- Consumes: `logging.NewHookLog`, `logging.PruneHookLogs` (Task 2).
- Produces: `runInteractiveCaptured(cmd *exec.Cmd, stdin io.Reader, out io.Writer) error`; `cappedWriter{limit int}` with `Write` + `String()`.

- [ ] **Step 1: Add the dependency**

Run:
```bash
go get github.com/creack/pty@latest
```
Expected: `go.mod`/`go.sum` gain `github.com/creack/pty`.

- [ ] **Step 2: Write failing tests**

Create `internal/core/hooks_capture_test.go`:

```go
package core

import "testing"

func TestCappedWriterKeepsTail(t *testing.T) {
	w := &cappedWriter{limit: 5}
	if _, err := w.Write([]byte("abcdefgh")); err != nil {
		t.Fatal(err)
	}
	if w.String() != "defgh" {
		t.Errorf("cappedWriter tail = %q, want defgh", w.String())
	}
}
```

Create `internal/core/hookpty_unix_test.go`:

```go
//go:build !windows

package core

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func TestRunInteractiveCapturedTeesOutput(t *testing.T) {
	cmd := exec.Command("sh", "-c", "echo hello-stdout; echo hello-stderr >&2")
	var out bytes.Buffer
	if err := runInteractiveCaptured(cmd, strings.NewReader(""), &out); err != nil {
		t.Fatalf("run: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "hello-stdout") {
		t.Errorf("captured output missing stdout: %q", got)
	}
	if !strings.Contains(got, "hello-stderr") {
		t.Errorf("captured output missing stderr (pty merges streams): %q", got)
	}
}

func TestRunInteractiveCapturedPropagatesExit(t *testing.T) {
	cmd := exec.Command("sh", "-c", "exit 3")
	var out bytes.Buffer
	if err := runInteractiveCaptured(cmd, strings.NewReader(""), &out); err == nil {
		t.Fatal("expected non-nil error for exit 3")
	}
}
```

- [ ] **Step 3: Run tests, verify they fail**

Run: `go test -p 1 ./internal/core/ -run 'CappedWriter|RunInteractiveCaptured'`
Expected: FAIL — `cappedWriter` and `runInteractiveCaptured` undefined.

- [ ] **Step 4: Implement the PTY runner (Unix)**

Create `internal/core/hookpty_unix.go`:

```go
//go:build !windows

package core

import (
	"io"
	"os"
	"os/signal"
	"os/exec"
	"syscall"

	"github.com/creack/pty"
	"golang.org/x/term"
)

// runInteractiveCaptured runs cmd attached to a pseudo-terminal, copying stdin
// into the pty and the pty output to out (typically a MultiWriter of the real
// terminal and a disk sink). The child sees a real TTY, so interactive tools,
// prompts, and colors behave exactly as with direct stdio. Returns cmd's exit
// error. stdout and stderr are merged (a single pty), as a terminal shows them.
func runInteractiveCaptured(cmd *exec.Cmd, stdin io.Reader, out io.Writer) error {
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return err
	}
	defer func() { _ = ptmx.Close() }()

	// Keep the pty sized to the controlling terminal.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			_ = pty.InheritSize(os.Stdin, ptmx)
		}
	}()
	ch <- syscall.SIGWINCH // initial resize
	defer signal.Stop(ch)

	// Put the outer terminal in raw mode so keystrokes pass through untouched;
	// the pty's line discipline handles echo/cooking. Skip when stdin isn't a
	// terminal (tests / non-tty callers).
	if f, ok := stdin.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		if oldState, err := term.MakeRaw(int(f.Fd())); err == nil {
			defer func() { _ = term.Restore(int(f.Fd()), oldState) }()
		}
	}

	go func() { _, _ = io.Copy(ptmx, stdin) }()
	_, _ = io.Copy(out, ptmx) // drains until the child closes the pty (EIO on Linux is expected)
	return cmd.Wait()
}
```

- [ ] **Step 5: Implement the Windows fallback**

Create `internal/core/hookpty_windows.go`:

```go
//go:build windows

package core

import (
	"io"
	"os"
	"os/exec"
)

// runInteractiveCaptured on Windows has no pty: run with direct stdio but tee
// stdout/stderr into out on a best-effort basis. herdr is macOS/Linux; this
// only keeps the Windows build green.
func runInteractiveCaptured(cmd *exec.Cmd, stdin io.Reader, out io.Writer) error {
	cmd.Stdin = stdin
	cmd.Stdout = io.MultiWriter(os.Stdout, out)
	cmd.Stderr = io.MultiWriter(os.Stderr, out)
	return cmd.Run()
}
```

- [ ] **Step 6: Add `cappedWriter` to hooks.go**

In `internal/core/hooks.go`, add `"io"` to the import block, and add near the top (after the imports):

```go
const (
	hookTailLimit  = 64 * 1024        // bytes of hook output kept in memory for logs/UI
	hookLogKeep    = 20               // per-run hook logs retained
	hookLogMaxAge  = 7 * 24 * time.Hour
)

// cappedWriter keeps only the last `limit` bytes written — a bounded tail so
// hook output can surface in logs/UI without unbounded memory.
type cappedWriter struct {
	buf   []byte
	limit int
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	if len(w.buf) > w.limit {
		w.buf = w.buf[len(w.buf)-w.limit:]
	}
	return len(p), nil
}

func (w *cappedWriter) String() string { return string(w.buf) }
```

- [ ] **Step 7: Route the interactive branch through the tee**

In `internal/core/hooks.go`, replace the interactive/non-interactive block (currently ~313-325) with:

```go
	var stdoutBuf, stderrBuf strings.Builder
	var hookLogPath string
	if interactive || wm.forceInteractive.Load() {
		// Interactive: run against a real TTY (op / make seed / read all work),
		// but tee the combined output to a per-run disk log and a capped tail so a
		// failure leaves a trace even if the pane closes or gren is killed mid-run.
		hookFile, path, logErr := logging.NewHookLog(string(hookType), ctx.BranchName)
		if logErr != nil {
			logging.Warn("could not open hook log, capture disabled: %v", logErr)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err = cmd.Run()
		} else {
			hookLogPath = path
			tail := &cappedWriter{limit: hookTailLimit}
			sink := io.MultiWriter(hookFile, tail)
			err = runInteractiveCaptured(cmd, os.Stdin, io.MultiWriter(os.Stdout, sink))
			_ = hookFile.Close()
			stdoutBuf.WriteString(tail.String())
			logging.Info("%s hook output → %s", hookType, path)
			logging.PruneHookLogs(hookLogKeep, hookLogMaxAge)
		}
	} else {
		cmd.Stdin = strings.NewReader(string(jsonData))
		cmd.Stdout = &stdoutBuf
		cmd.Stderr = &stderrBuf
		err = cmd.Run()
	}
```

Then in the failure branch (currently ~355), add the pointer after the existing `logging.Error(...)`:

```go
	if err != nil {
		logging.Error("%s hook failed: %v\nstdout: %s\nstderr: %s",
			hookType, err, result.Output, result.Stderr)
		if hookLogPath != "" {
			logging.Error("%s hook full output → %s", hookType, hookLogPath)
		}
	} else {
```

- [ ] **Step 8: Run tests, verify pass**

Run: `go test -p 1 ./internal/core/ -run 'CappedWriter|RunInteractiveCaptured'`
Expected: PASS.
Run: `go build ./...`
Expected: builds clean.

- [ ] **Step 9: Commit**

```bash
git add go.mod go.sum internal/core/hookpty_unix.go internal/core/hookpty_windows.go \
        internal/core/hookpty_unix_test.go internal/core/hooks_capture_test.go internal/core/hooks.go
git commit -S -m "feat(core): capture interactive hook output to disk via a pty tee"
```

---

## Task 4: Log abnormal termination (signals + panics)

**Files:**
- Modify: `internal/logging/logging.go` (add `LogPanic`, `LogTermination`)
- Modify: `main.go` (signal handler + panic recovery)
- Test: `internal/logging/logging_test.go`

**Interfaces:**
- Produces: `logging.LogPanic(recovered any, stack []byte)`; `logging.LogTermination(sig os.Signal)`.

- [ ] **Step 1: Write failing test**

Append to `internal/logging/logging_test.go`:

```go
func TestLogPanicWritesStackToDisk(t *testing.T) {
	tmpDir := t.TempDir()
	logFilePath := filepath.Join(tmpDir, "test.log")
	file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}

	origLogger, origLogFile, origEnabled := logger, logFile, enabled
	defer func() { logger, logFile, enabled = origLogger, origLogFile, origEnabled }()
	logFile, logger, enabled = file, log.New(file, "", 0), true

	LogPanic("boom", []byte("stackframe-xyz"))
	file.Close()

	data, err := os.ReadFile(logFilePath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "panic: boom") || !strings.Contains(content, "stackframe-xyz") {
		t.Errorf("log missing panic/stack, got: %s", content)
	}
}
```

- [ ] **Step 2: Run test, verify it fails**

Run: `go test -p 1 ./internal/logging/ -run TestLogPanicWritesStackToDisk`
Expected: FAIL — `LogPanic` undefined.

- [ ] **Step 3: Implement logging helpers**

Add to `internal/logging/logging.go`:

```go
// LogPanic records a recovered panic (value + stack) to disk, best-effort.
func LogPanic(recovered any, stack []byte) {
	Error("panic: %v\n%s", recovered, stack)
}

// LogTermination records that the process is exiting because of a signal, then
// closes the log so the line is flushed before the process goes away. Used from
// a signal handler, since Go runs no deferred funcs on signal death.
func LogTermination(sig os.Signal) {
	Warn("terminating on signal: %v", sig)
	Close()
}
```

- [ ] **Step 4: Run test, verify it passes**

Run: `go test -p 1 ./internal/logging/ -run TestLogPanicWritesStackToDisk`
Expected: PASS.

- [ ] **Step 5: Wire signal + panic handling into main**

In `main.go`, add imports `os/signal`, `syscall`, `runtime/debug` (keep existing imports). Immediately after `defer logging.Close()` (currently line 31), insert:

```go
	// Record abnormal termination. On SIGHUP (pane/terminal closed) or SIGTERM
	// (herdr killing the pane) Go runs no deferred funcs — without this the log
	// goes silent exactly when a hook was mid-run.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGTERM)
	go func() {
		s := <-sigCh
		logging.LogTermination(s)
		if sysSig, ok := s.(syscall.Signal); ok {
			os.Exit(128 + int(sysSig))
		}
		os.Exit(1)
	}()

	// Record panics to disk before they hit stderr and the process dies.
	defer func() {
		if r := recover(); r != nil {
			logging.LogPanic(r, debug.Stack())
			logging.Close()
			panic(r) // preserve original crash behaviour and exit code
		}
	}()
```

- [ ] **Step 6: Verify build + full logging tests**

Run: `go build ./...`
Expected: builds clean.
Run: `go test -p 1 ./internal/logging/`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/logging/logging.go internal/logging/logging_test.go main.go
git commit -S -m "feat: log abnormal termination on signal or panic"
```

> **Manual E2E (not automated — note in PR):** in a herdr worktree, `prefix+shift+g` to create a worktree whose `post-create` hook fails, then `gren logs --last` should show the captured hook output; closing the pane mid-hook should leave a `terminating on signal: hangup` line. Build/swap the local binary per `docs/debugging.md` §1 before releasing.

---

## Task 5: `gren logs` command

**Files:**
- Create: `internal/cli/logs.go`
- Create: `internal/cli/logs_test.go`
- Modify: `internal/cli/cli.go` (add `case "logs"`)
- Modify: `internal/cli/help.go`, `internal/cli/completion.go` (surface command)

**Interfaces:**
- Consumes: `logging.GetLogPath()`, `logging.HookLogDir()` (Tasks 1–2).
- Produces: `handleLogs(args []string) error`; pure helpers `tailLines(s string, n int) []string`, `lastErrorBlock(s string) string`.

- [ ] **Step 1: Write failing tests**

Create `internal/cli/logs_test.go`:

```go
package cli

import (
	"strings"
	"testing"
)

func TestTailLinesReturnsLastN(t *testing.T) {
	got := tailLines("a\nb\nc\nd\n", 2)
	if len(got) != 2 || got[0] != "c" || got[1] != "d" {
		t.Errorf("tailLines = %v, want [c d]", got)
	}
}

func TestLastErrorBlockFindsLastWithContinuation(t *testing.T) {
	logText := strings.Join([]string{
		"[2026-07-07 10:00:00.000] [INFO] started",
		"[2026-07-07 10:00:01.000] [ERROR] first fail",
		"[2026-07-07 10:00:02.000] [INFO] ok",
		"[2026-07-07 10:00:03.000] [ERROR] post-create hook failed: exit status 1",
		"stdout: boom",
		"more boom",
	}, "\n")
	got := lastErrorBlock(logText)
	if !strings.Contains(got, "exit status 1") || !strings.Contains(got, "more boom") {
		t.Errorf("lastErrorBlock missing last block/continuation: %q", got)
	}
	if strings.Contains(got, "first fail") {
		t.Errorf("lastErrorBlock returned an earlier error too: %q", got)
	}
}

func TestLastErrorBlockNoErrors(t *testing.T) {
	if got := lastErrorBlock("[2026-07-07 10:00:00.000] [INFO] fine"); !strings.Contains(got, "no [ERROR]") {
		t.Errorf("lastErrorBlock = %q, want no-error message", got)
	}
}
```

- [ ] **Step 2: Run tests, verify they fail**

Run: `go test -p 1 ./internal/cli/ -run 'TailLines|LastErrorBlock'`
Expected: FAIL — helpers undefined.

- [ ] **Step 3: Implement the command + helpers**

Create `internal/cli/logs.go`:

```go
package cli

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/langtind/gren/internal/logging"
)

// handleLogs implements `gren logs`.
func (c *CLI) handleLogs(args []string) error {
	fs := flag.NewFlagSet("logs", flag.ExitOnError)
	pathOnly := fs.Bool("path", false, "Print only the log file path")
	follow := fs.Bool("f", false, "Follow the log (like tail -f)")
	fs.BoolVar(follow, "follow", false, "Follow the log (like tail -f)")
	last := fs.Bool("last", false, "Print the last error block and its hook-output pointer")
	fs.BoolVar(last, "errors", false, "Alias for --last")
	hooks := fs.Bool("hooks", false, "List recent per-run hook output logs")
	n := fs.Int("n", 50, "Number of trailing lines to print")
	if err := fs.Parse(args); err != nil {
		return err
	}

	path := logging.GetLogPath()
	if path == "" {
		return fmt.Errorf("logging not initialised; no log path")
	}
	if *pathOnly {
		fmt.Println(path)
		return nil
	}
	if *hooks {
		return listHookLogs()
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read log: %w", err)
	}
	if *last {
		fmt.Println(lastErrorBlock(string(content)))
		return nil
	}
	for _, line := range tailLines(string(content), *n) {
		fmt.Println(line)
	}
	if *follow {
		return followFile(path)
	}
	return nil
}

// tailLines returns the last n lines of s, in order.
func tailLines(s string, n int) []string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if n > 0 && len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}

// lastErrorBlock returns the last "[ERROR]" line plus any non-timestamped
// continuation lines (captured stdout/stderr) that follow it, or a friendly
// message if there are none. Timestamped lines start with "[20".
func lastErrorBlock(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	idx := -1
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.Contains(lines[i], "[ERROR]") {
			idx = i
			break
		}
	}
	if idx == -1 {
		return "no [ERROR] entries in log"
	}
	end := idx + 1
	for end < len(lines) && !strings.HasPrefix(lines[end], "[20") {
		end++
	}
	return strings.Join(lines[idx:end], "\n")
}

// followFile prints new lines appended to path until interrupted.
func followFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		return err
	}
	reader := bufio.NewReader(f)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			fmt.Print(line)
			continue
		}
		if err != nil {
			time.Sleep(200 * time.Millisecond)
		}
	}
}

// listHookLogs prints the paths of recent per-run hook output logs.
func listHookLogs() error {
	dir := logging.HookLogDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Println("no hook logs yet")
		return nil
	}
	for _, e := range entries {
		if !e.IsDir() {
			fmt.Println(filepath.Join(dir, e.Name()))
		}
	}
	return nil
}
```

- [ ] **Step 4: Wire dispatch**

In `internal/cli/cli.go`, add to the `switch command` block (after `case "hook-run":`):

```go
	case "logs":
		return c.handleLogs(args[2:])
```

- [ ] **Step 5: Run tests + build**

Run: `go test -p 1 ./internal/cli/ -run 'TailLines|LastErrorBlock'`
Expected: PASS.
Run: `go build -o /tmp/gren-dev . && /tmp/gren-dev logs --path`
Expected: prints the log path (e.g. `~/Library/Logs/gren/gren.log`).

- [ ] **Step 6: Surface in help + completion**

In `internal/cli/help.go`, find the command list (grep for `shell-init`) and add a line modelled on the neighbours:

```go
	fmt.Println("  " + cyan("logs") + "            " + dim("Show gren's log (--path, -f, --last, --hooks)"))
```

In `internal/cli/completion.go`, add `logs` to the command word list in each of `bashCompletionScript`, `zshCompletionScript`, `fishCompletionScript` (grep for `hook-run` in each and add `logs` alongside it, matching the existing quoting/format).

- [ ] **Step 7: Verify build**

Run: `go build ./... && go test -p 1 ./internal/cli/`
Expected: builds clean; tests PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/cli/logs.go internal/cli/logs_test.go internal/cli/cli.go \
        internal/cli/help.go internal/cli/completion.go
git commit -S -m "feat(cli): add gren logs command"
```

---

## Task 6: Full verification + docs

**Files:**
- Modify: `internal/config`/`README`/`docs/debugging.md` as needed (documentation only)

- [ ] **Step 1: Full test suite**

Run: `go test -p 1 ./...`
Expected: all PASS.

- [ ] **Step 2: Document the log surface**

Add a short "Logs" note to `internal/gren` skill docs / `README.md` (grep for the existing "Debugging"/"Log File Location" section in `CLAUDE.md` for tone): mention `gren logs`, per-run hook logs under `<logdir>/hooks/`, `GREN_LOG_DIR`, and rotation.

- [ ] **Step 3: Real herdr validation (manual)**

Per `docs/debugging.md` §1: `go build -o /tmp/gren-dev . && cp /tmp/gren-dev /opt/homebrew/bin/gren`. In herdr, `prefix+shift+g`, create a worktree with a deliberately failing `post-create` hook, confirm the pane's failure is now readable via `gren logs --last` and the pointer file. Restore the brew binary afterward.

- [ ] **Step 4: Commit docs**

```bash
git add -A
git commit -S -m "docs: document gren logs and hook output capture"
```

---

## Self-Review

**Spec coverage:**
- Component 1 (PTY capture) → Tasks 2 + 3. ✅ per-run file, capped tail, Windows fallback, pointer line.
- Component 2 (abnormal-death logging) → Task 4. ✅ signals + panic.
- Component 3 (`gren logs` + rotation + `GREN_LOG_DIR` + retention) → Tasks 1, 2 (prune), 5. ✅
- Non-goal (herdr pane teardown) → untouched. ✅

**Placeholder scan:** No TBD/TODO; every code step has complete code. Completion-script edit (Task 5 Step 6) is guided by grep because the three scripts' exact quoting differs — the instruction names the anchor (`hook-run`) and the token to add (`logs`).

**Type consistency:** `runInteractiveCaptured(cmd *exec.Cmd, stdin io.Reader, out io.Writer) error` identical in unix/windows/tests. `cappedWriter{limit}` + `String()` consistent between hooks.go and its test. `NewHookLog`/`PruneHookLogs`/`HookLogDir` signatures match between definition (Task 2) and use (Task 3, Task 5). `LogPanic(any, []byte)` / `LogTermination(os.Signal)` match between definition and main.go use.
