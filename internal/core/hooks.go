package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/langtind/gren/internal/config"
	"github.com/langtind/gren/internal/logging"
)

type HookContext struct {
	WorktreePath string
	BranchName   string
	BaseBranch   string
	RepoRoot     string
}

type HookResult struct {
	Ran     bool
	Output  string
	Err     error
	Command string
}

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
	cmd.Env = append(os.Environ(),
		"GREN_WORKTREE_PATH="+ctx.WorktreePath,
		"GREN_BRANCH="+ctx.BranchName,
		"GREN_BASE_BRANCH="+ctx.BaseBranch,
		"GREN_REPO_ROOT="+ctx.RepoRoot,
		"GREN_HOOK_TYPE="+string(hookType),
	)

	output, err := cmd.CombinedOutput()
	result := HookResult{
		Ran:     true,
		Output:  string(output),
		Command: cmdDesc,
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
		RepoRoot:     repoRoot,
	}
	return wm.RunHook(config.HookPostMerge, ctx)
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
