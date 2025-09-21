package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"gren/internal/config"
	"gren/internal/git"
	"gren/internal/ui"
)

func main() {
	// Parse command line flags
	var showHelp = flag.Bool("help", false, "Show help message")
	flag.Parse()

	if *showHelp {
		fmt.Println("gren - Git Worktree Manager")
		fmt.Println()
		fmt.Println("A TUI application for managing git worktrees efficiently.")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  gren          Start the interactive interface")
		fmt.Println("  gren --help   Show this help message")
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