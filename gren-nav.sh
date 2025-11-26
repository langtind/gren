#!/bin/bash

# gren-nav.sh - Navigation wrapper for gren
# This script allows gren to change the directory of the current shell session
#
# Usage:
#   source gren-nav.sh
#   gren-nav                    # Start TUI and navigate on 'g' key
#   gren-nav navigate <name>    # Navigate to specific worktree
#   gren-nav nav <name>         # Short alias
#   gren-nav cd <name>          # Even shorter alias

TEMP_FILE="/tmp/gren_navigate"

# Function to run gren and handle navigation
gren-nav() {
    # Remove any existing temp file
    rm -f "$TEMP_FILE"

    if [ $# -eq 0 ]; then
        # No arguments - run TUI
        ./gren
    else
        # Pass arguments to gren CLI
        ./gren "$@"
    fi

    # Check if there's a navigation command to execute
    if [ -f "$TEMP_FILE" ]; then
        COMMAND=$(cat "$TEMP_FILE")
        rm -f "$TEMP_FILE"

        # Execute the navigation command
        eval "$COMMAND"

        # Print current directory for confirmation
        echo "📂 Now in: $(pwd)"
    fi
}

# Export the function so it's available in the current shell
export -f gren-nav

# Also create aliases for convenience
alias gn='gren-nav'
alias gcd='gren-nav navigate'
alias gnav='gren-nav navigate'

echo "🌿 gren navigation wrapper loaded!"
echo "Usage:"
echo "  gren-nav                    # Start TUI (press 'g' to navigate)"
echo "  gren-nav navigate <name>    # Navigate to specific worktree"
echo "  gn                          # Short alias for gren-nav"
echo "  gcd <name>                  # Navigate to worktree"
echo "  gnav <name>                 # Navigate to worktree"