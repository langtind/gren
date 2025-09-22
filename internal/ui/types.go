package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"gren/internal/config"
	"gren/internal/git"
)

// ViewState represents the current view/screen
type ViewState int

const (
	DashboardView ViewState = iota
	CreateView
	DeleteView
	InitView
	SettingsView
	OpenInView
	ConfigView
)

// Worktree represents a git worktree
type Worktree struct {
	Name      string
	Path      string
	Branch    string
	Status    string // "clean", "modified", "building", etc.
	IsCurrent bool   // true if this is the current worktree
}

// InitStep represents the current step in initialization
type InitStep int

const (
	InitStepWelcome InitStep = iota
	InitStepAnalysis
	InitStepRecommendations
	InitStepCustomization
	InitStepPreview
	InitStepCreated
	InitStepExecuting
	InitStepComplete
	InitStepCommitConfirm
	InitStepFinal
)

// InitState holds the state for project initialization
type InitState struct {
	currentStep       InitStep
	detectedFiles     []DetectedFile
	copyPatterns      []CopyPattern
	selected          int
	worktreeDir       string
	customizationMode string // "", "worktree", "patterns", "postcreate"
	editingText       string
	postCreateScript  string
	analysisComplete  bool   // whether project analysis is complete
	packageManager    string // detected package manager
	postCreateCmd     string // detected post-create command
}

// CopyPattern represents a file pattern to copy to worktrees
type CopyPattern struct {
	Pattern     string
	Type        string // "env", "config", "tool"
	IsGitIgnored bool
	Description string
	Enabled     bool   // whether this pattern is enabled
	Detected    bool   // whether this pattern was detected automatically
}

// DetectedFile represents a detected file that could be useful for worktrees
type DetectedFile struct {
	Path        string
	Type        string // "env", "config", "tool"
	IsGitIgnored bool
	Description string
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
	currentStep      CreateStep
	createMode       CreateMode
	branchName       string
	baseBranch       string
	branchStatuses   []BranchStatus
	availableBranches []BranchStatus // For existing branch selection
	selectedBranch   int
	selectedMode     int  // For branch mode selection
	showWarning      bool
	warningAccepted  bool
	selectedAction   int        // For the post-create actions
	actionsList      list.Model // Dropdown menu for post-create actions
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
	worktreePath    string               // Path of the selected worktree
	actions         []PostCreateAction   // Available actions
	selectedIndex   int                  // Currently selected action index
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

// DeleteState holds the state for worktree deletion
type DeleteState struct {
	currentStep       DeleteStep
	selectedWorktrees []int      // Indices of selected worktrees for deletion
	warnings          []string
	targetWorktree    *Worktree  // Specific worktree to delete (for single deletion)
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
	openInState  *OpenInState
	configState  *ConfigState

	// Screen dimensions
	width  int
	height int

	// Key bindings
	keys KeyMap
}

// KeyMap defines key bindings for the application
type KeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Left   key.Binding
	Right  key.Binding
	Enter  key.Binding
	Back   key.Binding
	Quit   key.Binding
	New    key.Binding
	Delete key.Binding
	Init   key.Binding
	Config key.Binding
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
	}
}