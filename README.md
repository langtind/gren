# 🌿 gren

A beautiful terminal UI for managing Git worktrees efficiently.

Gren makes it easy to create, manage, and switch between Git worktrees with an intuitive interface. Perfect for developers who work with multiple branches simultaneously or need to quickly test different features without stashing changes.

## Features

- ✨ Beautiful, intuitive TUI interface built with Bubble Tea
- 🚀 Fast worktree creation and management
- 🔧 Configurable post-create hooks and automation
- 📁 Smart file copying (env files, configs, etc.)
- 🎯 Project-specific setup workflows
- 🎨 Clean, modern terminal design
- ⌨️ Keyboard-driven navigation
- 🔍 Search and filter worktrees

## Installation

### Install with Go (Recommended)

```bash
go install github.com/langtind/gren@latest
```

### Download Pre-built Binaries

Download the latest release for your platform from [GitHub Releases](https://github.com/langtind/gren/releases).

### Build from Source

```bash
git clone https://github.com/langtind/gren.git
cd gren
go build -o gren .
```

## Quick Start

1. Navigate to any Git repository
2. Run `gren` to start the interactive interface
3. Use keyboard shortcuts to manage worktrees:
   - `↑↓` Navigate between worktrees
   - `Enter` Switch to selected worktree
   - `n` Create new worktree
   - `d` Delete worktrees
   - `i` Initialize gren configuration
   - `q` Quit

## Usage

```bash
gren          # Launch interactive TUI
gren --help   # Show help and keyboard shortcuts
gren --version # Show version information
```

## Development

This project uses:
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) for TUI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) for styling
- [Bubbles](https://github.com/charmbracelet/bubbles) for UI components

### Project Structure

```
gren/
├── cmd/gren/           # CLI commands
├── internal/
│   ├── ui/            # TUI components and views
│   ├── git/           # Git operations
│   ├── config/        # Configuration management
│   └── hooks/         # Hook system
├── old/               # Legacy bash scripts (reference)
└── main.go           # Entry point
```