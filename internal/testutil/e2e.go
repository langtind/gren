package testutil

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// E2EHarness provides a test harness for end-to-end testing of the gren CLI.
// It builds the binary once and runs commands against real git repositories.
type E2EHarness struct {
	t          *testing.T
	binaryPath string
	repoPath   string
	cleanup    func()
	env        []string

	// WorktreeDir is the directory where worktrees are created
	WorktreeDir string
}

// CommandResult holds the result of running a gren command.
type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Err      error
}

// NewE2EHarness creates a new E2E test harness.
// It sets up a temporary git repository with gren initialized.
func NewE2EHarness(t *testing.T) *E2EHarness {
	t.Helper()

	// Build the gren binary
	binaryPath := buildGrenBinary(t)

	// Create a temp git repo with initial commit
	repoPath, repoCleanup := CreateTempRepoWithCommit(t)

	// Calculate worktree directory
	worktreeDir := filepath.Join(filepath.Dir(repoPath), filepath.Base(repoPath)+"-worktrees")

	cleanup := func() {
		repoCleanup()
		os.RemoveAll(worktreeDir)
		// Note: binary is built to a temp dir and cleaned up automatically
	}

	return &E2EHarness{
		t:           t,
		binaryPath:  binaryPath,
		repoPath:    repoPath,
		cleanup:     cleanup,
		env:         os.Environ(),
		WorktreeDir: worktreeDir,
	}
}

// Cleanup removes all test artifacts.
func (h *E2EHarness) Cleanup() {
	h.cleanup()
}

// RepoPath returns the path to the test repository.
func (h *E2EHarness) RepoPath() string {
	return h.repoPath
}

// SetEnv adds or updates an environment variable for subsequent commands.
func (h *E2EHarness) SetEnv(key, value string) {
	for i, e := range h.env {
		if strings.HasPrefix(e, key+"=") {
			h.env[i] = key + "=" + value
			return
		}
	}
	h.env = append(h.env, key+"="+value)
}

// Run executes a gren command and returns the result.
func (h *E2EHarness) Run(args ...string) *CommandResult {
	h.t.Helper()

	cmd := exec.Command(h.binaryPath, args...)
	cmd.Dir = h.repoPath
	cmd.Env = h.env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	return &CommandResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
		Err:      err,
	}
}

// RunInDir executes a gren command in a specific directory.
func (h *E2EHarness) RunInDir(dir string, args ...string) *CommandResult {
	h.t.Helper()

	cmd := exec.Command(h.binaryPath, args...)
	cmd.Dir = dir
	cmd.Env = h.env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	return &CommandResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
		Err:      err,
	}
}

// Init initializes gren in the test repository.
func (h *E2EHarness) Init() *CommandResult {
	return h.Run("init")
}

// InitWithProject initializes gren with a custom project name.
func (h *E2EHarness) InitWithProject(projectName string) *CommandResult {
	return h.Run("init", "-project", projectName)
}

// CreateWorktree creates a new worktree with a new branch.
func (h *E2EHarness) CreateWorktree(name string) *CommandResult {
	return h.Run("create", "-n", name)
}

// CreateWorktreeWithHooks creates a new worktree with hooks auto-approved.
func (h *E2EHarness) CreateWorktreeWithHooks(name string) *CommandResult {
	return h.Run("create", "-n", name, "-y")
}

// CreateWorktreeFromBranch creates a worktree from an existing branch.
func (h *E2EHarness) CreateWorktreeFromBranch(name, branch string) *CommandResult {
	return h.Run("create", "-n", name, "--existing", "--branch", branch)
}

// DeleteWorktree deletes a worktree.
func (h *E2EHarness) DeleteWorktree(name string, force bool) *CommandResult {
	if force {
		return h.Run("delete", "-f", name)
	}
	return h.Run("delete", name)
}

// List lists all worktrees.
func (h *E2EHarness) List() *CommandResult {
	return h.Run("list")
}

// ListVerbose lists all worktrees with verbose output.
func (h *E2EHarness) ListVerbose() *CommandResult {
	return h.Run("list", "-v")
}

// Navigate navigates to a worktree.
func (h *E2EHarness) Navigate(name string) *CommandResult {
	return h.Run("navigate", name)
}

// Compare compares a worktree with the main worktree.
func (h *E2EHarness) Compare(name string) *CommandResult {
	return h.Run("compare", name)
}

// CompareWithDiff shows a diff of changes in a worktree.
func (h *E2EHarness) CompareWithDiff(name string) *CommandResult {
	return h.Run("compare", "--diff", name)
}

// CompareWithApply applies changes from a worktree to the main worktree.
func (h *E2EHarness) CompareWithApply(name string) *CommandResult {
	return h.Run("compare", "--apply", name)
}

// Git runs a git command in the test repository.
func (h *E2EHarness) Git(args ...string) *CommandResult {
	h.t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = h.repoPath

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	return &CommandResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
		Err:      err,
	}
}

