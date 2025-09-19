package ui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
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
)

// Worktree represents a git worktree
type Worktree struct {
	Name   string
	Path   string
	Branch string
	Status string // "clean", "modified", "building", etc.
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

// InitState holds the state for the initialization process
type InitState struct {
	currentStep      InitStep
	worktreeDir      string
	copyPatterns     []CopyPattern
	packageManager   string
	defaultBranch    string
	gitignoreFiles   []string
	detectedFiles    []DetectedFile
	postCreateCmd    string
	commitConfig     bool
	analysisComplete bool
	selected         int
	customizationMode string // "worktree", "patterns", "postcreate", "overview"
	editingText      string  // For text input fields
}

// CopyPattern represents a file pattern to copy
type CopyPattern struct {
	Pattern     string
	Description string
	Enabled     bool
	Detected    bool
}

// DetectedFile represents a file found during analysis
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

// CreateStep represents the current step in worktree creation
type CreateStep int

const (
	CreateStepBranchName CreateStep = iota
	CreateStepBaseBranch
	CreateStepConfirm
	CreateStepCreating
	CreateStepComplete
)

// BranchStatus represents the status of a git branch (copied from git package to avoid circular imports).
type BranchStatus struct {
	Name             string
	IsClean          bool
	UncommittedFiles int
	UntrackedFiles   int
	AheadCount       int
	BehindCount      int
	IsCurrent        bool
}

// CreateState holds the state for worktree creation
type CreateState struct {
	currentStep     CreateStep
	branchName      string
	baseBranch      string
	branchStatuses  []BranchStatus
	selectedBranch  int
	showWarning     bool
	warningAccepted bool
	selectedAction  int // For the post-create actions
}

// Model is the main TUI model.
type Model struct {
	// Current view state
	currentView ViewState

	// Dependencies
	gitRepo git.Repository
	configManager *config.Manager

	// State
	repoInfo    *git.RepoInfo
	config      *config.Config
	worktrees   []Worktree
	selected    int
	err         error
	initState   *InitState
	createState *CreateState

	// Screen dimensions
	width  int
	height int

	// Key bindings
	keys KeyMap
}

// KeyMap defines key bindings
type KeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Enter  key.Binding
	Back   key.Binding
	New    key.Binding
	Delete key.Binding
	Init   key.Binding
	Help   key.Binding
	Quit   key.Binding
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
		Enter: key.NewBinding(
			key.WithKeys("enter", " "),
			key.WithHelp("enter", "select"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		New: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "new worktree"),
		),
		Delete: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "delete"),
		),
		Init: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "initialize"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

// NewModel creates a new model with the given dependencies.
func NewModel(gitRepo git.Repository, configManager *config.Manager) Model {
	if gitRepo == nil {
		gitRepo = git.NewLocalRepository()
	}
	if configManager == nil {
		configManager = config.NewManager()
	}

	return Model{
		currentView:   DashboardView,
		gitRepo:       gitRepo,
		configManager: configManager,
		repoInfo:      nil, // Will be loaded from git
		config:        nil, // Will be loaded from config
		worktrees:     []Worktree{}, // Will be loaded from git
		selected:      0,
		initState:     nil,
		createState:   nil,
		keys:          DefaultKeyMap(),
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.loadProjectInfo(),
	)
}

// loadProjectInfo loads git repository information.
func (m Model) loadProjectInfo() tea.Cmd {
	return func() tea.Msg {
		info, err := m.gitRepo.GetRepoInfo(context.Background())
		return projectInfoMsg{info: info, err: err}
	}
}

// Messages
type projectInfoMsg struct {
	info *git.RepoInfo
	err  error
}

type initializeMsg struct {
	err error
}

type createInitMsg struct {
	branchStatuses []BranchStatus
	recommendedBase string
	err error
}

// updateProjectInfo updates the model with repository information.
func (m Model) updateProjectInfo(info *git.RepoInfo, err error) Model {
	if err != nil {
		m.err = fmt.Errorf("failed to load project info: %w", err)
		return m
	}

	m.repoInfo = info
	m.err = nil
	return m
}

// runInitialize runs the initialization process.
func (m Model) runInitialize() tea.Cmd {
	return func() tea.Msg {
		if m.repoInfo == nil {
			return initializeMsg{err: fmt.Errorf("no repository information available")}
		}

		// Create default configuration
		config, err := config.NewDefaultConfig(m.repoInfo.Name)
		if err != nil {
			return initializeMsg{err: fmt.Errorf("failed to create config: %w", err)}
		}

		// TODO: Detect project settings and create hook
		// For now, just save the basic config
		if err := m.configManager.Save(config); err != nil {
			return initializeMsg{err: fmt.Errorf("failed to save config: %w", err)}
		}

		return initializeMsg{err: nil}
	}
}

