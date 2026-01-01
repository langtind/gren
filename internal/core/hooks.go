package core

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/langtind/gren/internal/config"
	"github.com/langtind/gren/internal/logging"
)

// HookContext contains all information passed to hooks.
type HookContext struct {
	WorktreePath string
	BranchName   string
	BaseBranch   string
	RepoRoot     string
	TargetBranch string // For merge hooks
	ExecuteCmd   string // For post-start hook
}

// HookJSONContext is the JSON structure sent to hooks via stdin.
type HookJSONContext struct {
	HookType      string `json:"hook_type"`
	Branch        string `json:"branch"`
	Worktree      string `json:"worktree"`
	WorktreeName  string `json:"worktree_name"`
	Repo          string `json:"repo"`
	RepoRoot      string `json:"repo_root"`
	Commit        string `json:"commit"`
	ShortCommit   string `json:"short_commit"`
	DefaultBranch string `json:"default_branch"`
	TargetBranch  string `json:"target_branch,omitempty"`
	BaseBranch    string `json:"base_branch,omitempty"`
	ExecuteCmd    string `json:"execute_cmd,omitempty"`
}

// HookResult contains the result of running a hook.
type HookResult struct {
	Ran     bool
	Output  string
	Err     error
	Command string
	Name    string // Hook name for named hooks
}

// HookRunner handles hook execution with approval checking.
type HookRunner struct {
	configManager   *config.Manager
	approvalManager *config.ApprovalManager
	userConfig      *config.UserConfig
	projectID       string
}

// NewHookRunner creates a new hook runner.
func NewHookRunner(configManager *config.Manager) *HookRunner {
	projectID, _ := config.GetProjectID()
	userConfigManager := config.NewUserConfigManager()
	userConfig, _ := userConfigManager.Load()

	return &HookRunner{
		configManager:   configManager,
		approvalManager: config.NewApprovalManager(),
		userConfig:      userConfig,
		projectID:       projectID,
	}
}

// runHook runs a single simple hook by type (internal use only).
// For hooks with approval checking, use RunHooksWithApproval instead.
func (wm *WorktreeManager) runHook(hookType config.HookType, ctx HookContext) HookResult {
	cfg, err := wm.configManager.Load()
	if err != nil {
		logging.Debug("runHook: failed to load config: %v", err)
		return HookResult{Ran: false}
	}

	hookCmd := cfg.Hooks.Get(hookType)
	if hookCmd == "" {
		logging.Debug("runHook: no %s hook configured", hookType)
		return HookResult{Ran: false}
	}

	return wm.executeHook(hookType, hookCmd, ctx, "", false)
}

// RunHooksWithApproval runs all hooks of a type, with approval checking.
func (wm *WorktreeManager) RunHooksWithApproval(hookType config.HookType, ctx HookContext, autoYes bool) []HookResult {
	cfg, err := wm.configManager.Load()
	if err != nil {
		logging.Debug("RunHooksWithApproval: failed to load config: %v", err)
		return nil
	}

	hooks := cfg.GetAllHooks(hookType)
	if len(hooks) == 0 {
		logging.Debug("RunHooksWithApproval: no %s hooks configured", hookType)
		return nil
	}

	// Get project ID for approval
	projectID, _ := config.GetProjectID()
	approvalManager := config.NewApprovalManager()

	// Collect commands that need approval
	var commandsToApprove []string
	for _, hook := range hooks {
		if hook.Disabled {
			continue
		}
		if !approvalManager.IsApproved(projectID, hook.Command) {
			commandsToApprove = append(commandsToApprove, hook.Command)
		}
	}

	// Request approval if needed
	if len(commandsToApprove) > 0 && !autoYes {
		if !requestApproval(commandsToApprove, projectID, approvalManager) {
			logging.Info("RunHooksWithApproval: user declined hook approval")
			return nil
		}
	} else if len(commandsToApprove) > 0 && autoYes {
		// Auto-approve with -y flag
		approvalManager.ApproveAll(projectID, commandsToApprove)
	}

	// Run hooks
	var results []HookResult
	for _, hook := range hooks {
		if hook.Disabled {
			continue
		}
		result := wm.executeHook(hookType, hook.Command, ctx, hook.Name, hook.Interactive)
		results = append(results, result)

		// For fail-fast hooks (pre-remove, pre-merge), stop on first failure
		if result.Err != nil && (hookType == config.HookPreRemove || hookType == config.HookPreMerge) {
			break
		}
	}

	return results
}

