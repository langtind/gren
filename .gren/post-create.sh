#!/bin/bash
# gren post-create setup script
# This script runs in the new worktree directory after creation
#
# Configuration (edit as needed):
# WORKTREE_DIR="../gren-worktrees"
# PACKAGE_MANAGER="go"
# DEFAULT_SETUP_CMD="go mod tidy"
#
# Files found in .gitignore (development-relevant):
# .vscode/
# .idea/
# .env
# .env.local
# .env.*.local
#

set -e

echo "🚀 Setting up new worktree..."

# Copy development configuration files
echo "📋 Copying gitignored development files..."

# Read main repo path from config
MAIN_REPO_PATH=$(grep -o '"main_repo_path"[^,}]*' .gren/config.json | cut -d':' -f2 | tr -d '" ')
if [ -z "$MAIN_REPO_PATH" ]; then
    echo "Warning: Could not read main_repo_path from config, using fallback"
    MAIN_REPO_PATH="../"
fi

[ -d "$MAIN_REPO_PATH/.vscode/" ] && cp -r "$MAIN_REPO_PATH/.vscode/" . 2>/dev/null || true
[ -d "$MAIN_REPO_PATH/.idea/" ] && cp -r "$MAIN_REPO_PATH/.idea/" . 2>/dev/null || true
cp "$MAIN_REPO_PATH/.env" . 2>/dev/null || true
cp "$MAIN_REPO_PATH/.env.local" . 2>/dev/null || true
cp "$MAIN_REPO_PATH/.env.*.local" . 2>/dev/null || true

# Install dependencies / setup
echo "📦 Running: go mod tidy"
go mod tidy

# Setup direnv if .envrc exists
if [[ -f ".envrc" ]] && command -v direnv >/dev/null 2>&1; then
    echo "🔧 Running direnv allow..."
    direnv allow
fi

echo "✅ Worktree setup complete!"
