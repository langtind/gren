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
	"time"

	"github.com/langtind/gren/internal/config"
	"github.com/langtind/gren/internal/core"
	"github.com/langtind/gren/internal/directive"
	"github.com/langtind/gren/internal/git"
	"github.com/langtind/gren/internal/logging"
	"github.com/langtind/gren/internal/output"
)

// spinner provides a simple CLI spinner
type spinner struct {
	frames  []string
	index   int
	message string
	done    chan struct{}
	wg      sync.WaitGroup
	active  bool
}

func newSpinner(message string) *spinner {
	return &spinner{
		frames:  []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		message: message,
		done:    make(chan struct{}),
	}
}

func (s *spinner) Start() {
	// Don't show spinner if stdout is not a terminal (e.g., in tests or pipes)
	if !isTerminal() {
		return
	}
	s.active = true
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
	if !s.active {
		return
	}
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
	case "statusline":
		return c.handleStatusline(args[2:])
	case "merge":
		return c.handleMerge(args[2:])
	case "for-each":
		return c.handleForEach(args[2:])
	case "step":
		return c.handleStep(args[2:])
	case "completion":
		return c.handleCompletion(args[2:])
	case "__complete":
		return c.handleCompletionQuery(args[2:])
	case "config":
		return c.handleConfig(args[2:])
	case "hook-run":
		return c.handleHookRun(args[2:])
	case "help":
		if len(args) > 2 {
			ShowCommandHelp(args[2])
		} else {
			c.ShowColoredHelp()
		}
		return nil
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
		output.Warning(warning)
	}
	logging.Info("CLI create succeeded: %s at %s", *name, worktreePath)

	// Run post-create hook with approval checking
	branchName := *branch
	if branchName == "" {
		branchName = *name
	}
	c.worktreeManager.RunPostCreateHookWithApproval(worktreePath, branchName, effectiveBaseBranch, false)

	// Handle execute flag (-x)
	if *execute != "" {
		logging.Info("CLI create: writing execute directive for command: %s", *execute)
		if err := directive.WriteCDAndRun(worktreePath, *execute); err != nil {
			logging.Error("CLI create: failed to write execute directive: %v", err)
			return fmt.Errorf("worktree created but failed to set up execute command: %w", err)
		}

		// Run post-start hook with approval
		c.worktreeManager.RunPostStartHookWithApproval(worktreePath, branchName, *execute, false)
		// Don't print anything - shell wrapper will execute the command
	} else {
		// Print success output when not executing a command
		output.WorktreeCreated(*name, branchName, worktreePath)
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
		c.worktreeManager.EnrichWithCIStatus(worktrees)
	}

	if sp != nil {
		sp.Stop()
	}

	logging.Info("CLI list: found %d worktrees", len(worktrees))

	if len(worktrees) == 0 {
		output.Info("No worktrees found")
		return nil
	}

	// Get repo name for header
	repoInfo, _ := c.gitRepo.GetRepoInfo(ctx)
	repoName := ""
	if repoInfo != nil {
		repoName = repoInfo.Name
	}

	if *verbose {
		// Convert to output format
		var items []output.WorktreeListItem
		for _, wt := range worktrees {
			staleInfo := ""
			if wt.BranchStatus == "stale" {
				staleInfo = wt.StaleReason
			}
			prInfo := ""
			if wt.PRNumber > 0 {
				prInfo = fmt.Sprintf("#%d %s", wt.PRNumber, wt.PRState)
			}
			items = append(items, output.WorktreeListItem{
				Name:      wt.Name,
				Branch:    wt.Branch,
				Path:      wt.Path,
				IsCurrent: wt.IsCurrent,
				IsMain:    wt.IsMain,
				StaleInfo: staleInfo,
				PRInfo:    prInfo,
				CIStatus:  wt.CIStatus,
				Status:    wt.Status,
			})
		}
		output.PrintWorktreeList(items, repoName)
	} else {
		// Simple list with styled output
		var items []output.WorktreeListItem
		for _, wt := range worktrees {
			staleInfo := ""
			if wt.BranchStatus == "stale" {
				staleInfo = wt.StaleReason
			}
			items = append(items, output.WorktreeListItem{
				Name:      wt.Name,
				Branch:    wt.Branch,
				IsCurrent: wt.IsCurrent,
				StaleInfo: staleInfo,
				CIStatus:  wt.CIStatus,
			})
		}
		output.PrintSimpleWorktreeList(items)
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

	// Get worktree info for hook context
	worktrees, err := c.worktreeManager.ListWorktrees(ctx)
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}
	var targetWorktree *core.WorktreeInfo
	for _, wt := range worktrees {
		if wt.Name == worktreeName || wt.Branch == worktreeName {
			targetWorktree = &wt
			break
		}
	}
	if targetWorktree == nil {
		return fmt.Errorf("worktree '%s' not found", worktreeName)
	}

	// Run pre-remove hooks with approval (interactive prompt)
	results := c.worktreeManager.RunPreRemoveHookWithApproval(targetWorktree.Path, targetWorktree.Branch, false)
	if failed := core.FirstFailedHook(results); failed != nil {
		return fmt.Errorf("pre-remove hook failed: %s\n%s", failed.Err, failed.Output)
	}

	err = c.worktreeManager.DeleteWorktree(ctx, worktreeName, false)
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
		output.Errorf("No worktree found matching '%s'", query)
		output.Blank()
		output.Header("Available worktrees:")
		for _, wt := range worktrees {
			output.ListItem(wt.Name+" "+output.Dim("("+wt.Branch+")"), false)
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

	// Run post-switch hook with approval
	c.worktreeManager.RunPostSwitchHookWithApproval(targetWorktree.Path, targetWorktree.Branch, false)

	logging.Info("CLI navigate: wrote navigation directive for path %s", targetWorktree.Path)

	// Print styled output
	output.Successf("Switching to %s", output.Bold(targetWorktree.Name))

	if !directive.IsShellIntegrationActive() {
		// Show path only when shell wrapper won't show "Now in:"
		fmt.Println("📂 " + output.Path(targetWorktree.Path))
		output.Blank()
		output.Hint("Shell integration not detected. Run:")
		fmt.Printf("   eval \"$(gren shell-init zsh)\"  # or bash/fish\n")
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
	showMarkerHelp := func() {
		fmt.Println("Usage: gren marker <subcommand>")
		fmt.Println("\nManage Claude activity markers for branches")
		fmt.Println("\nSubcommands:")
		fmt.Println("  set      Set a marker for a branch")
		fmt.Println("  get      Get the marker for a branch")
		fmt.Println("  clear    Clear marker(s)")
		fmt.Println("  list     List all markers (default)")
		fmt.Println("\nMarker types:")
		fmt.Println("  working  🤖  Claude is actively working")
		fmt.Println("  waiting  💬  Claude is waiting for input")
		fmt.Println("  idle     💤  Claude session is idle")
		fmt.Println("\nExamples:")
		fmt.Println("  gren marker                          # List all markers")
		fmt.Println("  gren marker set working              # Set working on current branch")
		fmt.Println("  gren marker set waiting -branch feat # Set on specific branch")
		fmt.Println("  gren marker clear                    # Clear current branch marker")
		fmt.Println("  gren marker clear --all              # Clear all markers")
		fmt.Println("\nUse 'gren marker <subcommand> --help' for more information.")
	}

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
	case "--help", "-h", "help":
		showMarkerHelp()
		return nil
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

func (c *CLI) handleStatusline(args []string) error {
	fs := flag.NewFlagSet("statusline", flag.ExitOnError)

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: gren statusline\n")
		fmt.Fprintf(fs.Output(), "\nOutput a compact status for shell prompts\n\n")
		fmt.Fprintf(fs.Output(), "Format: <branch> [+N] [~N] [?N] [↑N]\n")
		fmt.Fprintf(fs.Output(), "  +N  = staged files\n")
		fmt.Fprintf(fs.Output(), "  ~N  = modified files\n")
		fmt.Fprintf(fs.Output(), "  ?N  = untracked files\n")
		fmt.Fprintf(fs.Output(), "  ↑N  = unpushed commits\n\n")
		fmt.Fprintf(fs.Output(), "Example shell integration:\n")
		fmt.Fprintf(fs.Output(), "  PROMPT='$(gren statusline) %%~ $ '  # zsh\n")
		fmt.Fprintf(fs.Output(), "  PS1='$(gren statusline) \\w $ '    # bash\n")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx := context.Background()
	worktrees, err := c.worktreeManager.ListWorktrees(ctx)
	if err != nil {
		return nil
	}

	var current *core.WorktreeInfo
	for i, wt := range worktrees {
		if wt.IsCurrent {
			current = &worktrees[i]
			break
		}
	}

	if current == nil {
		return nil
	}

	var parts []string
	parts = append(parts, current.Branch)

	if current.StagedCount > 0 {
		parts = append(parts, fmt.Sprintf("+%d", current.StagedCount))
	}
	if current.ModifiedCount > 0 {
		parts = append(parts, fmt.Sprintf("~%d", current.ModifiedCount))
	}
	if current.UntrackedCount > 0 {
		parts = append(parts, fmt.Sprintf("?%d", current.UntrackedCount))
	}
	if current.UnpushedCount > 0 {
		parts = append(parts, fmt.Sprintf("↑%d", current.UnpushedCount))
	}

	fmt.Println(strings.Join(parts, " "))
	return nil
}

func (c *CLI) handleMerge(args []string) error {
	fs := flag.NewFlagSet("merge", flag.ExitOnError)
	noSquash := fs.Bool("no-squash", false, "Preserve individual commits instead of squashing")
	noRemove := fs.Bool("no-remove", false, "Keep worktree after merge")
	noVerify := fs.Bool("no-verify", false, "Skip pre-merge and post-merge hooks")
	noRebase := fs.Bool("no-rebase", false, "Skip rebase (fail if not already rebased)")
	yes := fs.Bool("y", false, "Skip confirmation prompts")
	force := fs.Bool("f", false, "Force merge even with uncommitted changes")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: gren merge [target] [options]\n")
		fmt.Fprintf(fs.Output(), "\nMerge current worktree into target branch\n\n")
		fmt.Fprintf(fs.Output(), "Pipeline:\n")
		fmt.Fprintf(fs.Output(), "  1. Stage and commit uncommitted changes\n")
		fmt.Fprintf(fs.Output(), "  2. Squash commits into one (unless --no-squash)\n")
		fmt.Fprintf(fs.Output(), "  3. Rebase onto target (unless --no-rebase)\n")
		fmt.Fprintf(fs.Output(), "  4. Run pre-merge hooks (tests, lint)\n")
		fmt.Fprintf(fs.Output(), "  5. Fast-forward merge to target\n")
		fmt.Fprintf(fs.Output(), "  6. Remove worktree and branch (unless --no-remove)\n")
		fmt.Fprintf(fs.Output(), "  7. Run post-merge hooks\n\n")
		fmt.Fprintf(fs.Output(), "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(fs.Output(), "\nExamples:\n")
		fmt.Fprintf(fs.Output(), "  gren merge                  # Merge to default branch\n")
		fmt.Fprintf(fs.Output(), "  gren merge main             # Merge to main\n")
		fmt.Fprintf(fs.Output(), "  gren merge --no-remove      # Keep worktree after merge\n")
		fmt.Fprintf(fs.Output(), "  gren merge --no-squash      # Preserve commit history\n")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	target := ""
	if fs.NArg() > 0 {
		target = fs.Arg(0)
	}

	ctx := context.Background()

	opts := core.MergeOptions{
		Target: target,
		Squash: !*noSquash,
		Remove: !*noRemove,
		Verify: !*noVerify,
		Rebase: !*noRebase,
		Yes:    *yes,
		Force:  *force,
	}

	result, err := c.worktreeManager.Merge(ctx, opts)
	if err != nil {
		return err
	}

	if result.Skipped {
		fmt.Printf("⏭️  Merge skipped: %s\n", result.SkipReason)
		return nil
	}

	fmt.Printf("✅ Merged %s into %s\n", result.SourceBranch, result.TargetBranch)
	if result.CommitsSquashed > 0 {
		fmt.Printf("   Squashed %d commits\n", result.CommitsSquashed)
	}
	if result.WorktreeRemoved {
		fmt.Printf("   Removed worktree: %s\n", result.WorktreePath)
	}

	return nil
}

func (c *CLI) handleForEach(args []string) error {
	fs := flag.NewFlagSet("for-each", flag.ExitOnError)
	skipCurrent := fs.Bool("skip-current", false, "Skip the current worktree")
	skipMain := fs.Bool("skip-main", false, "Skip the main worktree")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: gren for-each [options] -- <command>\n")
		fmt.Fprintf(fs.Output(), "\nRun a command in all worktrees\n\n")
		fmt.Fprintf(fs.Output(), "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(fs.Output(), "\nTemplate variables:\n")
		fmt.Fprintf(fs.Output(), "  {{ branch }}           Branch name\n")
		fmt.Fprintf(fs.Output(), "  {{ branch | sanitize }} Branch with / replaced by -\n")
		fmt.Fprintf(fs.Output(), "  {{ worktree }}         Absolute path to worktree\n")
		fmt.Fprintf(fs.Output(), "  {{ worktree_name }}    Worktree directory name\n")
		fmt.Fprintf(fs.Output(), "  {{ repo }}             Repository name\n")
		fmt.Fprintf(fs.Output(), "  {{ repo_root }}        Absolute path to main repo\n")
		fmt.Fprintf(fs.Output(), "  {{ commit }}           Full HEAD commit SHA\n")
		fmt.Fprintf(fs.Output(), "  {{ short_commit }}     Short HEAD commit SHA\n")
		fmt.Fprintf(fs.Output(), "  {{ default_branch }}   Default branch (main/master)\n")
		fmt.Fprintf(fs.Output(), "\nExamples:\n")
		fmt.Fprintf(fs.Output(), "  gren for-each -- git status --short\n")
		fmt.Fprintf(fs.Output(), "  gren for-each -- npm install\n")
		fmt.Fprintf(fs.Output(), "  gren for-each -- \"echo Branch: {{ branch }}\"\n")
		fmt.Fprintf(fs.Output(), "  gren for-each --skip-main -- git pull\n")
	}

	// Check for help flag before looking for -- separator
	for _, arg := range args {
		if arg == "--help" || arg == "-h" || arg == "help" {
			fs.Usage()
			return nil
		}
	}

	dashIndex := -1
	for i, arg := range args {
		if arg == "--" {
			dashIndex = i
			break
		}
	}

	if dashIndex == -1 {
		fs.Usage()
		return fmt.Errorf("missing -- separator before command")
	}

	if err := fs.Parse(args[:dashIndex]); err != nil {
		return err
	}

	command := args[dashIndex+1:]
	if len(command) == 0 {
		fs.Usage()
		return fmt.Errorf("no command provided")
	}

	ctx := context.Background()

	opts := core.ForEachOptions{
		Command:     command,
		SkipCurrent: *skipCurrent,
		SkipMain:    *skipMain,
	}

	results, err := c.worktreeManager.ForEach(ctx, opts)
	if err != nil {
		return err
	}

	successCount := 0
	failCount := 0

	for _, r := range results {
		fmt.Printf("\n\033[1m%s\033[0m (%s)\n", r.Worktree.Branch, r.Worktree.Path)
		fmt.Print(r.Output)

		if r.Error != nil {
			fmt.Printf("\033[31m✗ Exit code: %d\033[0m\n", r.ExitCode)
			failCount++
		} else {
			successCount++
		}
	}

	fmt.Printf("\n---\n")
	fmt.Printf("✅ %d succeeded", successCount)
	if failCount > 0 {
		fmt.Printf(", \033[31m✗ %d failed\033[0m", failCount)
	}
	fmt.Println()

	if failCount > 0 {
		return fmt.Errorf("%d worktree(s) failed", failCount)
	}

	return nil
}

func (c *CLI) handleStep(args []string) error {
	showStepHelp := func() {
		fmt.Println("Usage: gren step <subcommand>")
		fmt.Println("\nRun individual workflow operations")
		fmt.Println("\nSubcommands:")
		fmt.Println("  commit     Stage and commit all changes")
		fmt.Println("  squash     Squash commits since target branch")
		fmt.Println("  push       Push current branch to local target branch")
		fmt.Println("  rebase     Rebase current branch onto target")
		fmt.Println("\nExamples:")
		fmt.Println("  gren step commit")
		fmt.Println("  gren step commit -m \"feat: add feature\"")
		fmt.Println("  gren step commit --llm")
		fmt.Println("  gren step squash")
		fmt.Println("  gren step squash main")
		fmt.Println("  gren step push")
		fmt.Println("  gren step rebase main")
		fmt.Println("\nUse 'gren step <subcommand> --help' for more information.")
	}

	if len(args) < 1 {
		showStepHelp()
		return fmt.Errorf("no subcommand provided")
	}

	subcommand := args[0]
	switch subcommand {
	case "commit":
		return c.handleStepCommit(args[1:])
	case "squash":
		return c.handleStepSquash(args[1:])
	case "push":
		return c.handleStepPush(args[1:])
	case "rebase":
		return c.handleStepRebase(args[1:])
	case "--help", "-h", "help":
		showStepHelp()
		return nil
	default:
		return fmt.Errorf("unknown step subcommand: %s (use: commit, squash, push, rebase)", subcommand)
	}
}

func (c *CLI) handleStepCommit(args []string) error {
	fs := flag.NewFlagSet("step commit", flag.ExitOnError)
	message := fs.String("m", "", "Commit message")
	useLLM := fs.Bool("llm", false, "Generate commit message using configured LLM")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: gren step commit [options]\n")
		fmt.Fprintf(fs.Output(), "\nStage and commit all changes\n\n")
		fmt.Fprintf(fs.Output(), "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(fs.Output(), "\nExamples:\n")
		fmt.Fprintf(fs.Output(), "  gren step commit                     # Commit with default message\n")
		fmt.Fprintf(fs.Output(), "  gren step commit -m \"feat: feature\"  # Commit with custom message\n")
		fmt.Fprintf(fs.Output(), "  gren step commit --llm               # Use LLM to generate message\n")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	opts := core.StepCommitOptions{
		Message: *message,
		UseLLM:  *useLLM,
	}

	if err := c.worktreeManager.StepCommit(opts); err != nil {
		return err
	}

	output.Success("Changes committed")
	return nil
}

func (c *CLI) handleStepSquash(args []string) error {
	fs := flag.NewFlagSet("step squash", flag.ExitOnError)
	message := fs.String("m", "", "Squash commit message")
	useLLM := fs.Bool("llm", false, "Generate commit message using configured LLM")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: gren step squash [target] [options]\n")
		fmt.Fprintf(fs.Output(), "\nSquash commits since target branch into one commit\n\n")
		fmt.Fprintf(fs.Output(), "Arguments:\n")
		fmt.Fprintf(fs.Output(), "  target    Target branch (default: main/master)\n\n")
		fmt.Fprintf(fs.Output(), "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(fs.Output(), "\nExamples:\n")
		fmt.Fprintf(fs.Output(), "  gren step squash                       # Squash to default branch\n")
		fmt.Fprintf(fs.Output(), "  gren step squash main                  # Squash to main\n")
		fmt.Fprintf(fs.Output(), "  gren step squash -m \"feat: feature\"    # Custom message\n")
		fmt.Fprintf(fs.Output(), "  gren step squash --llm                 # Use LLM for message\n")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	target := ""
	if fs.NArg() > 0 {
		target = fs.Arg(0)
	}

	opts := core.StepSquashOptions{
		Target:  target,
		Message: *message,
		UseLLM:  *useLLM,
	}

	if err := c.worktreeManager.StepSquash(opts); err != nil {
		return err
	}

	output.Success("Commits squashed")
	return nil
}

func (c *CLI) handleStepPush(args []string) error {
	fs := flag.NewFlagSet("step push", flag.ExitOnError)

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: gren step push [target]\n")
		fmt.Fprintf(fs.Output(), "\nPush current branch to local target branch (fast-forward)\n\n")
		fmt.Fprintf(fs.Output(), "This command fast-forwards the target branch to include your current commits.\n")
		fmt.Fprintf(fs.Output(), "Use this before switching to the target branch and pushing to remote.\n\n")
		fmt.Fprintf(fs.Output(), "Arguments:\n")
		fmt.Fprintf(fs.Output(), "  target    Target branch (default: main/master)\n\n")
		fmt.Fprintf(fs.Output(), "Examples:\n")
		fmt.Fprintf(fs.Output(), "  gren step push         # Push to default branch\n")
		fmt.Fprintf(fs.Output(), "  gren step push main    # Push to main\n")
		fmt.Fprintf(fs.Output(), "  gren step push develop # Push to develop\n")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	target := ""
	if fs.NArg() > 0 {
		target = fs.Arg(0)
	}

	if err := c.worktreeManager.StepPush(target); err != nil {
		return err
	}

	output.Success("Pushed to local target branch")
	return nil
}

func (c *CLI) handleStepRebase(args []string) error {
	fs := flag.NewFlagSet("step rebase", flag.ExitOnError)

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: gren step rebase [target]\n")
		fmt.Fprintf(fs.Output(), "\nRebase current branch onto target branch\n\n")
		fmt.Fprintf(fs.Output(), "Arguments:\n")
		fmt.Fprintf(fs.Output(), "  target    Target branch (default: main/master)\n\n")
		fmt.Fprintf(fs.Output(), "Examples:\n")
		fmt.Fprintf(fs.Output(), "  gren step rebase         # Rebase onto default branch\n")
		fmt.Fprintf(fs.Output(), "  gren step rebase main    # Rebase onto main\n")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	target := ""
	if fs.NArg() > 0 {
		target = fs.Arg(0)
	}

	if err := c.worktreeManager.StepRebase(target); err != nil {
		return err
	}

	output.Success("Rebased onto target branch")
	return nil
}

// Example user configuration file content
const userConfigExample = `# gren global user configuration
# See https://github.com/langtind/gren for documentation

# Default settings for all projects
[defaults]
# Template for worktree directory path
# Available variables: {{ repo }}, {{ branch }}, {{ branch | sanitize }}
worktree-dir = "../{{ repo }}-worktrees"

# Merge behavior defaults
remove-after-merge = true
squash-on-merge = true
rebase-on-merge = true

# LLM configuration for generating commit messages
# Requires an LLM tool like 'claude' or 'llm' CLI
[commit-generation]
# Examples:
#   command = "claude"
#   args = ["-p", "Write a conventional commit message for this diff. Output only the commit message, no explanation."]
#
#   command = "llm"
#   args = ["-m", "gpt-4", "Write a conventional commit message for this diff"]
command = "claude"
args = ["-p", "Write a conventional commit message for this diff. Output only the commit message, no explanation."]

# Global hooks (run for all projects)
[hooks]
# post-create = "echo 'Worktree created!'"
# post-switch = "direnv allow 2>/dev/null || true"

# Named hooks with optional branch filtering
# [[named-hooks.post-create]]
# name = "install-deps"
# command = "npm install"
# branches = ["feature/*", "fix/*"]
`

// Example project configuration file content
const projectConfigExample = `# gren project configuration
# See https://github.com/langtind/gren for documentation

version = "1.0.0"

# Worktree directory (absolute or relative to repository root)
worktree_dir = "../{{ repo }}-worktrees"

# Package manager: auto, npm, yarn, pnpm, bun
package_manager = "auto"

# Lifecycle hooks
[hooks]
# Run after creating a worktree
post-create = ".gren/post-create.sh"

# Run before removing a worktree (fail-fast: non-zero exit stops deletion)
# pre-remove = "echo 'About to remove worktree'"

# Run before merge (fail-fast: non-zero exit stops merge)
# pre-merge = "npm test"

# Run after successful merge
# post-merge = "echo 'Merge complete'"

# Named hooks with more control
# [[named-hooks.post-create]]
# name = "install-deps"
# command = "npm install"
# branches = ["feature/*"]
`

func (c *CLI) handleConfig(args []string) error {
	if len(args) == 0 {
		return c.handleConfigShow(args)
	}

	subcommand := args[0]
	subargs := args[1:]

	switch subcommand {
	case "create":
		return c.handleConfigCreate(subargs)
	case "show":
		return c.handleConfigShow(subargs)
	case "approvals":
		return c.handleConfigApprovals(subargs)
	case "--help", "-h", "help":
		c.showConfigHelp()
		return nil
	default:
		return fmt.Errorf("unknown config subcommand: %s (use: create, show, approvals)", subcommand)
	}
}

func (c *CLI) showConfigHelp() {
	fmt.Println("Usage: gren config <subcommand>")
	fmt.Println()
	fmt.Println("Manage gren configuration")
	fmt.Println()
	fmt.Println("Subcommands:")
	fmt.Println("  create     Create a configuration file with example values")
	fmt.Println("  show       Show current configuration status (default)")
	fmt.Println("  approvals  View or revoke approved hook commands")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  gren config                    # Show current config")
	fmt.Println("  gren config show               # Same as above")
	fmt.Println("  gren config create             # Create user config")
	fmt.Println("  gren config create --project   # Create project config")
	fmt.Println("  gren config approvals          # List approved hooks")
	fmt.Println("  gren config approvals --revoke # Revoke all approvals")
	fmt.Println()
	fmt.Println("Use 'gren config <subcommand> --help' for more information.")
}

func (c *CLI) handleConfigCreate(args []string) error {
	fs := flag.NewFlagSet("config create", flag.ExitOnError)
	project := fs.Bool("project", false, "Create project config instead of user config")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: gren config create [--project]\n")
		fmt.Fprintf(fs.Output(), "\nCreate a configuration file with example values\n\n")
		fmt.Fprintf(fs.Output(), "Without --project: Creates global user config at:\n")
		fmt.Fprintf(fs.Output(), "  macOS:   ~/Library/Application Support/gren/config.toml\n")
		fmt.Fprintf(fs.Output(), "  Linux:   ~/.config/gren/config.toml\n")
		fmt.Fprintf(fs.Output(), "  Windows: %%APPDATA%%\\gren\\config.toml\n\n")
		fmt.Fprintf(fs.Output(), "With --project: Creates project config at .gren/config.toml\n\n")
		fmt.Fprintf(fs.Output(), "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(fs.Output(), "\nExamples:\n")
		fmt.Fprintf(fs.Output(), "  gren config create            # Create user config\n")
		fmt.Fprintf(fs.Output(), "  gren config create --project  # Create project config\n")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *project {
		return c.createProjectConfig()
	}
	return c.createUserConfig()
}

func (c *CLI) createUserConfig() error {
	ucm := config.NewUserConfigManager()
	configPath := ucm.ConfigPath()

	// Check if file already exists
	if ucm.Exists() {
		fmt.Printf("ℹ️  User config already exists: %s\n\n", configPath)
		fmt.Println("💡 To view config, run: gren config show")
		return nil
	}

	// Create parent directory
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write the example config with values commented out
	commentedConfig := commentOutConfig(userConfigExample)
	if err := os.WriteFile(configPath, []byte(commentedConfig), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("✅ Created user config: %s\n\n", configPath)
	fmt.Println("💡 Edit this file to customize LLM settings and defaults")
	return nil
}

func (c *CLI) createProjectConfig() error {
	configPath := filepath.Join(config.ConfigDir, config.ConfigFileTOML)

	// Check if file already exists
	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("ℹ️  Project config already exists: %s\n\n", configPath)
		fmt.Println("💡 To view config, run: gren config show")
		return nil
	}

	// Create .gren directory
	if err := os.MkdirAll(config.ConfigDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write the example config with values commented out
	commentedConfig := commentOutConfig(projectConfigExample)
	if err := os.WriteFile(configPath, []byte(commentedConfig), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("✅ Created project config: %s\n\n", configPath)
	fmt.Println("💡 Edit this file to configure hooks for this repository")
	return nil
}

// commentOutConfig comments out all non-comment, non-empty lines
func commentOutConfig(content string) string {
	lines := strings.Split(content, "\n")
	result := make([]string, len(lines))

	for i, line := range lines {
		// Comment out non-empty lines that aren't already comments
		if line != "" && !strings.HasPrefix(line, "#") {
			result[i] = "# " + line
		} else {
			result[i] = line
		}
	}

	return strings.Join(result, "\n")
}

func (c *CLI) handleConfigShow(args []string) error {
	fs := flag.NewFlagSet("config show", flag.ExitOnError)

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: gren config show\n")
		fmt.Fprintf(fs.Output(), "\nShow current configuration status\n")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Show user config
	c.showUserConfig()
	fmt.Println()

	// Show project config
	c.showProjectConfig()
	fmt.Println()

	// Show shell integration status
	c.showShellStatus()

	return nil
}

func (c *CLI) handleConfigApprovals(args []string) error {
	fs := flag.NewFlagSet("config approvals", flag.ExitOnError)
	revoke := fs.Bool("revoke", false, "Revoke all approved commands for this project")
	all := fs.Bool("all", false, "Show/revoke approvals for all projects")

	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: gren config approvals [--revoke] [--all]\n")
		fmt.Fprintf(fs.Output(), "\nView or revoke approved hook commands\n\n")
		fmt.Fprintf(fs.Output(), "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(fs.Output(), "\nExamples:\n")
		fmt.Fprintf(fs.Output(), "  gren config approvals            # List approved hooks for this project\n")
		fmt.Fprintf(fs.Output(), "  gren config approvals --all      # List all approved hooks\n")
		fmt.Fprintf(fs.Output(), "  gren config approvals --revoke   # Revoke approvals for this project\n")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	am := config.NewApprovalManager()

	if *revoke {
		if *all {
			// TODO: Would need to add a RevokeAllProjects method
			return fmt.Errorf("--all with --revoke not yet supported")
		}

		projectID, err := config.GetProjectID()
		if err != nil {
			return fmt.Errorf("failed to get project ID: %w", err)
		}

		if err := am.RevokeAll(projectID); err != nil {
			return fmt.Errorf("failed to revoke approvals: %w", err)
		}

		output.Success("Revoked all hook approvals for this project")
		return nil
	}

	// List approvals
	if *all {
		c.showAllApprovals(am)
	} else {
		projectID, err := config.GetProjectID()
		if err != nil {
			return fmt.Errorf("failed to get project ID: %w", err)
		}
		c.showProjectApprovals(am, projectID)
	}

	return nil
}

func (c *CLI) showProjectApprovals(am *config.ApprovalManager, projectID string) {
	approved := am.ListApproved(projectID)

	fmt.Println("━━━ APPROVED HOOKS ━━━")
	fmt.Printf("📁 Project: %s\n\n", projectID)

	if len(approved) == 0 {
		fmt.Println("ℹ️  No approved hooks for this project")
		fmt.Println("💡 Hooks are approved when you confirm them during execution")
		return
	}

	for _, cmd := range approved {
		fmt.Printf("  ✅ %s\n", cmd)
	}
	fmt.Println()
	fmt.Println("💡 To revoke all: gren config approvals --revoke")
}

func (c *CLI) showAllApprovals(am *config.ApprovalManager) {
	// We need to expose all projects - let's add a method or read the file directly
	fmt.Println("━━━ ALL APPROVED HOOKS ━━━")
	fmt.Println()

	// For now, just show current project since we don't have ListAllProjects
	projectID, err := config.GetProjectID()
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}

	approved := am.ListApproved(projectID)
	if len(approved) == 0 {
		fmt.Println("ℹ️  No approved hooks found")
		return
	}

	fmt.Printf("📁 %s\n", projectID)
	for _, cmd := range approved {
		fmt.Printf("  ✅ %s\n", cmd)
	}
	fmt.Println()
	fmt.Println("💡 To revoke: gren config approvals --revoke")
}

func (c *CLI) showUserConfig() {
	ucm := config.NewUserConfigManager()
	configPath := ucm.ConfigPath()

	fmt.Println("━━━ USER CONFIG ━━━")
	fmt.Printf("📁 %s\n\n", configPath)

	if !ucm.Exists() {
		fmt.Println("ℹ️  Not found (using defaults)")
		fmt.Println("💡 To create one, run: gren config create")
		return
	}

	// Read and display the file
	content, err := os.ReadFile(configPath)
	if err != nil {
		fmt.Printf("❌ Error reading config: %v\n", err)
		return
	}

	if strings.TrimSpace(string(content)) == "" {
		fmt.Println("ℹ️  Empty file (using defaults)")
		return
	}

	// Show the config content
	fmt.Println(string(content))

	// Validate and show status
	userConfig, err := ucm.Load()
	if err != nil {
		fmt.Printf("⚠️  Parse error: %v\n", err)
		return
	}

	// Show LLM status
	if userConfig.CommitGenerator.Command != "" {
		fmt.Printf("✅ LLM configured: %s\n", userConfig.CommitGenerator.Command)
	}
}

func (c *CLI) showProjectConfig() {
	fmt.Println("━━━ PROJECT CONFIG ━━━")

	// Check if we're in a git repo
	ctx := context.Background()
	_, err := c.gitRepo.GetRepoInfo(ctx)
	if err != nil {
		fmt.Println("ℹ️  Not in a git repository")
		return
	}

	configPath := filepath.Join(config.ConfigDir, config.ConfigFileTOML)
	jsonPath := filepath.Join(config.ConfigDir, config.ConfigFileJSON)

	// Check which config exists
	var usedPath string
	if _, err := os.Stat(configPath); err == nil {
		usedPath = configPath
	} else if _, err := os.Stat(jsonPath); err == nil {
		usedPath = jsonPath
	}

	if usedPath == "" {
		fmt.Println("ℹ️  Not found")
		fmt.Println("💡 To create one, run: gren config create --project")
		fmt.Println("   Or use: gren init (interactive TUI setup)")
		return
	}

	fmt.Printf("📁 %s\n\n", usedPath)

	// Read and display the file
	content, err := os.ReadFile(usedPath)
	if err != nil {
		fmt.Printf("❌ Error reading config: %v\n", err)
		return
	}

	if strings.TrimSpace(string(content)) == "" {
		fmt.Println("ℹ️  Empty file")
		return
	}

	fmt.Println(string(content))

	// Validate
	cfg, err := c.configManager.Load()
	if err != nil {
		fmt.Printf("⚠️  Parse error: %v\n", err)
		return
	}

	// Show key settings
	fmt.Printf("📂 Worktree dir: %s\n", cfg.WorktreeDir)
	if cfg.PackageManager != "" && cfg.PackageManager != "auto" {
		fmt.Printf("📦 Package manager: %s\n", cfg.PackageManager)
	}
	if cfg.Hooks.PostCreate != "" {
		fmt.Printf("🪝 Post-create hook: %s\n", cfg.Hooks.PostCreate)
	}
}

func (c *CLI) showShellStatus() {
	fmt.Println("━━━ SHELL INTEGRATION ━━━")

	// Check if shell integration is active
	if os.Getenv("GREN_DIRECTIVE_FILE") != "" {
		fmt.Println("✅ Shell integration active")
		return
	}

	fmt.Println("ℹ️  Shell integration not detected")
	fmt.Println("💡 To enable navigation features, add to your shell config:")
	fmt.Println("   eval \"$(gren shell-init zsh)\"   # for zsh")
	fmt.Println("   eval \"$(gren shell-init bash)\"  # for bash")
}

// handleHookRun runs hooks with terminal access (used by TUI for interactive hooks).
// This is an internal command called by the TUI when it suspends to run interactive hooks.
func (c *CLI) handleHookRun(args []string) error {
	fs := flag.NewFlagSet("hook-run", flag.ExitOnError)
	hookType := fs.String("type", "", "Hook type (post-create, pre-remove, etc.)")
	worktreePath := fs.String("path", "", "Path to the worktree")
	branchName := fs.String("branch", "", "Branch name")
	baseBranch := fs.String("base", "", "Base branch")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *hookType == "" {
		return fmt.Errorf("hook type is required")
	}

	ht := config.HookType(*hookType)

	switch ht {
	case config.HookPostCreate:
		results := c.worktreeManager.RunPostCreateHookWithApproval(*worktreePath, *branchName, *baseBranch, true)
		if core.HooksFailed(results) {
			if failed := core.FirstFailedHook(results); failed != nil {
				return fmt.Errorf("hook failed: %v", failed.Err)
			}
		}
	case config.HookPreRemove:
		results := c.worktreeManager.RunPreRemoveHookWithApproval(*worktreePath, *branchName, true)
		if core.HooksFailed(results) {
			if failed := core.FirstFailedHook(results); failed != nil {
				return fmt.Errorf("hook failed: %v", failed.Err)
			}
		}
	case config.HookPreMerge:
		results := c.worktreeManager.RunPreMergeHookWithApproval(*worktreePath, *branchName, *baseBranch, true)
		if core.HooksFailed(results) {
			if failed := core.FirstFailedHook(results); failed != nil {
				return fmt.Errorf("hook failed: %v", failed.Err)
			}
		}
	case config.HookPostMerge:
		results := c.worktreeManager.RunPostMergeHookWithApproval(*worktreePath, *branchName, *baseBranch, true)
		if core.HooksFailed(results) {
			if failed := core.FirstFailedHook(results); failed != nil {
				return fmt.Errorf("hook failed: %v", failed.Err)
			}
		}
	case config.HookPostSwitch:
		results := c.worktreeManager.RunPostSwitchHookWithApproval(*worktreePath, *branchName, true)
		if core.HooksFailed(results) {
			if failed := core.FirstFailedHook(results); failed != nil {
				return fmt.Errorf("hook failed: %v", failed.Err)
			}
		}
	case config.HookPostStart:
		results := c.worktreeManager.RunPostStartHookWithApproval(*worktreePath, *branchName, "", true)
		if core.HooksFailed(results) {
			if failed := core.FirstFailedHook(results); failed != nil {
				return fmt.Errorf("hook failed: %v", failed.Err)
			}
		}
	default:
		return fmt.Errorf("unknown hook type: %s", *hookType)
	}

	return nil
}
