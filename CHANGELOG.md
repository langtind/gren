# Changelog

## [Unreleased]

### Added

- **`gren hook-run --interactive` (alias `--tty`).** Forces every hook to inherit the terminal (a real TTY) regardless of its own `interactive` setting, so an external tool — e.g. a terminal-multiplexer plugin — can run a repo's normal hooks in a pane where TTY-dependent setup works (1Password `op`, `make seed`). Unlike plain `hook-run` (auto-approve, used internally by the TUI), `--interactive` prompts for hook approval — persisted per project, so it's a one-time prompt — because a human is at the terminal. This runs the full hook config (inline, script, named, branch-filtered, and user-level hooks) instead of a single bootstrap script, so per-worktree template values like `{{ branch | hash_port }}` reach that flow too.

### Added (API)

- `core.WorktreeManager.SetForceInteractive(bool)` — make all hooks run with inherited stdio for the next run(s); mirrors `SetEventObserver`.

## [0.14.0] — 2026-07-04

### Added

- **Hook & template filters `hash_port` and `sanitize_db`.** `{{ branch | hash_port }}` maps a branch deterministically to a port in `10000–19999` (FNV-1a), and `{{ branch | sanitize_db }}` turns a branch into a safe database identifier (lowercased, non-alphanumerics → `_`, leading digit prefixed with `_`). Two parallel worktrees get stable, collision-averse ports and DB names with no shared state — the core of running parallel dev servers/agents side by side.
- **Inline hook commands now expand template variables.** Commands configured inline in `.gren/config.toml` (the `sh -c` path, e.g. `post-create = "echo PORT={{ branch | hash_port }} >> .env"`) run through the same template engine as `for-each` and `worktree_dir`. Substituted values are shell-quoted so a branch name containing shell metacharacters (git permits `$()`, backticks, `;`) can't inject commands — variables are therefore pre-quoted and hook commands must not wrap them in their own quotes. Script-file hooks are unchanged — they still receive context via args + `GREN_JSON_CONTEXT`.
- **`gren step eval <template>`** — expand a template string against the current worktree and print the result, exposing the engine to scripts: `PORT=$(gren step eval '{{ branch | hash_port }}') npm run dev` or `createdb "$(gren step eval '{{ branch | sanitize_db }}')"`.

### Added (API)

- `core.WorktreeManager.EvalTemplate(template)` — expand a template against the current worktree's context.

## [0.13.0] — 2026-07-04

### Fixed

- **`worktree_dir` templates now expand.** gren's config help advertises `worktree_dir = "../{{ repo }}-worktrees"`, but the value was used literally — you got a directory literally named `{{ repo }}`. It now runs through the template engine (`{{ repo }}`, `{{ branch }}`, `{{ branch | sanitize }}`).
- **Named-hook `branches` globs are now honored.** `[[named-hooks.post-create]] branches = ["feature/*"]` was documented but never read, so the hook ran on every branch. Hooks are now filtered by their branch globs (`filepath.Match`).
- **User-level named hooks now run.** Named hooks defined in the user config (`[[named-hooks.*]]`) were loaded but never merged into execution, so global hooks never fired. They now run for all repos, before project hooks.

### Added (API)

- `config.CollectHooks(project, user, hookType, branch)` and `config.BranchMatches(patterns, branch)` — assemble the hooks to run (user + project, branch-filtered, disabled-skipped) in one place.

## [0.12.0] — 2026-07-04

### Added

- **`gren create --no-hooks`** — create the worktree but skip the pre/post-create hooks, so a caller can run setup itself. This lets tooling (e.g. the herdr plugin) create at gren's configured `worktree_dir` and then run the post-create hook in a pane with a real TTY, so interactive setup (1Password `op`, `make seed`) works.

### Fixed

- The repo's own `.gren/post-create.sh` read `main_repo_path` from `.gren/config.json`, which no longer exists (config is `.gren/config.toml`, and gren passes the repo root to hooks as `$4`). It now uses `$4`. Dev tooling only; does not affect the gren binary.

## [0.11.0] — 2026-07-04

### Changed

- **`gren` now works without `gren init`.** A repository with no `.gren/config.toml` falls back to sensible defaults — worktrees under `../<repo>-worktrees`, package manager auto-detected, no hooks — instead of failing with `configuration not found: run 'gren init' first`. `config.Load()` now returns `DefaultRuntimeConfig()` when no config file exists; parse/validation errors on an *existing* file still surface as before. `gren init` remains for persisting customization (hooks, custom worktree dir). This matches how `git worktree` and worktrunk behave, so `gren create` (and tooling like the herdr plugin) works in any git repo.
- Config load errors now name the offending file — e.g. `invalid configuration in .gren/config.toml: worktree_dir cannot be empty` and `failed to parse .gren/config.toml: ...` — instead of a generic "config file" message, so a malformed or mistyped config is faster to locate and fix.

### Fixed

