package core

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/langtind/gren/internal/config"
	"github.com/langtind/gren/internal/events"
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
	Ran        bool
	Output     string // stdout captured in non-interactive mode (empty in interactive mode)
	Stderr     string // stderr captured in non-interactive mode (empty in interactive mode)
	Err        error
	Command    string
	Name       string         // Hook name for named hooks
	Events     []events.Event // structured phase events emitted by hook via $GREN_EVENTS_FILE
	EventsFile string         // absolute path to NDJSON events file for post-mortem
}

// FailureOutput returns a labeled combination of stderr and stdout for error
// reporting. stderr comes first because runtime traces (e.g. bash
// `bad substitution`, non-zero exit messages) land there — callers need to
// surface *where* the hook broke, not just normal progress. Empty sections
// are omitted so short error messages aren't padded with blank labels.
func (r HookResult) FailureOutput() string {
	var parts []string
	if s := strings.TrimRight(r.Stderr, "\n"); s != "" {
		parts = append(parts, "stderr:\n"+s)
	}
	if s := strings.TrimRight(r.Output, "\n"); s != "" {
		parts = append(parts, "stdout:\n"+s)
	}
	return strings.Join(parts, "\n")
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

	// User-level named hooks (best-effort) run for all repos, before project
	// hooks. CollectHooks also honors each hook's optional branch globs and
	// skips disabled hooks — so branch-filtered and global hooks now work.
	userCfg, _ := config.NewUserConfigManager().Load()
	hooks := config.CollectHooks(cfg, userCfg, hookType, ctx.BranchName)
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

		// For fail-fast hooks (pre-create, pre-remove, pre-merge),
		// stop on first failure — the caller treats Err on any of these
		// as a signal to abort the lifecycle operation entirely.
		if result.Err != nil && (hookType == config.HookPreCreate || hookType == config.HookPreRemove || hookType == config.HookPreMerge) {
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
		// Expand template variables (e.g. {{ branch | hash_port }}) in inline
		// commands so hooks can derive per-worktree ports/DB names. Values are
		// shell-quoted so a branch name with shell metacharacters can't inject
		// commands. Script-file hooks receive context via args + env, unchanged.
		expandedCmd := expandTemplateShellQuoted(hookCmd, wm.templateContextFromHook(ctx))
		cmd = exec.Command("sh", "-c", expandedCmd)
		cmdDesc = expandedCmd
	}

	// Use worktree path as working directory, but fall back to repo root if it
	// no longer exists (e.g. for post-remove hooks that run after deletion).
	if _, statErr := os.Stat(ctx.WorktreePath); statErr == nil {
		cmd.Dir = ctx.WorktreePath
	} else {
		cmd.Dir = ctx.RepoRoot
	}

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

	// Create a per-run NDJSON events file so hooks can emit structured
	// progress. Additive: hooks that don't emit events are unaffected.
	eventsPath, evErr := events.NewEventsFile(string(hookType), ctx.BranchName)
	if evErr != nil {
		logging.Warn("events: failed to create file, disabling for this run: %v", evErr)
		eventsPath = ""
	} else {
		cmd.Env = append(cmd.Env, "GREN_EVENTS_FILE="+eventsPath)
		// Best-effort retention sweep on every hook spawn.
		if dir, err := events.EventsDir(); err == nil {
			_ = events.Prune(dir, 20, 7*24*time.Hour)
		}
	}

	// Start live tailer + collector in goroutines if we have a file.
	var (
		collected    []events.Event
		invalidCount int
		mu           sync.Mutex
		tailCtx      context.Context
		tailCancel   context.CancelFunc
		tailerDone   chan struct{}
		consumerDone chan struct{}
	)
	if eventsPath != "" {
		tailCtx, tailCancel = context.WithCancel(context.Background())
		defer tailCancel() // ensure tailer exits even on unexpected paths
		evCh := make(chan events.Event, 256)
		invCh := make(chan string, 64)
		tailerDone = make(chan struct{})
		consumerDone = make(chan struct{})
		go func() {
			defer close(tailerDone)
			events.Tail(tailCtx, eventsPath, evCh, invCh)
		}()
		go func() {
			defer close(consumerDone)
			// Consume until both the tailer is done AND channels are drained.
			for {
				select {
				case ev, ok := <-evCh:
					if !ok {
						return
					}
					mu.Lock()
					collected = append(collected, ev)
					mu.Unlock()
					wm.emitEvent(ev)
				case line, ok := <-invCh:
					if !ok {
						return
					}
					invalidCount++
					logging.Warn("events: skipped invalid line: %s", line)
				case <-tailerDone:
					// Final drain — tailer closed, grab any remaining.
					for {
						select {
						case ev, ok := <-evCh:
							if !ok {
								return
							}
							mu.Lock()
							collected = append(collected, ev)
							mu.Unlock()
							wm.emitEvent(ev)
						case line, ok := <-invCh:
							if !ok {
								return
							}
							invalidCount++
							logging.Warn("events: skipped invalid line: %s", line)
						default:
							return
						}
					}
				}
			}
		}()
	}

	// Capture stdout and stderr separately in non-interactive mode so the UI
	// can distinguish a normal progress stream (stdout) from an error trace
	// (stderr) — critical when a hook exits non-zero and we need to surface
	// *where* it failed, not just that it failed.
	var stdoutBuf, stderrBuf strings.Builder
	if interactive || wm.forceInteractive.Load() {
		// Interactive mode: connect to terminal for user input
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
	} else {
		cmd.Stdin = strings.NewReader(string(jsonData))
		cmd.Stdout = &stdoutBuf
		cmd.Stderr = &stderrBuf
		err = cmd.Run()
	}

	if eventsPath != "" {
		tailCancel()
		<-tailerDone
		<-consumerDone
	}
	mu.Lock()
	eventsCopy := append([]events.Event(nil), collected...)
	mu.Unlock()
	beforeLen := len(eventsCopy)
	eventsCopy = finalizeEvents(eventsCopy, err)
	// Observer must see the synthetic interrupted event too; live UIs need it
	// to mark a still-running phase as failed when the hook exits without a
	// terminal status.
	for _, synth := range eventsCopy[beforeLen:] {
		wm.emitEvent(synth)
	}

	result := HookResult{
		Ran:        true,
		Output:     stdoutBuf.String(),
		Stderr:     stderrBuf.String(),
		Command:    cmdDesc,
		Name:       hookName,
		Err:        err,
		Events:     eventsCopy,
		EventsFile: eventsPath,
	}

	if err != nil {
		// Log stdout + stderr separately so the failure trace (typically in
		// stderr, e.g. `bad substitution`) isn't lost in a merged blob.
		logging.Error("%s hook failed: %v\nstdout: %s\nstderr: %s",
			hookType, err, result.Output, result.Stderr)
	} else {
		logging.Info("%s hook completed successfully", hookType)
		logging.Debug("%s hook output: %s", hookType, result.Output)
		if result.Stderr != "" {
			logging.Debug("%s hook stderr: %s", hookType, result.Stderr)
		}
	}

	return result
}