// executeHook runs a single hook command.
// If interactive is true, the hook runs with terminal access for user input.
func (wm *WorktreeManager) executeHook(hookType config.HookType, hookCmd string, ctx HookContext, hookName string, interactive bool) HookResult {
	logging.Info("Running %s hook: %s (interactive: %v)", hookType, hookCmd, interactive)

	var cmd *exec.Cmd
	var cmdDesc string

	if isExecutableScript(hookCmd, ctx.RepoRoot) {
		fullPath := resolveHookPath(hookCmd, ctx.RepoRoot)
		cmd = exec.Command(fullPath, ctx.WorktreePath, ctx.BranchName, ctx.BaseBranch, ctx.RepoRoot)
		cmdDesc = fmt.Sprintf("%s %s %s %s %s", fullPath, ctx.WorktreePath, ctx.BranchName, ctx.BaseBranch, ctx.RepoRoot)
	} else {
		cmd = exec.Command("sh", "-c", hookCmd)
		cmdDesc = hookCmd
	}

	cmd.Dir = ctx.WorktreePath

	// Build JSON context
	jsonCtx := buildJSONContext(hookType, ctx, wm)
	jsonData, err := json.Marshal(jsonCtx)
	if err != nil {
		logging.Warn("Failed to build JSON context: %v", err)
		jsonData = []byte("{}")
	}

	// Set environment variables
	cmd.Env = append(os.Environ(),
		"GREN_WORKTREE_PATH="+ctx.WorktreePath,
		"GREN_BRANCH="+ctx.BranchName,
		"GREN_BASE_BRANCH="+ctx.BaseBranch,
		"GREN_REPO_ROOT="+ctx.RepoRoot,
		"GREN_HOOK_TYPE="+string(hookType),
		"GREN_TARGET_BRANCH="+ctx.TargetBranch,
		"GREN_EXECUTE_CMD="+ctx.ExecuteCmd,
		"GREN_JSON_CONTEXT="+string(jsonData),
	)

	var output []byte
	if interactive {
		// Interactive mode: connect to terminal for user input
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
	} else {
		// Non-interactive mode: send JSON to stdin, capture output
		cmd.Stdin = strings.NewReader(string(jsonData))
		output, err = cmd.CombinedOutput()
	}

	result := HookResult{
		Ran:     true,
		Output:  string(output),
		Command: cmdDesc,
		Name:    hookName,
		Err:     err,
	}

	if err != nil {
		logging.Error("%s hook failed: %v, output: %s", hookType, err, result.Output)
	} else {
		logging.Info("%s hook completed successfully", hookType)
		logging.Debug("%s hook output: %s", hookType, result.Output)
	}

	return result
}

// buildJSONContext creates the JSON context for hooks.
func buildJSONContext(hookType config.HookType, ctx HookContext, wm *WorktreeManager) HookJSONContext {
	// Get commit info
	commit := ""
	shortCommit := ""
	if ctx.WorktreePath != "" {
		cmd := exec.Command("git", "rev-parse", "HEAD")
		cmd.Dir = ctx.WorktreePath
		if output, err := cmd.Output(); err == nil {
			commit = strings.TrimSpace(string(output))
			if len(commit) > 7 {
				shortCommit = commit[:7]
			}
		}
	}

	// Get default branch
	defaultBranch, _ := wm.getDefaultBranch()

	// Get repo name
	repoName := filepath.Base(ctx.RepoRoot)

	return HookJSONContext{
		HookType:      string(hookType),
		Branch:        ctx.BranchName,
		Worktree:      ctx.WorktreePath,
		WorktreeName:  filepath.Base(ctx.WorktreePath),
		Repo:          repoName,
		RepoRoot:      ctx.RepoRoot,
		Commit:        commit,
		ShortCommit:   shortCommit,
		DefaultBranch: defaultBranch,
		TargetBranch:  ctx.TargetBranch,
		BaseBranch:    ctx.BaseBranch,
		ExecuteCmd:    ctx.ExecuteCmd,
	}
}

// requestApproval asks the user to approve commands.
func requestApproval(commands []string, projectID string, am *config.ApprovalManager) bool {
	fmt.Println("\n⚠️  The following commands need approval to run:")
	fmt.Println()
	for i, cmd := range commands {
		fmt.Printf("  %d. %s\n", i+1, cmd)
	}
	fmt.Println()
	fmt.Print("Approve these commands? [y/N/a] (y=yes once, a=always): ")

	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.ToLower(strings.TrimSpace(response))

	switch response {
	case "y", "yes":
		return true
	case "a", "always":
		am.ApproveAll(projectID, commands)
		fmt.Println("✓ Commands approved for this project")
		return true
	default:
		return false
	}
}

// RunPreRemoveHookWithApproval runs pre-remove hooks with approval checking.
func (wm *WorktreeManager) RunPreRemoveHookWithApproval(worktreePath, branchName string, autoYes bool) []HookResult {
	repoRoot, _ := wm.getRepoRoot()
	ctx := HookContext{
		WorktreePath: worktreePath,
		BranchName:   branchName,
		RepoRoot:     repoRoot,
	}
	return wm.RunHooksWithApproval(config.HookPreRemove, ctx, autoYes)
}