// GitInDir runs a git command in a specific directory.
func (h *E2EHarness) GitInDir(dir string, args ...string) *CommandResult {
	h.t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	return &CommandResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
		Err:      err,
	}
}

// CreateBranch creates a new branch in the test repository.
func (h *E2EHarness) CreateBranch(name string) *CommandResult {
	return h.Git("branch", name)
}

// WriteFile writes content to a file in the test repository.
func (h *E2EHarness) WriteFile(relativePath, content string) error {
	h.t.Helper()
	fullPath := filepath.Join(h.repoPath, relativePath)

	// Create parent directories if needed
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(fullPath, []byte(content), 0644)
}

// WriteFileInWorktree writes content to a file in a worktree.
func (h *E2EHarness) WriteFileInWorktree(worktreeName, relativePath, content string) error {
	h.t.Helper()
	worktreePath := filepath.Join(h.WorktreeDir, worktreeName)
	fullPath := filepath.Join(worktreePath, relativePath)

	// Create parent directories if needed
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(fullPath, []byte(content), 0644)
}

// ReadFile reads content from a file in the test repository.
func (h *E2EHarness) ReadFile(relativePath string) (string, error) {
	fullPath := filepath.Join(h.repoPath, relativePath)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// FileExists checks if a file exists in the test repository.
func (h *E2EHarness) FileExists(relativePath string) bool {
	fullPath := filepath.Join(h.repoPath, relativePath)
	_, err := os.Stat(fullPath)
	return err == nil
}

// DirExists checks if a directory exists.
func (h *E2EHarness) DirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// WorktreeExists checks if a worktree directory exists.
func (h *E2EHarness) WorktreeExists(name string) bool {
	worktreePath := filepath.Join(h.WorktreeDir, name)
	return h.DirExists(worktreePath)
}

// GetWorktreePath returns the full path to a worktree.
func (h *E2EHarness) GetWorktreePath(name string) string {
	return filepath.Join(h.WorktreeDir, name)
}

// Assertions

// AssertSuccess asserts that the command succeeded.
func (r *CommandResult) AssertSuccess(t *testing.T) {
	t.Helper()
	if r.Err != nil {
		t.Fatalf("expected success, got error: %v\nstdout: %s\nstderr: %s", r.Err, r.Stdout, r.Stderr)
	}
	if r.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d\nstdout: %s\nstderr: %s", r.ExitCode, r.Stdout, r.Stderr)
	}
}

// AssertFailed asserts that the command failed.
func (r *CommandResult) AssertFailed(t *testing.T) {
	t.Helper()
	if r.Err == nil && r.ExitCode == 0 {
		t.Fatalf("expected failure, but command succeeded\nstdout: %s", r.Stdout)
	}
}

// AssertExitCode asserts a specific exit code.
func (r *CommandResult) AssertExitCode(t *testing.T, expected int) {
	t.Helper()
	if r.ExitCode != expected {
		t.Fatalf("expected exit code %d, got %d\nstdout: %s\nstderr: %s", expected, r.ExitCode, r.Stdout, r.Stderr)
	}
}

// AssertStdoutContains asserts that stdout contains a substring.
func (r *CommandResult) AssertStdoutContains(t *testing.T, substr string) {
	t.Helper()
	if !strings.Contains(r.Stdout, substr) {
		t.Fatalf("expected stdout to contain %q, got:\n%s", substr, r.Stdout)
	}
}

// AssertStdoutNotContains asserts that stdout does not contain a substring.
func (r *CommandResult) AssertStdoutNotContains(t *testing.T, substr string) {
	t.Helper()
	if strings.Contains(r.Stdout, substr) {
		t.Fatalf("expected stdout to NOT contain %q, got:\n%s", substr, r.Stdout)
	}
}

// AssertStderrContains asserts that stderr contains a substring.
func (r *CommandResult) AssertStderrContains(t *testing.T, substr string) {
	t.Helper()
	if !strings.Contains(r.Stderr, substr) {
		t.Fatalf("expected stderr to contain %q, got:\n%s", substr, r.Stderr)
	}
}

// buildGrenBinary builds the gren binary for testing.
// The binary is cached per test run.
var cachedBinaryPath string

func buildGrenBinary(t *testing.T) string {
	t.Helper()

	if cachedBinaryPath != "" {
		// Verify it still exists
		if _, err := os.Stat(cachedBinaryPath); err == nil {
			return cachedBinaryPath
		}
	}

	// Find the project root (where go.mod is)
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	projectRoot := findProjectRoot(wd)
	if projectRoot == "" {
		t.Fatalf("could not find project root (go.mod)")
	}

	// Create temp directory for binary
	tmpDir, err := os.MkdirTemp("", "gren-e2e-bin-*")
	if err != nil {
		t.Fatalf("failed to create temp dir for binary: %v", err)
	}

	binaryPath := filepath.Join(tmpDir, "gren")

	// Build the binary
	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to build gren binary: %v\noutput: %s", err, output)
	}

	cachedBinaryPath = binaryPath
	return binaryPath
}

func findProjectRoot(start string) string {
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