// finalizeEvents appends a synthetic interrupted event when the hook exited
// non-zero and a phase is still open — i.e. the most recent event for a
// phase+app pair is a start with no matching ok/error after it. Tracking
// open state chronologically (rather than a "ever closed" set) correctly
// handles re-opened phases like `start → ok → start → exit 1`.
func finalizeEvents(evs []events.Event, hookErr error) []events.Event {
	if hookErr == nil {
		return evs
	}
	type key struct{ phase, app string }
	open := map[key]int{} // key -> index of current open start event, -1 if closed
	for i, e := range evs {
		k := key{e.Phase, e.App}
		switch e.Status {
		case events.StatusStart:
			open[k] = i
		case events.StatusOK, events.StatusError:
			open[k] = -1
		}
	}
	// Find the latest open start.
	openIdx := -1
	for _, idx := range open {
		if idx > openIdx {
			openIdx = idx
		}
	}
	if openIdx < 0 {
		return evs
	}
	e := evs[openIdx]
	return append(evs, events.Event{
		TS:     time.Now().UTC(),
		Phase:  e.Phase,
		App:    e.App,
		Status: events.StatusInterrupted,
		Detail: "hook exited before phase completed",
	})
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

// templateContextFromHook builds the TemplateContext used to expand inline
// hook commands. It mirrors buildJSONContext so inline `{{ ... }}` variables
// resolve to the same values hooks receive via GREN_JSON_CONTEXT.
func (wm *WorktreeManager) templateContextFromHook(ctx HookContext) TemplateContext {
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

	defaultBranch, _ := wm.getDefaultBranch()

	return TemplateContext{
		Branch:          ctx.BranchName,
		BranchSanitized: sanitizeBranch(ctx.BranchName),
		Worktree:        ctx.WorktreePath,
		WorktreeName:    filepath.Base(ctx.WorktreePath),
		Repo:            filepath.Base(ctx.RepoRoot),
		RepoRoot:        ctx.RepoRoot,
		Commit:          commit,
		ShortCommit:     shortCommit,
		DefaultBranch:   defaultBranch,
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

// RunPostRemoveHookWithApproval runs post-remove hooks with approval checking.
// The hook runs from the repo root since the worktree directory no longer exists.
// Hook failures are best-effort and do not affect the outcome of the removal.
func (wm *WorktreeManager) RunPostRemoveHookWithApproval(worktreePath, branchName string, autoYes bool) []HookResult {
	repoRoot, _ := wm.getRepoRoot()
	ctx := HookContext{
		WorktreePath: worktreePath,
		BranchName:   branchName,
		RepoRoot:     repoRoot,
	}
	return wm.RunHooksWithApproval(config.HookPostRemove, ctx, autoYes)
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

// RunPreCreateHookWithApproval runs the pre-create hook with approval checking.
// Returns the hook results. If autoYes is true, hooks are auto-approved.
//
// Pre-create runs BEFORE the worktree directory exists, so WorktreePath in
// the context is the path that WILL be created — useful for environment
// preflight (docker up, env vars present, etc) but the script can't `cd`
// into it. Failure (Err != nil on any hook) aborts the create.
func (wm *WorktreeManager) RunPreCreateHookWithApproval(branchName, baseBranch string, autoYes bool) []HookResult {
	repoRoot, _ := wm.getRepoRoot()

	ctx := HookContext{
		// Pre-create has no worktree path yet — keep field set to repo root
		// so scripts that do `cd "$1"` still land somewhere sensible.
		WorktreePath: repoRoot,
		BranchName:   branchName,
		BaseBranch:   baseBranch,
		RepoRoot:     repoRoot,
	}

	return wm.RunHooksWithApproval(config.HookPreCreate, ctx, autoYes)
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
