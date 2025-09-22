package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/langtind/gren/internal/config"
	"github.com/langtind/gren/internal/git"
	"github.com/langtind/gren/internal/ui"
)

// Version information - will be injected at build time
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	// Parse command line flags
	var showHelp = flag.Bool("help", false, "Show help message")
	var showVersion = flag.Bool("version", false, "Show version information")
	flag.Parse()

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

	if *showHelp {
		fmt.Println("gren - Git Worktree Manager")
		fmt.Printf("version %s\n", version)
		fmt.Println()
		fmt.Println("A TUI application for managing git worktrees efficiently.")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  gren            Start the interactive interface")
		fmt.Println("  gren --help     Show this help message")
		fmt.Println("  gren --version  Show version information")
		fmt.Println()
		fmt.Println("Controls:")
		fmt.Println("  ↑↓      Navigate between worktrees")
		fmt.Println("  Enter   Open selected worktree")
		fmt.Println("  n       Create new worktree")
		fmt.Println("  d       Delete worktrees")
		fmt.Println("  i       Initialize gren in this repository")
		fmt.Println("  q       Quit")
		return
	}

	// Create dependencies
	gitRepo := git.NewLocalRepository()
	configManager := config.NewManager()

	// Create the model with dependencies
	m := ui.NewModel(gitRepo, configManager)

	// Create the program
	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),       // Use alternate screen buffer
		tea.WithMouseCellMotion(), // Enable mouse support
	)

	// Run the program
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}