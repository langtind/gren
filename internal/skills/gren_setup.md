---
name: gren-setup
description: Generate a post-create hook script for gren worktree setup
disable-model-invocation: true
allowed-tools: Read, Glob, Grep, Bash
---

# Generate gren post-create hook script

You are generating a bash post-create hook script for **gren**, a git worktree manager.
This script runs automatically after a new worktree is created. Its job is to set up
the new worktree so it mirrors the development environment of the main worktree.

## Script interface

The script receives these positional arguments:
- `$1` = WORKTREE_PATH (absolute path to the new worktree)
- `$2` = BRANCH_NAME (name of the branch)
- `$3` = BASE_BRANCH (the branch it was created from)
- `$4` = REPO_ROOT (absolute path to the main repository root)

## Your task

Explore this project thoroughly using Read, Glob, Grep, and Bash, then generate a
tailored post-create hook script. Do NOT guess — actually read files and check what exists.

### Step 1: Discover project structure

Use the tools to investigate:

1. **Environment files**: Run `Glob` for `.env*` patterns. For each found file, run
   `Bash: git check-ignore <file>` to determine if it's gitignored. Only gitignored
   files should be symlinked.

2. **Project type and dependencies**: Read `package.json`, `go.mod`, `Cargo.toml`,
   `requirements.txt`, `pyproject.toml`, `Gemfile`, `composer.json`, or similar files
   to understand the project type and determine the correct install command.

3. **Package manager detection** (for Node.js projects): Check for lock files in this
   priority order:
   - `bun.lockb` or `bun.lock` → bun
   - `yarn.lock` → yarn
   - `pnpm-lock.yaml` → pnpm
   - `"packageManager"` field in `package.json` → use specified manager
   - `package-lock.json` or `package.json` exists → npm (fallback)

4. **Tool version files**: Check for `.nvmrc`, `.node-version`, `.tool-versions`,
   `.python-version`, `.ruby-version`, `.java-version`, `.go-version`.

5. **Direnv**: Check if `.envrc` exists and whether it's gitignored.

6. **Gitignored config directories**: Check if these exist AND are gitignored:
   - `.claude/` (Claude Code config)
   - `.cursor/` (Cursor IDE config)
   - `.idea/` (JetBrains config)
   - `.vscode/` (VS Code config — but note: `.vscode/` is often tracked, so check!)

7. **Read `.gitignore`**: Read the `.gitignore` file to understand the full picture
   of what's excluded from version control.

8. **Submodules**: Check if `.gitmodules` exists — if so, the script should init submodules.

9. **Build artifacts / caches**: Look for patterns that suggest build output directories
   that should NOT be symlinked (e.g., `node_modules/`, `dist/`, `build/`, `target/`).

### Step 2: Generate the script

Based on your findings, generate a bash script with these sections:

#### Header
```bash
#!/usr/bin/env bash
set -euo pipefail

WORKTREE_PATH="$1"
BRANCH_NAME="$2"
BASE_BRANCH="$3"
REPO_ROOT="$4"

echo "Setting up worktree: $BRANCH_NAME"
```

#### Symlink gitignored files
For each gitignored file/directory you found, create a symlink from REPO_ROOT to WORKTREE_PATH.

Rules:
- Use `ln -sf` for files
- For directories, use `ln -sfn` (the `-n` prevents creating a symlink inside an existing directory)
- Always check existence with `[ -f ... ]` or `[ -d ... ]` before symlinking
- NEVER symlink files that are tracked by git (not gitignored)
- NEVER symlink `node_modules/`, `dist/`, `build/`, `target/`, `.git/` or build artifacts

Example:
```bash
# Symlink environment files
[ -f "$REPO_ROOT/.env" ] && ln -sf "$REPO_ROOT/.env" "$WORKTREE_PATH/.env"
[ -f "$REPO_ROOT/.env.local" ] && ln -sf "$REPO_ROOT/.env.local" "$WORKTREE_PATH/.env.local"

# Symlink config directories
[ -d "$REPO_ROOT/.claude" ] && ln -sfn "$REPO_ROOT/.claude" "$WORKTREE_PATH/.claude"
```

#### Install dependencies
Based on the detected package manager / project type, install dependencies.
Always check if the tool exists before running:

```bash
# Example for Node.js with bun
if command -v bun &>/dev/null; then
    echo "Installing dependencies with bun..."
    cd "$WORKTREE_PATH" && bun install
fi
```

#### Initialize submodules (if applicable)
```bash
cd "$WORKTREE_PATH" && git submodule update --init --recursive
```

#### Direnv (if applicable)
```bash
if [ -f "$WORKTREE_PATH/.envrc" ] && command -v direnv &>/dev/null; then
    cd "$WORKTREE_PATH" && direnv allow
fi
```

#### Footer
```bash
echo "Worktree setup complete!"
```

## Quality requirements

- **Idempotent**: Safe to run multiple times without side effects
- **No hardcoded paths**: Only use the $1-$4 arguments and relative references
- **Defensive**: Check existence before every operation
- **Progress output**: Print what's being done with `echo` statements
- **No unnecessary operations**: Only include sections relevant to what you actually found
- **Correct symlink flags**: Use `-sfn` for directories, `-sf` for files

## Output format

Output ONLY the bash script. No explanations, no markdown code fences, no commentary.
Start with `#!/usr/bin/env bash` and end with the final echo statement.

## Example output

Here is an example of what a complete script might look like for a typical Node.js project:

```
#!/usr/bin/env bash
set -euo pipefail

WORKTREE_PATH="$1"
BRANCH_NAME="$2"
BASE_BRANCH="$3"
REPO_ROOT="$4"

echo "Setting up worktree: $BRANCH_NAME"

# Symlink environment files (gitignored)
echo "Linking environment files..."
[ -f "$REPO_ROOT/.env" ] && ln -sf "$REPO_ROOT/.env" "$WORKTREE_PATH/.env"
[ -f "$REPO_ROOT/.env.local" ] && ln -sf "$REPO_ROOT/.env.local" "$WORKTREE_PATH/.env.local"

# Symlink Claude Code config (gitignored)
[ -d "$REPO_ROOT/.claude" ] && ln -sfn "$REPO_ROOT/.claude" "$WORKTREE_PATH/.claude"

# Install dependencies
if command -v bun &>/dev/null; then
    echo "Installing dependencies with bun..."
    cd "$WORKTREE_PATH" && bun install
else
    echo "Warning: bun not found, skipping dependency installation"
fi

# Direnv
if [ -f "$WORKTREE_PATH/.envrc" ] && command -v direnv &>/dev/null; then
    echo "Allowing direnv..."
    cd "$WORKTREE_PATH" && direnv allow
fi

echo "Worktree setup complete!"
```