// initializeCreateState initializes the create worktree state.
func (m Model) initializeCreateState() tea.Cmd {
	return func() tea.Msg {
		if m.repoInfo == nil {
			return createInitMsg{err: fmt.Errorf("no repository information available")}
		}

		// Get branch statuses
		statuses, err := m.gitRepo.GetBranchStatuses(context.Background())
		if err != nil {
			return createInitMsg{err: fmt.Errorf("failed to get branch statuses: %w", err)}
		}

		// Convert git.BranchStatus to ui.BranchStatus
		var uiStatuses []BranchStatus
		for _, status := range statuses {
			uiStatuses = append(uiStatuses, BranchStatus{
				Name:             status.Name,
				IsClean:          status.IsClean,
				UncommittedFiles: status.UncommittedFiles,
				UntrackedFiles:   status.UntrackedFiles,
				AheadCount:       status.AheadCount,
				BehindCount:      status.BehindCount,
				IsCurrent:        status.IsCurrent,
			})
		}

		// Get recommended base branch
		recommended, err := m.gitRepo.GetRecommendedBaseBranch(context.Background())
		if err != nil {
			// Continue with empty recommendation if this fails
			recommended = ""
		}

		return createInitMsg{
			branchStatuses:  uiStatuses,
			recommendedBase: recommended,
			err:             nil,
		}
	}
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case projectInfoMsg:
		m = m.updateProjectInfo(msg.info, msg.err)
		return m, nil

	case initializeMsg:
		if msg.err != nil {
			m.err = fmt.Errorf("initialization failed: %w", msg.err)
		} else {
			// Refresh project info after successful initialization
			if m.repoInfo != nil {
				m.repoInfo.IsInitialized = true
			}
			m.err = nil
		}
		return m, nil

	case createInitMsg:
		if msg.err != nil {
			m.err = fmt.Errorf("failed to initialize create state: %w", msg.err)
			m.currentView = DashboardView
		} else {
			// Initialize create state
			m.createState = &CreateState{
				currentStep:     CreateStepBranchName,
				branchName:      "",
				baseBranch:      msg.recommendedBase,
				branchStatuses:  msg.branchStatuses,
				selectedBranch:  0,
				showWarning:     false,
				warningAccepted: false,
				selectedAction:  0,
			}

			// Find the index of recommended base branch
			for i, status := range msg.branchStatuses {
				if status.Name == msg.recommendedBase {
					m.createState.selectedBranch = i
					break
				}
			}
		}
		return m, nil

	case projectAnalysisCompleteMsg:
		if m.initState != nil {
			m.initState.analysisComplete = true
		}
		return m, nil

	case initExecutionCompleteMsg:
		if m.initState != nil {
			m.initState.currentStep = InitStepComplete
		}
		return m, nil

	case worktreeCreatedMsg:
		if m.createState != nil {
			m.createState.currentStep = CreateStepComplete
		}
		return m, nil

	case scriptCreateCompleteMsg:
		// Script files created, show confirmation
		if m.initState != nil {
			m.initState.currentStep = InitStepCreated
			if msg.err != nil {
				// Could show error to user, for now just continue
			}
		}
		return m, nil

	case scriptEditCompleteMsg:
		// Script was opened in editor, go to complete step
		if m.initState != nil {
			m.initState.currentStep = InitStepComplete
			if msg.err != nil {
				// Could show error to user, for now just continue
			}
		}
		return m, nil

	case commitCompleteMsg:
		// Commit completed, go to final step
		if m.initState != nil {
			m.initState.currentStep = InitStepFinal
			if msg.err != nil {
				// Could show error to user, for now just continue
			}
		}
		// Mark as initialized even if commit failed
		if m.repoInfo != nil {
			m.repoInfo.IsInitialized = true
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		// Handle view-specific keys first
		if m.currentView == InitView {
			return m.handleInitKeys(msg)
		}
		if m.currentView == CreateView {
			return m.handleCreateKeys(msg)
		}

		// Global keys
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit

		case key.Matches(msg, m.keys.Up):
			if m.selected > 0 {
				m.selected--
			}

		case key.Matches(msg, m.keys.Down):
			if m.selected < len(m.worktrees)-1 {
				m.selected++
			}

		case key.Matches(msg, m.keys.Enter):
			// TODO: Navigate to selected worktree
			return m, nil

		case key.Matches(msg, m.keys.New):
			// Only allow creating worktrees if initialized
			if m.repoInfo != nil && m.repoInfo.IsGitRepo && m.repoInfo.IsInitialized {
				m.currentView = CreateView
				return m, m.initializeCreateState()
			}
			return m, nil

		case key.Matches(msg, m.keys.Delete):
			m.currentView = DeleteView
			return m, nil

		case key.Matches(msg, m.keys.Init):
			if m.repoInfo != nil && m.repoInfo.IsGitRepo && !m.repoInfo.IsInitialized {
				m.currentView = InitView
				m.initState = &InitState{
					currentStep:       InitStepWelcome,
					worktreeDir:       fmt.Sprintf("../%s-worktrees", m.repoInfo.Name),
					copyPatterns:      []CopyPattern{}, // Will be populated by analysis
					packageManager:    m.detectPackageManager(),
					defaultBranch:     m.repoInfo.CurrentBranch,
					gitignoreFiles:    []string{},
					detectedFiles:     []DetectedFile{}, // Will be populated by analysis
					postCreateCmd:     m.detectPostCreateCommand(),
					commitConfig:      true,
					analysisComplete:  false,
					selected:          0,
					customizationMode: "",
					editingText:       "",
				}
				return m, nil
			}
			return m, nil

		case key.Matches(msg, m.keys.Back):
			m.currentView = DashboardView
			return m, nil
		}
	}

	return m, nil
}

