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

Gren uses a two-level TOML configuration system:

### User Configuration (Global)
Located at:
- **macOS**: `~/Library/Application Support/gren/config.toml`
- **Linux**: `~/.config/gren/config.toml` (or `$XDG_CONFIG_HOME/gren/config.toml`)
- **Windows**: `%APPDATA%\gren\config.toml`

Managed by `internal/config/user_config.go`:
- `UserConfigManager` - Load/save global config
- `MergeConfigs()` - Merge user and project configs (project takes precedence)

### Project Configuration
Created via `gren init` in `.gren/config.toml`:
- Worktree directory location
- Post-create hooks and commands
- LLM commit generation settings
- Named hooks with branch filtering

### Post-Create Hook Features
The generated post-create script includes:
- Symlinking environment files (`.env*` patterns) to main worktree
- Symlinking Claude configuration (if gitignored)
- Smart package manager detection and dependency installation
- Direnv integration if `.envrc` exists

## Hook System

### Hook Types
All hook types defined in `internal/config/hooks.go`:
- `post-create` - After worktree creation
- `pre-remove` - Before worktree deletion
- `pre-merge` - Before merging
- `post-merge` - After successful merge
- `post-switch` - After switching worktrees
- `post-start` - After `-x` command execution

### Named Hooks
Defined in `internal/config/user_config.go`:
```go
type NamedHook struct {
    Name     string   `toml:"name"`
    Command  string   `toml:"command"`
    Branches []string `toml:"branches,omitempty"`  // Glob patterns
    Disabled bool     `toml:"disabled,omitempty"`
}
```

### Hook JSON Context
Hooks receive context via `GREN_CONTEXT` env var and stdin. Structure in `internal/core/hooks.go`:
```go
type HookJSONContext struct {
    HookType      string `json:"hook_type"`
    Branch        string `json:"branch"`
    Worktree      string `json:"worktree"`
    WorktreeName  string `json:"worktree_name"`
    Repo          string `json:"repo"`
    RepoRoot      string `json:"repo_root"`
    Commit        string `json:"commit"`
    ShortCommit   string `json:"short_commit"`
    DefaultBranch string `json:"default_branch"`
    TargetBranch  string `json:"target_branch,omitempty"`
    BaseBranch    string `json:"base_branch,omitempty"`
    ExecuteCmd    string `json:"execute_cmd,omitempty"`
}
```

### Hook Approval System
Security feature in `internal/config/approval.go`:
- `ApprovalManager` - Manages approved hook commands per project
- Approvals stored in `~/.local/share/gren/approvals/<project-id>.json`
- Commands must be approved before execution
- `GetProjectID()` creates unique project identifier from git remote

## CI Provider Abstraction

### Provider Interface
Defined in `internal/git/provider.go`:
```go
type CIProvider interface {
    Name() string
    GetPRInfo(branch string) (*PRInfo, error)
    GetCIStatus(branch string) (*CIInfo, error)
}
```

### Implementations
- `GitHubProvider` - Uses `gh` CLI for GitHub repos
- `GitLabProvider` - Uses `glab` CLI for GitLab repos
- `DetectProvider(remoteURL)` auto-detects from remote URL

### Status Normalization
GitLab states are normalized to GitHub-style states for consistent UI display.

## LLM Commit Generation

### Core Components
Located in `internal/core/llm.go`:
- `LLMGenerator` - Generates commit messages via external LLM tools
- `FilterLockFiles()` - Removes lock file diffs (package-lock.json, etc.)
- `TruncateDiff()` - Truncates large diffs to fit context limits
- `CleanCommitMessage()` - Strips markdown/quotes from LLM output
- `ExpandPromptTemplate()` - Processes `{{ diff }}`, `{{ branch }}`, `{{ repo }}` placeholders
- `ValidateConventionalCommit()` - Validates conventional commit format

### Configuration
```toml
[commit-generation]
command = "llm"
args = ["-m", "claude-sonnet"]
template = "Write commit message for:\n\n{{ diff }}"
```

## Shell Completions

### Completion Scripts
Located in `internal/cli/completion.go`:
- `bashCompletionScript` - Bash completion with COMPREPLY/compgen
- `zshCompletionScript` - Zsh completion with #compdef and _arguments
- `fishCompletionScript` - Fish completion with complete -c

Features:
- All commands and subcommands
- Command-specific flags
- Dynamic worktree name completion
- Dynamic branch name completion

### Colored CLI Help
Located in `internal/cli/help.go`:
- ANSI color output for command help
- Terminal detection for color support
- Helper functions: `colorize()`, `bold()`, `dim()`, `cyan()`, etc.

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

### 1. Create and push tag
```bash
# View commits since last tag
git log --oneline $(git describe --tags --abbrev=0)..HEAD

# Create annotated tag with changelog
git tag -a v0.X.0 -m "v0.X.0

Features:
- Feature description

Fixes:
- Fix description
"

# Push branch and tag
git push origin main && git push origin v0.X.0
```

### 2. Update GitHub release notes
After GitHub Actions completes, update with detailed changelog:
```bash
gh release edit v0.X.0 --notes "$(cat <<'EOF'
## What's New

### 🆕 Feature Name
- Description

## Improvements
- Improvement description

## Bug Fixes
- Fix description

## Installation

### Homebrew (macOS)
\`\`\`bash
brew tap langtind/tap
brew install gren
# or upgrade
brew upgrade gren
\`\`\`

### Go
\`\`\`bash
go install github.com/langtind/gren@v0.X.0
\`\`\`

**Full Changelog**: https://github.com/langtind/gren/compare/v0.PREV.0...v0.X.0
EOF
)"
```

### 3. Update Homebrew tap
```bash
# Get checksums
gh release download v0.X.0 --pattern checksums.txt --output -

# Update ../homebrew-tap/Formula/gren.rb with:
# - New version number
# - SHA256 for darwin-arm64.tar.gz
# - SHA256 for darwin-amd64.tar.gz

# Commit and push
cd ../homebrew-tap
git add Formula/gren.rb
git commit -m "Update gren to v0.X.0"
git push
```

## Debugging

### Log File Location
- **macOS**: `~/Library/Logs/gren/gren.log`
- **Linux**: `~/.local/state/gren/logs/gren.log`

View recent logs:
```bash
tail -100 ~/Library/Logs/gren/gren.log  # macOS
```

## Dependencies

- **Bubble Tea**: TUI framework - handles all terminal input/output and state management
- **Lipgloss**: Styling library for the TUI - defines colors, borders, layouts
- **Bubbles**: Pre-built TUI components like lists and spinners

## File Structure

The actual project structure:
- `main.go` - Entry point with CLI/TUI routing
- `internal/core/` - Shared business logic:
  - `worktree.go` - WorktreeManager for CRUD operations
  - `hooks.go` - Hook execution with JSON context
  - `llm.go` - LLM-based commit message generation
- `internal/ui/` - TUI implementation (Bubble Tea components)
- `internal/cli/` - CLI command handlers:
  - `cli.go` - Main command routing
  - `help.go` - Colored help output
  - `completion.go` - Shell completion scripts
- `internal/git/` - Git operations:
  - `repository.go` - Repository interface
  - `branches.go` - Branch operations
  - `provider.go` - CI provider abstraction (GitHub/GitLab)
- `internal/config/` - Configuration management:
  - `config.go` - Project config
  - `user_config.go` - Global user config
  - `hooks.go` - Hook type definitions
  - `approval.go` - Hook approval security system
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