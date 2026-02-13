# Gren Command Reference

Complete reference for all gren commands and options.

## Worktree Management

### `gren create`

Create a new worktree with various options.

**Syntax:**
```bash
gren create -n <name> [options]
```

**Options:**
- `-n, --name <name>` - Worktree name (required)
- `-b, --branch <branch>` - Branch name (defaults to name)
- `--base <branch>` - Base branch for new branch (defaults to main/master)
- `--existing` - Use existing branch instead of creating new one
- `-d, --dir <path>` - Custom worktree directory
- `-x, --execute <cmd>` - Command to execute after creation
- `-y, --yes` - Auto-approve hooks without prompting

**Examples:**
```bash
# Create new worktree with new branch
gren create -n feat-auth

# Create from existing branch
gren create -n hotfix --branch hotfix/critical-bug --existing

# Create from remote branch
gren create -n feature --branch feature/new-api

# Create with custom base branch
gren create -n experiment --base develop

# Create and start Claude Code
gren create -n feat-ui -x claude
```

### `gren list`

List all worktrees with status information.

**Syntax:**
```bash
gren list [-v]
```

**Options:**
- `-v, --verbose` - Show detailed status

**Output includes:**
- Worktree name and path
- Branch name
- Status (clean, modified, unpushed, missing)
- Commit counts (staged, modified, untracked, unpushed)
- PR status (if GitHub CLI available)
- CI status (if GitHub CLI available)

### `gren delete`

Delete a worktree.

**Syntax:**
```bash
gren delete <name> [options]
```

**Options:**
- `-f, --force` - Force deletion (ignore uncommitted changes)
- `-y, --yes` - Auto-approve hooks without prompting

**Behavior:**
- Runs pre-remove hooks (if configured)
- Deinitializes submodules (if present)
- Removes worktree directory
- Preserves the branch (safe by default)

### `gren cleanup`

Delete all stale worktrees (merged, closed PRs, remote gone).

**Syntax:**
```bash
gren cleanup [options]
```

**Options:**
- `-y, --yes` - Auto-approve all deletions
- `--dry-run` - Show what would be deleted without deleting

**Detects stale worktrees:**
- Branches merged into main/master
- Branches with deleted remote tracking
- Closed or merged GitHub PRs
- Branches with no unique commits

### `gren switch`

Navigate to a worktree (requires shell integration).

**Syntax:**
```bash
gren switch <name>
gcd <name>  # Alias (with shell integration)
```

## Configuration

### `gren init`

Initialize gren in the current repository.

**Interactive wizard:**
1. Detects project type, package manager, env files
2. Offers AI-powered script generation (if Claude Code installed)
3. Creates `.gren/config.toml` and `.gren/post-create.sh`
4. Optionally commits configuration

**Options:**
- Non-interactive mode not available (use TUI)

### `gren config`

Manage configuration files.

**Subcommands:**
```bash
gren config show           # Display current configuration
gren config create         # Create user config interactively
gren config edit           # Open config in $EDITOR
```

### `gren install-skill`

Install the Claude Code skill for gren.

**Syntax:**
```bash
gren install-skill [options]
```

**Options:**
- `-p, --path <dir>` - Parent directory (default: `~/.claude/skills/`)
- `-f, --force` - Overwrite existing files without prompting

**Installs to:**
- Default: `~/.claude/skills/gren/`
- Custom: `<path>/gren/`

**Files installed:**
- `SKILL.md` - Main skill file
- `reference.md` - This file
- `examples.md` - Usage examples

## Git Operations

### `gren merge`

Merge current worktree into target branch.

**Syntax:**
```bash
gren merge [target] [options]
```

**Options:**
- `--squash` - Squash commits before merging
- `--rebase` - Rebase onto target before merging
- `--remove` - Remove worktree after successful merge
- `--verify` - Run hooks (pre-merge, post-merge, pre-remove)
- `-y, --yes` - Auto-approve hooks
- `--force` - Force merge (ignore uncommitted changes)

**Default target:** `main` or `master`

**Example workflow:**
```bash
# Squash commits and merge to main, then remove worktree
gren merge main --squash --remove --verify
```

### `gren step`

Step-by-step git workflow commands.

**Subcommands:**
```bash
gren step commit [options]           # Stage and commit changes
gren step squash [target] [options]  # Squash commits
gren step push [target]              # Fast-forward push to target
gren step rebase [target]            # Rebase onto target
```

**Commit options:**
- `-m, --message <msg>` - Commit message
- `--llm` - Generate commit message with LLM

**Squash options:**
- `-m, --message <msg>` - Squash commit message
- `--llm` - Generate message with LLM
- `[target]` - Target branch (default: main/master)