// handleInitKeys handles keyboard input for the init view
func (m Model) handleInitKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.initState == nil {
		return m, nil
	}

	// Debug: log what key was pressed and current step
	// fmt.Printf("DEBUG: Key '%s' pressed in step %d\n", msg.String(), m.initState.currentStep)

	// Handle customization step specially
	if m.initState.currentStep == InitStepCustomization {
		return m.handleCustomizationKeys(msg)
	}

	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Back):
		if m.initState.currentStep == InitStepWelcome {
			m.currentView = DashboardView
			return m, nil
		}
		// Go back one step
		if m.initState.currentStep > InitStepWelcome {
			m.initState.currentStep--
		}
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		switch m.initState.currentStep {
		case InitStepWelcome:
			// Start analysis
			m.initState.currentStep = InitStepAnalysis
			return m, m.runProjectAnalysis()

		case InitStepAnalysis:
			if m.initState.analysisComplete {
				m.initState.currentStep = InitStepRecommendations
			}
			return m, nil

		case InitStepRecommendations:
			// Create script files
			m.initState.currentStep = InitStepCreated
			return m, m.createScriptFiles()


		case InitStepCreated:
			// Enter key does nothing on this step - we handle y/n separately
			return m, nil

		case InitStepPreview:
			m.initState.currentStep = InitStepExecuting
			return m, m.runInitialization()

		case InitStepComplete:
			// Go to commit confirmation
			m.initState.currentStep = InitStepCommitConfirm
			return m, nil

		case InitStepCommitConfirm:
			// Should not reach here - y/n handled separately
			return m, nil

		case InitStepFinal:
			// Mark project as initialized
			if m.repoInfo != nil {
				m.repoInfo.IsInitialized = true
			}
			m.currentView = DashboardView
			return m, nil
		}

	case msg.String() == "y" || msg.String() == "Y":
		if m.initState.currentStep == InitStepRecommendations {
			m.initState.currentStep = InitStepPreview
		} else if m.initState.currentStep == InitStepCreated {
			// Open script in editor
			return m, m.openScriptInEditor()
		} else if m.initState.currentStep == InitStepCommitConfirm {
			// Commit configuration to git
			return m, m.commitConfiguration()
		}
		return m, nil

	case msg.String() == "c":
		if m.initState.currentStep == InitStepRecommendations {
			m.initState.currentStep = InitStepCustomization
		}
		return m, nil

	case msg.String() == "n" || msg.String() == "N":
		if m.initState.currentStep == InitStepRecommendations {
			m.initState.currentStep = InitStepWelcome
		} else if m.initState.currentStep == InitStepCreated {
			// Skip opening, go to complete
			m.initState.currentStep = InitStepComplete
		} else if m.initState.currentStep == InitStepCommitConfirm {
			// Skip commit, go to final
			m.initState.currentStep = InitStepFinal
			// Mark as initialized since we're skipping commit
			if m.repoInfo != nil {
				m.repoInfo.IsInitialized = true
			}
		}
		return m, nil
	}

	return m, nil
}

// handleCustomizationKeys handles keyboard input for the customization step
func (m Model) handleCustomizationKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.initState == nil {
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.Back):
		if m.initState.customizationMode != "" {
			// Go back to main customization menu
			m.initState.customizationMode = ""
			m.initState.editingText = ""
			m.initState.selected = 0
		} else {
			// Go back to recommendations
			m.initState.currentStep = InitStepRecommendations
		}
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		if m.initState.customizationMode == "" {
			// Main menu - enter sub-mode
			options := []string{"worktree", "patterns", "postcreate"}
			if m.initState.selected < len(options) {
				m.initState.customizationMode = options[m.initState.selected]
				m.initState.selected = 0

				// Initialize editing text for text fields
				if m.initState.customizationMode == "worktree" {
					m.initState.editingText = m.initState.worktreeDir
				}
			}
		} else if m.initState.customizationMode == "postcreate" {
			// Post-create menu - choose simple command or script
			if m.initState.selected == 0 {
				// Simple command
				m.initState.customizationMode = "simplecommand"
				m.initState.editingText = m.initState.postCreateCmd
			} else if m.initState.selected == 1 {
				// Custom script - open in external editor
				return m, m.openPostCreateScript()
			}
		} else {
			// Sub-mode - save and return to main menu
			if m.initState.customizationMode == "worktree" {
				if m.initState.editingText != "" {
					m.initState.worktreeDir = m.initState.editingText
				}
			} else if m.initState.customizationMode == "simplecommand" {
				if m.initState.editingText != "" {
					m.initState.postCreateCmd = m.initState.editingText
				}
			}
			// For patterns, just exit (toggling is handled by space)

			m.initState.customizationMode = ""
			m.initState.editingText = ""
			m.initState.selected = 0
		}
		return m, nil

	case key.Matches(msg, m.keys.Up):
		if m.initState.customizationMode == "" {
			// Main menu navigation
			if m.initState.selected > 0 {
				m.initState.selected--
			}
		} else if m.initState.customizationMode == "patterns" {
			// Pattern navigation
			if m.initState.selected > 0 {
				m.initState.selected--
			}
		} else if m.initState.customizationMode == "postcreate" {
			// Post-create option navigation (2 options)
			if m.initState.selected > 0 {
				m.initState.selected--
			}
		}
		return m, nil

	case key.Matches(msg, m.keys.Down):
		if m.initState.customizationMode == "" {
			// Main menu navigation (3 options)
			if m.initState.selected < 2 {
				m.initState.selected++
			}
		} else if m.initState.customizationMode == "patterns" {
			// Pattern navigation
			if m.initState.selected < len(m.initState.copyPatterns)-1 {
				m.initState.selected++
			}
		} else if m.initState.customizationMode == "postcreate" {
			// Post-create option navigation (2 options)
			if m.initState.selected < 1 {
				m.initState.selected++
			}
		}
		return m, nil

	case msg.String() == " ":
		// Space key for toggling patterns
		if m.initState.customizationMode == "patterns" {
			if m.initState.selected < len(m.initState.copyPatterns) {
				m.initState.copyPatterns[m.initState.selected].Enabled = !m.initState.copyPatterns[m.initState.selected].Enabled
			}
		}
		return m, nil

	default:
		// Handle text input for editable fields
		if m.initState.customizationMode == "worktree" || m.initState.customizationMode == "simplecommand" {
			switch msg.Type {
			case tea.KeyBackspace:
				if len(m.initState.editingText) > 0 {
					m.initState.editingText = m.initState.editingText[:len(m.initState.editingText)-1]
				}
			case tea.KeyRunes:
				m.initState.editingText += string(msg.Runes)
			}
		}
		return m, nil
	}

	return m, nil
}