// RunPreMergeHookWithApproval runs pre-merge hooks with approval checking.
func (wm *WorktreeManager) RunPreMergeHookWithApproval(worktreePath, branchName, targetBranch string, autoYes bool) []HookResult {
	repoRoot, _ := wm.getRepoRoot()
	ctx := HookContext{
		WorktreePath: worktreePath,
		BranchName:   branchName,
		BaseBranch:   targetBranch,
		TargetBranch: targetBranch,
		RepoRoot:     repoRoot,
	}
	return wm.RunHooksWithApproval(config.HookPreMerge, ctx, autoYes)
}

// RunPostMergeHookWithApproval runs post-merge hooks with approval checking.
func (wm *WorktreeManager) RunPostMergeHookWithApproval(worktreePath, branchName, targetBranch string, autoYes bool) []HookResult {
	repoRoot, _ := wm.getRepoRoot()
	ctx := HookContext{
		WorktreePath: worktreePath,
		BranchName:   branchName,
		BaseBranch:   targetBranch,
		TargetBranch: targetBranch,
		RepoRoot:     repoRoot,
	}
	return wm.RunHooksWithApproval(config.HookPostMerge, ctx, autoYes)
}

// RunPostSwitchHookWithApproval runs post-switch hooks with approval checking.
func (wm *WorktreeManager) RunPostSwitchHookWithApproval(worktreePath, branchName string, autoYes bool) []HookResult {
	repoRoot, _ := wm.getRepoRoot()
	ctx := HookContext{
		WorktreePath: worktreePath,
		BranchName:   branchName,
		RepoRoot:     repoRoot,
	}
	return wm.RunHooksWithApproval(config.HookPostSwitch, ctx, autoYes)
}

// RunPostStartHookWithApproval runs post-start hooks with approval checking.
func (wm *WorktreeManager) RunPostStartHookWithApproval(worktreePath, branchName, executeCmd string, autoYes bool) []HookResult {
	repoRoot, _ := wm.getRepoRoot()
	ctx := HookContext{
		WorktreePath: worktreePath,
		BranchName:   branchName,
		RepoRoot:     repoRoot,
		ExecuteCmd:   executeCmd,
	}
	return wm.RunHooksWithApproval(config.HookPostStart, ctx, autoYes)
}

// HooksFailed checks if any hook in the results failed.
func HooksFailed(results []HookResult) bool {
	for _, r := range results {
		if r.Err != nil {
			return true
		}
	}
	return false
}

// FirstFailedHook returns the first failed hook result, or nil if all succeeded.
func FirstFailedHook(results []HookResult) *HookResult {
	for _, r := range results {
		if r.Err != nil {
			return &r
		}
	}
	return nil
}

func isExecutableScript(hookCmd, repoRoot string) bool {
	if strings.Contains(hookCmd, " ") && !strings.HasSuffix(hookCmd, ".sh") {
		return false
	}

	fullPath := resolveHookPath(hookCmd, repoRoot)
	info, err := os.Stat(fullPath)
	if err != nil {
		return false
	}

	return info.Mode()&0111 != 0
}

func resolveHookPath(hookCmd, repoRoot string) string {
	if filepath.IsAbs(hookCmd) {
		return hookCmd
	}
	return filepath.Join(repoRoot, hookCmd)
}

// GetUnapprovedHooks returns a list of hook commands that need approval for the given hook type.
func (wm *WorktreeManager) GetUnapprovedHooks(hookType config.HookType) []string {
	cfg, err := wm.configManager.Load()
	if err != nil {
		return nil
	}

	hooks := cfg.GetAllHooks(hookType)
	if len(hooks) == 0 {
		return nil
	}

	projectID, _ := config.GetProjectID()
	am := config.NewApprovalManager()

	var unapproved []string
	for _, hook := range hooks {
		if hook.Disabled {
			continue
		}
		if !am.IsApproved(projectID, hook.Command) {
			unapproved = append(unapproved, hook.Command)
		}
	}

	return unapproved
}

// HasInteractiveHooks returns true if any hook of the given type is marked as interactive.
func (wm *WorktreeManager) HasInteractiveHooks(hookType config.HookType) bool {
	cfg, err := wm.configManager.Load()
	if err != nil {
		return false
	}

	hooks := cfg.GetAllHooks(hookType)
	for _, hook := range hooks {
		if !hook.Disabled && hook.Interactive {
			return true
		}
	}

	return false
}

// RunPostCreateHookWithApproval runs the post-create hook with approval checking.
// Returns the hook results. If autoYes is true, hooks are auto-approved.
func (wm *WorktreeManager) RunPostCreateHookWithApproval(worktreePath, branchName, baseBranch string, autoYes bool) []HookResult {
	repoRoot, _ := wm.getRepoRoot()

	// Ensure absolute path
	absWorktreePath := worktreePath
	if !filepath.IsAbs(worktreePath) {
		absWorktreePath = filepath.Join(repoRoot, worktreePath)
	}

	ctx := HookContext{
		WorktreePath: absWorktreePath,
		BranchName:   branchName,
		BaseBranch:   baseBranch,
		RepoRoot:     repoRoot,
	}

	return wm.RunHooksWithApproval(config.HookPostCreate, ctx, autoYes)
}
