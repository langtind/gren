# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Gren is a Git worktree manager with both TUI (Terminal UI) and CLI interfaces. It's built in Go using the Bubble Tea framework for the TUI and provides intuitive worktree management with smart project initialization.

## Architecture

### Core Components

- **Dual Interface Design**: The application supports both TUI and CLI modes, determined at runtime by command line arguments
- **main.go**: Entry point that routes between TUI and CLI modes based on arguments
- **internal/core**: Shared business logic for worktree operations (creation, deletion, listing)
- **internal/ui**: TUI implementation using Bubble Tea framework
- **internal/cli**: CLI command handlers and parsing
- **internal/git**: Git repository operations and status checking
- **internal/config**: Configuration management and project initialization

### Key Architectural Patterns

1. **Command Routing**: `main.go` parses arguments and delegates to either TUI or CLI handlers
2. **Shared Core Logic**: Both TUI and CLI use the same `internal/core.WorktreeManager` for operations
3. **Repository Abstraction**: `internal/git.Repository` interface allows for testable Git operations
4. **State Management**: TUI uses Bubble Tea's model-update-view pattern with complex state transitions

## Development Commands

### Building
```bash
go build -o gren .
```

### Building with Version Info (for releases)
```bash
VERSION=v1.0.0
COMMIT=$(git rev-parse --short HEAD)
DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}"
go build -ldflags="$LDFLAGS" -o gren .
```

### Running Locally
```bash
./gren                    # Start TUI
./gren --help            # Show help
./gren create -n test    # CLI command example
```

### Testing CLI vs TUI Modes
- No arguments or flags only → TUI mode
- Any non-flag arguments → CLI mode
- `--help` or `--version` → CLI mode

## Important Implementation Details

### Package Manager Detection
The init workflow detects package managers in this priority order:
1. `bun.lockb` or `bun.lock` files → bun
2. `yarn.lock` → yarn
3. `pnpm-lock.yaml` → pnpm
4. `"packageManager": "bun@..."` in package.json → bun
5. `package.json` exists → npm (fallback)

### Environment File Discovery
Uses `filepath.Glob(".env*")` to find ALL `.env.*` files, not just common ones. This ensures projects with custom env files (`.env.staging`, `.env.preview`, etc.) are properly detected.

### Worktree Path Resolution
- Default worktree directory: `../<repo-name>-worktrees`
- Configurable via `.gren/config.json`
- Post-create hooks run from the new worktree directory

### Missing Worktree Detection and Cleanup
The TUI automatically detects and handles missing/prunable worktrees:
- Parses `git worktree list --porcelain` output to identify "prunable" entries
- Displays missing worktrees with "❌ Missing" status and error styling
- Provides 'p' key binding to run `git worktree prune` and clean up missing entries
- Auto-refreshes worktree list after successful pruning operation

### TUI State Management
The TUI has complex state transitions between:
- Dashboard (main view with worktree listing)
- Create worktree (multi-step wizard)
- Delete worktree (confirmation flow)
- Init workflow (project setup with smart detection)
- Configuration management
- Open in... (action selection for worktrees)

Each state has its own rendering and input handling methods in `internal/ui/`.

### Version Display
Version information is displayed in both:
- Dashboard view (top-right, above the main box)
- Init welcome screen (top-right positioning)

### README.md Generation
When creating new worktrees, a README.md file is automatically generated in the worktree root directory containing:
- Information about gren worktree management
- Links to the gren repository for documentation
- Basic usage instructions

## Configuration System

Projects can be initialized with `gren init` which creates:
- `.gren/config.json` - Project configuration with worktree directory and post-create hooks
- `.gren/post-create.sh` - Executable hook script with smart dependency installation

The config includes:
- Auto-detected package manager and post-create commands
- Configurable worktree directory location

### Post-Create Hook Features
The generated post-create script includes:
- Symlinking environment files (`.env*` patterns) to main worktree
- Symlinking Claude configuration (if gitignored)
- Smart package manager detection and dependency installation
- Direnv integration if `.envrc` exists

