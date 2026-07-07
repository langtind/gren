# Design: Hook logging & observability for herdr (and terminal) failures

- **Date:** 2026-07-07
- **Status:** Approved (pending spec review)
- **Repo:** gren

## Problem

When gren runs a worktree hook inside a herdr pane and the hook fails, the pane
often closes before the user can read what went wrong — and gren's on-disk log
does **not** contain the answer either. Investigation of the real logs
(`~/Library/Logs/gren/gren.log` + `~/.config/herdr/herdr-server.log`) established:

1. **gren already logs to disk** on every path (`main.go:28` calls
   `logging.Init()`), including CLI `hook-run`.
2. **In non-interactive mode, hook stdout/stderr are captured and logged**
   (`hooks.go:320-325`, `355-366`). The one recent non-interactive failure — a
   `pre-remove` hook (2026-07-06 16:07) — was fully diagnosable from the log.
3. **In interactive mode the output is NOT captured.** `--interactive`/`--tty`
   sets `forceInteractive` (`cli.go:2638`, `worktree.go:32`), which routes hook
   execution through `hooks.go:314-319`, wiring the child straight to the pane
   TTY (`cmd.Stdout = os.Stdout`). `HookResult.Output`/`Stderr` stay empty, so a
   failure logs `hook failed: exit status N` with **blank** output.
4. The herdr integration hits **exactly** this path: `prefix+shift+g` →
   `gren.open` (`~/.config/herdr/config.toml:25`) → plugin picker → bootstrap
   pane → `gren hook-run --type post-create --interactive` (`bootstrap.sh:56`).
5. **Signal death skips all logging.** herdr panes are torn down with SIGHUP /
   SIGKILL (`herdr-server.log`: `signal=Hangup`, `signal=Kill`,
   `PaneDied for unknown pane`). On a signal, Go runs no `defer`, so
   `logging.Close()` and the failure log never fire → total silence. The
   plugin's `read -rn1` "press any key to close" guard (`bootstrap.sh:60`) is
   also killed with the pane, which is why the pane vanishes despite the guard.

Net: the moment you most need the output — a hook failure in a pane that then
closes — is precisely when the current log is blank. The recent successful
runs confirm the gap benignly: they logged `post-create hook output:` followed
by nothing.

## Goals

- Capture interactive hook output to disk **as it happens**, so a failure leaves
  a readable trace even if the pane closes or the process is killed.
- Record **how** gren died (signal / panic) instead of going silent.
- Make the log easy to reach and read (`gren logs`), and stop it growing
  unbounded / being polluted by test runs.
- Preserve the interactive setup flow exactly — a real TTY for `op`, `make seed`,
  `read` prompts, colors. No regression to what works today.

## Non-goals

