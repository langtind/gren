package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/langtind/gren/internal/config"
	"github.com/langtind/gren/internal/core"
	"github.com/langtind/gren/internal/directive"
	"github.com/langtind/gren/internal/git"
	"github.com/langtind/gren/internal/logging"
)

// spinner provides a simple CLI spinner
type spinner struct {
	frames  []string
	index   int
	message string
	done    chan struct{}
	wg      sync.WaitGroup
}

func newSpinner(message string) *spinner {
	return &spinner{
		frames:  []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		message: message,
		done:    make(chan struct{}),
	}
}

func (s *spinner) Start() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-s.done:
				fmt.Printf("\r\033[K") // Clear line
				return
			case <-ticker.C:
				fmt.Printf("\r%s %s", s.frames[s.index], s.message)
				s.index = (s.index + 1) % len(s.frames)
			}
		}
	}()
}

func (s *spinner) Stop() {
	close(s.done)
	s.wg.Wait()
}

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
	case "cleanup":
		return c.handleCleanup(args[2:])
	case "init":
		return c.handleInit(args[2:])
	case "navigate", "nav", "cd", "switch":
		return c.handleNavigate(args[2:])
	case "shell-init":
		return c.handleShellInit(args[2:])
	case "compare":
		return c.handleCompare(args[2:])
	case "marker":
		return c.handleMarker(args[2:])
	case "setup-claude-plugin":
		return c.handleSetupClaudePlugin(args[2:])
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
	execute := fs.String("x", "", "Command to run after creating worktree (e.g., -x claude)")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: gren create -n <name> [options]\n")
		fmt.Fprintf(fs.Output(), "\nCreate a new git worktree\n\n")
		fmt.Fprintf(fs.Output(), "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(fs.Output(), "\nExamples:\n")
		fmt.Fprintf(fs.Output(), "  gren create -n feature-branch\n")
		fmt.Fprintf(fs.Output(), "  gren create -n hotfix -b main\n")
		fmt.Fprintf(fs.Output(), "  gren create -n existing-feature --existing --branch feature-branch\n")
		fmt.Fprintf(fs.Output(), "  gren create -n feat-auth -x claude              # Create and start Claude\n")
		fmt.Fprintf(fs.Output(), "  gren create -n feat-ui -x \"npm run dev\"         # Create and start dev server\n")
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

	logging.Info("CLI create: name=%s, branch=%s, base=%s, existing=%v, dir=%s, execute=%s",
		*name, *branch, effectiveBaseBranch, *existing, *worktreeDir, *execute)

	req := core.CreateWorktreeRequest{
		Name:        *name,
		Branch:      *branch,
		BaseBranch:  effectiveBaseBranch,
		IsNewBranch: !*existing,
		WorktreeDir: *worktreeDir,
	}

	ctx := context.Background()
	worktreePath, warning, err := c.worktreeManager.CreateWorktree(ctx, req)
	if err != nil {
		logging.Error("CLI create failed: %v", err)
		return err
	}

	if warning != "" {
		fmt.Printf("⚠ %s\n", warning)
	}
	logging.Info("CLI create succeeded: %s at %s", *name, worktreePath)

	// Handle execute flag (-x)
	if *execute != "" {
		logging.Info("CLI create: writing execute directive for command: %s", *execute)
		if err := directive.WriteCDAndRun(worktreePath, *execute); err != nil {
			logging.Error("CLI create: failed to write execute directive: %v", err)
			return fmt.Errorf("worktree created but failed to set up execute command: %w", err)
		}
		// Don't print anything - shell wrapper will execute the command
	}

	return nil
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

	// Show spinner while fetching data (when GitHub is available)
	var sp *spinner
	if c.worktreeManager.CheckGitHubAvailability() == core.GitHubAvailable {
		sp = newSpinner("Fetching worktree status...")
		sp.Start()
	}

	ctx := context.Background()
	worktrees, err := c.worktreeManager.ListWorktrees(ctx)
	if err != nil {
		if sp != nil {
			sp.Stop()
		}
		logging.Error("CLI list failed: %v", err)
		return err
	}

	// Enrich with GitHub status if available
	if c.worktreeManager.CheckGitHubAvailability() == core.GitHubAvailable {
		logging.Debug("CLI list: enriching with GitHub status")
		c.worktreeManager.EnrichWithGitHubStatus(worktrees)
	}

	if sp != nil {
		sp.Stop()
	}

	logging.Info("CLI list: found %d worktrees", len(worktrees))

	if len(worktrees) == 0 {
		fmt.Println("No worktrees found")
		return nil
	}

	if *verbose {
		// Verbose table output
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "BRANCH\tSTATUS\tSTALE\tPR\tPATH")
		for _, wt := range worktrees {
			current := ""
			if wt.IsCurrent {
				current = "*"
			}
			staleInfo := ""
			if wt.BranchStatus == "stale" {
				staleInfo = wt.StaleReason
			}
			prInfo := ""
			if wt.PRNumber > 0 {
				prInfo = fmt.Sprintf("#%d %s", wt.PRNumber, wt.PRState)
			}
			fmt.Fprintf(w, "%s%s\t%s\t%s\t%s\t%s\n", current, wt.Branch, wt.Status, staleInfo, prInfo, wt.Path)
		}
		w.Flush()
	} else {
		// Simple list
		for _, wt := range worktrees {
			prefix := "  "
			if wt.IsCurrent {
				prefix = "* "
			}
			stale := ""
			if wt.BranchStatus == "stale" {
				stale = " [stale: " + wt.StaleReason + "]"
			}
			fmt.Printf("%s%s (%s)%s\n", prefix, wt.Name, wt.Branch, stale)
		}
	}

	return nil
}

