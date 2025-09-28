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

### TUI State Management
The TUI has complex state transitions between:
- Dashboard (main view)
- Create worktree (multi-step wizard)
- Delete worktree (confirmation flow)
- Init workflow (project setup)
- Configuration management

Each state has its own rendering and input handling methods in `internal/ui/`.

## Configuration System

Projects can be initialized with `gren init` which creates:
- `.gren/config.json` - Project configuration
- `.gren/post-create.sh` - Executable hook script (if configured)

The config includes patterns for files to copy to new worktrees and post-create automation.

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

## File Structure Notes

The README.md shows an outdated structure mentioning `cmd/gren/` which doesn't exist. The actual structure is:
- `main.go` - Entry point
- `internal/` - All implementation packages
- `.github/workflows/` - CI/CD automation
- No separate `cmd/` directory

## Common Tasks

### Adding New CLI Commands
1. Add command handler to `internal/cli/cli.go`
2. Add case in `ParseAndExecute()`
3. Implement business logic in `internal/core/` if shared with TUI

### Adding New TUI Views
1. Create view renderer in `internal/ui/`
2. Add state enum to `internal/ui/types.go`
3. Add state handling in `internal/ui/handlers.go`
4. Wire up navigation in main update loop