# Directory Switching with gren

When you press Enter on a worktree in gren, it outputs a `cd` command that you can either:

## Option 1: Manual execution
Copy and paste the cd command that gren outputs:
```bash
./gren
# Select a worktree with arrow keys and press Enter
# gren outputs: cd '/path/to/worktree'
# Copy and paste this command
cd '/path/to/worktree'
```

## Option 2: Use the wrapper function (Recommended)

Add this function to your shell profile (`.zshrc`, `.bashrc`, etc.):

```bash
grencd() {
    local output
    output=$(gren "$@")
    local exit_code=$?

    # If gren output starts with "cd ", execute it
    if [[ $output == cd* ]]; then
        eval "$output"
        echo "🚀 Switched to worktree: $(basename "$(pwd)")"
        echo "📍 Path: $(pwd)"
    elif [[ -n "$output" ]]; then
        echo "$output"
    fi

    return $exit_code
}

# Create alias to replace gren command
alias gren='grencd'
```

Then reload your shell:
```bash
source ~/.zshrc  # or ~/.bashrc
```

Now when you run `gren` and press Enter on a worktree, it will automatically change to that directory!

## How it works

1. The wrapper function captures gren's output
2. If the output starts with "cd ", it executes the command using `eval`
3. This changes the current shell's directory instead of just a subprocess
4. You end up in the selected worktree directory

This is the same pattern used by tools like `z`, `autojump`, and `fzf` for directory navigation.