// handleCreateKeys handles keyboard input for the create view
func (m Model) handleCreateKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.createState == nil {
		return m, nil
	}

	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Back):
		if m.createState.currentStep == CreateStepBranchName {
			m.currentView = DashboardView
			return m, nil
		}
		// Go back one step
		if m.createState.currentStep > CreateStepBranchName {
			m.createState.currentStep--
		}
		return m, nil

	case key.Matches(msg, m.keys.Up):
		if m.createState.currentStep == CreateStepBaseBranch {
			if m.createState.selectedBranch > 0 {
				m.createState.selectedBranch--
				m.createState.warningAccepted = false
			}
		} else if m.createState.currentStep == CreateStepComplete {
			if m.createState.selectedAction > 0 {
				m.createState.selectedAction--
			}
		}
		return m, nil

	case key.Matches(msg, m.keys.Down):
		if m.createState.currentStep == CreateStepBaseBranch {
			if m.createState.selectedBranch < len(m.createState.branchStatuses)-1 {
				m.createState.selectedBranch++
				m.createState.warningAccepted = false
			}
		} else if m.createState.currentStep == CreateStepComplete {
			// Get available actions to check bounds
			actions := m.getAvailableActions()
			if m.createState.selectedAction < len(actions)-1 {
				m.createState.selectedAction++
			}
		}
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		switch m.createState.currentStep {
		case CreateStepBranchName:
			if isValidBranchName(m.createState.branchName) {
				m.createState.currentStep = CreateStepBaseBranch
			}
			return m, nil

		case CreateStepBaseBranch:
			if len(m.createState.branchStatuses) > 0 && m.createState.selectedBranch < len(m.createState.branchStatuses) {
				selectedStatus := m.createState.branchStatuses[m.createState.selectedBranch]
				if !selectedStatus.IsClean && !m.createState.warningAccepted {
					// Show warning, don't advance
					m.createState.showWarning = true
					return m, nil
				}
				m.createState.baseBranch = selectedStatus.Name
				m.createState.currentStep = CreateStepConfirm
			}
			return m, nil

		case CreateStepConfirm:
			m.createState.currentStep = CreateStepCreating
			return m, m.createWorktree()

		case CreateStepComplete:
			// Execute the selected action
			actions := m.getAvailableActions()
			if m.createState.selectedAction < len(actions) {
				selectedAction := actions[m.createState.selectedAction]

				// Execute the action if it has a command
				if selectedAction.Command != "" {
					_ = m.executeAction(selectedAction) // Ignore errors for now
				}
			}

			// Always return to dashboard after action
			m.currentView = DashboardView
			return m, nil
		}

	case msg.String() == "y":
		if m.createState.currentStep == CreateStepBaseBranch && m.createState.showWarning {
			m.createState.warningAccepted = true
			m.createState.showWarning = false
		}
		return m, nil

	default:
		// Handle text input for branch name
		if m.createState.currentStep == CreateStepBranchName {
			switch msg.Type {
			case tea.KeyBackspace:
				if len(m.createState.branchName) > 0 {
					m.createState.branchName = m.createState.branchName[:len(m.createState.branchName)-1]
				}
			case tea.KeyRunes:
				m.createState.branchName += string(msg.Runes)
			}
		}
		return m, nil
	}

	return m, nil
}

// runProjectAnalysis runs the project analysis step
func (m Model) runProjectAnalysis() tea.Cmd {
	return func() tea.Msg {
		// Analyze the project structure
		detectedFiles := m.analyzeProject()
		copyPatterns := m.generateCopyPatterns(detectedFiles)

		// Update the init state with analysis results
		if m.initState != nil {
			m.initState.detectedFiles = detectedFiles
			m.initState.copyPatterns = copyPatterns
		}

		return projectAnalysisCompleteMsg{}
	}
}

