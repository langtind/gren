# Gren Usage Examples

Real-world examples and patterns for using gren effectively.

## Workflow Examples

### Example 1: Feature Development

**Scenario:** Start working on a new authentication feature

```bash
# Start TUI
gren

# In TUI:
# - Press 'n' to create new worktree
# - Choose "Create new branch"
# - Enter branch name: feature/authentication
# - Select base branch: main
# - Confirm
# - Post-create hook runs automatically (installs deps, symlinks .env)
# - Press 'g' to navigate to worktree

# Or via CLI:
gren create -n feat-auth --base main -x claude
# Creates worktree and starts Claude Code in it
```

### Example 2: Hotfix on Production

**Scenario:** Critical bug in production, need to fix immediately

```bash
# Create hotfix worktree from production branch
gren create -n hotfix-critical --branch production --existing

# Fix the bug...

# Merge back to production with verification
gren merge production --verify

# Clean up
gren delete hotfix-critical
```

### Example 3: Code Review

**Scenario:** Review a colleague's PR without disrupting your current work

```bash
# Create worktree from their remote branch
gren create -n review-pr-123 --branch feature/new-api

# Review in TUI:
gren
# Select the review worktree
# Press 'enter' to open in VSCode

# When done reviewing:
gren delete review-pr-123
```

### Example 4: Multiple Parallel Features

**Scenario:** Working on 3 features simultaneously

```bash
# Create worktrees for each feature
gren create -n feat-auth
gren create -n feat-payments
gren create -n feat-notifications

# Switch between them quickly
gren                    # TUI: select and press 'g'
# Or:
gcd feat-auth          # With shell integration
gcd feat-payments
gcd feat-notifications

# When features are done, clean up
gren cleanup            # Removes merged/stale worktrees
```

## Post-Create Script Examples

### Example 1: Frontend Project (Next.js)

`.gren/post-create.sh`:
```bash
#!/bin/bash
set -e

WORKTREE_PATH="$1"
BRANCH_NAME="$2"
REPO_ROOT="$4"

echo "🔧 Setting up Next.js worktree: $BRANCH_NAME"

# Symlink gitignored env files
for env_file in .env.local .env.development.local; do
  if [ -f "$REPO_ROOT/$env_file" ]; then
    ln -sf "$REPO_ROOT/$env_file" "$WORKTREE_PATH/$env_file"
    echo "   Linked $env_file"
  fi
done

# Install dependencies with correct package manager
cd "$WORKTREE_PATH"
if [ -f "bun.lockb" ]; then
  echo "   Installing dependencies with bun..."
  bun install
elif [ -f "pnpm-lock.yaml" ]; then
  echo "   Installing dependencies with pnpm..."
  pnpm install
else
  echo "   Installing dependencies with npm..."
  npm install
fi

echo "✅ Worktree ready!"
```

### Example 2: Backend Project (Python/FastAPI)

`.gren/post-create.sh`:
```bash
#!/bin/bash
set -e

WORKTREE_PATH="$1"
BRANCH_NAME="$2"
REPO_ROOT="$4"

echo "🔧 Setting up Python worktree: $BRANCH_NAME"

# Symlink .env
if [ -f "$REPO_ROOT/.env" ]; then
  ln -sf "$REPO_ROOT/.env" "$WORKTREE_PATH/.env"
fi

cd "$WORKTREE_PATH"

# Create/activate virtual environment
if [ ! -d ".venv" ]; then
  echo "   Creating virtual environment..."
  python3 -m venv .venv
fi

source .venv/bin/activate

# Install dependencies
echo "   Installing dependencies..."
pip install -r requirements.txt

echo "✅ Worktree ready! Activate venv with: source .venv/bin/activate"
```

### Example 3: Monorepo with Multiple Services

`.gren/post-create.sh`:
```bash
#!/bin/bash
set -e

WORKTREE_PATH="$1"
BRANCH_NAME="$2"
REPO_ROOT="$4"

echo "🔧 Setting up monorepo worktree: $BRANCH_NAME"

cd "$WORKTREE_PATH"

# Symlink shared .env files
ln -sf "$REPO_ROOT/.env" .env
ln -sf "$REPO_ROOT/.env.secrets" .env.secrets

# Install root dependencies
npm install

# Install service dependencies
for service in frontend backend api; do
  if [ -d "$service" ]; then
    echo "   Installing $service dependencies..."
    (cd "$service" && npm install)
  fi
done

echo "✅ Monorepo worktree ready!"
```

### Example 4: Go Project with Tools

`.gren/post-create.sh`:
```bash
#!/bin/bash
set -e

WORKTREE_PATH="$1"
BRANCH_NAME="$2"

echo "🔧 Setting up Go worktree: $BRANCH_NAME"

cd "$WORKTREE_PATH"

# Download dependencies
go mod download

# Install development tools
go install golang.org/x/tools/gopls@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Generate code if needed
if [ -f "generate.go" ]; then
  go generate ./...
fi

echo "✅ Go worktree ready!"
```

## Hook Examples

### Pre-merge Hook: Run Tests

