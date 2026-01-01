package cli

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
	colorCyan   = "\033[36m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
)

// isTerminal checks if stdout is a terminal
func isTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// colorize returns colored string if in terminal, otherwise plain
func colorize(s, color string) string {
	if !isTerminal() {
		return s
	}
	return color + s + colorReset
}

// bold returns bold text
func bold(s string) string {
	return colorize(s, colorBold)
}

// dim returns dimmed text
func dim(s string) string {
	return colorize(s, colorDim)
}

// cyan returns cyan colored text
func cyan(s string) string {
	return colorize(s, colorCyan)
}

// green returns green colored text
func green(s string) string {
	return colorize(s, colorGreen)
}

// yellow returns yellow colored text
func yellow(s string) string {
	return colorize(s, colorYellow)
}

// blue returns blue colored text
func blue(s string) string {
	return colorize(s, colorBlue)
}

// ShowColoredHelp shows the colorized help message
func (c *CLI) ShowColoredHelp() {
	fmt.Println()
	fmt.Println(bold(green("gren")) + " - Git Worktree Manager with TUI")
	fmt.Println()

	fmt.Println(bold("USAGE"))
	fmt.Println("  " + cyan("gren") + "                        " + dim("Start interactive TUI"))
	fmt.Println("  " + cyan("gren") + " " + yellow("<command>") + " " + dim("[options]") + "    " + dim("Run a command"))
	fmt.Println()

	fmt.Println(bold("COMMANDS"))
	fmt.Println()

	// Worktree Management
	fmt.Println("  " + bold("Worktree Management"))
	printCommand("create", "-n <name>", "Create a new worktree")
	printCommand("list", "[-v]", "List all worktrees")
	printCommand("delete", "<name>", "Delete a worktree")
	printCommand("cleanup", "", "Delete all stale worktrees")
	fmt.Println()

	// Navigation
	fmt.Println("  " + bold("Navigation"))
	printCommand("switch", "<name>", "Navigate to a worktree")
	printCommand("compare", "<name>", "Compare changes between worktrees")
	fmt.Println()

	// Git Operations
	fmt.Println("  " + bold("Git Operations"))
	printCommand("merge", "[target]", "Merge current worktree into target")
	printCommand("for-each", "-- <cmd>", "Run command in all worktrees")
	printCommand("step commit", "", "Stage and commit all changes")
	printCommand("step squash", "[target]", "Squash commits since target")
	fmt.Println()

	// Configuration
	fmt.Println("  " + bold("Configuration"))
	printCommand("init", "", "Initialize gren in repository")
	printCommand("shell-init", "<shell>", "Generate shell integration")
	printCommand("completion", "<shell>", "Generate shell completions")
	printCommand("help", "<topic>", "Show detailed help (e.g. hooks)")
	fmt.Println()

	// Claude Integration
	fmt.Println("  " + bold("Claude Integration"))
	printCommand("marker", "<set|get|clear|list>", "Manage activity markers")
	printCommand("setup-claude-plugin", "", "Create Claude plugin hooks")
	printCommand("statusline", "", "Output status for shell prompts")
	fmt.Println()

	fmt.Println(bold("FLAGS"))
	fmt.Println("  " + yellow("--help") + "      " + dim("Show help for gren or a command"))
	fmt.Println("  " + yellow("--version") + "   " + dim("Show version information"))
	fmt.Println()

	fmt.Println(bold("EXAMPLES"))
	fmt.Println()
	fmt.Println("  " + dim("# Start TUI"))
	fmt.Println("  $ " + cyan("gren"))
	fmt.Println()
	fmt.Println("  " + dim("# Create new worktree and start Claude"))
	fmt.Println("  $ " + cyan("gren create -n feat-auth -x claude"))
	fmt.Println()
	fmt.Println("  " + dim("# Merge feature branch to main"))
	fmt.Println("  $ " + cyan("gren merge main"))
	fmt.Println()
	fmt.Println("  " + dim("# Run npm install in all worktrees"))
	fmt.Println("  $ " + cyan("gren for-each -- npm install"))
	fmt.Println()

	fmt.Println(bold("CONFIGURATION"))
	fmt.Println()
	fmt.Println("  Project config: " + cyan(".gren/config.toml"))
	fmt.Println("  User config:    " + cyan("~/.config/gren/config.toml"))
	fmt.Println()

	fmt.Println(bold("DOCUMENTATION"))
	fmt.Println()
	fmt.Println("  " + blue("https://github.com/langtind/gren"))
	fmt.Println()
}