// analyzeProject scans the current directory for relevant files
func (m Model) analyzeProject() []DetectedFile {
	var detected []DetectedFile

	// Get patterns from .gitignore
	gitIgnorePatterns := m.parseGitIgnore()

	// File descriptions
	fileDescriptions := map[string]string{
		".env":           "Environment variables",
		".env.local":     "Local environment variables",
		".env.example":   "Environment template",
		".envrc":         "Direnv configuration",
		".nvmrc":         "Node version",
		".claude/":       "Claude configuration",
		".vscode/":       "VS Code configuration",
		".idea/":         "IntelliJ IDEA configuration",
		".ruby-version":  "Ruby version",
		".python-version": "Python version",
		".tool-versions": "Tool versions (asdf)",
	}

	// Check gitignored patterns that exist
	for _, pattern := range gitIgnorePatterns {
		// Check if files/directories matching this pattern exist
		matches, err := filepath.Glob(pattern)
		if err == nil && len(matches) > 0 {
			for _, match := range matches {
				description := fileDescriptions[pattern]
				if description == "" {
					description = fmt.Sprintf("Development file (%s)", pattern)
				}

				// Determine type
				fileType := "config"
				if strings.Contains(pattern, "env") {
					fileType = "env"
				}

				detected = append(detected, DetectedFile{
					Path:         match,
					Type:         fileType,
					IsGitIgnored: true,
					Description:  description,
				})
			}
		}
	}

	return detected
}

// generateCopyPatterns creates copy patterns based on detected files
func (m Model) generateCopyPatterns(detectedFiles []DetectedFile) []CopyPattern {
	var patterns []CopyPattern

	// Pattern mappings
	patternMap := map[string]struct {
		pattern     string
		description string
		autoEnable  bool
	}{
		".env":         {".env*", "Environment files", true},
		".env.local":   {".env*", "Environment files", true},
		".env.example": {".env*", "Environment files", true},
		".envrc":       {".envrc", "Direnv configuration", true},
		".nvmrc":       {".nvmrc", "Node version", false},
		".vscode/":     {".vscode/", "VS Code settings", true},
		".claude/":     {".claude/", "Claude configuration", true},
	}

	// Track which patterns we've already added
	addedPatterns := make(map[string]bool)

	for _, file := range detectedFiles {
		if mapping, exists := patternMap[file.Path]; exists {
			if !addedPatterns[mapping.pattern] {
				patterns = append(patterns, CopyPattern{
					Pattern:     mapping.pattern,
					Description: mapping.description,
					Enabled:     mapping.autoEnable && file.IsGitIgnored, // Only enable if gitignored
					Detected:    true,
				})
				addedPatterns[mapping.pattern] = true
			}
		}
	}

	// Add some common patterns that might not be detected yet
	commonPatterns := []CopyPattern{
		{".gitattributes", "Git attributes", false, false},
		{"*.code-workspace", "VS Code workspace", false, false},
	}

	for _, pattern := range commonPatterns {
		if !addedPatterns[pattern.Pattern] {
			patterns = append(patterns, pattern)
		}
	}

	return patterns
}

// parseGitIgnore reads .gitignore and returns development-relevant patterns
func (m Model) parseGitIgnore() []string {
	gitIgnoreContent, err := os.ReadFile(".gitignore")
	if err != nil {
		return []string{} // No .gitignore file
	}

	var devPatterns []string
	lines := strings.Split(string(gitIgnoreContent), "\n")

	// Development-relevant patterns we care about
	devRelevantPatterns := map[string]bool{
		".env":           true,
		".env.*":         true,
		".env*":          true,
		".envrc":         true,
		".claude/":       true,
		".claude":        true,
		".vscode/":       true,
		".vscode":        true,
		".idea/":         true,
		".idea":          true,
		".nvmrc":         true,
		".ruby-version":  true,
		".python-version": true,
		".tool-versions": true,
		"*.local":        true,
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Remove leading slash if present
		if strings.HasPrefix(line, "/") {
			line = line[1:]
		}

		// Check if this is a development-relevant pattern
		if devRelevantPatterns[line] {
			devPatterns = append(devPatterns, line)
		} else {
			// Also check for wildcards and partial matches
			for pattern := range devRelevantPatterns {
				if strings.Contains(line, strings.TrimSuffix(pattern, "*")) {
					devPatterns = append(devPatterns, line)
					break
				}
			}
		}
	}

	return devPatterns
}

// isGitIgnored checks if a file matches gitignored patterns
func (m Model) isGitIgnored(filename string) bool {
	patterns := m.parseGitIgnore()

	for _, pattern := range patterns {
		// Handle wildcards
		if strings.Contains(pattern, "*") {
			if matched, _ := filepath.Match(pattern, filename); matched {
				return true
			}
		} else {
			// Exact match or directory match
			if pattern == filename ||
			   (strings.HasSuffix(pattern, "/") && strings.HasPrefix(filename, pattern)) ||
			   (!strings.HasSuffix(pattern, "/") && strings.HasPrefix(filename, pattern+"/")) {
				return true
			}
		}
	}

	return false
}

