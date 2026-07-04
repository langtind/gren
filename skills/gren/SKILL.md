---
name: gren
description: Git worktree management with gren CLI. Use when creating, managing, or configuring worktrees, generating setup scripts, or working with parallel development workflows. Automatically invoked when user mentions worktrees, gren commands, or parallel development.
---

# Gren - Git Worktree Manager

Gren is a CLI and TUI tool for managing git worktrees efficiently. It simplifies parallel development by allowing multiple branches to be checked out simultaneously in separate directories.

## When to use gren

Use gren when:
- User wants to create, list, delete, or switch between worktrees
- User asks about parallel development or working on multiple branches
- User needs to generate post-create setup scripts
- User wants to configure worktree settings
- User mentions "worktree" or "gren" explicitly

## Quick Command Reference

### Creating Worktrees
```bash
gren                                    # Start TUI (recommended)
gren create -n <name>                   # Create new worktree with new branch
gren create -n <name> --branch <branch> # Create from existing/remote branch
gren create -n <name> -x <cmd>          # Create and execute command
```

### Managing Worktrees
```bash
gren list                               # List all worktrees
gren delete <name>                      # Delete worktree
gren switch <name>                      # Navigate to worktree
gren cleanup                            # Delete all stale worktrees
```

### Configuration
```bash
gren init                               # Initialize gren in project
gren install-skill                      # Install this skill to Claude
gren config create                      # Create user config
```

## Common Tasks

### When user asks: "Create a new worktree"

1. **Interactive (recommended):**
   ```bash
   gren
   # Then press 'n' to create new worktree
   ```

2. **Command line:**
   ```bash
   gren create -n <descriptive-name>
   ```

### When user asks: "Create worktree from remote branch"

Gren now supports creating worktrees directly from remote branches:

```bash
# From TUI:
gren
# Press 'n' → Choose "Use existing branch" → Select remote branch (🌐 icon)

# From CLI:
gren create -n <name> --branch <remote-branch-name>
# Example: gren create -n feature-auth --branch feature/authentication
```

Remote branches are shown with 🌐 icon and automatically create local tracking branches.

### When working inside herdr (register the worktree in the sidebar)

If you're running inside herdr — the `HERDR_ENV` environment variable is `1` — a
worktree you create with `gren create` is invisible to the human's herdr UI until
you register it. After a successful create, open it in herdr's sidebar:

```bash
result=$(gren create -n "<name>" --format=json)
wt=$(echo "$result" | jq -r '.path')

if [ "${HERDR_ENV:-}" = "1" ] && command -v herdr >/dev/null 2>&1; then
  herdr worktree open --path "$wt" --cwd "$PWD" --no-focus
fi
```

- Use `--no-focus` so you don't yank the human's current view; use `--focus` only
  if they asked to jump into the new worktree.
- This emits herdr's `worktree.opened` event (not `worktree.created`), so the
  gren-herdr plugin's setup pane does **not** re-run — no double setup.

**Interactive-setup caveat:** `gren create` runs the post-create hooks in this
(non-interactive) invocation, so hooks that need a human at a terminal — 1Password
`op` (TouchID), `make seed` prompts — will hang or fail. For such repos, either
create with `--no-hooks` and let the human run setup, or tell them to create via
herdr's picker (`prefix+shift+g`), which runs setup in a real TTY pane. For
non-interactive setups (dependency install, env symlinks, port derivation) the
flow above is complete on its own.

### When user asks: "Generate/create post-create setup script"

**Option 1: Use gren init (AI-powered, recommended)**
```bash
gren init
# Choose "Generate setup script with Claude Code" option
# Claude will analyze the project and generate a tailored script
```

**Option 2: Manual generation**

Help user create `.gren/post-create.sh` by analyzing the project:

1. **Detect environment files:**
   ```bash
   find . -name ".env*" -maxdepth 1
   git check-ignore .env  # Check if gitignored
   ```