`.gren/config.toml`:
```toml
[hooks.pre-merge]
hooks = [
  "npm run test",
  "npm run lint"
]
```

### Post-merge Hook: Deploy

`.gren/config.toml`:
```toml
[hooks.post-merge]
hooks = [
  "git push origin main",
  "./deploy.sh production"
]
```

### Named Hook: Install Only for Feature Branches

`~/.config/gren/config.toml`:
```toml
[named-hooks.install-deps]
command = "npm install"
branches = ["feature/*", "fix/*", "refactor/*"]
disabled = false
```

## For-Each Examples

### Example 1: Update All Worktrees

```bash
# Pull latest changes in all worktrees
gren for-each -- git pull

# Update dependencies everywhere
gren for-each -- npm install

# Run tests in all worktrees
gren for-each --parallel -- npm test
```

### Example 2: Branch-Specific Commands

```bash
# Tag each worktree with its branch name
gren for-each -- git tag "snapshot-{{ branch | sanitize }}"

# Create branch-specific build artifacts
gren for-each -- npm run build -- --output-dir "dist-{{ branch | sanitize }}"
```

### Example 3: Cleanup Across Worktrees

```bash
# Remove node_modules in all worktrees
gren for-each -- rm -rf node_modules

# Clean build artifacts
gren for-each -- npm run clean

# Prune old branches (skip current work)
gren for-each --skip-current -- git branch -d merged-branch
```

## Advanced Patterns

### Pattern 1: Stacked Branches

Work on dependent features in separate worktrees:

```bash
# Create feature 1
gren create -n feat-api --base main

# Create feature 2 based on feature 1
gren create -n feat-ui --base feat-api

# When feat-api is merged:
# Rebase feat-ui onto main
cd ../worktrees/feat-ui
gren step rebase main
```

### Pattern 2: Emergency Context Switch

You're working on a feature but need to fix a production bug:

```bash
# You're in feat-new-ui worktree
# Production bug reported!

# Quick switch to create hotfix
gren create -n hotfix-prod --branch production --existing -x "vim app.py"

# Fix the bug, commit, merge
gren step commit -m "fix: critical production bug"
gren merge production --verify

# Return to your feature work
gcd feat-new-ui
```

### Pattern 3: Experiment Safely

Try risky refactoring without affecting main work:

```bash
# Create experiment worktree
gren create -n experiment-refactor

# Try the refactoring...
# If it works: merge it
# If it fails: just delete the worktree

gren delete experiment-refactor  # No harm done
```

### Pattern 4: Review Multiple PRs

```bash
# Create worktrees for each PR to review
for pr in 123 124 125; do
  gren create -n review-pr-$pr --branch pr-$pr
done

# Review each in separate terminal/editor
# When done:
gren cleanup  # Removes all review worktrees
```

## Configuration Examples

### Minimal Configuration

`.gren/config.toml`:
```toml
worktree_dir = "../my-project-worktrees"

[post-create]
hooks = ["npm install"]
```

### Full Configuration with LLM

`~/.config/gren/config.toml`:
```toml
[defaults]
worktree_dir = "../{repo}-worktrees"

[commit-generation]
command = "llm"
args = ["-m", "claude-sonnet-4-5"]
template = """
Generate a conventional commit message for these changes.
Be concise and specific.

{{ diff }}
"""

[hooks.post-create]
hooks = [
  "echo 'New worktree created for {{ branch }}'",
]

[named-hooks.frontend-setup]
command = "cd frontend && npm install && npm run dev"
branches = ["feature/fe-*", "frontend/*"]

[named-hooks.backend-setup]
command = "cd backend && uv venv && uv pip install -r requirements.txt"
branches = ["feature/be-*", "backend/*"]

[named-hooks.full-stack]
command = "npm install && cd backend && uv pip install -r requirements.txt"
branches = ["feature/fs-*", "fullstack/*"]
```

## Tips & Tricks

### Tip 1: Use Descriptive Names

**Good:**
```bash
gren create -n feat-user-auth
gren create -n fix-payment-timeout
gren create -n refactor-api-client
```

**Avoid:**
```bash
gren create -n test     # Too generic
gren create -n wt1      # Not descriptive
gren create -n feature  # Which feature?
```

### Tip 2: Leverage Remote Branches

No need to fetch branches locally first:

```bash
# See remote branch in GitHub/GitLab
# Create worktree directly:
gren create -n review-feature --branch feature/colleague-work

# Gren automatically creates local tracking branch
```

### Tip 3: Use Shell Integration

Set up once, navigate forever:

```bash
# Instead of:
cd ../worktrees/feat-auth

# Just:
gcd feat-auth

# Or from TUI:
gren  # Press 'g' on any worktree
```

### Tip 4: Automate Common Workflows

Create shell aliases:

```bash
# In ~/.zshrc or ~/.bashrc:
alias gn='gren create -n'
alias gls='gren list'
alias gcd='gren switch'
alias gm='gren merge main --squash --remove --verify'
```

### Tip 5: Use For-Each for Bulk Operations

