package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/langtind/gren/internal/config"
	"github.com/langtind/gren/internal/core"
	"github.com/langtind/gren/internal/git"
	"github.com/langtind/gren/internal/logging"
)

// CLI handles command-line interface operations
type CLI struct {
	gitRepo         git.Repository
	configManager   *config.Manager
	worktreeManager *core.WorktreeManager
}

// NewCLI creates a new CLI instance
func NewCLI(gitRepo git.Repository, configManager *config.Manager) *CLI {
	worktreeManager := core.NewWorktreeManager(gitRepo, configManager)
	return &CLI{
		gitRepo:         gitRepo,
		configManager:   configManager,
		worktreeManager: worktreeManager,
	}
}

// ParseAndExecute parses command line arguments and executes the appropriate command
func (c *CLI) ParseAndExecute(args []string) error {
	if len(args) < 2 {
		logging.Error("CLI: no command provided")
		return fmt.Errorf("no command provided")
	}

	command := args[1]
	logging.Info("CLI command: %s, args: %v", command, args[2:])

	switch command {
	case "create":
		return c.handleCreate(args[2:])
	case "list":
		return c.handleList(args[2:])
	case "delete":
		return c.handleDelete(args[2:])
	case "init":
		return c.handleInit(args[2:])
	case "navigate", "nav", "cd":
		return c.handleNavigate(args[2:])
	case "shell-init":
		return c.handleShellInit(args[2:])
	default:
		logging.Error("CLI: unknown command: %s", command)
		return fmt.Errorf("unknown command: %s", command)
	}
}