// handleDelete handles the delete command
func (c *CLI) handleDelete(args []string) error {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)
	force := fs.Bool("f", false, "Force deletion without confirmation")
	dryRun := fs.Bool("dry-run", false, "Show what would be deleted without actually deleting")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: gren delete [options] <worktree-name>\n")
		fmt.Fprintf(fs.Output(), "\nDelete a git worktree\n\n")
		fmt.Fprintf(fs.Output(), "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(fs.Output(), "\nExamples:\n")
		fmt.Fprintf(fs.Output(), "  gren delete feature-branch\n")
		fmt.Fprintf(fs.Output(), "  gren delete -f old-feature\n")
		fmt.Fprintf(fs.Output(), "  gren delete --dry-run feature-branch\n")
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
	logging.Info("CLI delete: worktree=%s, force=%v, dry-run=%v", worktreeName, *force, *dryRun)

	// Dry run mode - just show what would happen
	if *dryRun {
		fmt.Printf("[dry-run] Would delete worktree: %s\n", worktreeName)
		return nil
	}

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
	err := c.worktreeManager.DeleteWorktree(ctx, worktreeName, false)
	if err != nil {
		logging.Error("CLI delete failed: %v", err)
	} else {
		logging.Info("CLI delete succeeded: %s", worktreeName)
	}
	return err
}

