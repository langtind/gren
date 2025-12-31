package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ═══════════════════════════════════════════════════════════════════════════
// COLOR PALETTE - Edit these to change the entire app's look
// ═══════════════════════════════════════════════════════════════════════════

var (
	// Brand / Primary colors
	ColorPrimary   = lipgloss.AdaptiveColor{Light: "#16a34a", Dark: "#22c55e"} // Green - main brand color
	ColorSecondary = lipgloss.AdaptiveColor{Light: "#6366f1", Dark: "#818cf8"} // Indigo - accent color
	ColorAccent    = lipgloss.AdaptiveColor{Light: "#f59e0b", Dark: "#fbbf24"} // Amber - highlights

	// Semantic colors
	ColorSuccess = lipgloss.AdaptiveColor{Light: "#16a34a", Dark: "#4ade80"} // Green - positive status
	ColorWarning = lipgloss.AdaptiveColor{Light: "#ea580c", Dark: "#fb923c"} // Orange - warnings
	ColorError   = lipgloss.AdaptiveColor{Light: "#dc2626", Dark: "#f87171"} // Red - errors

	// Text hierarchy (light to dark on dark theme)
	ColorTextPrimary   = lipgloss.AdaptiveColor{Light: "#1f2937", Dark: "#f9fafb"} // Brightest - names, important
	ColorTextSecondary = lipgloss.AdaptiveColor{Light: "#374151", Dark: "#e5e7eb"} // Slightly dimmer
	ColorTextMuted     = lipgloss.AdaptiveColor{Light: "#6b7280", Dark: "#9ca3af"} // Dimmer - timestamps, labels
	ColorTextSubtle    = lipgloss.AdaptiveColor{Light: "#9ca3af", Dark: "#9ca3af"} // Subtle but still readable - paths

	// Backgrounds
	ColorBgNormal          = lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#111827"} // Normal background
	ColorBgSelected        = lipgloss.AdaptiveColor{Light: "#dbeafe", Dark: "#1e3a5f"} // Selected row (blue tint)
	ColorBgCurrent         = lipgloss.AdaptiveColor{Light: "#ecfdf5", Dark: "#052e16"} // Current worktree (green tint)
	ColorBgCurrentSelected = lipgloss.AdaptiveColor{Light: "#bbf7d0", Dark: "#14532d"} // Current + selected

	// Borders
	ColorBorder    = lipgloss.AdaptiveColor{Light: "#d1d5db", Dark: "#374151"} // Normal borders
	ColorBorderDim = lipgloss.AdaptiveColor{Light: "#e5e7eb", Dark: "#1f2937"} // Subtle borders
)

// ═══════════════════════════════════════════════════════════════════════════
// DASHBOARD TABLE COLORS - Specific colors for each column
// ═══════════════════════════════════════════════════════════════════════════

var (
	// Column text colors - tweak these to adjust dashboard appearance
	DashboardNameColor        = ColorTextPrimary // Worktree name - brightest
	DashboardNameCurrentColor = ColorPrimary     // Current worktree name - green
	DashboardBranchColor      = ColorSecondary   // Branch name - indigo
	DashboardCommitColor      = ColorTextMuted   // Last commit time - muted
	DashboardPathColor        = ColorTextMuted   // File path - same as commit for readability
	DashboardMainTagColor     = ColorTextSubtle  // [main] tag - slightly dimmer

	// Header color
	DashboardHeaderColor = lipgloss.AdaptiveColor{Light: "#ffffff", Dark: "#ffffff"} // White
)

// Legacy aliases for backward compatibility
var (
	ColorText          = ColorTextPrimary
	ColorHighlight     = ColorBgSelected
	ColorSurface       = ColorBgNormal
	ColorCurrentBranch = lipgloss.AdaptiveColor{Light: "#7c3aed", Dark: "#a78bfa"} // Purple
)

// Layout constants
const (
	HeaderHeight = 8 // Logo (6 lines) + spacing
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
				Foreground(DashboardHeaderColor).
				Bold(true).
				Padding(0, 1)

	TableRowStyle = lipgloss.NewStyle().
			Padding(0, 1)

	TableRowSelectedStyle = lipgloss.NewStyle().
				Background(ColorBgSelected).
				Padding(0, 1)

	TableRowCurrentStyle = lipgloss.NewStyle().
				Background(ColorBgCurrent).
				Padding(0, 1)

	TableRowCurrentSelectedStyle = lipgloss.NewStyle().
					Background(ColorBgCurrentSelected).
					Padding(0, 1)

	TableCellStyle = lipgloss.NewStyle().
			Padding(0, 1)
)