## TUI Features

### Dashboard
- Lists all worktrees with status indicators (Clean, Modified, Missing)
- Shows current worktree with highlighting
- Displays branch information and worktree paths
- Keyboard navigation with arrow keys or vim-style `jk`
- Responsive layout: vertical stacking on narrow terminals (< 160 columns)
- Help overlay accessible with `?` key

### Key Bindings
- `↑/k` - Move up
- `↓/j` - Move down
- `n` - Create new worktree
- `d` - Delete selected worktree
- `p` - Prune missing worktrees
- `c` - Open configuration
- `i` - Initialize project (if not initialized)
- `g` - Navigate to worktree folder (requires shell integration)
- `enter` - Open selected worktree in external applications
- `?` - Show help overlay
- `q` - Quit application

### Status Indicators
- 🟢 Clean - No uncommitted changes
- 🟡 Modified - Has uncommitted changes
- 🔴 Untracked - Has untracked files
- 📝 Changes - Mixed modified and untracked files
- ❌ Missing - Worktree directory no longer exists (prunable)

### Modal Overlays
Delete confirmation and completion are shown as modal overlays on the dashboard, not as separate views. This keeps context visible while performing operations.

### Responsive Layout
The dashboard uses different layouts based on terminal width:
- **Wide (≥ 160 columns)**: Horizontal layout with table and preview panel side by side
- **Narrow (< 160 columns)**: Vertical layout with simple worktree list above and details panel below

The threshold is defined as `NarrowWidthThreshold = 160` in `internal/ui/dashboard.go`.

## Release Process

Releases are automated via GitHub Actions:
1. Push a git tag starting with `v` (e.g., `v0.1.4`)
2. GitHub Actions builds binaries for multiple platforms
3. Creates GitHub release with binaries and checksums
4. Update `../homebrew-tap/Formula/gren.rb` with new version and checksums

## Dependencies

- **Bubble Tea**: TUI framework - handles all terminal input/output and state management
- **Lipgloss**: Styling library for the TUI - defines colors, borders, layouts
- **Bubbles**: Pre-built TUI components like lists and spinners

## File Structure

The actual project structure:
- `main.go` - Entry point with CLI/TUI routing
- `internal/core/` - Shared business logic (WorktreeManager)
- `internal/ui/` - TUI implementation (Bubble Tea components)
- `internal/cli/` - CLI command handlers
- `internal/git/` - Git operations and repository interface
- `internal/config/` - Configuration management and initialization
- `.github/workflows/` - CI/CD automation

## Common Tasks

### Adding New CLI Commands
1. Add command handler to `internal/cli/cli.go`
2. Add case in `ParseAndExecute()`
3. Implement business logic in `internal/core/` if shared with TUI

### Adding New TUI Views
1. Create view renderer in `internal/ui/`
2. Add state enum to `internal/ui/types.go`
3. Add state handling in update loop in `internal/ui/model.go`
4. Wire up navigation and key bindings

### Adding New TUI Features
1. Add key binding to `KeyMap` struct in `internal/ui/types.go`
2. Add key binding to `DefaultKeyMap()` function
3. Implement command function in `internal/ui/commands.go`
4. Add message type in `internal/ui/messages.go` if async operation
5. Handle message in main update loop in `internal/ui/model.go`
6. Update help text in relevant view renderers

## Testing and Development

### Current Working Directory Behavior
Both TUI and CLI respect the current working directory when:
- Detecting git repositories
- Creating worktree configurations
- Resolving relative paths in configuration

### Error Handling
The application gracefully handles:
- Non-git repositories (shows initialization prompt)
- Missing worktree directories (shows missing status with cleanup option)
- Failed operations (displays error messages with context)
- Configuration errors (falls back to sensible defaults)

## Commit Message Guidelines

- Use descriptive commit messages in English
- Focus on what was changed and why
- Do not reference external tools or assistants in commit messages
- Follow conventional commit format when appropriate