// handleCleanup handles the cleanup command (delete all stale worktrees)
func (c *CLI) handleCleanup(args []string) error {
	fs := flag.NewFlagSet("cleanup", flag.ExitOnError)
	skipConfirmation := fs.Bool("f", false, "Skip confirmation prompt")
	forceDelete := fs.Bool("force-delete", false, "Force delete even with uncommitted changes")
	dryRun := fs.Bool("dry-run", false, "Show what would be deleted without actually deleting")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: gren cleanup [options]\n")
		fmt.Fprintf(fs.Output(), "\nDelete all stale worktrees (merged PRs, gone remotes)\n\n")
		fmt.Fprintf(fs.Output(), "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(fs.Output(), "\nExamples:\n")
		fmt.Fprintf(fs.Output(), "  gren cleanup --dry-run           # See what would be deleted\n")
		fmt.Fprintf(fs.Output(), "  gren cleanup                     # Delete with confirmation\n")
		fmt.Fprintf(fs.Output(), "  gren cleanup -f                  # Delete without confirmation\n")
		fmt.Fprintf(fs.Output(), "  gren cleanup --force-delete      # Force delete (ignore uncommitted changes)\n")
		fmt.Fprintf(fs.Output(), "  gren cleanup -f --force-delete   # Skip confirmation and force delete\n")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	logging.Info("CLI cleanup: skip-confirmation=%v, force-delete=%v, dry-run=%v", *skipConfirmation, *forceDelete, *dryRun)

	// Show spinner while fetching data
	sp := newSpinner("Fetching worktree status...")
	sp.Start()

	ctx := context.Background()
	worktrees, err := c.worktreeManager.ListWorktrees(ctx)
	if err != nil {
		sp.Stop()
		logging.Error("CLI cleanup: failed to list worktrees: %v", err)
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	// Check GitHub availability and enrich with PR status
	if c.worktreeManager.CheckGitHubAvailability() == core.GitHubAvailable {
		logging.Debug("CLI cleanup: enriching with GitHub status")
		c.worktreeManager.EnrichWithGitHubStatus(worktrees)
	}

	sp.Stop()

	// Find stale worktrees
	var staleWorktrees []core.WorktreeInfo
	for _, wt := range worktrees {
		if wt.BranchStatus == "stale" {
			staleWorktrees = append(staleWorktrees, wt)
		}
	}

	if len(staleWorktrees) == 0 {
		fmt.Println("No stale worktrees found")
		return nil
	}

	// Show what will be deleted
	fmt.Printf("Found %d stale worktree(s):\n", len(staleWorktrees))
	hasAnySubmodules := false
	for _, wt := range staleWorktrees {
		reason := wt.StaleReason
		if wt.PRNumber > 0 {
			reason = fmt.Sprintf("%s (PR #%d %s)", reason, wt.PRNumber, wt.PRState)
		}
		submoduleIndicator := ""
		if wt.HasSubmodules {
			submoduleIndicator = " 📦"
			hasAnySubmodules = true
		}
		fmt.Printf("  - %s [%s]%s\n", wt.Branch, reason, submoduleIndicator)
	}
	if hasAnySubmodules {
		fmt.Println("\n  📦 = has submodules (will use force delete automatically)")
	}

	// Dry run mode - just show what would happen
	if *dryRun {
		fmt.Println("\n[dry-run] No worktrees were deleted")
		return nil
	}

	// Confirmation unless skip confirmation is specified
	if !*skipConfirmation {
		fmt.Printf("\nDelete these %d worktrees? (y/N): ", len(staleWorktrees))
		var response string
		fmt.Scanln(&response)
		response = strings.ToLower(strings.TrimSpace(response))
		if response != "y" && response != "yes" {
			logging.Info("CLI cleanup: user cancelled")
			fmt.Println("Cancelled")
			return nil
		}
	}

	// Delete stale worktrees
	var deleted, failed int
	for _, wt := range staleWorktrees {
		err := c.worktreeManager.DeleteWorktree(ctx, wt.Name, *forceDelete)
		if err != nil {
			logging.Error("CLI cleanup: failed to delete %s: %v", wt.Name, err)
			fmt.Printf("  ✗ Failed to delete %s: %v\n", wt.Branch, err)
			failed++
		} else {
			logging.Info("CLI cleanup: deleted %s", wt.Name)
			fmt.Printf("  ✓ Deleted %s\n", wt.Branch)
			deleted++
		}
	}

	fmt.Printf("\nDeleted %d worktree(s)", deleted)
	if failed > 0 {
		fmt.Printf(", %d failed", failed)
	}
	fmt.Println()

	return nil
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

	// CLI defaults to tracking .gren in git (TUI has interactive prompt)
	trackGrenInGit := true
	result := config.Initialize(projectName, trackGrenInGit)
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

func (c *CLI) handleNavigate(args []string) error {
	fs := flag.NewFlagSet("navigate", flag.ExitOnError)

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: gren switch <branch-or-name>\n")
		fmt.Fprintf(fs.Output(), "\nNavigate to a worktree by branch name or worktree name\n\n")
		fmt.Fprintf(fs.Output(), "Special identifiers:\n")
		fmt.Fprintf(fs.Output(), "  -   Switch to previous worktree (like cd -)\n")
		fmt.Fprintf(fs.Output(), "  @   Current worktree (useful in scripts)\n\n")
		fmt.Fprintf(fs.Output(), "Matching priority:\n")
		fmt.Fprintf(fs.Output(), "  1. Exact worktree name match\n")
		fmt.Fprintf(fs.Output(), "  2. Exact branch name match\n")
		fmt.Fprintf(fs.Output(), "  3. Partial branch name match (e.g., 'auth' matches 'feature/auth')\n\n")
		fmt.Fprintf(fs.Output(), "Examples:\n")
		fmt.Fprintf(fs.Output(), "  gren switch feat-auth           # Switch by worktree name\n")
		fmt.Fprintf(fs.Output(), "  gren switch feature/auth        # Switch by branch name\n")
		fmt.Fprintf(fs.Output(), "  gren switch auth                # Partial match\n")
		fmt.Fprintf(fs.Output(), "  gren switch -                   # Previous worktree\n")
		fmt.Fprintf(fs.Output(), "  gren navigate feature-branch    # Alias\n")
		fmt.Fprintf(fs.Output(), "  gren cd feature-branch          # Alias\n")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		logging.Error("CLI navigate: worktree identifier is required")
		fs.Usage()
		return fmt.Errorf("worktree identifier is required")
	}

	query := fs.Arg(0)
	logging.Info("CLI navigate: query=%s", query)

	ctx := context.Background()
	worktrees, err := c.worktreeManager.ListWorktrees(ctx)
	if err != nil {
		logging.Error("CLI navigate: failed to list worktrees: %v", err)
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	currentPath := getCurrentWorktreePath(worktrees)

	var targetWorktree *core.WorktreeInfo

	switch query {
	case "-":
		prevPath, err := getPreviousWorktree()
		if err != nil || prevPath == "" {
			return fmt.Errorf("no previous worktree")
		}
		for i, wt := range worktrees {
			if wt.Path == prevPath {
				targetWorktree = &worktrees[i]
				break
			}
		}
		if targetWorktree == nil {
			return fmt.Errorf("previous worktree no longer exists: %s", prevPath)
		}
	case "@":
		for i, wt := range worktrees {
			if wt.IsCurrent {
				targetWorktree = &worktrees[i]
				break
			}
		}
		if targetWorktree == nil {
			return fmt.Errorf("not in a worktree")
		}
	default:
		targetWorktree = findWorktreeByQuery(worktrees, query)
	}

	if targetWorktree == nil {
		logging.Error("CLI navigate: no worktree matching '%s'", query)
		fmt.Printf("No worktree found matching '%s'\n\n", query)
		fmt.Println("Available worktrees:")
		for _, wt := range worktrees {
			fmt.Printf("  %s (%s)\n", wt.Name, wt.Branch)
		}
		return fmt.Errorf("worktree '%s' not found", query)
	}

	if currentPath != "" && currentPath != targetWorktree.Path {
		_ = setPreviousWorktree(currentPath)
	}

	if err := directive.WriteCD(targetWorktree.Path); err != nil {
		logging.Error("CLI navigate: failed to write navigation directive: %v", err)
		return fmt.Errorf("failed to write navigation command: %w", err)
	}

	logging.Info("CLI navigate: wrote navigation directive for path %s", targetWorktree.Path)
	if !directive.IsShellIntegrationActive() {
		fmt.Printf("Navigation command written. Ensure shell integration is set up.\n")
		fmt.Printf("Run: eval \"$(gren shell-init zsh)\"  # or bash/fish\n")
	}
	return nil
}

func findWorktreeByQuery(worktrees []core.WorktreeInfo, query string) *core.WorktreeInfo {
	query = strings.ToLower(query)

	for i, wt := range worktrees {
		if strings.ToLower(wt.Name) == query {
			return &worktrees[i]
		}
	}

	for i, wt := range worktrees {
		if strings.ToLower(wt.Branch) == query {
			return &worktrees[i]
		}
	}

	for i, wt := range worktrees {
		branch := strings.ToLower(wt.Branch)
		if strings.HasSuffix(branch, "/"+query) || strings.HasSuffix(branch, "-"+query) {
			return &worktrees[i]
		}
	}

	for i, wt := range worktrees {
		branch := strings.ToLower(wt.Branch)
		if strings.Contains(branch, query) {
			return &worktrees[i]
		}
	}

	return nil
}

func getPreviousWorktree() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "config", "--local", "gren.previousWorktree")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func setPreviousWorktree(path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "config", "--local", "gren.previousWorktree", path)
	return cmd.Run()
}

func getCurrentWorktreePath(worktrees []core.WorktreeInfo) string {
	for _, wt := range worktrees {
		if wt.IsCurrent {
			return wt.Path
		}
	}
	return ""
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

const bashZshInit = `# gren shell integration
# Uses directive files for navigation and command execution
# See: https://github.com/langtind/gren

# Only initialize if gren is available
if command -v gren >/dev/null 2>&1 || [[ -n "${GREN_BIN:-}" ]]; then

    # Override gren command with directive file passing.
    # Creates a temp file, passes path via GREN_DIRECTIVE_FILE, sources it after.
    # GREN_BIN can override the binary path (for testing dev builds).
    gren() {
        # Completion mode: call binary directly, no directive file needed.
        if [[ -n "${COMPLETE:-}" ]]; then
            command "${GREN_BIN:-gren}" "$@"
            return
        fi

        local directive_file exit_code=0
        directive_file="$(mktemp)"

        GREN_DIRECTIVE_FILE="$directive_file" command "${GREN_BIN:-gren}" "$@" || exit_code=$?

        if [[ -s "$directive_file" ]]; then
            source "$directive_file"
            # Show new directory if we changed
            if [[ "$PWD" != "$OLDPWD" ]]; then
                echo "📂 Now in: $(pwd)"
                # Auto-allow direnv if available
                if command -v direnv &> /dev/null && [[ -f ".envrc" ]]; then
                    direnv allow 2>/dev/null
                fi
            fi
        fi

        rm -f "$directive_file"
        return "$exit_code"
    }

    # Convenient aliases for navigation
    alias gcd='gren navigate'
    alias gnav='gren navigate'
fi
`

const fishInit = `# gren shell integration for fish
# Uses directive files for navigation and command execution
# See: https://github.com/langtind/gren

# Only initialize if gren is available
if command -v gren >/dev/null 2>&1; or set -q GREN_BIN

    # Override gren command with directive file passing.
    # Creates a temp file, passes path via GREN_DIRECTIVE_FILE, sources it after.
    # GREN_BIN can override the binary path (for testing dev builds).
    function gren
        # Completion mode: call binary directly, no directive file needed.
        if set -q COMPLETE
            command (set -q GREN_BIN; and echo $GREN_BIN; or echo gren) $argv
            return
        end

        set -l directive_file (mktemp)
        set -l exit_code 0
        set -l old_pwd $PWD

        GREN_DIRECTIVE_FILE=$directive_file command (set -q GREN_BIN; and echo $GREN_BIN; or echo gren) $argv
        or set exit_code $status

        if test -s $directive_file
            source $directive_file
            # Show new directory if we changed
            if test "$PWD" != "$old_pwd"
                echo "📂 Now in: "(pwd)
                # Auto-allow direnv if available
                if command -v direnv >/dev/null 2>&1; and test -f ".envrc"
                    direnv allow 2>/dev/null
                end
            end
        end

        rm -f $directive_file
        return $exit_code
    end

    # Convenient aliases for navigation
    alias gcd='gren navigate'
    alias gnav='gren navigate'
end
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
	fmt.Println("  gren compare <name>    Compare changes from another worktree")
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

// handleCompare handles the compare command
func (c *CLI) handleCompare(args []string) error {
	fs := flag.NewFlagSet("compare", flag.ExitOnError)
	diff := fs.Bool("diff", false, "Show unified diff output for all files")
	apply := fs.Bool("apply", false, "Apply all changes from source to current worktree")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: gren compare <worktree-name> [options]\n")
		fmt.Fprintf(fs.Output(), "\nCompare changes between worktrees\n\n")
		fmt.Fprintf(fs.Output(), "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(fs.Output(), "\nExamples:\n")
		fmt.Fprintf(fs.Output(), "  gren compare feature-branch           # List changed files\n")
		fmt.Fprintf(fs.Output(), "  gren compare feature-branch --diff    # Show diff output\n")
		fmt.Fprintf(fs.Output(), "  gren compare feature-branch --apply   # Apply all changes\n")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		fs.Usage()
		return fmt.Errorf("worktree name is required")
	}

	sourceWorktree := fs.Arg(0)
	ctx := context.Background()

	logging.Info("CLI compare: comparing %s to current worktree", sourceWorktree)

	// Get the comparison result
	result, err := c.worktreeManager.CompareWorktrees(ctx, sourceWorktree)
	if err != nil {
		return fmt.Errorf("compare failed: %w", err)
	}

	if len(result.Files) == 0 {
		fmt.Println("No changes found between worktrees")
		return nil
	}

	// Handle apply mode
	if *apply {
		fmt.Printf("Applying %d file(s) from %s...\n", len(result.Files), sourceWorktree)
		if err := c.worktreeManager.ApplyChanges(ctx, sourceWorktree, result.Files); err != nil {
			return fmt.Errorf("apply failed: %w", err)
		}
		fmt.Printf("Successfully applied %d file(s)\n", len(result.Files))
		return nil
	}

	// Handle diff mode
	if *diff {
		return c.showCompareWithDiff(sourceWorktree, result)
	}

	// Default: show file list
	fmt.Printf("Changes from %s → %s:\n\n", result.SourceWorktree, result.TargetWorktree)
	for _, file := range result.Files {
		statusIcon := "?"
		switch file.Status {
		case core.FileAdded:
			statusIcon = "+"
		case core.FileModified:
			statusIcon = "~"
		case core.FileDeleted:
			statusIcon = "-"
		}
		committedTag := ""
		if file.IsCommitted {
			committedTag = " (committed)"
		}
		fmt.Printf("  %s %s%s\n", statusIcon, file.Path, committedTag)
	}
	fmt.Printf("\n%d file(s) changed\n", len(result.Files))
	fmt.Println("\nUse --diff to see detailed changes or --apply to copy files")

	return nil
}

// showCompareWithDiff shows the comparison with unified diff output
func (c *CLI) showCompareWithDiff(sourceWorktree string, result *core.CompareResult) error {
	ctx := context.Background()

	// Get worktree paths
	worktrees, err := c.worktreeManager.ListWorktrees(ctx)
	if err != nil {
		return err
	}

	var sourcePath, currentPath string
	for _, wt := range worktrees {
		if wt.Name == sourceWorktree || wt.Path == sourceWorktree {
			sourcePath = wt.Path
		}
		if wt.IsCurrent {
			currentPath = wt.Path
		}
	}

	// Validate paths were found
	if sourcePath == "" {
		return fmt.Errorf("source worktree '%s' not found", sourceWorktree)
	}
	if currentPath == "" {
		return fmt.Errorf("current worktree not found")
	}

	fmt.Printf("Changes from %s → %s:\n", result.SourceWorktree, result.TargetWorktree)
	fmt.Println(strings.Repeat("=", 60))

	for _, file := range result.Files {
		fmt.Printf("\n--- %s ---\n", file.Path)

		// Validate path to prevent path traversal attacks
		if err := validateFilePath(file.Path); err != nil {
			fmt.Printf("[Error: %v]\n", err)
			continue
		}

		switch file.Status {
		case core.FileAdded:
			fmt.Println("[NEW FILE]")
			// Show file content using filepath.Join for safe path construction
			srcFile := filepath.Join(sourcePath, file.Path)
			content, err := os.ReadFile(srcFile)
			if err != nil {
				fmt.Printf("[Error reading file: %v]\n", err)
			} else {
				lines := strings.Split(string(content), "\n")
				for _, line := range lines {
					fmt.Printf("+ %s\n", line)
				}
			}
		case core.FileDeleted:
			fmt.Println("[DELETED]")
		case core.FileModified:
			// Use git diff for cross-platform compatibility (works on Windows via Git)
			currentFile := filepath.Join(currentPath, file.Path)
			sourceFile := filepath.Join(sourcePath, file.Path)
			cmd := exec.Command("git", "diff", "--no-index", "--", currentFile, sourceFile)
			output, _ := cmd.CombinedOutput()
			if len(output) > 0 {
				fmt.Println(string(output))
			} else {
				fmt.Println("[Binary or no diff available]")
			}
		default:
			fmt.Printf("[Unknown status: %v]\n", file.Status)
		}
	}

	return nil
}

func validateFilePath(path string) error {
	if strings.Contains(path, "..") {
		return fmt.Errorf("invalid path (contains '..'): %s", path)
	}
	if filepath.IsAbs(path) {
		return fmt.Errorf("invalid path (absolute path not allowed): %s", path)
	}
	return nil
}

func (c *CLI) handleMarker(args []string) error {
	if len(args) == 0 {
		return c.handleMarkerList()
	}

	subcommand := args[0]
	subargs := args[1:]

	switch subcommand {
	case "set":
		return c.handleMarkerSet(subargs)
	case "clear":
		return c.handleMarkerClear(subargs)
	case "get":
		return c.handleMarkerGet(subargs)
	case "list":
		return c.handleMarkerList()
	default:
		return fmt.Errorf("unknown marker subcommand: %s (use: set, clear, get, list)", subcommand)
	}
}

func (c *CLI) handleMarkerSet(args []string) error {
	fs := flag.NewFlagSet("marker set", flag.ExitOnError)
	branch := fs.String("branch", "", "Branch name (defaults to current branch)")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: gren marker set <type> [-branch <name>]\n")
		fmt.Fprintf(fs.Output(), "\nSet a Claude activity marker for a branch\n\n")
		fmt.Fprintf(fs.Output(), "Marker types:\n")
		fmt.Fprintf(fs.Output(), "  working  🤖  Claude is actively working\n")
		fmt.Fprintf(fs.Output(), "  waiting  💬  Claude is waiting for input\n")
		fmt.Fprintf(fs.Output(), "  idle     💤  Claude session is idle\n")
		fmt.Fprintf(fs.Output(), "\nOptions:\n")
		fs.PrintDefaults()
		fmt.Fprintf(fs.Output(), "\nExamples:\n")
		fmt.Fprintf(fs.Output(), "  gren marker set working                    # Set working marker on current branch\n")
		fmt.Fprintf(fs.Output(), "  gren marker set waiting -branch feat-auth  # Set waiting marker on specific branch\n")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		fs.Usage()
		return fmt.Errorf("marker type is required")
	}

	markerTypeStr := fs.Arg(0)
	markerType, err := core.ParseMarkerType(markerTypeStr)
	if err != nil {
		return err
	}

	targetBranch := *branch
	if targetBranch == "" {
		ctx := context.Background()
		currentBranch, err := c.gitRepo.GetCurrentBranch(ctx)
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}
		targetBranch = currentBranch
	}

	ctx := context.Background()
	mm := core.NewMarkerManager()
	if err := mm.SetMarker(ctx, targetBranch, markerType); err != nil {
		return err
	}

	logging.Info("CLI marker set: branch=%s, type=%s", targetBranch, markerType)
	fmt.Printf("%s Marker set for %s\n", markerType, targetBranch)
	return nil
}

func (c *CLI) handleMarkerClear(args []string) error {
	fs := flag.NewFlagSet("marker clear", flag.ExitOnError)
	branch := fs.String("branch", "", "Branch name (defaults to current branch)")
	all := fs.Bool("all", false, "Clear all markers in the repository")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: gren marker clear [-branch <name>] [-all]\n")
		fmt.Fprintf(fs.Output(), "\nClear Claude activity marker for a branch\n\n")
		fmt.Fprintf(fs.Output(), "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx := context.Background()
	mm := core.NewMarkerManager()

	if *all {
		if err := mm.ClearAllMarkers(ctx); err != nil {
			return err
		}
		logging.Info("CLI marker clear: cleared all markers")
		fmt.Println("Cleared all markers")
		return nil
	}

	targetBranch := *branch
	if targetBranch == "" {
		currentBranch, err := c.gitRepo.GetCurrentBranch(ctx)
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}
		targetBranch = currentBranch
	}

	if err := mm.ClearMarker(ctx, targetBranch); err != nil {
		return err
	}

	logging.Info("CLI marker clear: branch=%s", targetBranch)
	fmt.Printf("Cleared marker for %s\n", targetBranch)
	return nil
}

func (c *CLI) handleMarkerGet(args []string) error {
	fs := flag.NewFlagSet("marker get", flag.ExitOnError)
	branch := fs.String("branch", "", "Branch name (defaults to current branch)")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: gren marker get [-branch <name>]\n")
		fmt.Fprintf(fs.Output(), "\nGet Claude activity marker for a branch\n\n")
		fmt.Fprintf(fs.Output(), "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	targetBranch := *branch
	ctx := context.Background()
	if targetBranch == "" {
		currentBranch, err := c.gitRepo.GetCurrentBranch(ctx)
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}
		targetBranch = currentBranch
	}

	mm := core.NewMarkerManager()
	marker, err := mm.GetMarker(ctx, targetBranch)
	if err != nil {
		return err
	}

	if marker == "" {
		fmt.Printf("No marker set for %s\n", targetBranch)
	} else {
		fmt.Printf("%s %s\n", marker, targetBranch)
	}
	return nil
}

func (c *CLI) handleMarkerList() error {
	ctx := context.Background()
	mm := core.NewMarkerManager()

	markers, err := mm.ListMarkers(ctx)
	if err != nil {
		return err
	}

	if len(markers) == 0 {
		fmt.Println("No markers set")
		return nil
	}

	fmt.Println("Branch markers:")
	for branch, marker := range markers {
		fmt.Printf("  %s %s\n", marker, branch)
	}
	return nil
}

func (c *CLI) handleSetupClaudePlugin(args []string) error {
	fs := flag.NewFlagSet("setup-claude-plugin", flag.ExitOnError)
	force := fs.Bool("f", false, "Overwrite existing files")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: gren setup-claude-plugin [-f]\n")
		fmt.Fprintf(fs.Output(), "\nCreate .claude-plugin directory with hooks for Claude activity tracking\n\n")
		fmt.Fprintf(fs.Output(), "This enables Claude to automatically set markers when working:\n")
		fmt.Fprintf(fs.Output(), "  🤖 working - Claude is actively processing\n")
		fmt.Fprintf(fs.Output(), "  💬 waiting - Claude is waiting for input\n\n")
		fmt.Fprintf(fs.Output(), "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	return core.SetupClaudePlugin(*force)
}
