package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/langtind/gren/internal/config"
	"github.com/langtind/gren/internal/git"
)

// ViewState represents the current view/screen
type ViewState int

const (
	DashboardView ViewState = iota
	CreateView
	DeleteView
	CleanupView
	InitView
	SettingsView
	OpenInView
	ConfigView
	ToolsView
	CompareView
)

// Worktree represents a git worktree
type Worktree struct {
	Name           string
	Path           string
	Branch         string
	Status         string // "clean", "modified", "building", etc.
	IsCurrent      bool   // true if this is the current worktree
	IsMain         bool   // true if this is the main worktree (where .git directory lives)
	LastCommit     string // Relative time of last commit (e.g., "2h ago")
	StagedCount    int    // Number of staged files (ready to commit)
	ModifiedCount  int    // Number of modified files (not staged)
	UntrackedCount int    // Number of untracked files
	UnpushedCount  int    // Number of unpushed commits
	HasSubmodules  bool   // true if worktree has submodules (requires --force to delete)

	// Stale detection fields
	BranchStatus string // "active", "stale", or "" if not yet checked
	StaleReason  string // "merged_locally", "no_unique_commits", "remote_gone", "pr_merged", "pr_closed"

	// GitHub PR fields (populated async, empty if gh unavailable or no PR)
	PRNumber int    // PR number, 0 if no PR
	PRState  string // "OPEN", "MERGED", "CLOSED", "DRAFT", "" if unknown
	PRURL    string // Full URL to PR for "Open in browser"
}

// InitStep represents the current step in initialization
type InitStep int

const (
	InitStepWelcome InitStep = iota
	InitStepAnalysis
	InitStepRecommendations
	InitStepGrenConfig // Ask whether to track .gren in git
	InitStepCustomization
	InitStepPreview
	InitStepCreated
	InitStepExecuting
	InitStepComplete
	InitStepCommitConfirm
	InitStepFinal
	InitStepAIGenerating
	InitStepAIResult
)

// InitState holds the state for project initialization
type InitState struct {
	currentStep        InitStep
	detectedFiles      []DetectedFile
	selected           int
	worktreeDir        string
	customizationMode  string // "", "worktree", "postcreate"
	editingText        string
	postCreateScript   string
	analysisComplete   bool   // whether project analysis is complete
	packageManager     string // detected package manager
	postCreateCmd      string // detected post-create command
	aiGeneratedScript  string // AI-generated setup script content
	aiError            string // Error message from AI generation
	trackGrenInGit     bool   // whether to track .gren/ in git or add to .gitignore
	recommendationMode int    // 0=Accept, 1=Customize, 2=AI (saved from recommendations step)
}

// DetectedFile represents a detected file that could be useful for worktrees
type DetectedFile struct {
	Path         string
	Type         string // "env", "config", "tool"
	IsGitIgnored bool
	Description  string
}

// PostCreateAction represents an action available after worktree creation
type PostCreateAction struct {
	Name        string
	Icon        string
	Command     string
	Args        []string
	Available   bool
	Description string
}

// Implement list.Item interface for PostCreateAction
func (a PostCreateAction) FilterValue() string {
	return a.Name
}

func (a PostCreateAction) Title() string {
	return fmt.Sprintf("%s %s", a.Icon, a.Name)
}

func (a PostCreateAction) Desc() string {
	return a.Description
}

// CreateMode represents the type of branch creation
type CreateMode int

const (
	CreateModeNewBranch CreateMode = iota
	CreateModeExistingBranch
)

// CreateStep represents the current step in worktree creation
type CreateStep int

const (
	CreateStepBranchMode CreateStep = iota
	CreateStepBranchName
	CreateStepExistingBranch
	CreateStepBaseBranch
	CreateStepConfirm
	CreateStepCreating
	CreateStepComplete
)

// BranchStatus represents the status of a git branch (copied from git package to avoid circular imports).
// This allows us to show branch information without depending on git internals.
type BranchStatus struct {
	Name             string
	IsClean          bool
	UncommittedFiles int
	UntrackedFiles   int
	IsCurrent        bool
	AheadCount       int
	BehindCount      int
}

// CreateState holds the state for worktree creation
type CreateState struct {
	currentStep               CreateStep
	createMode                CreateMode
	branchName                string
	baseBranch                string
	branchStatuses            []BranchStatus
	filteredBranches          []BranchStatus // Filtered list for search (base branch)
	availableBranches         []BranchStatus // For existing branch selection
	filteredAvailableBranches []BranchStatus // Filtered list for search (existing branch)
	selectedBranch            int
	scrollOffset              int    // For scrolling long branch lists
	searchQuery               string // For fzf-like filtering
	isSearching               bool   // Whether search mode is active
	selectedMode              int    // For branch mode selection
	showWarning               bool
	warningAccepted           bool
	selectedAction            int           // For the post-create actions
	actionsList               list.Model    // Dropdown menu for post-create actions
	spinner                   spinner.Model // Spinner for creating step
	createWarning             string        // Warning from worktree creation (e.g., unpushed commits)
}

