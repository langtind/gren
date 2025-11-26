# Navigation Feature for gren

This document describes the navigation functionality that allows you to change your shell's current directory to a worktree location directly from gren.

## Overview

The navigation feature solves the limitation where TUI applications can't change the parent shell's directory. It uses a temporary file mechanism combined with a shell wrapper to enable true directory navigation.

## How It Works

1. **Command Execution**: When you use navigation (CLI command or TUI key press), gren writes a `cd` command to `/tmp/gren_navigate`
2. **Wrapper Script**: A shell function/script reads this temp file and executes the command in the current shell
3. **Directory Change**: Your shell's working directory changes to the worktree location

## CLI Usage

### Basic Commands

```bash
# Navigate to a specific worktree
gren navigate worktree-name
gren nav worktree-name      # Short alias
gren cd worktree-name       # Even shorter alias
```

### Examples

```bash
# Navigate to feature branch worktree
gren navigate feature-login

# Navigate using short alias
gren nav feature-login

# Navigate using cd alias
gren cd feature-login
```

## TUI Usage

1. Run `gren` (or `gren-nav` if using wrapper)
2. Use arrow keys to select a worktree
3. Press `g` to navigate to the selected worktree
4. The TUI will quit and you'll be in the worktree directory

## Setup

### Option 1: Manual Setup (Per Session)

```bash
# Source the wrapper script
source ./gren-nav.sh

# Now you can use:
gren-nav                    # Start TUI with navigation
gren-nav navigate <name>    # CLI navigation
gn                          # Short alias for gren-nav
gcd <name>                  # Navigate to worktree
gnav <name>                 # Navigate to worktree
```

### Option 2: Automatic Setup (Permanent)

```bash
# Install wrapper to your shell profile
./install-nav.sh

# Restart terminal or source your shell config
source ~/.zshrc   # or ~/.bashrc, ~/.bash_profile

# Now navigation works in all new terminals
gren-nav navigate feature-branch
```

## Key Bindings

### TUI Key Bindings

- `g` - Navigate to selected worktree (Go to)
- `enter` - Open "Open in..." menu (includes applications)
- `↑/↓` or `k/j` - Navigate between worktrees
- `n` - Create new worktree
- `d` - Delete worktree
- `p` - Prune missing worktrees
- `c` - Configure gren
- `q` - Quit

## Installation Options

### Development/Local Use

```bash
# Build gren
go build -o gren .

# Use the local wrapper
source ./gren-nav.sh
gren-nav
```

### System-wide Installation

```bash
# Build and install navigation wrapper
go build -o gren .
./install-nav.sh

# Or specify custom gren binary location
./install-nav.sh /usr/local/bin/gren
```

## Troubleshooting

### Navigation Not Working

1. **Check temp file**: After navigation command, check if `/tmp/gren_navigate` exists and has content
2. **Wrapper not loaded**: Make sure you've sourced the wrapper script or installed it properly
3. **Wrong shell**: The wrapper works with bash/zsh. Fish shell needs manual adaptation

### TUI Navigation Not Working

1. **Key binding**: Make sure you're pressing `g` (not `enter`)
2. **Worktree selected**: Ensure a worktree is selected (highlighted)
3. **Repository initialized**: Make sure you've run `gren init` first

### CLI Navigation Not Working

1. **Worktree exists**: Check `gren list` to see available worktrees
2. **Correct name**: Worktree names are case-sensitive
3. **Path permissions**: Ensure you have access to the worktree directory

## Advanced Usage

### Custom Wrapper Function

You can create your own wrapper function:

```bash
my-gren-nav() {
    local temp_file="/tmp/gren_navigate"
    rm -f "$temp_file"

    gren "$@"

    if [ -f "$temp_file" ]; then
        local command=$(cat "$temp_file")
        rm -f "$temp_file"
        eval "$command"
        echo "📂 Navigated to: $(pwd)"
    fi
}
```

### Integration with Other Tools

```bash
# Navigate and immediately run a command
nav-and-run() {
    gren navigate "$1"
    if [ -f "/tmp/gren_navigate" ]; then
        eval "$(cat /tmp/gren_navigate)"
        rm -f "/tmp/gren_navigate"
        shift
        "$@"  # Run remaining arguments as command
    fi
}

# Usage: nav-and-run feature-branch npm start
```

## Comparison with "Open in..." Feature

| Feature | Navigation (`g`) | Open in... (`enter`) |
|---------|------------------|----------------------|
| **Purpose** | Change current shell directory | Open in external application |
| **Result** | Stay in same terminal | Opens new window/application |
| **Use case** | Continue working in CLI | Switch to GUI tool |
| **Shell session** | Same session | New session/window |

## Security Note

The navigation feature writes commands to `/tmp/gren_navigate`. This file is created with 0644 permissions and is cleaned up after use. The temp file only contains `cd` commands with quoted paths to prevent injection attacks.

## Technical Details

### Temp File Format
```bash
cd "/path/to/worktree"
```

### Shell Compatibility
- ✅ bash
- ✅ zsh
- ❓ fish (manual setup required)
- ❓ other shells (untested)

### Error Handling
- Invalid worktree names show clear error messages
- Missing worktrees are detected before writing navigation commands
- Temp file cleanup prevents stale navigation commands