// handleCreate handles the create command
func (c *CLI) handleCreate(args []string) error {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	name := fs.String("n", "", "Name for the new worktree (required)")
	branch := fs.String("branch", "", "Branch name (defaults to worktree name if creating new branch)")
	baseBranch := fs.String("b", "", "Base branch to create from (defaults to recommended base branch)")
	existing := fs.Bool("existing", false, "Use existing branch instead of creating new one")
	worktreeDir := fs.String("dir", "", "Directory to create worktrees in")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: gren create -n <name> [options]\n")
		fmt.Fprintf(fs.Output(), "\nCreate a new git worktree\n\n")
		fmt.Fprintf(fs.Output(), "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(fs.Output(), "\nExamples:\n")
		fmt.Fprintf(fs.Output(), "  gren create -n feature-branch\n")
		fmt.Fprintf(fs.Output(), "  gren create -n hotfix -b main\n")
		fmt.Fprintf(fs.Output(), "  gren create -n existing-feature --existing --branch feature-branch\n")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *name == "" {
		logging.Error("CLI create: worktree name is required")
		fs.Usage()
		return fmt.Errorf("worktree name is required")
	}

	// If no base branch specified for CLI, default to current branch
	effectiveBaseBranch := *baseBranch
	if effectiveBaseBranch == "" && !*existing {
		currentBranch, err := c.gitRepo.GetCurrentBranch(context.Background())
		if err != nil {
			logging.Warn("CLI create: failed to get current branch, will use recommended: %v", err)
		} else {
			effectiveBaseBranch = currentBranch
			logging.Debug("CLI create: using current branch as base: %s", effectiveBaseBranch)
		}
	}

	logging.Info("CLI create: name=%s, branch=%s, base=%s, existing=%v, dir=%s",
		*name, *branch, effectiveBaseBranch, *existing, *worktreeDir)

	req := core.CreateWorktreeRequest{
		Name:        *name,
		Branch:      *branch,
		BaseBranch:  effectiveBaseBranch,
		IsNewBranch: !*existing,
		WorktreeDir: *worktreeDir,
	}

	ctx := context.Background()
	err := c.worktreeManager.CreateWorktree(ctx, req)
	if err != nil {
		logging.Error("CLI create failed: %v", err)
	} else {
		logging.Info("CLI create succeeded: %s", *name)
	}
	return err
}

// handleList handles the list command
func (c *CLI) handleList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	verbose := fs.Bool("v", false, "Show verbose output")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: gren list [options]\n")
		fmt.Fprintf(fs.Output(), "\nList all git worktrees\n\n")
		fmt.Fprintf(fs.Output(), "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	logging.Debug("CLI list: verbose=%v", *verbose)

	ctx := context.Background()
	worktrees, err := c.worktreeManager.ListWorktrees(ctx)
	if err != nil {
		logging.Error("CLI list failed: %v", err)
		return err
	}

	logging.Info("CLI list: found %d worktrees", len(worktrees))

	if len(worktrees) == 0 {
		fmt.Println("No worktrees found")
		return nil
	}

	if *verbose {
		// Verbose table output
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tPATH\tBRANCH\tCURRENT")
		for _, wt := range worktrees {
			current := ""
			if wt.IsCurrent {
				current = "*"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", wt.Name, wt.Path, wt.Branch, current)
		}
		w.Flush()
	} else {
		// Simple list
		for _, wt := range worktrees {
			prefix := "  "
			if wt.IsCurrent {
				prefix = "* "
			}
			fmt.Printf("%s%s (%s)\n", prefix, wt.Name, wt.Branch)
		}
	}

	return nil
}

// handleDelete handles the delete command
func (c *CLI) handleDelete(args []string) error {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)
	force := fs.Bool("f", false, "Force deletion without confirmation")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: gren delete [options] <worktree-name>\n")
		fmt.Fprintf(fs.Output(), "\nDelete a git worktree\n\n")
		fmt.Fprintf(fs.Output(), "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(fs.Output(), "\nExamples:\n")
		fmt.Fprintf(fs.Output(), "  gren delete feature-branch\n")
		fmt.Fprintf(fs.Output(), "  gren delete -f old-feature\n")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		logging.Error("CLI delete: worktree name is required")
		fs.Usage()
		return fmt.Errorf("worktree name is required")
	}

	worktreeName := fs.Arg(0)
	logging.Info("CLI delete: worktree=%s, force=%v", worktreeName, *force)

	// Confirmation unless force is specified
	if !*force {
		fmt.Printf("Delete worktree '%s'? (y/N): ", worktreeName)
		var response string
		fmt.Scanln(&response)
		response = strings.ToLower(strings.TrimSpace(response))
		if response != "y" && response != "yes" {
			logging.Info("CLI delete: user cancelled deletion of %s", worktreeName)
			fmt.Println("Cancelled")
			return nil
		}
		logging.Info("CLI delete: user confirmed deletion of %s", worktreeName)
	}

	ctx := context.Background()
	err := c.worktreeManager.DeleteWorktree(ctx, worktreeName)
	if err != nil {
		logging.Error("CLI delete failed: %v", err)
	} else {
		logging.Info("CLI delete succeeded: %s", worktreeName)
	}
	return err
}

// handleInit handles the init command (non-interactive)
func (c *CLI) handleInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	project := fs.String("project", "", "Project name (defaults to repository name)")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: gren init [options]\n")
		fmt.Fprintf(fs.Output(), "\nInitialize gren in the current repository\n\n")
		fmt.Fprintf(fs.Output(), "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	projectName := *project
	if projectName == "" {
		// Get repository name
		ctx := context.Background()
		repoInfo, err := c.gitRepo.GetRepoInfo(ctx)
		if err != nil {
			logging.Error("CLI init: failed to get repo info: %v", err)
			return fmt.Errorf("failed to get repository info: %w", err)
		}
		projectName = repoInfo.Name
	}

	logging.Info("CLI init: project=%s", projectName)

	result := config.Initialize(projectName)
	if result.Error != nil {
		logging.Error("CLI init failed: %v", result.Error)
		return fmt.Errorf("initialization failed: %w", result.Error)
	}

	logging.Info("CLI init succeeded: configCreated=%v, hookCreated=%v", result.ConfigCreated, result.HookCreated)

	fmt.Printf("✅ %s\n", result.Message)
	if result.ConfigCreated {
		fmt.Println("📝 Configuration file created")
	}
	if result.HookCreated {
		fmt.Println("🪝 Post-create hook script created")
	}

	return nil
}

// handleNavigate handles the navigate command
func (c *CLI) handleNavigate(args []string) error {
	fs := flag.NewFlagSet("navigate", flag.ExitOnError)

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: gren navigate [worktree-name]\n")
		fmt.Fprintf(fs.Output(), "\nNavigate to a worktree directory by writing command to temp file\n\n")
		fmt.Fprintf(fs.Output(), "Examples:\n")
		fmt.Fprintf(fs.Output(), "  gren navigate feature-branch\n")
		fmt.Fprintf(fs.Output(), "  gren nav feature-branch\n")
		fmt.Fprintf(fs.Output(), "  gren cd feature-branch\n")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		logging.Error("CLI navigate: worktree name is required")
		fs.Usage()
		return fmt.Errorf("worktree name is required")
	}

	worktreeName := fs.Arg(0)
	logging.Info("CLI navigate: worktree=%s", worktreeName)

	// Get list of worktrees to find the path
	ctx := context.Background()
	worktrees, err := c.worktreeManager.ListWorktrees(ctx)
	if err != nil {
		logging.Error("CLI navigate: failed to list worktrees: %v", err)
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	// Find the worktree by name
	var targetWorktree *core.WorktreeInfo
	for _, wt := range worktrees {
		if wt.Name == worktreeName {
			targetWorktree = &wt
			break
		}
	}

	if targetWorktree == nil {
		logging.Error("CLI navigate: worktree '%s' not found", worktreeName)
		return fmt.Errorf("worktree '%s' not found", worktreeName)
	}

	// Write navigation command to temp file
	tempFile := "/tmp/gren_navigate"
	command := fmt.Sprintf("cd \"%s\"", targetWorktree.Path)

	if err := os.WriteFile(tempFile, []byte(command), 0644); err != nil {
		logging.Error("CLI navigate: failed to write navigation command: %v", err)
		return fmt.Errorf("failed to write navigation command: %w", err)
	}

	logging.Info("CLI navigate: wrote navigation command to %s for path %s", tempFile, targetWorktree.Path)
	fmt.Printf("Navigation command written. Use the gren wrapper script to navigate to %s\n", targetWorktree.Path)
	return nil
}

// handleShellInit handles the shell-init command for setting up navigation
func (c *CLI) handleShellInit(args []string) error {
	fs := flag.NewFlagSet("shell-init", flag.ExitOnError)

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: gren shell-init <shell>\n")
		fmt.Fprintf(fs.Output(), "\nGenerate shell integration code for navigation functionality\n\n")
		fmt.Fprintf(fs.Output(), "Supported shells: bash, zsh, fish\n\n")
		fmt.Fprintf(fs.Output(), "Examples:\n")
		fmt.Fprintf(fs.Output(), "  eval \"$(gren shell-init zsh)\"     # Add to ~/.zshrc\n")
		fmt.Fprintf(fs.Output(), "  eval \"$(gren shell-init bash)\"    # Add to ~/.bashrc\n")
		fmt.Fprintf(fs.Output(), "  gren shell-init fish >> ~/.config/fish/config.fish\n")
		fmt.Fprintf(fs.Output(), "\nAfter setup, use gren normally:\n")
		fmt.Fprintf(fs.Output(), "  gren                         # TUI with navigation (press 'g')\n")
		fmt.Fprintf(fs.Output(), "  gren navigate <name>         # Navigate to worktree\n")
		fmt.Fprintf(fs.Output(), "  gcd <name>                   # Short alias\n")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		logging.Error("CLI shell-init: shell type required")
		fs.Usage()
		return fmt.Errorf("shell type required")
	}

	shell := fs.Arg(0)
	logging.Info("CLI shell-init: shell=%s", shell)
	switch shell {
	case "bash", "zsh":
		fmt.Print(bashZshInit)
	case "fish":
		fmt.Print(fishInit)
	default:
		logging.Error("CLI shell-init: unsupported shell: %s", shell)
		return fmt.Errorf("unsupported shell: %s (supported: bash, zsh, fish)", shell)
	}

	return nil
}