// DeleteStep represents the current step in worktree deletion
type DeleteStep int

const (
	DeleteStepSelection DeleteStep = iota
	DeleteStepConfirm
	DeleteStepDeleting
	DeleteStepComplete
)

// OpenInState holds the state for the "Open in..." view
type OpenInState struct {
	worktreePath  string             // Path of the selected worktree
	actions       []PostCreateAction // Available actions
	selectedIndex int                // Currently selected action index
}

// ConfigFile represents a configuration file
type ConfigFile struct {
	Name        string // Display name
	Path        string // File path
	Icon        string // Display icon
	Description string // File description
}

// ConfigState holds the state for the config view
type ConfigState struct {
	files         []ConfigFile // Available config files
	selectedIndex int          // Currently selected file index
}

// CompareState holds the state for the compare view
type CompareState struct {
	sourceWorktree   string            // Name of the source worktree being compared
	sourcePath       string            // Path to source worktree
	files            []CompareFileItem // List of files with selection state
	selectedIndex    int               // Currently selected file index
	scrollOffset     int               // For scrolling long file lists
	selectAll        bool              // Whether all files are selected
	applyInProgress  bool              // Whether apply operation is running
	applyComplete    bool              // Whether apply operation completed
	applyError       string            // Error message from apply operation
	appliedCount     int               // Number of files successfully applied
	diffContent      string            // Diff content for currently selected file
	diffScrollOffset int               // Scroll offset for diff viewer
	diffFocused      bool              // Whether diff panel is focused (for scrolling)
}

// CompareFileItem represents a file in the compare view with selection state
type CompareFileItem struct {
	Path        string
	Status      string // "added", "modified", "deleted"
	IsCommitted bool
	Selected    bool
}

// DeleteState holds the state for worktree deletion
type DeleteState struct {
	currentStep       DeleteStep
	selectedWorktrees []int // Indices of selected worktrees for deletion
	warnings          []string
	targetWorktree    *Worktree // Specific worktree to delete (for single deletion)
	forceDelete       bool      // Use --force flag (when user confirms deletion of dirty worktree)
}

// CleanupState holds the state for bulk stale worktree cleanup with live progress
type CleanupState struct {
	staleWorktrees      []Worktree     // Worktrees to be cleaned up
	confirmed           bool           // Whether user confirmed the action
	selectedIndices     map[int]bool   // Which worktrees are selected for deletion
	selectedIndicesList []int          // Sorted list of selected indices (built when cleanup starts)
	cursorIndex         int            // Current cursor position in selection list
	forceDelete         bool           // Force delete even with uncommitted changes
	inProgress          bool           // Cleanup currently running
	currentIndex        int            // Index being deleted (-1 = none)
	deletedIndices      map[int]bool   // Successfully deleted indices
	failedWorktrees     map[int]string // Failed index → error message
	totalCleaned        int            // Success count
	totalFailed         int            // Failure count
	cleanupSpinner      spinner.Model  // Spinner for current deletion
}

// Model holds the entire application state
type Model struct {
	// Current view state
	currentView ViewState
	// Dependencies
	gitRepo       git.Repository
	configManager *config.Manager
	// State
	repoInfo     *git.RepoInfo
	config       *config.Config
	worktrees    []Worktree
	selected     int
	err          error
	initState    *InitState
	createState  *CreateState
	deleteState  *DeleteState
	cleanupState *CleanupState
	openInState  *OpenInState
	configState  *ConfigState
	compareState *CompareState

	// Screen dimensions
	width  int
	height int

	// Version info
	version string

	// Key bindings
	keys KeyMap

	// Help overlay
	helpVisible bool

	// GitHub loading state
	githubLoading bool
	githubSpinner spinner.Model

	// Delete operation spinner
	deleteSpinner spinner.Model
}

// KeyMap defines key bindings for the application
type KeyMap struct {
	Up       key.Binding
	Down     key.Binding
	Left     key.Binding
	Right    key.Binding
	Enter    key.Binding
	Back     key.Binding
	Quit     key.Binding
	New      key.Binding
	Delete   key.Binding
	Init     key.Binding
	Config   key.Binding
	Prune    key.Binding
	Navigate key.Binding
	Help     key.Binding
	Tools    key.Binding
	Compare  key.Binding
}

// HelpState holds the state for the help overlay
type HelpState struct {
	visible bool
}

// DefaultKeyMap returns default key bindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "move left"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "move right"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		New: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "new worktree"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete worktree"),
		),
		Init: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "initialize"),
		),
		Config: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "config"),
		),
		Prune: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "prune missing"),
		),
		Navigate: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "navigate to worktree"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Tools: key.NewBinding(
			key.WithKeys("t"),
			key.WithHelp("t", "tools"),
		),
		Compare: key.NewBinding(
			key.WithKeys("m"),
			key.WithHelp("m", "compare/merge"),
		),
	}
}