```bash
# Update all worktrees at once
gren for-each -- sh -c 'git pull && npm install'

# Run type check across all worktrees
gren for-each --parallel -- npm run typecheck

# Find todos across all worktrees
gren for-each -- grep -r "TODO" src/
```

## Common Patterns by Project Type

### Next.js / React

```bash
# Create feature worktree
gren create -n feat-dashboard

# Post-create hook:
# - Symlink .env.local
# - Run npm install
# - Start dev server

# Merge when done
gren merge main --squash --remove
```

### Go Projects

```bash
# Create worktree for experiment
gren create -n experiment-channels

# Work in isolation
# No dependency installation needed (Go modules)

# Merge if successful
gren merge main
```

### Python Projects

```bash
# Create feature worktree
gren create -n feat-ml-model

# Post-create hook:
# - Create venv
# - Install requirements
# - Activate venv

# Test in isolation
gren step commit
gren merge main --verify  # Runs pre-merge tests
```

### Monorepo

```bash
# Create worktree for cross-cutting change
gren create -n refactor-shared-types

# Post-create hook installs all service dependencies
# Work across multiple packages

# Use for-each to verify
gren for-each -- npm run build
gren for-each -- npm test

# Merge when all green
gren merge main --squash
```

## Troubleshooting Examples

### Problem: "Can't delete worktree, has uncommitted changes"

```bash
# Option 1: Commit changes first
cd ../worktrees/feat-old
git add -A
git commit -m "WIP: save work"
gren delete feat-old

# Option 2: Force delete (discards changes)
gren delete feat-old -f
```

### Problem: "Remote branch not showing in TUI"

```bash
# Fetch remote branches first
git fetch origin

# Then start TUI
gren
# Remote branches now visible with 🌐 icon
```

### Problem: "Hook fails during creation"

```bash
# View hook output
cat .gren/post-create.sh

# Test hook manually
.gren/post-create.sh /path/to/test-worktree test-branch main $(pwd)

# Fix the script, then create worktree again
```

### Problem: "Want to preserve uncommitted changes when deleting"

```bash
# Commit to a temporary branch first
cd ../worktrees/feat-experiment
git add -A
git commit -m "WIP: preserve work"
git push origin feat-experiment

# Now safe to delete worktree
gren delete feat-experiment

# Can recreate later from remote:
gren create -n feat-experiment --branch feat-experiment
```

## Integration Examples

### With Linear/GitHub Issues

```bash
# Create worktree from issue number
gren create -n "eng-$(linear issue)"

# Example: ENG-123
gren create -n eng-123

# Commit with issue reference
git commit -m "feat: implement feature (ENG-123)"
```

### With Tmux

```bash
# Create tmux session per worktree
gren for-each -- tmux new-session -d -s "{{ worktree_name }}" -c "{{ worktree }}"

# List all sessions
tmux ls

# Attach to specific worktree session
tmux attach -t feat-auth
```

### With Direnv

`.gren/post-create.sh`:
```bash
#!/bin/bash
set -e

WORKTREE_PATH="$1"
REPO_ROOT="$4"

# Symlink .envrc if it exists and is gitignored
if [ -f "$REPO_ROOT/.envrc" ]; then
  ln -sf "$REPO_ROOT/.envrc" "$WORKTREE_PATH/.envrc"
  echo "   Linked .envrc"

  # Allow direnv
  cd "$WORKTREE_PATH"
  direnv allow
fi

# Install dependencies...
npm install
```

### With Docker

```bash
# Build in all worktrees
gren for-each -- docker build -t "myapp:{{ branch | sanitize }}" .

# Run tests in Docker
gren for-each -- docker run --rm "myapp:{{ branch | sanitize }}" npm test
```

## Real-World Workflow

### Full Feature Development Cycle

```bash
# 1. Create feature worktree
gren create -n feat-user-profile

# 2. Develop feature
# ... code, code, code ...

# 3. Commit with LLM-generated message
gren step commit --llm

# 4. Squash commits before merging
gren step squash main --llm

# 5. Merge to main with verification (runs tests)
gren merge main --verify --remove

# Done! Worktree auto-removed, back in main
```

### Emergency Production Fix

```bash
# 1. Create hotfix from production
gren create -n hotfix-$(date +%Y%m%d) --branch production --existing

# 2. Fix the bug
vim src/critical.py

# 3. Commit and merge immediately
git add -A
git commit -m "fix: critical production bug"
gren merge production

# 4. Navigate back and clean up
gcd main
gren delete hotfix-$(date +%Y%m%d)

# 5. Also merge to main to prevent regression
git checkout main
git merge production
```

### Code Review Workflow

```bash
# 1. Reviewer creates worktree from PR branch
gren create -n review-pr-456 --branch feature/new-api

# 2. Review code in IDE
code ../worktrees/review-pr-456

# 3. Leave comments, test locally
npm test
npm run lint

# 4. When done, delete review worktree
gren delete review-pr-456
```

## See Also

- [Main skill documentation](SKILL.md)
- [Complete command reference](reference.md)
