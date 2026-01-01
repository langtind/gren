package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/langtind/gren/internal/cli"
	"github.com/langtind/gren/internal/config"
	"github.com/langtind/gren/internal/git"
	"github.com/langtind/gren/internal/logging"
	"github.com/langtind/gren/internal/ui"
	"golang.org/x/term"
)

// Version information - will be injected at build time for GitHub releases
var (
	version = "dev" // Default for local development, overridden by ldflags in releases
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	// Initialize logging
	if err := logging.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to initialize logging: %v\n", err)
	}
	defer logging.Close()

	// Parse command line flags
	var showHelp = flag.Bool("help", false, "Show help message")
	var showVersion = flag.Bool("version", false, "Show version information")
	flag.Parse()

	logging.Info("gren %s started, args: %v", version, os.Args)

	if *showVersion {
		fmt.Printf("gren version %s\n", version)
		if commit != "unknown" {
			fmt.Printf("commit: %s\n", commit)
		}
		if date != "unknown" {
			fmt.Printf("built: %s\n", date)
		}
		return
	}

	// Create dependencies
	gitRepo := git.NewLocalRepository()
	configManager := config.NewManager()

	// Check for config migration (only if we're in an initialized project)
	if configManager.Exists() {
		checkAndPromptMigration(configManager)
	}

	// Check if we have CLI commands (anything beyond flags)
	args := os.Args
	cliArgs := []string{}

	// Filter out flag arguments to get command arguments
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if !(*showHelp) && !(*showVersion) && !strings.HasPrefix(arg, "-") {
			cliArgs = append(cliArgs, args[i:]...)
			break
		}
	}

	// If we have CLI commands, use CLI mode
	if len(cliArgs) > 0 {
		cliHandler := cli.NewCLI(gitRepo, configManager)
		if err := cliHandler.ParseAndExecute(append([]string{"gren"}, cliArgs...)); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Show help if requested or if no commands provided
	if *showHelp {
		cliHandler := cli.NewCLI(gitRepo, configManager)
		cliHandler.ShowColoredHelp()
		return
	}

	// Default to TUI mode
	// Create the model with dependencies
	m := ui.NewModel(gitRepo, configManager, version)

	// Create the program
	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),       // Use alternate screen buffer
		tea.WithMouseCellMotion(), // Enable mouse support
	)

	// Run the program
	finalModel, err := p.Run()
	if err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}

	// Print exit message if set (e.g., after navigation)
	if model, ok := finalModel.(ui.Model); ok && model.ExitMessage != "" {
		fmt.Println(model.ExitMessage)
	}
}

// checkAndPromptMigration checks if config needs migration and prompts the user.
func checkAndPromptMigration(configManager *config.Manager) {
	// Skip interactive prompt if not running in a terminal
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return
	}

	needsMigration, result, err := configManager.NeedsMigration()
	if err != nil {
		logging.Error("Failed to check migration status: %v", err)
		return
	}

	if !needsMigration {
		return
	}

	// Build migration message
	var changes []string
	if result.WasJSON {
		changes = append(changes, "JSON → TOML")
	}
	if result.OldVersion != config.CurrentConfigVersion {
		changes = append(changes, fmt.Sprintf("v%s → v%s", result.OldVersion, config.CurrentConfigVersion))
	}

	fmt.Printf("⚙️  Config update available (%s)\n", strings.Join(changes, ", "))
	fmt.Print("Update config? [Y/n]: ")

	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.ToLower(strings.TrimSpace(response))

	// Default is yes (empty, "y", or "yes")
	if response == "" || response == "y" || response == "yes" {
		migrationResult, err := configManager.Migrate()
		if err != nil {
			fmt.Printf("❌ Migration failed: %v\n", err)
			logging.Error("Config migration failed: %v", err)
			return
		}

		fmt.Print("✅ Config updated")
		if len(migrationResult.FieldsMigrated) > 0 {
			fmt.Printf(" (%s)", strings.Join(migrationResult.FieldsMigrated, ", "))
		}
		fmt.Println()
	}
}