2. **Detect package manager:**
   - Check for lock files: `bun.lockb`, `yarn.lock`, `pnpm-lock.yaml`, `package-lock.json`
   - Check `package.json` for `packageManager` field

3. **Generate script template:**
   ```bash
   #!/bin/bash
   set -e

   WORKTREE_PATH="$1"
   BRANCH_NAME="$2"
   BASE_BRANCH="$3"
   REPO_ROOT="$4"

   echo "Setting up worktree: $BRANCH_NAME"

   # Symlink gitignored env files
   if [ -f "$REPO_ROOT/.env" ]; then
     ln -sf "$REPO_ROOT/.env" "$WORKTREE_PATH/.env"
   fi

   # Install dependencies
   cd "$WORKTREE_PATH"
   npm install  # or: bun install, yarn, pnpm install

   echo "✅ Worktree setup complete!"
   ```

See [post-create-examples.md](post-create-examples.md) for more templates.

### When user asks: "Delete a worktree"

```bash
gren delete <worktree-name>
# Or use TUI: gren → select worktree → press 'd'
```

Gren automatically handles:
- Submodule cleanup
- Pre-remove hooks
- Git worktree removal
- Preserves the branch (safe by default)

## Configuration

### Project Config (`.gren/config.toml`)
```toml
worktree_dir = "../project-worktrees"

[post-create]
hooks = [
  "npm install",
  "cp .env.example .env"
]

[commit-generation]
command = "llm"
args = ["-m", "claude-sonnet"]
```

### User Config (`~/.config/gren/config.toml`)
```toml
[defaults]
worktree_dir = "../{repo}-worktrees"

[commit-generation]
command = "llm"
args = ["-m", "claude-sonnet"]

[hooks.post-create]
hooks = []

[named-hooks]
install-deps = { command = "npm install", branches = ["feature/*"] }
```

## Advanced Features

### Named Hooks
```toml
[named-hooks.install-deps]
command = "npm install"
branches = ["feature/*", "fix/*"]  # Glob patterns
disabled = false
```

### LLM Commit Generation
```bash
gren step commit --llm  # Generate commit message with LLM
```

### Merge Workflow
```bash
gren merge main         # Merge current worktree into main
gren merge --squash     # Squash commits before merge
gren merge --remove     # Remove worktree after merge
```

### For-Each Command
```bash
gren for-each -- git pull                    # Pull in all worktrees
gren for-each -- npm run build               # Build all worktrees
gren for-each -- echo "Branch: {{ branch }}" # Template variables
```

## TUI Keyboard Shortcuts

- `n` - Create new worktree
- `d` - Delete selected worktree
- `p` - Prune missing worktrees
- `g` - Navigate to worktree folder (requires shell integration)
- `enter` - Open worktree in editor/terminal
- `i` - Initialize project
- `c` - Open configuration
- `?` - Show help
- `q` - Quit

## Best Practices

1. **Use descriptive worktree names:** `feat-auth`, `fix-bug-123`, `refactor-api`
2. **Configure post-create hooks:** Automate environment setup
3. **Use shell integration:** Enable `g` navigation in TUI
4. **Clean up stale worktrees:** Run `gren cleanup` periodically
5. **Use TUI for discovery:** Interactive mode helps explore options

## Troubleshooting

### "Branch already checked out in another worktree"
- A branch can only be checked out once across all worktrees
- Delete the existing worktree first, or create with a different branch

### "Remote branch not showing in TUI"
- Run `git fetch origin` first
- Remote branches show with 🌐 icon
- Gren automatically creates local tracking branch when selected

### "Post-create hook fails"
- Check `.gren/post-create.sh` for errors
- Verify script has execute permissions: `chmod +x .gren/post-create.sh`
- Run manually to debug: `.gren/post-create.sh <path> <branch> <base> <repo-root>`

## See Also

- [Complete command reference](reference.md)
- [Usage examples and patterns](examples.md)
- [GitHub repository](https://github.com/langtind/gren)
