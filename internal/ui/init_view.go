package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)


// initView renders the initialization wizard
func (m Model) initView() string {
	if m.initState == nil {
		return "Initializing..."
	}

	switch m.initState.currentStep {
	case InitStepWelcome:
		return m.renderWelcomeStep()
	case InitStepAnalysis:
		return m.renderAnalysisStep()
	case InitStepRecommendations:
		return m.renderRecommendationsStep()
	case InitStepCustomization:
		return m.renderCustomizationStep()
	case InitStepPreview:
		return m.renderPreviewStep()
	case InitStepCreated:
		return m.renderCreatedStep()
	case InitStepExecuting:
		return m.renderExecutingStep()
	case InitStepComplete:
		return m.renderCompleteStep()
	case InitStepCommitConfirm:
		return m.renderCommitConfirmStep()
	case InitStepFinal:
		return m.renderFinalStep()
	default:
		return "Unknown step"
	}
}

// renderWelcomeStep shows the initial welcome
func (m Model) renderWelcomeStep() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("🚀 Welcome to gren"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("Let's set up worktree management for your project!"))
	content.WriteString("\n\n")

	if m.repoInfo != nil {
		content.WriteString(WorktreePathStyle.Render(fmt.Sprintf("📁 Project: %s", m.repoInfo.Name)))
		content.WriteString("\n")
		content.WriteString(WorktreePathStyle.Render(fmt.Sprintf("🌿 Current branch: %s", m.repoInfo.CurrentBranch)))
		content.WriteString("\n\n")
	}

	content.WriteString(WorktreePathStyle.Render("I'll analyze your project and suggest the best configuration."))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("You'll have full control to customize everything."))
	content.WriteString("\n\n")

	content.WriteString(StatusCleanStyle.Render("Ready to begin? This will take just a moment."))
	content.WriteString("\n\n")

	content.WriteString(HelpStyle.Render("[enter] Start analysis  [q] Quit"))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}

// renderAnalysisStep shows project analysis in progress
func (m Model) renderAnalysisStep() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("🔍 Analyzing Project"))
	content.WriteString("\n\n")

	if !m.initState.analysisComplete {
		content.WriteString(SpinnerStyle.Render("⠋ Scanning project structure..."))
		content.WriteString("\n")
		content.WriteString(WorktreePathStyle.Render("🔎 Looking for configuration files"))
		content.WriteString("\n")
		content.WriteString(WorktreePathStyle.Render("📦 Detecting package manager"))
		content.WriteString("\n")
		content.WriteString(WorktreePathStyle.Render("🌿 Checking git configuration"))
		content.WriteString("\n\n")

		content.WriteString(WorktreePathStyle.Render("This will only take a moment..."))
	} else {
		// Show analysis results
		content.WriteString(StatusCleanStyle.Render("📊 Analysis Complete!"))
		content.WriteString("\n\n")

		if m.repoInfo != nil {
			content.WriteString(WorktreeNameStyle.Render(fmt.Sprintf("📁 Project: %s (Go project)", m.repoInfo.Name)))
			content.WriteString("\n")
			content.WriteString(WorktreeBranchStyle.Render(fmt.Sprintf("🌿 Current branch: %s", m.repoInfo.CurrentBranch)))
			content.WriteString("\n")
		}

		content.WriteString(WorktreePathStyle.Render("📦 Package manager: None detected (Go project)"))
		content.WriteString("\n\n")

		// Show files that will be copied (only if there are any)
		if len(m.initState.detectedFiles) > 0 {
			content.WriteString(WorktreeNameStyle.Render("📋 Files to copy to new worktrees:"))
			content.WriteString("\n")
			for _, file := range m.initState.detectedFiles {
				line := fmt.Sprintf("   ✅ %s - %s", file.Path, file.Description)
				content.WriteString(WorktreePathStyle.Render(line))
				content.WriteString("\n")
			}
			content.WriteString("\n")
		}

		content.WriteString("\n")
		content.WriteString(HelpStyle.Render("[enter] Continue to recommendations"))
	}

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}