// printCommand formats and prints a command with description
func printCommand(cmd, args, desc string) {
	cmdPart := cyan(cmd)
	if args != "" {
		cmdPart += " " + yellow(args)
	}
	// Pad to align descriptions
	padding := 28 - len(cmd) - len(args)
	if args != "" {
		padding--
	}
	if padding < 2 {
		padding = 2
	}
	fmt.Printf("    %s%s%s\n", cmdPart, strings.Repeat(" ", padding), dim(desc))
}

// ShowCommandHelp shows help for a specific command
func ShowCommandHelp(command string) {
	switch command {
	case "create":
		showCreateHelp()
	case "merge":
		showMergeHelp()
	case "for-each":
		showForEachHelp()
	case "step":
		showStepHelp()
	case "hooks":
		showHooksHelp()
	default:
		fmt.Printf("No detailed help available for '%s'\n", command)
		fmt.Println("Use 'gren --help' for general help")
	}
}

func showCreateHelp() {
	fmt.Println()
	fmt.Println(bold("NAME"))
	fmt.Println("  " + cyan("gren create") + " - Create a new git worktree")
	fmt.Println()
	fmt.Println(bold("SYNOPSIS"))
	fmt.Println("  gren create " + yellow("-n <name>") + " [options]")
	fmt.Println()
	fmt.Println(bold("OPTIONS"))
	fmt.Println("  " + yellow("-n <name>") + "          " + dim("Worktree name (required)"))
	fmt.Println("  " + yellow("-b <branch>") + "        " + dim("Base branch to create from"))
	fmt.Println("  " + yellow("--branch <name>") + "    " + dim("Branch name (defaults to worktree name)"))
	fmt.Println("  " + yellow("--existing") + "         " + dim("Use existing branch instead of creating new"))
	fmt.Println("  " + yellow("--dir <path>") + "       " + dim("Directory for worktrees"))
	fmt.Println("  " + yellow("-x <command>") + "       " + dim("Command to run after creation"))
	fmt.Println()
	fmt.Println(bold("EXAMPLES"))
	fmt.Println("  $ gren create -n feat-auth")
	fmt.Println("  $ gren create -n hotfix -b main")
	fmt.Println("  $ gren create -n feat-ui -x claude")
	fmt.Println("  $ gren create -n existing-feature --existing --branch feature/old")
	fmt.Println()
}

func showMergeHelp() {
	fmt.Println()
	fmt.Println(bold("NAME"))
	fmt.Println("  " + cyan("gren merge") + " - Merge current worktree into target branch")
	fmt.Println()
	fmt.Println(bold("SYNOPSIS"))
	fmt.Println("  gren merge " + yellow("[target]") + " [options]")
	fmt.Println()
	fmt.Println(bold("PIPELINE"))
	fmt.Println("  1. Stage and commit uncommitted changes")
	fmt.Println("  2. Squash commits into one (unless --no-squash)")
	fmt.Println("  3. Rebase onto target (unless --no-rebase)")
	fmt.Println("  4. Run pre-merge hooks")
	fmt.Println("  5. Fast-forward merge to target")
	fmt.Println("  6. Remove worktree and branch (unless --no-remove)")
	fmt.Println("  7. Run post-merge hooks")
	fmt.Println()
	fmt.Println(bold("OPTIONS"))
	fmt.Println("  " + yellow("--no-squash") + "    " + dim("Preserve individual commits"))
	fmt.Println("  " + yellow("--no-remove") + "    " + dim("Keep worktree after merge"))
	fmt.Println("  " + yellow("--no-verify") + "    " + dim("Skip pre-merge and post-merge hooks"))
	fmt.Println("  " + yellow("--no-rebase") + "    " + dim("Skip rebase (fail if not already rebased)"))
	fmt.Println("  " + yellow("-y") + "             " + dim("Skip confirmation prompts"))
	fmt.Println("  " + yellow("-f") + "             " + dim("Force merge even with uncommitted changes"))
	fmt.Println()
	fmt.Println(bold("EXAMPLES"))
	fmt.Println("  $ gren merge                  " + dim("# Merge to default branch"))
	fmt.Println("  $ gren merge main             " + dim("# Merge to main"))
	fmt.Println("  $ gren merge --no-remove      " + dim("# Keep worktree"))
	fmt.Println("  $ gren merge --no-squash      " + dim("# Preserve history"))
	fmt.Println()
}

