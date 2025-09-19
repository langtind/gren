package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"gren/internal/ui"
)

func main() {
	// Create the model with dependencies
	m := ui.NewModel(nil, nil) // Use default dependencies

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