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
)

// CLI handles command-line interface operations
type CLI struct {
	gitRepo        git.Repository
	configManager  *config.Manager
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
		return fmt.Errorf("no command provided")
	}

	command := args[1]

	switch command {
	case "create":
		return c.handleCreate(args[2:])
	case "list":
		return c.handleList(args[2:])
	case "delete":
		return c.handleDelete(args[2:])
	case "init":
		return c.handleInit(args[2:])
	default:
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
		fs.Usage()
		return fmt.Errorf("worktree name is required")
	}

	req := core.CreateWorktreeRequest{
		Name:        *name,
		Branch:      *branch,
		BaseBranch:  *baseBranch,
		IsNewBranch: !*existing,
		WorktreeDir: *worktreeDir,
	}

	ctx := context.Background()
	return c.worktreeManager.CreateWorktree(ctx, req)
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

	ctx := context.Background()
	worktrees, err := c.worktreeManager.ListWorktrees(ctx)
	if err != nil {
		return err
	}

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
		fs.Usage()
		return fmt.Errorf("worktree name is required")
	}

	worktreeName := fs.Arg(0)

	// Confirmation unless force is specified
	if !*force {
		fmt.Printf("Delete worktree '%s'? (y/N): ", worktreeName)
		var response string
		fmt.Scanln(&response)
		response = strings.ToLower(strings.TrimSpace(response))
		if response != "y" && response != "yes" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	ctx := context.Background()
	return c.worktreeManager.DeleteWorktree(ctx, worktreeName)
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
			return fmt.Errorf("failed to get repository info: %w", err)
		}
		projectName = repoInfo.Name
	}

	result := config.Initialize(projectName)
	if result.Error != nil {
		return fmt.Errorf("initialization failed: %w", result.Error)
	}

	fmt.Printf("✅ %s\n", result.Message)
	if result.ConfigCreated {
		fmt.Println("📝 Configuration file created")
	}
	if result.HookCreated {
		fmt.Println("🪝 Post-create hook script created")
	}

	return nil
}

// ShowHelp shows the general help message
func (c *CLI) ShowHelp() {
	fmt.Println("gren - Git Worktree Manager")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  gren                    Start interactive TUI")
	fmt.Println("  gren create -n <name>   Create new worktree")
	fmt.Println("  gren list              List all worktrees")
	fmt.Println("  gren delete <name>     Delete a worktree")
	fmt.Println("  gren init              Initialize gren in repository")
	fmt.Println()
	fmt.Println("Global Options:")
	fmt.Println("  --help                 Show this help message")
	fmt.Println("  --version              Show version information")
	fmt.Println()
	fmt.Println("Use 'gren <command> --help' for more information about a command.")
}