// getAvailableActions returns a list of available post-create actions
func (m Model) getAvailableActions() []PostCreateAction {
	if m.createState == nil {
		return []PostCreateAction{}
	}

	worktreePath := fmt.Sprintf("../gren-worktrees/%s", m.createState.branchName)

	allActions := []PostCreateAction{
		{
			Name:        "Open in Cursor",
			Icon:        "📂",
			Command:     "cursor",
			Args:        []string{worktreePath},
			Description: "Open worktree in Cursor editor",
		},
		{
			Name:        "Open in VS Code",
			Icon:        "⚡",
			Command:     "code",
			Args:        []string{worktreePath},
			Description: "Open worktree in Visual Studio Code",
		},
		{
			Name:        "Open in Claude Code",
			Icon:        "🤖",
			Command:     "claude",
			Args:        []string{"code", worktreePath},
			Description: "Open worktree in Claude Code",
		},
		{
			Name:        "Open in Finder",
			Icon:        "📁",
			Command:     "open",
			Args:        []string{worktreePath},
			Description: "Open worktree directory in Finder",
		},
	}

	// Check which commands are available
	var availableActions []PostCreateAction
	for _, action := range allActions {
		action.Available = isCommandAvailable(action.Command)
		if action.Available {
			availableActions = append(availableActions, action)
		}
	}

	// Always add return to dashboard option
	availableActions = append(availableActions, PostCreateAction{
		Name:        "Return to dashboard",
		Icon:        "🔙",
		Command:     "",
		Args:        nil,
		Available:   true,
		Description: "Go back to main dashboard",
	})

	return availableActions
}

// isCommandAvailable checks if a command is available in the system PATH
func isCommandAvailable(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}

// executeAction executes the selected post-create action
func (m Model) executeAction(action PostCreateAction) error {
	if action.Command == "" {
		// Special case for "Return to dashboard" - no command to execute
		return nil
	}

	cmd := exec.Command(action.Command, action.Args...)
	return cmd.Start() // Use Start() instead of Run() to not wait for completion
}

// openPostCreateScript opens the post-create script in an external editor
func (m Model) openPostCreateScript() tea.Cmd {
	return func() tea.Msg {
		// Create .gren directory if it doesn't exist
		if err := os.MkdirAll(".gren", 0755); err != nil {
			return scriptEditCompleteMsg{err: err}
		}

		scriptPath := ".gren/post-create.sh"

		// Create a basic script template if file doesn't exist
		if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
			template := `#!/bin/bash
# Post-create script for new worktrees
# This script runs in the new worktree directory after creation

set -e

echo "Setting up new worktree..."

# Example: Install dependencies
# npm install
# go mod tidy
# pip install -r requirements.txt

# Example: Copy additional files
# cp ../.env.example .env

# Example: Initialize development environment
# make setup
# docker-compose up -d

echo "Worktree setup complete!"
`
			if err := os.WriteFile(scriptPath, []byte(template), 0755); err != nil {
				return scriptEditCompleteMsg{err: err}
			}
		}

		// Try to open in available editors
		editors := []string{"code", "cursor", "zed", "vim", "nano"}

		for _, editor := range editors {
			if isCommandAvailable(editor) {
				cmd := exec.Command(editor, scriptPath)
				if err := cmd.Start(); err == nil {
					// Clear the post-create command since we're using a script
					if m.initState != nil {
						m.initState.postCreateCmd = ""
					}
					return scriptEditCompleteMsg{err: nil}
				}
			}
		}

		return scriptEditCompleteMsg{err: fmt.Errorf("no suitable editor found (tried: %v)", editors)}
	}
}

// scriptEditCompleteMsg indicates script editing is complete
type scriptEditCompleteMsg struct {
	err error
}

// commitCompleteMsg indicates git commit is complete
type commitCompleteMsg struct {
	err error
}

// commitConfiguration commits the .gren/ configuration to git
func (m Model) commitConfiguration() tea.Cmd {
	return func() tea.Msg {
		// Add .gren/ to .gitignore if not already there
		gitignoreContent, err := os.ReadFile(".gitignore")
		if err == nil {
			gitignoreStr := string(gitignoreContent)
			if !strings.Contains(gitignoreStr, ".gren/") {
				// Add .gren/ to .gitignore
				f, err := os.OpenFile(".gitignore", os.O_APPEND|os.O_WRONLY, 0644)
				if err == nil {
					f.WriteString("\n# gren worktree configuration\n.gren/\n")
					f.Close()
				}
			}
		}

		// Add files to git
		cmd := exec.Command("git", "add", ".gren/config.json", ".gren/post-create.sh", ".gitignore")
		if err := cmd.Run(); err != nil {
			return commitCompleteMsg{err: fmt.Errorf("failed to add files: %w", err)}
		}

		// Commit the configuration
		cmd = exec.Command("git", "commit", "-m", "Add gren worktree configuration")
		if err := cmd.Run(); err != nil {
			return commitCompleteMsg{err: fmt.Errorf("failed to commit: %w", err)}
		}

		return commitCompleteMsg{err: nil}
	}
}

// createAndOpenScript creates the setup script and opens it in an editor
func (m Model) createAndOpenScript() tea.Cmd {
	return func() tea.Msg {
		if m.repoInfo == nil || m.initState == nil {
			return scriptEditCompleteMsg{err: fmt.Errorf("missing project info")}
		}

		// Create .gren directory
		if err := os.MkdirAll(".gren", 0755); err != nil {
			return scriptEditCompleteMsg{err: err}
		}

		// Generate the setup script with configuration
		scriptContent := m.generateSetupScript()

		// Write the script
		scriptPath := ".gren/post-create.sh"
		if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
			return scriptEditCompleteMsg{err: err}
		}

		// Create basic config.json
		configContent := m.generateConfigFile()
		configPath := ".gren/config.json"
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			return scriptEditCompleteMsg{err: err}
		}

		// Try to open script in available editors
		editors := []string{"code", "cursor", "zed", "vim", "nano"}

		for _, editor := range editors {
			if isCommandAvailable(editor) {
				cmd := exec.Command(editor, scriptPath)
				if err := cmd.Start(); err == nil {
					return scriptEditCompleteMsg{err: nil}
				}
			}
		}

		// If no editor found, still continue (user can edit manually)
		return scriptEditCompleteMsg{err: nil}
	}
}

