package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Modern color palette inspired by lazygit, gitui, and modern terminal aesthetics
var (
	// Base colors - softer, more refined
	ColorPrimary   = lipgloss.AdaptiveColor{Light: "#16a34a", Dark: "#22c55e"} // Green
	ColorSecondary = lipgloss.AdaptiveColor{Light: "#6366f1", Dark: "#818cf8"} // Indigo
	ColorAccent    = lipgloss.AdaptiveColor{Light: "#f59e0b", Dark: "#fbbf24"} // Amber
	ColorSuccess   = lipgloss.AdaptiveColor{Light: "#16a34a", Dark: "#4ade80"} // Green
	ColorWarning   = lipgloss.AdaptiveColor{Light: "#ea580c", Dark: "#fb923c"} // Orange
	ColorError     = lipgloss.AdaptiveColor{Light: "#dc2626", Dark: "#f87171"} // Red

	// Neutral colors
	ColorText       = lipgloss.AdaptiveColor{Light: "#1f2937", Dark: "#f3f4f6"}
	ColorTextMuted  = lipgloss.AdaptiveColor{Light: "#6b7280", Dark: "#9ca3af"}
	ColorTextSubtle = lipgloss.AdaptiveColor{Light: "#9ca3af", Dark: "#6b7280"}
	ColorBorder     = lipgloss.AdaptiveColor{Light: "#d1d5db", Dark: "#374151"}
	ColorBorderDim  = lipgloss.AdaptiveColor{Light: "#e5e7eb", Dark: "#1f2937"}
	ColorHighlight  = lipgloss.AdaptiveColor{Light: "#dbeafe", Dark: "#1e3a5f"}
	ColorSurface    = lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#111827"}

	// Special purpose
	ColorCurrentBranch = lipgloss.AdaptiveColor{Light: "#7c3aed", Dark: "#a78bfa"} // Purple for current
)

// Layout constants
const (
	HeaderHeight = 3
	FooterHeight = 2
)

// ═══════════════════════════════════════════════════════════════════════════
// Header Bar Styles
// ═══════════════════════════════════════════════════════════════════════════

var (
	HeaderBarStyle = lipgloss.NewStyle().
			Background(lipgloss.AdaptiveColor{Light: "#f3f4f6", Dark: "#1f2937"}).
			Padding(0, 2)

	LogoStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	HeaderInfoStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted)

	HeaderBranchStyle = lipgloss.NewStyle().
				Foreground(ColorSecondary).
				Bold(true)
)

// ═══════════════════════════════════════════════════════════════════════════
// Table Styles
// ═══════════════════════════════════════════════════════════════════════════

var (
	TableHeaderStyle = lipgloss.NewStyle().
				Foreground(ColorTextMuted).
				Bold(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderBottom(true).
				BorderForeground(ColorBorderDim).
				Padding(0, 1)

	TableRowStyle = lipgloss.NewStyle().
			Padding(0, 1)

	TableRowSelectedStyle = lipgloss.NewStyle().
				Background(ColorHighlight).
				Foreground(ColorText).
				Padding(0, 1)

	TableRowCurrentStyle = lipgloss.NewStyle().
				Background(lipgloss.AdaptiveColor{Light: "#ecfdf5", Dark: "#052e16"}).
				Padding(0, 1)

	TableRowCurrentSelectedStyle = lipgloss.NewStyle().
					Background(lipgloss.AdaptiveColor{Light: "#bbf7d0", Dark: "#14532d"}).
					Foreground(ColorText).
					Padding(0, 1)

	TableCellStyle = lipgloss.NewStyle().
			Padding(0, 1)
)

// ═══════════════════════════════════════════════════════════════════════════
// Worktree Item Styles
// ═══════════════════════════════════════════════════════════════════════════

var (
	WorktreeNameStyle = lipgloss.NewStyle().
				Foreground(ColorText).
				Bold(true)

	WorktreeNameCurrentStyle = lipgloss.NewStyle().
					Foreground(ColorCurrentBranch).
					Bold(true)

	WorktreePathStyle = lipgloss.NewStyle().
				Foreground(ColorTextSubtle)

	WorktreeBranchStyle = lipgloss.NewStyle().
				Foreground(ColorSecondary)
)

// ═══════════════════════════════════════════════════════════════════════════
// Status Badge Styles
// ═══════════════════════════════════════════════════════════════════════════

var (
	StatusCleanStyle = lipgloss.NewStyle().
				Foreground(ColorSuccess)

	StatusModifiedStyle = lipgloss.NewStyle().
				Foreground(ColorWarning)

	StatusUnpushedStyle = lipgloss.NewStyle().
				Foreground(ColorSecondary)

	StatusMissingStyle = lipgloss.NewStyle().
				Foreground(ColorError)

	StatusBuildingStyle = lipgloss.NewStyle().
				Foreground(ColorAccent)
)

// ═══════════════════════════════════════════════════════════════════════════
// Footer / Help Bar Styles
// ═══════════════════════════════════════════════════════════════════════════

var (
	FooterBarStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(ColorBorderDim).
			Padding(0, 1)

	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	HelpTextStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted)

	HelpSeparatorStyle = lipgloss.NewStyle().
				Foreground(ColorBorderDim)
)

