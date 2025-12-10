# 🌿 gren

A beautiful terminal UI for managing Git worktrees efficiently.

Gren makes it easy to create, manage, and switch between Git worktrees with an intuitive interface. Perfect for developers who work with multiple branches simultaneously or need to quickly test different features without stashing changes.

![gren screenshot](assets/screenshot.png)

## About the Name

**gren** (pronounced "grain" /ɡreːn/) is the Norwegian word for "branch" 🌿

Just as git uses branches to organize code, gren helps you manage the physical directories (worktrees) where those branches live.

## Features

- ✨ Beautiful, intuitive TUI interface built with Bubble Tea
- 🚀 Fast worktree creation and management
- 🔧 Configurable post-create hooks and automation
- 🔗 Smart file symlinking (env files, configs, etc.)
- 🎯 Project-specific setup workflows
- 🎨 Clean, modern terminal design with responsive layout
- ⌨️ Keyboard-driven navigation with built-in help overlay
- 🔍 Search and filter branches

## Installation

### Homebrew (macOS - Recommended)

```bash
brew tap langtind/tap
brew install gren
```

Or install directly:
```bash
brew install langtind/tap/gren
```

### Download Pre-built Binaries

Download the latest release for your platform from [GitHub Releases](https://github.com/langtind/gren/releases).

### Install with Go

```bash
go install github.com/langtind/gren@latest
```

*Note: `go install` builds from source and may show "dev" as version. For proper version numbers, use Homebrew or download pre-built binaries.*

### Build from Source

```bash
git clone https://github.com/langtind/gren.git
cd gren
go build -o gren .
```

## Shell Integration (Required for Navigation)

To enable the `g` key binding and CLI navigation commands, add this to your shell config:

**Zsh** (~/.zshrc):
```bash
eval "$(gren shell-init zsh)"
```

**Bash** (~/.bashrc):
```bash
eval "$(gren shell-init bash)"
```

**Fish** (~/.config/fish/config.fish):
```fish
gren shell-init fish | source
```

This enables:
- `g` key in TUI to navigate directly to a worktree folder
- `gcd <name>` CLI alias for quick navigation
- `gren navigate <name>` command

## Quick Start

1. Navigate to any Git repository
2. Run `gren` to start the interactive interface
3. Use keyboard shortcuts to manage worktrees:
   - `↑↓` or `jk` Navigate between worktrees
   - `Enter` Open in... menu (IDE, terminal, Finder)
   - `g` Navigate to worktree folder (requires shell integration)
   - `n` Create new worktree
   - `d` Delete worktree
   - `p` Prune stale worktrees
   - `c` Configure gren
   - `i` Initialize gren configuration
   - `?` Show help overlay
   - `q` Quit

## Usage

```bash
gren          # Launch interactive TUI
gren --help   # Show help and keyboard shortcuts
gren --version # Show version information
```

## CLI Examples

### Initialize a project

```bash
cd my-project
gren init
```

This creates `.gren/config.json` and `.gren/post-create.sh` in your repository.

### Configure post-create hook

Edit `.gren/post-create.sh` to run setup commands when creating new worktrees:

```bash
#!/bin/bash
WORKTREE_PATH="$1"
cd "$WORKTREE_PATH"

# Install dependencies
bun install

# Symlink environment files (stays in sync with main repo)
ln -sf "../.env" .env.local
```

### Create a worktree

```bash
# Create new branch "my-feature" from current branch
gren create -n my-feature

# Create new branch "bugfix" from develop
gren create -n bugfix -b develop

# Check out existing branch "feature-123" into a worktree
gren create -n feature-123 -existing
```

The post-create hook runs automatically after worktree creation.

## Development

This project uses:
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) for TUI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) for styling
- [Bubbles](https://github.com/charmbracelet/bubbles) for UI components

### Project Structure

```
gren/
├── internal/
│   ├── cli/           # CLI commands
│   ├── config/        # Configuration management
│   ├── core/          # Worktree operations
│   ├── git/           # Git operations
│   ├── logging/       # Logging utilities
│   └── ui/            # TUI components and views
└── main.go            # Entry point
```