// ═══════════════════════════════════════════════════════════════════════════
// Dashboard Column Styles - Used in renderWorktreeRow
// ═══════════════════════════════════════════════════════════════════════════

var (
	// Name column
	DashboardNameStyle = lipgloss.NewStyle().
				Foreground(DashboardNameColor).
				Bold(true)

	DashboardNameCurrentStyle = lipgloss.NewStyle().
					Foreground(DashboardNameCurrentColor).
					Bold(true)

	// Branch column
	DashboardBranchStyle = lipgloss.NewStyle().
				Foreground(DashboardBranchColor)

	// Last commit column
	DashboardCommitStyle = lipgloss.NewStyle().
				Foreground(DashboardCommitColor)

	// Path column
	DashboardPathStyle = lipgloss.NewStyle().
				Foreground(DashboardPathColor)

	// Main tag style
	DashboardMainTagStyle = lipgloss.NewStyle().
				Foreground(DashboardMainTagColor)
)

// Legacy worktree styles (for backward compatibility)
var (
	WorktreeNameStyle        = DashboardNameStyle
	WorktreeNameCurrentStyle = DashboardNameCurrentStyle
	WorktreeBranchStyle      = DashboardBranchStyle
	WorktreePathStyle        = DashboardPathStyle
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

	StatusWarningStyle = lipgloss.NewStyle().
				Foreground(ColorWarning)
)

// ═══════════════════════════════════════════════════════════════════════════
// Footer / Help Bar Styles
// ═══════════════════════════════════════════════════════════════════════════