// ═══════════════════════════════════════════════════════════════════════════
// Panel Styles
// ═══════════════════════════════════════════════════════════════════════════

var (
	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1, 2)

	PanelActiveStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary).
				Padding(1, 2)

	PanelTitleStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true).
			Padding(0, 1)
)

// ═══════════════════════════════════════════════════════════════════════════
// Dialog / Modal Styles
// ═══════════════════════════════════════════════════════════════════════════

var (
	DialogStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(1, 2)

	DialogTitleStyle = lipgloss.NewStyle().
				Foreground(ColorPrimary).
				Bold(true).
				MarginBottom(1)

	DialogTextStyle = lipgloss.NewStyle().
			Foreground(ColorText)
)

// ═══════════════════════════════════════════════════════════════════════════
// Form / Input Styles
// ═══════════════════════════════════════════════════════════════════════════

var (
	InputLabelStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted).
			MarginBottom(0)

	InputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1)

	InputFocusedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary).
				Padding(0, 1)
)

// ═══════════════════════════════════════════════════════════════════════════
// Menu / Selection Styles
// ═══════════════════════════════════════════════════════════════════════════

var (
	MenuItemStyle = lipgloss.NewStyle().
			Padding(0, 2)

	MenuItemSelectedStyle = lipgloss.NewStyle().
				Background(ColorHighlight).
				Foreground(ColorText).
				Padding(0, 2)

	MenuItemDisabledStyle = lipgloss.NewStyle().
				Foreground(ColorTextSubtle).
				Padding(0, 2)
)

// ═══════════════════════════════════════════════════════════════════════════
// Message / Alert Styles
// ═══════════════════════════════════════════════════════════════════════════

var (
	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorError).
			Bold(true)

	WarningStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	InfoStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary)
)

// ═══════════════════════════════════════════════════════════════════════════
// Legacy styles for backward compatibility
// ═══════════════════════════════════════════════════════════════════════════

var (
	// These maintain compatibility with existing code
	PrimaryColor    = lipgloss.Color("#22c55e") // Green (was teal)
	SecondaryColor  = lipgloss.Color("#818cf8")
	AccentColor     = lipgloss.Color("#fbbf24")
	ErrorColor      = lipgloss.Color("#f87171")
	MutedColor      = lipgloss.Color("#9ca3af")
	BackgroundColor = lipgloss.Color("#111827")
	TextColor       = lipgloss.Color("#f3f4f6")

	BaseStyle = lipgloss.NewStyle().
			Foreground(ColorText)

	HeaderStyle = PanelStyle.Copy()

	TitleStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted)

	SelectedMenuItemStyle = MenuItemSelectedStyle.Copy()

	WorktreeItemStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorBorder).
				Padding(0, 1).
				MarginBottom(0)

	WorktreeSelectedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary).
				Padding(0, 1).
				MarginBottom(0)

	HelpStyle = HelpTextStyle.Copy()

	ProgressStyle = lipgloss.NewStyle().
			Foreground(ColorPrimary)

	SpinnerStyle = lipgloss.NewStyle().
			Foreground(ColorAccent)
)

// ═══════════════════════════════════════════════════════════════════════════
// Helper Functions
// ═══════════════════════════════════════════════════════════════════════════

// HelpItem creates a formatted help item with key and description
func HelpItem(key, desc string) string {
	return HelpKeyStyle.Render(key) + HelpTextStyle.Render(" "+desc)
}

// HelpBar creates a formatted help bar from multiple items
func HelpBar(items ...string) string {
	separator := HelpSeparatorStyle.Render("  │  ")
	return strings.Join(items, separator)
}

// StatusBadge returns styled status text (simple version)
func StatusBadge(status string) string {
	switch status {
	case "clean":
		return StatusCleanStyle.Render("✓")
	case "modified":
		return StatusModifiedStyle.Render("M")
	case "untracked":
		return StatusModifiedStyle.Render("?")
	case "mixed":
		return StatusModifiedStyle.Render("M?")
	case "unpushed":
		return StatusUnpushedStyle.Render("↑")
	case "missing":
		return StatusMissingStyle.Render("✗")
	default:
		return StatusCleanStyle.Render("✓")
	}
}

// StatusBadgeDetailed returns styled status with counts in git-style format
// Uses: +N staged, ~N modified, ?N untracked, ↑N unpushed (like warp/lazygit)
func StatusBadgeDetailed(status string, staged, modified, untracked, unpushed int) string {
	var parts []string

	// Staged files (ready to commit) - green with +
	if staged > 0 {
		parts = append(parts, StatusCleanStyle.Render(fmt.Sprintf("+%d", staged)))
	}
	// Modified files (not staged) - yellow/orange with ~
	if modified > 0 {
		parts = append(parts, StatusModifiedStyle.Render(fmt.Sprintf("~%d", modified)))
	}
	// Untracked files - yellow/orange with ?
	if untracked > 0 {
		parts = append(parts, StatusModifiedStyle.Render(fmt.Sprintf("?%d", untracked)))
	}
	// Unpushed commits - blue with ↑
	if unpushed > 0 {
		parts = append(parts, StatusUnpushedStyle.Render(fmt.Sprintf("↑%d", unpushed)))
	}

	if len(parts) == 0 {
		return StatusCleanStyle.Render("✓")
	}

	return strings.Join(parts, " ")
}