func showForEachHelp() {
	fmt.Println()
	fmt.Println(bold("NAME"))
	fmt.Println("  " + cyan("gren for-each") + " - Run a command in all worktrees")
	fmt.Println()
	fmt.Println(bold("SYNOPSIS"))
	fmt.Println("  gren for-each [options] " + yellow("-- <command>"))
	fmt.Println()
	fmt.Println(bold("OPTIONS"))
	fmt.Println("  " + yellow("--skip-current") + "   " + dim("Skip the current worktree"))
	fmt.Println("  " + yellow("--skip-main") + "      " + dim("Skip the main worktree"))
	fmt.Println()
	fmt.Println(bold("TEMPLATE VARIABLES"))
	fmt.Println("  " + cyan("{{ branch }}") + "           " + dim("Branch name"))
	fmt.Println("  " + cyan("{{ branch | sanitize }}") + " " + dim("Branch with / replaced by -"))
	fmt.Println("  " + cyan("{{ worktree }}") + "         " + dim("Absolute path to worktree"))
	fmt.Println("  " + cyan("{{ worktree_name }}") + "    " + dim("Worktree directory name"))
	fmt.Println("  " + cyan("{{ repo }}") + "             " + dim("Repository name"))
	fmt.Println("  " + cyan("{{ repo_root }}") + "        " + dim("Absolute path to main repo"))
	fmt.Println("  " + cyan("{{ commit }}") + "           " + dim("Full HEAD commit SHA"))
	fmt.Println("  " + cyan("{{ short_commit }}") + "     " + dim("Short HEAD commit SHA"))
	fmt.Println("  " + cyan("{{ default_branch }}") + "   " + dim("Default branch (main/master)"))
	fmt.Println()
	fmt.Println(bold("EXAMPLES"))
	fmt.Println("  $ gren for-each -- git status --short")
	fmt.Println("  $ gren for-each -- npm install")
	fmt.Println("  $ gren for-each -- \"echo Branch: {{ branch }}\"")
	fmt.Println("  $ gren for-each --skip-main -- git pull")
	fmt.Println()
}

func showStepHelp() {
	fmt.Println()
	fmt.Println(bold("NAME"))
	fmt.Println("  " + cyan("gren step") + " - Commit and squash operations")
	fmt.Println()
	fmt.Println(bold("SUBCOMMANDS"))
	fmt.Println()
	fmt.Println("  " + cyan("gren step commit") + " [options]")
	fmt.Println("    Stage and commit all changes")
	fmt.Println()
	fmt.Println("    " + yellow("-m <message>") + "   " + dim("Commit message"))
	fmt.Println("    " + yellow("--llm") + "          " + dim("Generate message using configured LLM"))
	fmt.Println()
	fmt.Println("  " + cyan("gren step squash") + " [target] [options]")
	fmt.Println("    Squash commits since target branch into one")
	fmt.Println()
	fmt.Println("    " + yellow("-m <message>") + "   " + dim("Squash commit message"))
	fmt.Println("    " + yellow("--llm") + "          " + dim("Generate message using configured LLM"))
	fmt.Println()
	fmt.Println(bold("EXAMPLES"))
	fmt.Println("  $ gren step commit")
	fmt.Println("  $ gren step commit -m \"feat: add feature\"")
	fmt.Println("  $ gren step commit --llm")
	fmt.Println("  $ gren step squash main")
	fmt.Println("  $ gren step squash --llm")
	fmt.Println()
}

