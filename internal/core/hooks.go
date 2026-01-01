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

// RunHook runs a single hook by type.
func (wm *WorktreeManager) RunHook(hookType config.HookType, ctx HookContext) HookResult {
	cfg, err := wm.configManager.Load()
	if err != nil {
		logging.Debug("RunHook: failed to load config: %v", err)
		return HookResult{Ran: false}
	}

	hookCmd := cfg.Hooks.Get(hookType)
	if hookCmd == "" {
		logging.Debug("RunHook: no %s hook configured", hookType)
		return HookResult{Ran: false}
	}

	return wm.executeHook(hookType, hookCmd, ctx, "")
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
		result := wm.executeHook(hookType, hook.Command, ctx, hook.Name)
		results = append(results, result)

		// For fail-fast hooks (pre-remove, pre-merge), stop on first failure
		if result.Err != nil && (hookType == config.HookPreRemove || hookType == config.HookPreMerge) {
			break
		}
	}

	return results
}

// executeHook runs a single hook command.
func (wm *WorktreeManager) executeHook(hookType config.HookType, hookCmd string, ctx HookContext, hookName string) HookResult {
	logging.Info("Running %s hook: %s", hookType, hookCmd)

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

	// Send JSON context to stdin
	cmd.Stdin = strings.NewReader(string(jsonData))

	output, err := cmd.CombinedOutput()
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

func (wm *WorktreeManager) RunPreRemoveHook(worktreePath, branchName string) HookResult {
	repoRoot, _ := wm.getRepoRoot()
	ctx := HookContext{
		WorktreePath: worktreePath,
		BranchName:   branchName,
		RepoRoot:     repoRoot,
	}
	return wm.RunHook(config.HookPreRemove, ctx)
}

func (wm *WorktreeManager) RunPreMergeHook(worktreePath, branchName, targetBranch string) HookResult {
	repoRoot, _ := wm.getRepoRoot()
	ctx := HookContext{
		WorktreePath: worktreePath,
		BranchName:   branchName,
		BaseBranch:   targetBranch,
		TargetBranch: targetBranch,
		RepoRoot:     repoRoot,
	}
	return wm.RunHook(config.HookPreMerge, ctx)
}

func (wm *WorktreeManager) RunPostMergeHook(worktreePath, branchName, targetBranch string) HookResult {
	repoRoot, _ := wm.getRepoRoot()
	ctx := HookContext{
		WorktreePath: worktreePath,
		BranchName:   branchName,
		BaseBranch:   targetBranch,
		TargetBranch: targetBranch,
		RepoRoot:     repoRoot,
	}
	return wm.RunHook(config.HookPostMerge, ctx)
}

// RunPostSwitchHook runs the post-switch hook after navigating to a worktree.
func (wm *WorktreeManager) RunPostSwitchHook(worktreePath, branchName string) HookResult {
	repoRoot, _ := wm.getRepoRoot()
	ctx := HookContext{
		WorktreePath: worktreePath,
		BranchName:   branchName,
		RepoRoot:     repoRoot,
	}
	return wm.RunHook(config.HookPostSwitch, ctx)
}

// RunPostStartHook runs the post-start hook after starting an execute command.
func (wm *WorktreeManager) RunPostStartHook(worktreePath, branchName, executeCmd string) HookResult {
	repoRoot, _ := wm.getRepoRoot()
	ctx := HookContext{
		WorktreePath: worktreePath,
		BranchName:   branchName,
		RepoRoot:     repoRoot,
		ExecuteCmd:   executeCmd,
	}
	return wm.RunHook(config.HookPostStart, ctx)
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