// generateSetupScript creates the setup script with detected configuration
func (m Model) generateSetupScript() string {
	var script strings.Builder

	script.WriteString("#!/bin/bash\n")
	script.WriteString("# gren post-create setup script\n")
	script.WriteString("# This script runs in the new worktree directory after creation\n")
	script.WriteString("#\n")
	script.WriteString("# Configuration (edit as needed):\n")
	script.WriteString(fmt.Sprintf("# WORKTREE_DIR=\"%s\"\n", m.initState.worktreeDir))
	script.WriteString(fmt.Sprintf("# PACKAGE_MANAGER=\"%s\"\n", m.initState.packageManager))
	if m.initState.postCreateCmd != "" {
		script.WriteString(fmt.Sprintf("# DEFAULT_SETUP_CMD=\"%s\"\n", m.initState.postCreateCmd))
	}
	script.WriteString("#\n")

	// Get patterns from .gitignore
	gitIgnorePatterns := m.parseGitIgnore()
	if len(gitIgnorePatterns) > 0 {
		script.WriteString("# Files found in .gitignore (development-relevant):\n")
		for _, pattern := range gitIgnorePatterns {
			script.WriteString(fmt.Sprintf("# %s\n", pattern))
		}
		script.WriteString("#\n")
	}

	script.WriteString("\nset -e\n\n")
	script.WriteString("echo \"🚀 Setting up new worktree...\"\n\n")

	// Generate copy commands from gitignore patterns
	script.WriteString("# Copy development configuration files\n")
	script.WriteString("echo \"📋 Copying gitignored development files...\"\n\n")

	copyCommands := m.generateCopyCommands(gitIgnorePatterns)
	for _, cmd := range copyCommands {
		script.WriteString(cmd + "\n")
	}

	script.WriteString("\n")

	// Add setup command
	if m.initState.postCreateCmd != "" {
		script.WriteString("# Install dependencies / setup\n")
		script.WriteString(fmt.Sprintf("echo \"📦 Running: %s\"\n", m.initState.postCreateCmd))
		script.WriteString(fmt.Sprintf("%s\n", m.initState.postCreateCmd))
		script.WriteString("\n")
	}

	script.WriteString("echo \"✅ Worktree setup complete!\"\n")

	return script.String()
}

// generateCopyCommands creates copy commands for gitignored patterns
func (m Model) generateCopyCommands(patterns []string) []string {
	var commands []string

	// Add a comment about how to read config for dynamic source path
	commands = append(commands, "# Read main repo path from config")
	commands = append(commands, "MAIN_REPO_PATH=$(grep -o '\"main_repo_path\"[^,}]*' .gren/config.json | cut -d':' -f2 | tr -d '\" ')")
	commands = append(commands, "if [ -z \"$MAIN_REPO_PATH\" ]; then")
	commands = append(commands, "    echo \"Warning: Could not read main_repo_path from config, using fallback\"")
	commands = append(commands, "    MAIN_REPO_PATH=\"../\"")
	commands = append(commands, "fi")
	commands = append(commands, "")

	for _, pattern := range patterns {
		// Generate appropriate copy command based on pattern
		if strings.HasSuffix(pattern, "/") || (!strings.Contains(pattern, ".") && !strings.Contains(pattern, "*")) {
			// Directory pattern
			commands = append(commands, fmt.Sprintf("[ -d \"$MAIN_REPO_PATH/%s\" ] && cp -r \"$MAIN_REPO_PATH/%s\" . 2>/dev/null || true", pattern, pattern))
		} else {
			// File pattern (including wildcards)
			commands = append(commands, fmt.Sprintf("cp \"$MAIN_REPO_PATH/%s\" . 2>/dev/null || true", pattern))
		}
	}

	return commands
}

// generateConfigFile creates a basic config.json
func (m Model) generateConfigFile() string {
	// Get current directory as main repository path
	currentDir, _ := os.Getwd()

	config := map[string]interface{}{
		"main_repo_path":   currentDir,
		"worktree_dir":     m.initState.worktreeDir,
		"package_manager":  m.initState.packageManager,
		"copy_patterns":    []string{},
		"post_create_hook": "./post-create.sh",
	}

	// Add enabled patterns
	var patterns []string
	for _, pattern := range m.initState.copyPatterns {
		if pattern.Enabled {
			patterns = append(patterns, pattern.Pattern)
		}
	}
	config["copy_patterns"] = patterns

	// Convert to JSON (simplified)
	jsonStr := "{\n"
	jsonStr += fmt.Sprintf("  \"worktree_dir\": \"%s\",\n", config["worktree_dir"])
	jsonStr += fmt.Sprintf("  \"package_manager\": \"%s\",\n", config["package_manager"])
	jsonStr += fmt.Sprintf("  \"post_create_hook\": \"%s\",\n", config["post_create_hook"])
	jsonStr += "  \"copy_patterns\": [\n"
	for i, pattern := range patterns {
		comma := ","
		if i == len(patterns)-1 {
			comma = ""
		}
		jsonStr += fmt.Sprintf("    \"%s\"%s\n", pattern, comma)
	}
	jsonStr += "  ]\n"
	jsonStr += "}\n"

	return jsonStr
}

