# 🌿 gren

A beautiful terminal UI for managing Git worktrees.

## Features

- ✨ Beautiful, intuitive TUI interface
- 🚀 Fast worktree creation and management
- 🔧 Configurable post-create hooks
- 📁 Smart file copying (env files, configs)
- 🎯 Project-specific setup workflows
- 🎨 Clean, modern design

## Installation

```bash
go build -o gren .
./gren
```

## Usage

```bash
gren init    # Initialize project for worktree management
gren         # Launch TUI interface
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