func showHooksHelp() {
	fmt.Println()
	fmt.Println(bold("HOOKS"))
	fmt.Println()
	fmt.Println("  Gren supports hooks at various lifecycle points. Hooks are configured")
	fmt.Println("  in " + cyan(".gren/config.toml") + " and run automatically during operations.")
	fmt.Println()

	fmt.Println(bold("AVAILABLE HOOKS"))
	fmt.Println()
	fmt.Println("  " + cyan("post-create") + "   " + dim("After creating a new worktree"))
	fmt.Println("  " + cyan("post-switch") + "   " + dim("After switching to a worktree"))
	fmt.Println("  " + cyan("pre-merge") + "     " + dim("Before merging (blocks on failure)"))
	fmt.Println("  " + cyan("post-merge") + "    " + dim("After successful merge"))
	fmt.Println("  " + cyan("pre-remove") + "    " + dim("Before deleting a worktree"))
	fmt.Println("  " + cyan("post-start") + "    " + dim("After starting command with -x flag"))
	fmt.Println()

	fmt.Println(bold("SIMPLE HOOKS"))
	fmt.Println()
	fmt.Println("  Define a single command per hook in " + cyan("config.toml") + ":")
	fmt.Println()
	fmt.Println("    " + dim("[hooks]"))
	fmt.Println("    " + yellow("post-create") + " = " + green("\"npm install\""))
	fmt.Println("    " + yellow("post-switch") + " = " + green("\"npm run dev\""))
	fmt.Println("    " + yellow("pre-merge") + " = " + green("\"npm test\""))
	fmt.Println()

	fmt.Println(bold("SCRIPT HOOKS"))
	fmt.Println()
	fmt.Println("  For complex setup, point to a script file:")
	fmt.Println()
	fmt.Println("    " + dim("[hooks]"))
	fmt.Println("    " + yellow("post-create") + " = " + green("\".gren/post-create.sh\""))
	fmt.Println()
	fmt.Println("  " + dim("The script receives context via GREN_CONTEXT env var and stdin."))
	fmt.Println()

	fmt.Println(bold("NAMED HOOKS"))
	fmt.Println()
	fmt.Println("  Define multiple hooks with names and optional branch filtering:")
	fmt.Println()
	fmt.Println("    " + dim("[[named-hooks.post-create]]"))
	fmt.Println("    " + yellow("name") + " = " + green("\"install\""))
	fmt.Println("    " + yellow("command") + " = " + green("\"npm install\""))
	fmt.Println()
	fmt.Println("    " + dim("[[named-hooks.post-create]]"))
	fmt.Println("    " + yellow("name") + " = " + green("\"setup-env\""))
	fmt.Println("    " + yellow("command") + " = " + green("\"ln -sf ../.env .env\""))
	fmt.Println("    " + yellow("branches") + " = " + green("[\"feature/*\"]") + "  " + dim("# Only for feature branches"))
	fmt.Println()

	fmt.Println(bold("HOOK CONTEXT (JSON)"))
	fmt.Println()
	fmt.Println("  Hooks receive rich context via " + cyan("GREN_CONTEXT") + " env var:")
	fmt.Println()
	fmt.Println("    " + dim("{"))
	fmt.Println("      " + yellow("\"hook_type\"") + ":       " + green("\"post-create\""))
	fmt.Println("      " + yellow("\"branch\"") + ":          " + green("\"feature/auth\""))
	fmt.Println("      " + yellow("\"worktree\"") + ":        " + green("\"/path/to/worktree\""))
	fmt.Println("      " + yellow("\"worktree_name\"") + ":   " + green("\"auth\""))
	fmt.Println("      " + yellow("\"repo\"") + ":            " + green("\"my-project\""))
	fmt.Println("      " + yellow("\"default_branch\"") + ":  " + green("\"main\""))
	fmt.Println("    " + dim("}"))
	fmt.Println()

	fmt.Println(bold("EXAMPLE SCRIPT"))
	fmt.Println()
	fmt.Println("    " + dim("#!/bin/bash"))
	fmt.Println("    " + dim("CONTEXT=$(cat)"))
	fmt.Println("    " + dim("BRANCH=$(echo \"$CONTEXT\" | jq -r '.branch')"))
	fmt.Println("    " + dim("echo \"Setting up $BRANCH...\""))
	fmt.Println("    " + dim("npm install"))
	fmt.Println()

	fmt.Println(bold("SECURITY"))
	fmt.Println()
	fmt.Println("  Gren requires explicit approval for hook commands:")
	fmt.Println("  • New hooks prompt for approval before running")
	fmt.Println("  • Approvals stored per-project")
	fmt.Println("  • View approved: " + cyan("gren config approvals"))
	fmt.Println("  • Revoke all: " + cyan("gren config approvals --revoke"))
	fmt.Println()
}