var (
	FooterBarStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(ColorBorder).
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
			Foreground(ColorTextPrimary)
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
				Background(ColorBgSelected).
				Foreground(ColorTextPrimary).
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
			Foreground(ColorTextPrimary)

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
// bgColor is optional - pass empty AdaptiveColor{} for no background
func StatusBadgeDetailed(status, branchStatus string, staged, modified, untracked, unpushed, prNumber int, prState string, bgColor lipgloss.AdaptiveColor) string {
	var parts []string

	// Create styles with background color to maintain row background
	cleanStyle := StatusCleanStyle
	modifiedStyle := StatusModifiedStyle
	unpushedStyle := StatusUnpushedStyle
	staleStyle := lipgloss.NewStyle().Foreground(ColorTextMuted)

	// PR state colors
	prOpenStyle := lipgloss.NewStyle().Foreground(ColorSuccess)
	prMergedStyle := lipgloss.NewStyle().Foreground(ColorSecondary) // Indigo/purple for merged
	prClosedStyle := lipgloss.NewStyle().Foreground(ColorError)

	if bgColor.Dark != "" || bgColor.Light != "" {
		cleanStyle = cleanStyle.Background(bgColor)
		modifiedStyle = modifiedStyle.Background(bgColor)
		unpushedStyle = unpushedStyle.Background(bgColor)
		staleStyle = staleStyle.Background(bgColor)
		prOpenStyle = prOpenStyle.Background(bgColor)
		prMergedStyle = prMergedStyle.Background(bgColor)
		prClosedStyle = prClosedStyle.Background(bgColor)
	}

	// Stale branch indicator (merged or remote gone)
	if branchStatus == "stale" {
		parts = append(parts, staleStyle.Render("💤"))
	}

	// Git status indicators (always show, even for stale branches)
	// Staged files (ready to commit) - green with +
	if staged > 0 {
		prefix := ""
		if len(parts) > 0 {
			prefix = " "
		}
		parts = append(parts, cleanStyle.Render(fmt.Sprintf("%s+%d", prefix, staged)))
	}
	// Modified files (not staged) - yellow/orange with ~
	if modified > 0 {
		prefix := ""
		if len(parts) > 0 {
			prefix = " "
		}
		parts = append(parts, modifiedStyle.Render(fmt.Sprintf("%s~%d", prefix, modified)))
	}
	// Untracked files - yellow/orange with ?
	if untracked > 0 {
		prefix := ""
		if len(parts) > 0 {
			prefix = " "
		}
		parts = append(parts, modifiedStyle.Render(fmt.Sprintf("%s?%d", prefix, untracked)))
	}
	// Unpushed commits - blue with ↑
	if unpushed > 0 {
		prefix := ""
		if len(parts) > 0 {
			prefix = " "
		}
		parts = append(parts, unpushedStyle.Render(fmt.Sprintf("%s↑%d", prefix, unpushed)))
	}

	// Clean status if no other indicators and not stale
	if len(parts) == 0 {
		parts = append(parts, cleanStyle.Render("✓"))
	}

	// Add PR badge if PR exists
	if prNumber > 0 {
		prefix := " "
		prBadge := fmt.Sprintf("#%d", prNumber)
		switch prState {
		case "OPEN":
			parts = append(parts, prOpenStyle.Render(prefix+prBadge))
		case "MERGED":
			parts = append(parts, prMergedStyle.Render(prefix+prBadge))
		case "CLOSED":
			parts = append(parts, prClosedStyle.Render(prefix+prBadge))
		default:
			parts = append(parts, prOpenStyle.Render(prefix+prBadge))
		}
	}

	// Join without separator - spacing is included in each part
	return strings.Join(parts, "")
}

func CIStatusBadge(ciStatus string, bgColor lipgloss.AdaptiveColor) string {
	if ciStatus == "" {
		return ""
	}

	var style lipgloss.Style
	var symbol string

	switch ciStatus {
	case "success":
		symbol = "●"
		style = lipgloss.NewStyle().Foreground(ColorSuccess)
	case "failure":
		symbol = "●"
		style = lipgloss.NewStyle().Foreground(ColorError)
	case "pending":
		symbol = "●"
		style = lipgloss.NewStyle().Foreground(ColorWarning)
	default:
		symbol = "○"
		style = lipgloss.NewStyle().Foreground(ColorTextMuted)
	}

	if bgColor.Dark != "" || bgColor.Light != "" {
		style = style.Background(bgColor)
	}

	return style.Render(symbol)
}

// ═══════════════════════════════════════════════════════════════════════════
// Wizard / View Styles
// ═══════════════════════════════════════════════════════════════════════════

var (
	// WizardContainerStyle wraps wizard content with consistent padding
	WizardContainerStyle = lipgloss.NewStyle().
				Padding(1, 2)

	// WizardTitleStyle for main wizard titles
	WizardTitleStyle = lipgloss.NewStyle().
				Foreground(ColorPrimary).
				Bold(true).
				MarginBottom(1)

	// WizardSubtitleStyle for secondary titles
	WizardSubtitleStyle = lipgloss.NewStyle().
				Foreground(ColorTextPrimary).
				Bold(true)

	// WizardDescStyle for descriptions
	WizardDescStyle = lipgloss.NewStyle().
			Foreground(ColorTextMuted)

	// WizardInputStyle for text input fields
	WizardInputStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary).
				Padding(0, 1)

	// WizardListItemStyle for list items
	WizardListItemStyle = lipgloss.NewStyle().
				Foreground(ColorTextPrimary)

	// WizardListItemSelectedStyle for selected list items
	WizardListItemSelectedStyle = lipgloss.NewStyle().
					Foreground(ColorPrimary).
					Bold(true)

	// WizardWarningStyle for warning boxes
	WizardWarningStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorWarning).
				Foreground(ColorWarning).
				Padding(0, 1)

	// WizardSuccessStyle for success messages
	WizardSuccessStyle = lipgloss.NewStyle().
				Foreground(ColorSuccess).
				Bold(true)

	// WizardDangerStyle for dangerous action confirmations
	WizardDangerStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorError).
				Foreground(ColorError).
				Padding(0, 1)

	// ConfirmModalStyle for confirmation modals
	ConfirmModalStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorError).
				Padding(1, 2)
)

// WizardHeader creates a styled wizard header with title
func WizardHeader(title string) string {
	return WizardTitleStyle.Render(title)
}

// WizardOption renders a selectable option in a list
func WizardOption(label string, selected bool) string {
	prefix := "  "
	if selected {
		prefix = "▶ "
		return WizardListItemSelectedStyle.Render(prefix + label)
	}
	return WizardListItemStyle.Render(prefix + label)
}

// WizardHelpBar creates a help bar for wizard views
func WizardHelpBar(items ...string) string {
	separator := HelpSeparatorStyle.Render(" • ")
	var styledItems []string
	for _, item := range items {
		styledItems = append(styledItems, HelpTextStyle.Render(item))
	}
	return strings.Join(styledItems, separator)
}