- Fixing the herdr-side pane teardown (why the pane gets SIGHUP'd). That is a
  herdr/plugin concern; this work makes it *diagnosable*, not fixed.
- Per-worktree log correlation beyond the per-run hook file (no PID tagging of
  every line in the main log).
- Windows interactive capture (herdr is macOS/Linux; Windows keeps today's
  direct passthrough).

## Design

Three components. All changes are in the **gren** Go project.

### Component 1 — Capture interactive hook output via a PTY tee

The child must run against a real TTY *and* gren must see the bytes. Chosen
approach (decided): allocate a pseudo-terminal and tee its output.

- Add dependency `github.com/creack/pty` (Unix; small, de-facto standard).
- In `executeHook`, the interactive branch (`interactive || forceInteractive`)
  becomes, on Unix:
  - Open a **per-run hook log file** (Component 3 helper):
    `<logdir>/hooks/<hooktype>-<branch-sanitized>-<unixnano>.log`.
  - Build the output sink: `io.MultiWriter(hookFile, capBuf)` where `capBuf` is a
    bounded (~64 KiB tail) in-memory buffer so `HookResult.Output` is non-empty
    for the UI / failure log.
  - Put the outer terminal (`os.Stdin`) into raw mode with `golang.org/x/term`
    (already a dependency); `defer term.Restore`.
  - `ptmx, err := pty.Start(cmd)` (sets the child's stdio to the pty slave).
  - Set initial size + handle SIGWINCH via `pty.InheritSize(os.Stdin, ptmx)`.
  - `go io.Copy(ptmx, os.Stdin)` (input); `io.Copy(io.MultiWriter(os.Stdout,
    sink), ptmx)` (output, blocks to EOF); then `err = cmd.Wait()`.
  - Close the hook file; write one pointer line to the main log:
    `hook output → <path>`.
- Result wiring: `result.Output = capBuf.String()`, `result.Stderr = ""`
  (a single PTY merges stdout+stderr — expected and how a terminal shows it).
  The existing failure log (`hooks.go:355`) now has real content + the pointer.
- **Windows** (`//go:build windows`): keep today's direct `os.Stdout` passthrough
  (no capture); log a debug note. Not a herdr target.
- Non-interactive mode is unchanged (separate stdout/stderr string buffers).

Rationale for a per-run file (not inline in `gren.log`): raw setup output (dozens
of `make seed` lines, ANSI) would swamp the structured main log; a sibling file
mirrors the existing `events` NDJSON pattern and is what `gren logs --last`
points at.

### Component 2 — Log abnormal termination

- In `main.go`, install `signal.Notify(sigCh, SIGHUP, SIGTERM)` early. On signal:
  `logging.Warn("terminating on signal: %v", s)`, `logging.Close()`,
  `os.Exit(128 + signum)`. This makes pane teardown (SIGHUP) and herdr kill
  (SIGTERM) leave a cause-of-death line + flush.
- SIGINT is **not** globally trapped: during an interactive hook the outer
  terminal is in raw mode, so Ctrl-C passes through the PTY to the child's line
  discipline and interrupts the hook (unchanged behaviour).
- Panic: a top-level `defer recover()` in `main()` logs `panic: <v>` + stack
  (`runtime/debug.Stack()`) to disk, then re-panics (exit behaviour unchanged).
  Limitation noted: cannot recover panics in other goroutines.
- Because the log file is `O_APPEND` and unbuffered, every streamed line from
  Component 1 is already on disk before death — so even a SIGKILL preserves the
  partial output. Component 2 only adds the "why".

### Component 3 — `gren logs` command + hygiene

- **`gren logs`** (new CLI command, `internal/cli`):
  - default: print the log path, then the last N (≈50) lines.
  - `--path`: print only the path (for `$EDITOR "$(gren logs --path)"`).
  - `-f` / `--follow`: native tail-follow (seek end, poll).
  - `--last` / `--errors`: print the last `[ERROR]` block and, if the run has a
    `hook output → <path>` pointer, the tail of that per-run hook file.
  - `--hooks`: list recent per-run hook logs under `<logdir>/hooks/`.
- **Rotation** in `logging.Init`: before opening, if `gren.log` > 5 MiB, rotate
  `gren.log` → `.1` → `.2` → `.3` (keep 3), then open fresh. No new dependency.
- **`GREN_LOG_DIR` override** in `getLogDir()`: if set, use it verbatim. The test
  suite sets it to `t.TempDir()` so tests stop writing to the real user log
  (today's 11:31 test burst is exactly this pollution). Documented for users too.
- **Hook-log retention**: `PruneHookLogs()` keeps ~20 files / 7 days, mirroring
  `events.Prune`; swept best-effort on each interactive hook spawn.

## File-level changes

- `internal/core/hooks.go` — interactive branch tees through PTY; failure log
  uses captured tail + pointer.
- `internal/core/hooks_pty_unix.go` / `hooks_pty_windows.go` — build-tagged PTY
  runner vs. passthrough fallback.
- `internal/logging/logging.go` — `GREN_LOG_DIR` override; size rotation;
  `NewHookLog(hookType, branch)` + `HookLogDir()` + `PruneHookLogs()`.
- `main.go` — signal handler (SIGHUP/SIGTERM) + panic recovery.
- `internal/cli/cli.go` (+ `help.go`, `completion.go`) — `logs` command, flags,
  help text, completions.
- `go.mod` / `go.sum` — add `github.com/creack/pty`.

## Testing

Run with `-p 1` (repo convention — tests `os.Chdir`).

- **logging**: rotation (write >5 MiB, assert rotated + fresh + backups kept);
  `GREN_LOG_DIR` override honored; `NewHookLog` path + `PruneHookLogs` retention.
- **PTY capture**: run `executeHook` interactively under a PTY (per
  `docs/debugging.md` §2, `script`/creack-pty), assert the per-run hook file
  contains the hook's echoed output and `HookResult.Output` is non-empty on
  failure. This is the regression guard for the core gap.
- **`gren logs`**: seed a temp log via `GREN_LOG_DIR`; assert `--path`, tail,
  and `--last` (finds the seeded ERROR block).
- **Signal/panic**: unit-test the panic-recovery writes a `panic:` line to disk.
  Signal-kill (`SIGHUP` to a pty'd `hook-run`) is an optional/flaky E2E — mark
  as such, don't gate CI on it.

## Risks / notes

- PTY merges stdout+stderr for interactive runs (documented; non-interactive
  keeps them separate).
- Raw-mode terminal must be restored on every exit path (`defer` + it's moot on
  pane teardown since the pane is closing anyway).
- `creack/pty` is Unix-only → build-tag fallback for Windows.
- Validate the real herdr flow (build local, swap over the brew binary per
  `docs/debugging.md` §1) before releasing — do not ship on unit tests alone.