// createScriptFiles creates the script and config files without opening them
func (m Model) createScriptFiles() tea.Cmd {
	return func() tea.Msg {
		if m.repoInfo == nil || m.initState == nil {
			return scriptCreateCompleteMsg{err: fmt.Errorf("missing project info")}
		}

		// Create .gren directory
		if err := os.MkdirAll(".gren", 0755); err != nil {
			return scriptCreateCompleteMsg{err: err}
		}

		// Generate and write the setup script
		scriptContent := m.generateSetupScript()
		scriptPath := ".gren/post-create.sh"
		if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
			return scriptCreateCompleteMsg{err: err}
		}

		// Create basic config.json
		configContent := m.generateConfigFile()
		configPath := ".gren/config.json"
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			return scriptCreateCompleteMsg{err: err}
		}

		return scriptCreateCompleteMsg{err: nil}
	}
}

// openScriptInEditor opens the post-create script in an available editor
func (m Model) openScriptInEditor() tea.Cmd {
	return func() tea.Msg {
		scriptPath := ".gren/post-create.sh"

		// Try to open script in available editors
		editors := []string{"code", "cursor", "zed", "vim", "nano"}

		for _, editor := range editors {
			if isCommandAvailable(editor) {
				cmd := exec.Command(editor, scriptPath)
				if err := cmd.Start(); err == nil {
					return scriptEditCompleteMsg{err: nil}
				}
			}
		}

		// If no editor found, still continue
		return scriptEditCompleteMsg{err: fmt.Errorf("no suitable editor found (tried: %v)", editors)}
	}
}

// scriptCreateCompleteMsg indicates script creation is complete
type scriptCreateCompleteMsg struct {
	err error
}

// detectPackageManager detects the package manager based on project files
func (m Model) detectPackageManager() string {
	if _, err := os.Stat("package.json"); err == nil {
		// Check for lock files to determine package manager
		if _, err := os.Stat("bun.lockb"); err == nil {
			return "bun"
		}
		if _, err := os.Stat("pnpm-lock.yaml"); err == nil {
			return "pnpm"
		}
		if _, err := os.Stat("yarn.lock"); err == nil {
			return "yarn"
		}
		return "npm"
	}
	if _, err := os.Stat("go.mod"); err == nil {
		return "go"
	}
	if _, err := os.Stat("Cargo.toml"); err == nil {
		return "cargo"
	}
	if _, err := os.Stat("requirements.txt"); err == nil {
		return "pip"
	}
	return "none"
}

// detectPostCreateCommand suggests a post-create command based on the package manager
func (m Model) detectPostCreateCommand() string {
	pm := m.detectPackageManager()
	switch pm {
	case "bun":
		return "bun install"
	case "npm":
		return "npm install"
	case "pnpm":
		return "pnpm install"
	case "yarn":
		return "yarn install"
	case "go":
		return "go mod tidy"
	case "cargo":
		return "cargo check"
	case "pip":
		return "pip install -r requirements.txt"
	default:
		return ""
	}
}

// runInitialization runs the actual initialization
func (m Model) runInitialization() tea.Cmd {
	return func() tea.Msg {
		if m.repoInfo == nil {
			return initExecutionCompleteMsg{}
		}

		// Create configuration with user's chosen settings
		config, err := config.NewDefaultConfig(m.repoInfo.Name)
		if err != nil {
			return initExecutionCompleteMsg{}
		}

		// Apply user customizations from initState
		if m.initState != nil {
			config.WorktreeDir = m.initState.worktreeDir
			config.PackageManager = m.initState.packageManager
			config.PostCreateHook = m.initState.postCreateCmd

			// Convert copy patterns to string slice
			var patterns []string
			for _, pattern := range m.initState.copyPatterns {
				if pattern.Enabled {
					patterns = append(patterns, pattern.Pattern)
				}
			}
			config.CopyPatterns = patterns
		}

		// Save configuration
		if err := m.configManager.Save(config); err != nil {
			return initExecutionCompleteMsg{}
		}

		// TODO: Create post-create hook script
		// TODO: Update .gitignore
		// TODO: Commit changes if requested

		return initExecutionCompleteMsg{}
	}
}

// createWorktree creates the actual worktree
func (m Model) createWorktree() tea.Cmd {
	return func() tea.Msg {
		// TODO: Implement actual worktree creation
		return worktreeCreatedMsg{branchName: m.createState.branchName}
	}
}

// Additional message types
type projectAnalysisCompleteMsg struct{}
type initExecutionCompleteMsg struct{}
type worktreeCreatedMsg struct {
	branchName string
}

// View renders the current view
func (m Model) View() string {
	switch m.currentView {
	case DashboardView:
		return m.dashboardView()
	case CreateView:
		return m.createView()
	case DeleteView:
		return m.deleteView()
	case InitView:
		return m.initView()
	case SettingsView:
		return m.settingsView()
	default:
		return m.dashboardView()
	}
}