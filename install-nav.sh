#!/bin/bash

# install-nav.sh - Install gren navigation wrapper
# This script installs the gren navigation wrapper to make it available system-wide

set -e

GREN_BINARY="${1:-./gren}"
SHELL_TYPE="${SHELL##*/}"

# Detect shell configuration file
case "$SHELL_TYPE" in
    "zsh")
        SHELL_CONFIG="$HOME/.zshrc"
        ;;
    "bash")
        if [[ "$OSTYPE" == "darwin"* ]]; then
            SHELL_CONFIG="$HOME/.bash_profile"
        else
            SHELL_CONFIG="$HOME/.bashrc"
        fi
        ;;
    "fish")
        SHELL_CONFIG="$HOME/.config/fish/config.fish"
        echo "Fish shell detected - please manually add the function to $SHELL_CONFIG"
        exit 1
        ;;
    *)
        SHELL_CONFIG="$HOME/.profile"
        ;;
esac

echo "🌿 Installing gren navigation wrapper..."
echo "Shell: $SHELL_TYPE"
echo "Config file: $SHELL_CONFIG"
echo "Gren binary: $GREN_BINARY"

# Verify gren binary exists
if [ ! -f "$GREN_BINARY" ]; then
    echo "❌ Error: gren binary not found at $GREN_BINARY"
    echo "Please build gren first: go build -o gren ."
    exit 1
fi

# Get absolute path to gren binary
GREN_PATH=$(realpath "$GREN_BINARY")

# Create the wrapper function
WRAPPER_FUNCTION="
# gren navigation wrapper - added by install-nav.sh
gren-nav() {
    local TEMP_FILE=\"/tmp/gren_navigate\"

    # Remove any existing temp file
    rm -f \"\$TEMP_FILE\"

    if [ \$# -eq 0 ]; then
        # No arguments - run TUI
        \"$GREN_PATH\"
    else
        # Pass arguments to gren CLI
        \"$GREN_PATH\" \"\$@\"
    fi

    # Check if there's a navigation command to execute
    if [ -f \"\$TEMP_FILE\" ]; then
        local COMMAND=\$(cat \"\$TEMP_FILE\")
        rm -f \"\$TEMP_FILE\"

        # Execute the navigation command
        eval \"\$COMMAND\"

        # Print current directory for confirmation
        echo \"📂 Now in: \$(pwd)\"
    fi
}

# Aliases for convenience
alias gn='gren-nav'
alias gcd='gren-nav navigate'
alias gnav='gren-nav navigate'
"

# Check if wrapper is already installed
if grep -q "gren navigation wrapper" "$SHELL_CONFIG" 2>/dev/null; then
    echo "🔄 Updating existing gren navigation wrapper..."
    # Remove existing wrapper
    sed -i.bak '/# gren navigation wrapper/,/alias gnav=/d' "$SHELL_CONFIG"
else
    echo "➕ Adding gren navigation wrapper..."
fi

# Add the wrapper function
echo "$WRAPPER_FUNCTION" >> "$SHELL_CONFIG"

echo "✅ gren navigation wrapper installed!"
echo ""
echo "To start using it:"
echo "  source $SHELL_CONFIG"
echo "  # OR restart your terminal"
echo ""
echo "Usage:"
echo "  gren-nav                    # Start TUI (press 'g' to navigate)"
echo "  gren-nav navigate <name>    # Navigate to specific worktree"
echo "  gn                          # Short alias"
echo "  gcd <name>                  # Navigate to worktree"
echo "  gnav <name>                 # Navigate to worktree"