const bashZshInit = `# gren navigation wrapper
_gren_original_command=$(which gren)

gren() {
    local TEMP_FILE="/tmp/gren_navigate"
    rm -f "$TEMP_FILE"

    "$_gren_original_command" "$@"
    local exit_code=$?

    if [ -f "$TEMP_FILE" ]; then
        local COMMAND=$(cat "$TEMP_FILE")
        rm -f "$TEMP_FILE"
        eval "$COMMAND"
        echo "📂 Now in: $(pwd)"
        # Auto-allow direnv if available
        if command -v direnv &> /dev/null && [ -f ".envrc" ]; then
            direnv allow 2>/dev/null
        fi
    fi

    return $exit_code
}

# Convenient aliases for navigation
alias gcd='gren navigate'
alias gnav='gren navigate'
`

const fishInit = `# gren navigation wrapper for fish shell
set _gren_original_command (which gren)

function gren
    set TEMP_FILE "/tmp/gren_navigate"
    rm -f $TEMP_FILE

    $_gren_original_command $argv
    set exit_code $status

    if test -f $TEMP_FILE
        set COMMAND (cat $TEMP_FILE)
        rm -f $TEMP_FILE
        eval $COMMAND
        echo "📂 Now in: "(pwd)
        # Auto-allow direnv if available
        if command -v direnv &> /dev/null; and test -f ".envrc"
            direnv allow 2>/dev/null
        end
    end

    return $exit_code
end

# Convenient aliases for navigation
alias gcd='gren navigate'
alias gnav='gren navigate'
`

// ShowHelp shows the general help message
func (c *CLI) ShowHelp() {
	fmt.Println("gren - Git Worktree Manager")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  gren                    Start interactive TUI")
	fmt.Println("  gren create -n <name>   Create new worktree")
	fmt.Println("  gren list              List all worktrees")
	fmt.Println("  gren delete <name>     Delete a worktree")
	fmt.Println("  gren navigate <name>   Navigate to worktree (requires shell setup)")
	fmt.Println("  gren shell-init <shell> Generate shell integration for navigation")
	fmt.Println("  gren init              Initialize gren in repository")
	fmt.Println()
	fmt.Println("Global Options:")
	fmt.Println("  --help                 Show this help message")
	fmt.Println("  --version              Show version information")
	fmt.Println()
	fmt.Println("Use 'gren <command> --help' for more information about a command.")
}
