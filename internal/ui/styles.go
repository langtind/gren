package ui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors inspired by Git and nature
	PrimaryColor    = lipgloss.Color("#22c55e") // Green
	SecondaryColor  = lipgloss.Color("#3b82f6") // Blue
	AccentColor     = lipgloss.Color("#f59e0b") // Amber
	ErrorColor      = lipgloss.Color("#ef4444") // Red
	MutedColor      = lipgloss.Color("#6b7280") // Gray
	BackgroundColor = lipgloss.Color("#111827") // Dark
	TextColor       = lipgloss.Color("#f9fafb") // Light

	// Base styles
	BaseStyle = lipgloss.NewStyle().
			Foreground(TextColor).
			Background(BackgroundColor)

	// Header styles
	HeaderStyle = lipgloss.NewStyle().
			Foreground(PrimaryColor).
			Bold(true).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(PrimaryColor).
			Padding(0, 1).
			MarginBottom(1)

	TitleStyle = lipgloss.NewStyle().
			Foreground(PrimaryColor).
			Bold(true).
			Align(lipgloss.Center)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			Align(lipgloss.Center).
			MarginBottom(1)

	// Menu and selection styles
	MenuItemStyle = lipgloss.NewStyle().
			Padding(0, 2).
			MarginBottom(1)

	SelectedMenuItemStyle = lipgloss.NewStyle().
				Foreground(BackgroundColor).
				Background(PrimaryColor).
				Bold(true).
				Padding(0, 2).
				MarginBottom(1)

	// Worktree list styles
	WorktreeItemStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(MutedColor).
				Padding(1, 2).
				MarginBottom(1)

	WorktreeSelectedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(PrimaryColor).
				Padding(1, 2).
				MarginBottom(1)

	WorktreeNameStyle = lipgloss.NewStyle().
				Foreground(PrimaryColor).
				Bold(true)

	WorktreePathStyle = lipgloss.NewStyle().
				Foreground(MutedColor)

	WorktreeBranchStyle = lipgloss.NewStyle().
				Foreground(SecondaryColor)

	// Status styles
	StatusCleanStyle = lipgloss.NewStyle().
				Foreground(PrimaryColor).
				Bold(true)

	StatusModifiedStyle = lipgloss.NewStyle().
				Foreground(AccentColor).
				Bold(true)

	StatusBuildingStyle = lipgloss.NewStyle().
				Foreground(SecondaryColor).
				Bold(true)

	// Help text styles
	HelpStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			Align(lipgloss.Center).
			MarginTop(1)

	// Progress and loading styles
	ProgressStyle = lipgloss.NewStyle().
			Foreground(PrimaryColor)

	SpinnerStyle = lipgloss.NewStyle().
			Foreground(AccentColor)

	// Error styles
	ErrorStyle = lipgloss.NewStyle().
			Foreground(ErrorColor).
			Bold(true).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ErrorColor).
			Padding(0, 1)
)