// renderRecommendationsStep shows smart recommendations
func (m Model) renderRecommendationsStep() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("📋 Setup Configuration"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("I'll create a setup script with smart defaults:"))
	content.WriteString("\n\n")

	// Show detected info
	content.WriteString(WorktreeNameStyle.Render("📁 Detected project info:"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render(fmt.Sprintf("   🌿 Worktree location: %s", m.initState.worktreeDir)))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render(fmt.Sprintf("   📦 Package manager: %s", m.initState.packageManager)))
	content.WriteString("\n")
	if m.initState.postCreateCmd != "" {
		content.WriteString(WorktreePathStyle.Render(fmt.Sprintf("   ⚡ Setup command: %s", m.initState.postCreateCmd)))
		content.WriteString("\n")
	}

	// Show detected files that will be copied (all are now gitignored)
	if len(m.initState.detectedFiles) > 0 {
		content.WriteString(WorktreePathStyle.Render(fmt.Sprintf("   📋 Found %d development files to copy", len(m.initState.detectedFiles))))
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(WorktreeNameStyle.Render("🛠️ What happens next:"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   1. Create setup script with all configuration"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   2. Ask if you want to open it in your editor"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   3. You can review and edit as needed"))
	content.WriteString("\n\n")

	content.WriteString(StatusCleanStyle.Render("Ready to create the setup script?"))
	content.WriteString("\n\n")

	content.WriteString(HelpStyle.Render("[enter] Create script  [esc] Back"))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}

// renderCreatedStep shows script creation confirmation
func (m Model) renderCreatedStep() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("✅ Setup Script Created!"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("Your setup script has been created with smart defaults:"))
	content.WriteString("\n\n")

	content.WriteString(WorktreePathStyle.Render("📁 .gren/config.json - Basic configuration"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("📜 .gren/post-create.sh - Setup script with your project settings"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("💡 The script includes:"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render(fmt.Sprintf("   • Worktree location: %s", m.initState.worktreeDir)))
	content.WriteString("\n")
	if m.initState.postCreateCmd != "" {
		content.WriteString(WorktreePathStyle.Render(fmt.Sprintf("   • Setup command: %s", m.initState.postCreateCmd)))
		content.WriteString("\n")
	}
	content.WriteString(WorktreePathStyle.Render(fmt.Sprintf("   • %d file patterns to copy", len(m.initState.copyPatterns))))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   • Configuration as comments for easy editing"))
	content.WriteString("\n\n")

	content.WriteString(StatusCleanStyle.Render("Would you like to open the script in your editor now?"))
	content.WriteString("\n\n")

	content.WriteString(HelpStyle.Render("[y] Yes, open in editor  [n] No, finish setup  [esc] Back"))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}

// renderCustomizationStep shows customization options
func (m Model) renderCustomizationStep() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("🛠️ Customize Setup"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("Customize your worktree configuration:"))
	content.WriteString("\n\n")

	// If we're in a specific mode, show that interface
	if m.initState.customizationMode == "worktree" {
		return m.renderWorktreeCustomization()
	} else if m.initState.customizationMode == "patterns" {
		return m.renderPatternsCustomization()
	} else if m.initState.customizationMode == "postcreate" {
		return m.renderPostCreateCustomization()
	} else if m.initState.customizationMode == "simplecommand" {
		return m.renderSimpleCommandEdit()
	}

	// Main customization menu
	options := []struct {
		name        string
		icon        string
		description string
		mode        string
	}{
		{"Worktree Location", "📂", fmt.Sprintf("Currently: %s", m.initState.worktreeDir), "worktree"},
		{"File Patterns", "📋", fmt.Sprintf("%d patterns configured", len(m.initState.copyPatterns)), "patterns"},
		{"Post-Create Command", "⚡", fmt.Sprintf("Currently: %s", m.initState.postCreateCmd), "postcreate"},
	}

	for i, option := range options {
		var style lipgloss.Style
		if i == m.initState.selected {
			style = WorktreeSelectedStyle
		} else {
			style = WorktreeItemStyle
		}

		optionText := fmt.Sprintf("%s %s", option.icon, option.name)
		content.WriteString(style.Render(optionText))
		content.WriteString("\n")
		content.WriteString(WorktreePathStyle.Render(fmt.Sprintf("   %s", option.description)))
		content.WriteString("\n\n")
	}

	content.WriteString(HelpStyle.Render("[enter] Edit selected  [↑↓] Navigate  [esc] Back"))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}

// renderWorktreeCustomization shows worktree directory editing
func (m Model) renderWorktreeCustomization() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("📂 Worktree Location"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("Configure where worktrees will be created:"))
	content.WriteString("\n\n")

	// Show current path being edited
	inputStyle := WorktreeItemStyle
	if m.initState.editingText != "" {
		inputStyle = WorktreeSelectedStyle
	}

	displayPath := m.initState.worktreeDir
	if m.initState.editingText != "" {
		displayPath = m.initState.editingText
	}

	pathInput := fmt.Sprintf("📁 %s▮", displayPath)
	content.WriteString(inputStyle.Width(m.width-8).Render(pathInput))
	content.WriteString("\n\n")

	content.WriteString(WorktreePathStyle.Render("💡 This will create: ../your-path/branch-name/"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("💡 Relative to current repository directory"))
	content.WriteString("\n\n")

	content.WriteString(HelpStyle.Render("[type] Edit path  [enter] Save  [esc] Cancel"))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}

// renderPatternsCustomization shows file pattern editing
func (m Model) renderPatternsCustomization() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("📋 File Patterns"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("Configure which files to copy to new worktrees:"))
	content.WriteString("\n\n")

	// Show patterns with toggle checkboxes
	for i, pattern := range m.initState.copyPatterns {
		var style lipgloss.Style
		if i == m.initState.selected {
			style = WorktreeSelectedStyle
		} else {
			style = WorktreeItemStyle
		}

		checkbox := "☐"
		if pattern.Enabled {
			checkbox = "✅"
		}

		detectedText := ""
		if pattern.Detected {
			detectedText = " (detected)"
		}

		patternText := fmt.Sprintf("%s %s - %s%s", checkbox, pattern.Pattern, pattern.Description, detectedText)
		content.WriteString(style.Render(patternText))
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("💡 Only gitignored files should be copied to avoid conflicts"))
	content.WriteString("\n\n")

	content.WriteString(HelpStyle.Render("[space] Toggle  [↑↓] Navigate  [enter] Done  [esc] Cancel"))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}

// renderPostCreateCustomization shows post-create command editing
func (m Model) renderPostCreateCustomization() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("⚡ Post-Create Setup"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("Choose how to configure post-create actions:"))
	content.WriteString("\n\n")

	// Two options: simple command or script file
	options := []struct {
		name        string
		icon        string
		description string
	}{
		{"Simple Command", "⚡", "Single command like 'npm install' or 'go mod tidy'"},
		{"Custom Script", "📝", "Open .gren/post-create.sh in external editor"},
	}

	for i, option := range options {
		var style lipgloss.Style
		if i == m.initState.selected {
			style = WorktreeSelectedStyle
		} else {
			style = WorktreeItemStyle
		}

		optionText := fmt.Sprintf("%s %s", option.icon, option.name)
		content.WriteString(style.Render(optionText))
		content.WriteString("\n")
		content.WriteString(WorktreePathStyle.Render(fmt.Sprintf("   %s", option.description)))
		content.WriteString("\n\n")
	}

	// Show current configuration
	content.WriteString(WorktreeNameStyle.Render("Current configuration:"))
	content.WriteString("\n")
	if m.initState.postCreateCmd != "" {
		content.WriteString(WorktreePathStyle.Render(fmt.Sprintf("⚡ Command: %s", m.initState.postCreateCmd)))
	} else {
		content.WriteString(WorktreePathStyle.Render("📝 Custom script (will be created)"))
	}
	content.WriteString("\n\n")

	content.WriteString(HelpStyle.Render("[enter] Select  [↑↓] Navigate  [esc] Back"))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}

// renderSimpleCommandEdit shows simple command editing
func (m Model) renderSimpleCommandEdit() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("⚡ Simple Command"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("Enter a single command to run after creating worktree:"))
	content.WriteString("\n\n")

	// Show current command being edited
	inputStyle := WorktreeItemStyle
	if m.initState.editingText != "" {
		inputStyle = WorktreeSelectedStyle
	}

	displayCmd := m.initState.postCreateCmd
	if m.initState.editingText != "" {
		displayCmd = m.initState.editingText
	}

	cmdInput := fmt.Sprintf("⚡ %s▮", displayCmd)
	content.WriteString(inputStyle.Width(m.width-8).Render(cmdInput))
	content.WriteString("\n\n")

	// Show suggestions based on detected package manager
	content.WriteString(WorktreeNameStyle.Render("💡 Common commands:"))
	content.WriteString("\n")

	suggestions := []string{
		"go mod tidy",
		"npm install",
		"bun install",
		"pnpm install",
		"yarn install",
		"pip install -r requirements.txt",
		"cargo check",
	}

	for _, suggestion := range suggestions {
		if suggestion == displayCmd {
			content.WriteString(WorktreeSelectedStyle.Render(fmt.Sprintf("  ✓ %s (current)", suggestion)))
		} else {
			content.WriteString(WorktreePathStyle.Render(fmt.Sprintf("  • %s", suggestion)))
		}
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(HelpStyle.Render("[type] Edit command  [enter] Save  [esc] Cancel"))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}

// renderPreviewStep shows final preview before execution
func (m Model) renderPreviewStep() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("📝 Preview Configuration"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("This will create:"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("├─ .gren/config.json"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("├─ .gren/post-create.sh"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("└─ Update .gitignore"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("🗂️ Git integration:"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("✅ Add .gren/ to .gitignore"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("✅ Commit configuration"))
	content.WriteString("\n\n")

	content.WriteString(StatusCleanStyle.Render("Ready to initialize?"))
	content.WriteString("\n\n")

	content.WriteString(HelpStyle.Render("[enter] Initialize  [esc] Back"))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}

// renderExecutingStep shows progress during execution
func (m Model) renderExecutingStep() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("🚀 Initializing gren"))
	content.WriteString("\n\n")

	content.WriteString(SpinnerStyle.Render("⠋ Creating configuration files..."))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("✅ Created .gren/config.json"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("⏳ Generating .gren/post-create.sh"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("⏸️ Updating .gitignore"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("⏸️ Committing to git"))
	content.WriteString("\n\n")

	content.WriteString(WorktreePathStyle.Render("This will only take a moment..."))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}

// renderWorktreeLocationStep shows worktree location configuration
func (m Model) renderWorktreeLocationStep() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("📂 Worktree Location"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("Where should worktrees be created?"))
	content.WriteString("\n\n")

	// Current selection
	locationStyle := WorktreeItemStyle
	if m.initState.selected == 0 {
		locationStyle = WorktreeSelectedStyle
	}

	location := fmt.Sprintf("📁 %s", m.initState.worktreeDir)
	content.WriteString(locationStyle.Render(location))
	content.WriteString("\n\n")

	content.WriteString(WorktreePathStyle.Render("This will create: ../gren-worktrees/feature-branch/"))
	content.WriteString("\n\n")

	content.WriteString(HelpStyle.Render("[enter] Continue  [e] Edit path  [esc] Back"))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}

// renderFilePatternsStep shows file copy configuration
func (m Model) renderFilePatternsStep() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("📋 Files to Copy"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("Select files to copy to new worktrees:"))
	content.WriteString("\n\n")

	// File patterns
	patterns := []struct {
		pattern     string
		description string
		enabled     bool
		detected    bool
	}{
		{".env*", "Environment files", true, true},
		{".claude/", "Claude configuration", true, true},
		{".nvmrc", "Node version", false, false},
		{".envrc", "Direnv configuration", false, false},
	}

	for i, p := range patterns {
		var style lipgloss.Style
		if i == m.initState.selected {
			style = WorktreeSelectedStyle
		} else {
			style = WorktreeItemStyle
		}

		checkbox := "☐"
		if p.enabled {
			checkbox = "✅"
		}

		detected := ""
		if p.detected {
			detected = " (detected)"
		}

		item := fmt.Sprintf("%s %s - %s%s", checkbox, p.pattern, p.description, detected)
		content.WriteString(style.Render(item))
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(HelpStyle.Render("[space] Toggle  [a] Add custom  [enter] Continue  [esc] Back"))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}

// renderPostCreateStep shows post-create hook configuration
func (m Model) renderPostCreateStep() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("📦 Post-Create Setup"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("What should happen after creating a worktree?"))
	content.WriteString("\n\n")

	// Setup options
	options := []struct {
		name        string
		description string
		enabled     bool
	}{
		{"Install dependencies", "Run bun install", true},
		{"Setup direnv", "Run direnv allow (if .envrc exists)", true},
		{"Custom script", "Run additional setup commands", false},
	}

	for i, opt := range options {
		var style lipgloss.Style
		if i == m.initState.selected {
			style = WorktreeSelectedStyle
		} else {
			style = WorktreeItemStyle
		}

		checkbox := "☐"
		if opt.enabled {
			checkbox = "✅"
		}

		item := fmt.Sprintf("%s %s - %s", checkbox, opt.name, opt.description)
		content.WriteString(style.Render(item))
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("💡 This will generate .gren/post-create.sh"))
	content.WriteString("\n\n")

	content.WriteString(HelpStyle.Render("[space] Toggle  [p] Preview script  [enter] Continue  [esc] Back"))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}

// renderGitIntegrationStep shows git integration options
func (m Model) renderGitIntegrationStep() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("📝 Git Integration"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("Commit configuration to git?"))
	content.WriteString("\n\n")

	// Git actions
	actions := []struct {
		name        string
		description string
		enabled     bool
	}{
		{"Add to .gitignore", "Add .gren/ to .gitignore", true},
		{"Commit config", "Commit .gren/config.json and post-create.sh", true},
	}

	for i, action := range actions {
		var style lipgloss.Style
		if i == m.initState.selected {
			style = WorktreeSelectedStyle
		} else {
			style = WorktreeItemStyle
		}

		checkbox := "☐"
		if action.enabled {
			checkbox = "✅"
		}

		item := fmt.Sprintf("%s %s - %s", checkbox, action.name, action.description)
		content.WriteString(style.Render(item))
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("📄 Commit message: \"Initialize gren worktree management\""))
	content.WriteString("\n\n")

	content.WriteString(HelpStyle.Render("[space] Toggle  [enter] Initialize  [esc] Back"))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}

// renderCompleteStep shows completion
func (m Model) renderCompleteStep() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("✅ Setup Complete!"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("gren is now configured and ready to use!"))
	content.WriteString("\n\n")

	content.WriteString(WorktreePathStyle.Render("📁 Configuration: .gren/config.json"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("📜 Setup script: .gren/post-create.sh"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("🛠️ What you can do now:"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   • Edit the setup script in your editor"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   • Create your first worktree with 'n'"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   • Commit the .gren/ configuration to git"))
	content.WriteString("\n\n")

	content.WriteString(StatusCleanStyle.Render("Configuration is ready!"))
	content.WriteString("\n\n")

	content.WriteString(HelpStyle.Render("[enter] Continue to commit setup"))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}

// renderCommitConfirmStep shows commit confirmation
func (m Model) renderCommitConfirmStep() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("📝 Commit Configuration"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("Should the .gren/ configuration be committed to git?"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("💡 Benefits of committing:"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   ✅ Configuration available in all worktrees"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   ✅ Team members get same setup"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   ✅ Post-create script works immediately"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("📁 Files to commit:"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   📄 .gren/config.json"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   📜 .gren/post-create.sh"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   📝 .gitignore (add .gren/ if needed)"))
	content.WriteString("\n\n")

	content.WriteString(StatusCleanStyle.Render("Commit message: \"Add gren worktree configuration\""))
	content.WriteString("\n\n")

	content.WriteString(HelpStyle.Render("[y] Yes, commit  [n] Skip commit  [esc] Back"))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}

// renderFinalStep shows final completion
func (m Model) renderFinalStep() string {
	var content strings.Builder

	content.WriteString(TitleStyle.Render("🎉 Setup Complete!"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("gren is now configured and ready to use!"))
	content.WriteString("\n\n")

	content.WriteString(WorktreePathStyle.Render("📁 Configuration: .gren/config.json"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("📜 Setup script: .gren/post-create.sh"))
	content.WriteString("\n\n")

	content.WriteString(WorktreeNameStyle.Render("🛠️ What you can do now:"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   • Create your first worktree with 'n'"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   • Edit the setup script if needed"))
	content.WriteString("\n")
	content.WriteString(WorktreePathStyle.Render("   • Share with your team"))
	content.WriteString("\n\n")

	content.WriteString(StatusCleanStyle.Render("Happy coding! 🚀"))
	content.WriteString("\n\n")

	content.WriteString(HelpStyle.Render("[enter] Return to dashboard  [q] Quit"))

	return HeaderStyle.Width(m.width - 4).Render(content.String())
}