### `gren for-each`

Run a command in all worktrees.

**Syntax:**
```bash
gren for-each [options] -- <command> [args...]
```

**Options:**
- `--skip-current` - Skip current worktree
- `--skip-main` - Skip main worktree
- `--parallel` - Run in parallel (default: sequential)

**Template variables:**
- `{{ branch }}` - Branch name
- `{{ branch | sanitize }}` - Branch name with / → -
- `{{ worktree }}` - Absolute path to worktree
- `{{ worktree_name }}` - Worktree directory name
- `{{ repo }}` - Repository name
- `{{ repo_root }}` - Absolute path to main repo
- `{{ commit }}` - Full HEAD commit SHA
- `{{ short_commit }}` - Short commit SHA
- `{{ default_branch }}` - Default branch (main/master)

**Examples:**
```bash
# Pull in all worktrees
gren for-each -- git pull

# Build all worktrees
gren for-each -- npm run build

# Use template variables
gren for-each -- echo "Building {{ branch }}"

# Skip current and run in parallel
gren for-each --skip-current --parallel -- npm test
```

## Shell Integration

### Setup

Add to shell config for navigation support:

**Zsh (~/.zshrc):**
```bash
eval "$(gren shell-init zsh)"
```

**Bash (~/.bashrc):**
```bash
eval "$(gren shell-init bash)"
```

**Fish (~/.config/fish/config.fish):**
```bash
gren shell-init fish | source
```

**Features enabled:**
- `g` key in TUI to navigate to worktree
- `gcd <name>` alias for quick navigation
- `gren navigate <name>` command

### Completions

**Install completions:**
```bash
# Bash
gren completion bash > /etc/bash_completion.d/gren

# Zsh
gren completion zsh > /usr/local/share/zsh/site-functions/_gren

# Fish
gren completion fish > ~/.config/fish/completions/gren.fish
```

## Hooks

Gren supports lifecycle hooks at various points:

### Hook Types

| Hook | Trigger | Use case |
|------|---------|----------|
| `post-create` | After worktree creation | Install deps, symlink files |
| `pre-remove` | Before worktree deletion | Backup data, cleanup |
| `pre-merge` | Before merging | Run tests, lint |
| `post-merge` | After successful merge | Deploy, notify team |
| `post-switch` | After switching worktrees | Update IDE workspace |
| `post-start` | After `-x` command execution | Custom post-execution tasks |

### Hook Context

Hooks receive JSON context via `$GREN_CONTEXT` environment variable:

```json
{
  "hook_type": "post-create",
  "branch": "feature/auth",
  "worktree": "/path/to/worktree",
  "worktree_name": "feat-auth",
  "repo": "myproject",
  "repo_root": "/path/to/main-repo",
  "commit": "abc123...",
  "short_commit": "abc123",
  "default_branch": "main"
}
```

### Configuration

**In `.gren/config.toml` (project-specific):**
```toml
[hooks.post-create]
hooks = [
  "npm install",
  "cp $REPO_ROOT/.env.example .env"
]
```

**In `~/.config/gren/config.toml` (user-global):**
```toml
[named-hooks.install-deps]
command = "npm install"
branches = ["feature/*", "fix/*"]
disabled = false
```

### Named Hooks with Branch Filtering

```toml
[named-hooks.setup-frontend]
command = "cd frontend && npm install"
branches = ["feature/fe-*", "frontend/*"]

[named-hooks.setup-backend]
command = "cd backend && pip install -r requirements.txt"
branches = ["feature/be-*", "backend/*"]
```

## Claude Integration

### Activity Markers

Track current focus across worktrees:

```bash
gren marker set "Working on authentication"
gren marker get                                # Show current marker
gren marker clear                              # Clear marker
gren marker list                               # List all markers
```

Use in shell prompt with `gren statusline`.

### Setup Claude Plugin

Install gren hooks for Claude Code plugin:

```bash
gren setup-claude-plugin
```

Creates hooks that:
- Track activity markers automatically
- Integrate with Claude Code workflows

### Statusline for Shell Prompts

```bash
gren statusline
# Output: gren:feat-auth (3↑ 2M) 📝 Working on auth
```

**Integration example (Fish):**
```fish
function fish_right_prompt
    set_color brblack
    gren statusline 2>/dev/null
    set_color normal
end
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Invalid arguments |
| 130 | User cancelled (Ctrl+C) |

## See Also

- [Usage examples and patterns](examples.md)
- [GitHub repository](https://github.com/langtind/gren)
- [Report issues](https://github.com/langtind/gren/issues)
