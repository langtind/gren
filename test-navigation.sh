#!/bin/bash

# test-navigation.sh - Test the gren navigation functionality

echo "🧪 Testing gren navigation functionality..."
echo

# Test CLI navigation
echo "1. Testing CLI navigation..."
echo "   Current directory: $(pwd)"

# Navigate using gren CLI
./gren navigate test-navigation

# Check if temp file was created
if [ -f "/tmp/gren_navigate" ]; then
    echo "   ✅ Navigation temp file created successfully"
    COMMAND=$(cat /tmp/gren_navigate)
    echo "   📄 Command: $COMMAND"

    # Execute the command to test
    echo "   🚀 Executing navigation command..."
    eval "$COMMAND"
    echo "   📂 After navigation: $(pwd)"

    # Go back to original directory
    cd -
    echo "   ↩️  Back to: $(pwd)"

    # Clean up temp file
    rm -f /tmp/gren_navigate
else
    echo "   ❌ Navigation temp file was not created"
fi

echo

# Test with wrapper function (simulated)
echo "2. Testing wrapper function behavior..."
echo "   This simulates what the gren-nav wrapper would do:"

# Define the wrapper function for testing
gren-nav-test() {
    local TEMP_FILE="/tmp/gren_navigate"

    # Remove any existing temp file
    rm -f "$TEMP_FILE"

    # Run gren with arguments
    ./gren "$@"

    # Check if there's a navigation command to execute
    if [ -f "$TEMP_FILE" ]; then
        local COMMAND=$(cat "$TEMP_FILE")
        rm -f "$TEMP_FILE"

        echo "   🔄 Wrapper would execute: $COMMAND"
        # For testing, we'll show what would happen instead of actually doing it
        echo "   📂 Would navigate to: $(echo "$COMMAND" | cut -d'"' -f2)"
        return 0
    else
        echo "   ℹ️  No navigation command found"
        return 1
    fi
}

# Test the wrapper
echo "   Testing: gren-nav-test navigate test-navigation"
if gren-nav-test navigate test-navigation; then
    echo "   ✅ Wrapper test successful"
else
    echo "   ❌ Wrapper test failed"
fi

echo
echo "3. Testing error cases..."

# Test with non-existent worktree
echo "   Testing navigation to non-existent worktree..."
if ./gren navigate non-existent-worktree 2>/dev/null; then
    echo "   ❌ Should have failed for non-existent worktree"
else
    echo "   ✅ Correctly failed for non-existent worktree"
fi

echo
echo "🎉 Navigation testing complete!"
echo
echo "To use navigation in your shell:"
echo "1. Source the wrapper: source ./gren-nav.sh"
echo "2. Use: gren-nav navigate test-navigation"
echo "   Or:   gcd test-navigation"
echo "   Or:   gnav test-navigation"
echo
echo "For TUI navigation:"
echo "1. Run: ./gren"
echo "2. Navigate to a worktree with arrow keys"
echo "3. Press 'g' to navigate to that worktree"