- `gren create --format=json` now reports **pre-create** hook results (#42). The JSON output previously carried only post-create hooks, and the pre-create-failure path returned a bare error with no JSON at all. Pre- and post-create results are now concatenated in lifecycle order, and a `CreateJSON` document is emitted even when a pre-create hook aborts the create — so machine-readable callers always see which hooks ran and why a create failed. The non-zero exit code is preserved.

### Added (API)

- `config.DefaultRuntimeConfig()` — the all-defaults configuration used for repositories without a `.gren` config file (empty `WorktreeDir` resolved to `../<repo>-worktrees` by consumers, `PackageManager` `auto`, no hooks).

## [0.10.0] — 2026-05-23

### Added

- **`pre-create` lifecycle hook** (#40). Runs before the worktree directory is created. Non-zero exit aborts the create — fail-fast like `pre-remove` and `pre-merge` — so preflight checks (docker stack up, secrets present, migrations clean) no longer leave an orphan worktree behind when they fail. Wired through all three layers: `config.HookPreCreate`, `WorktreeManager.RunPreCreateHookWithApproval`, and the CLI create-flow. `gren help hooks` lists it.
- **`--format=json` on `gren create`** (#39). Emits a single machine-readable object on stdout with `name`, `branch`, `path`, and a `hooks[]` array reporting `ran` / `ok` / captured `output` / `stderr` for each configured hook. Suppresses the human "Worktree created" banner and the navigate prompt (machine-mode signal). Mutually exclusive with `-x` (which writes a shell directive — interactive only). Mirrors the existing `gren list --format=json` shape so AI agents and CI scripts can branch on `.hooks[].ok` instead of scraping emoji-laden stdout.
- Live hook phase reporting in the TUI (#37). When a non-interactive post-create (or any) hook runs, a modal now shows each `emit_event` phase landing with its glyph — `…` while running, `✓`/`✗`/`⊘` once resolved — instead of freezing the TUI until the hook exits. The modal auto-dismisses 1.5s after a clean run; on failure it persists with the error, a stderr tail, a stdout tail, and the path to the NDJSON event log so users can see *where* the hook broke without digging through `gren.log`.
- Live phase streaming in the CLI too (#37). `gren create`, `gren hook-run`, and every other hook-triggering command now stream phase events to stderr as they happen. The batch summary still prints at the end for post-mortem.

### Fixed

- **`gren create` no longer hangs when stdin is not a TTY** (#38). Previously `Scanln` on the "Navigate to worktree?" prompt blocked indefinitely under piped stdin (CI, AI agents, scripts) — the worktree was created on disk but the process never returned. Now the prompt is guarded with `term.IsTerminal`, matching the pattern already used in the delete-confirmation flow. Interactive sessions are unaffected.

### Changed

- `HookResult.Output` is now stdout only; stderr is captured separately in the new `HookResult.Stderr` field (#37). Previously `CombinedOutput` merged the two, which buried runtime failure traces (e.g. bash `bad substitution`) inside normal progress output. The split lets the TUI highlight stderr as the failure cause and keeps the gren log readable.
- Release notes are now auto-grouped by Conventional Commit prefix (#41). The GitHub release for each tag now categorises entries under 🚀 Features / 🐛 Bugfixes / 🧹 Refactoring / 📚 Docs / 🧪 Tests / 🏗️ Build·chore·CI, links each item back to its PR, and credits the author.

### Added (API)

- `WorktreeManager.SetEventObserver(func(events.Event))` — register a callback invoked for every parsed phase event (including the synthetic `interrupted` event on non-zero exit). Used by the TUI's live modal and the CLI's stderr streamer.
- `WorktreeManager.RunPreCreateHookWithApproval(branchName, baseBranch, autoYes)` — new entry point for the pre-create lifecycle. Fail-fast (`Err != nil` on any result aborts the caller's lifecycle operation).
- `config.HookPreCreate`, `Hooks.PreCreate`, `ProjectNamedHooks.PreCreate` — new hook type wired into `Hooks.Get` and `GetNamedHooks` switches.

## [0.9.0] — 2026-04-21

### Added

- Structured event protocol for hook scripts. Hooks can now write NDJSON lines to `$GREN_EVENTS_FILE` to report phase-level progress (`start`, `ok`, `error`). The CLI prints a phase summary after each hook run so users see exactly what completed. If a hook exits non-zero with a phase still open, gren synthesizes an `interrupted` event — silent `SIGINT`/`SIGKILL` deaths can no longer be mistaken for success. See `README` § Hook Event Protocol. Hooks that don't emit events are unaffected.
- Event files are retained under `$XDG_STATE_HOME/gren/events/` (Linux, honoring the variable), `~/Library/Application Support/gren/events/` (macOS), or `$TMPDIR/gren/events/`. Old files are pruned on each hook spawn: anything older than 7 days is removed, capped at the 20 newest.
- `.gren/post-create.sh` generated by `gren init` now ships with an `emit_event` shell helper and a `trap` on `INT`/`TERM` for clean interrupt reporting.
