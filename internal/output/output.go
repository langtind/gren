// Package output provides styled terminal output for the CLI
package output

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Symbols for message prefixes
const (
	SymbolSuccess  = "✓"
	SymbolError    = "✗"
	SymbolWarning  = "▲"
	SymbolProgress = "◎"
	SymbolHint     = "↳"
	SymbolInfo     = "○"
	SymbolBranch   = "⎇"
	SymbolFolder   = "📁"
	SymbolTree     = "🌳"
)

// Styles for different message types
var (
	// Colors
	colorSuccess = lipgloss.Color("#22c55e") // green
	colorError   = lipgloss.Color("#ef4444") // red
	colorWarning = lipgloss.Color("#eab308") // yellow
	colorCyan    = lipgloss.Color("#06b6d4") // cyan
	colorDim     = lipgloss.Color("#6b7280") // gray
	colorBold    = lipgloss.Color("#ffffff") // white

	// Styled symbols
	successStyle  = lipgloss.NewStyle().Foreground(colorSuccess)
	errorStyle    = lipgloss.NewStyle().Foreground(colorError)
	warningStyle  = lipgloss.NewStyle().Foreground(colorWarning)
	progressStyle = lipgloss.NewStyle().Foreground(colorCyan)
	hintStyle     = lipgloss.NewStyle().Foreground(colorDim)
	infoStyle     = lipgloss.NewStyle().Foreground(colorDim)

	// Text styles
	boldStyle   = lipgloss.NewStyle().Bold(true)
	dimStyle    = lipgloss.NewStyle().Foreground(colorDim)
	cyanStyle   = lipgloss.NewStyle().Foreground(colorCyan)
	greenStyle  = lipgloss.NewStyle().Foreground(colorSuccess)
	yellowStyle = lipgloss.NewStyle().Foreground(colorWarning)
	redStyle    = lipgloss.NewStyle().Foreground(colorError)

	// Header style
	headerStyle = lipgloss.NewStyle().
			Foreground(colorCyan).
			Bold(true)
)

// Success prints a success message with green checkmark
func Success(message string) {
	fmt.Println(successStyle.Render(SymbolSuccess) + " " + greenStyle.Render(message))
}

// Successf prints a formatted success message
func Successf(format string, args ...interface{}) {
	Success(fmt.Sprintf(format, args...))
}

// Error prints an error message with red X
func Error(message string) {
	fmt.Fprintln(os.Stderr, errorStyle.Render(SymbolError)+" "+redStyle.Render(message))
}

// Errorf prints a formatted error message
func Errorf(format string, args ...interface{}) {
	Error(fmt.Sprintf(format, args...))
}

// Warning prints a warning message with yellow triangle
func Warning(message string) {
	fmt.Println(warningStyle.Render(SymbolWarning) + " " + yellowStyle.Render(message))
}

// Warningf prints a formatted warning message
func Warningf(format string, args ...interface{}) {
	Warning(fmt.Sprintf(format, args...))
}

// Progress prints a progress message with cyan circle
func Progress(message string) {
	fmt.Println(progressStyle.Render(SymbolProgress) + " " + cyanStyle.Render(message))
}

// Progressf prints a formatted progress message
func Progressf(format string, args ...interface{}) {
	Progress(fmt.Sprintf(format, args...))
}

// Hint prints a hint message with dim arrow
func Hint(message string) {
	fmt.Println(hintStyle.Render(SymbolHint) + " " + dimStyle.Render(message))
}

// Hintf prints a formatted hint message
func Hintf(format string, args ...interface{}) {
	Hint(fmt.Sprintf(format, args...))
}

// Info prints an info message with dim circle
func Info(message string) {
	fmt.Println(infoStyle.Render(SymbolInfo) + " " + message)
}

// Infof prints a formatted info message
func Infof(format string, args ...interface{}) {
	Info(fmt.Sprintf(format, args...))
}

// Header prints a section header
func Header(title string) {
	fmt.Println(headerStyle.Render(title))
}

// Bold returns bolded text
func Bold(text string) string {
	return boldStyle.Render(text)
}

// Dim returns dimmed text
func Dim(text string) string {
	return dimStyle.Render(text)
}

// Cyan returns cyan text
func Cyan(text string) string {
	return cyanStyle.Render(text)
}

// Green returns green text
func Green(text string) string {
	return greenStyle.Render(text)
}

// Yellow returns yellow text
func Yellow(text string) string {
	return yellowStyle.Render(text)
}

// Red returns red text
func Red(text string) string {
	return redStyle.Render(text)
}

// Path formats a file path (shortens home directory to ~)
func Path(p string) string {
	home, err := os.UserHomeDir()
	if err == nil && strings.HasPrefix(p, home) {
		p = "~" + strings.TrimPrefix(p, home)
	}
	return dimStyle.Render(p)
}

// Branch formats a branch name
func Branch(name string) string {
	return boldStyle.Render(name)
}

// KeyValue prints a key-value pair with proper formatting
func KeyValue(key, value string) {
	fmt.Printf("%s %s\n", dimStyle.Render(key+":"), value)
}

// ListItem prints a list item with bullet
func ListItem(text string, current bool) {
	prefix := "  "
	if current {
		prefix = greenStyle.Render("▸ ")
	}
	fmt.Printf("%s%s\n", prefix, text)
}

// Blank prints an empty line
func Blank() {
	fmt.Println()
}

// WorktreeHeader prints the header for worktree operations
func WorktreeHeader(repoName string) {
	fmt.Println(cyanStyle.Render(SymbolTree) + " " + Bold("Git Worktree Manager"))
	KeyValue("Project", repoName)
	Blank()
}

// WorktreeCreated prints the success message after creating a worktree
func WorktreeCreated(name, branch, path string) {
	Success("Worktree created")
	Blank()
	KeyValue("Branch", Branch(branch))
	KeyValue("Path", Path(path))
}

// WorktreeSwitched prints info when switching to a worktree
func WorktreeSwitched(name, path string) {
	Successf("Switched to %s", Bold(name))
	KeyValue("Path", Path(path))
}

// WorktreeRemoved prints info when removing a worktree
func WorktreeRemoved(name string) {
	Successf("Removed worktree %s", Bold(name))
}

// WorktreeList prints a formatted list of worktrees
type WorktreeListItem struct {
	Name      string
	Branch    string
	Path      string
	IsCurrent bool
	IsMain    bool
	StaleInfo string
	PRInfo    string
	CIStatus  string
	Status    string
}

// PrintWorktreeList prints a nicely formatted worktree list
func PrintWorktreeList(items []WorktreeListItem, repoName string) {
	WorktreeHeader(repoName)

	for i, item := range items {
		prefix := "  "
		if item.IsCurrent {
			prefix = greenStyle.Render("▸ ")
		}

		// Build the main line
		name := item.Name
		if item.IsMain {
			name = boldStyle.Render(name) + dimStyle.Render(" (main)")
		} else {
			name = boldStyle.Render(name)
		}

		// Add branch if different from name
		if item.Branch != item.Name && item.Branch != "" {
			name += " " + dimStyle.Render("on") + " " + cyanStyle.Render(item.Branch)
		}

		// Add status indicators
		var indicators []string

		if item.Status != "" && item.Status != "clean" {
			indicators = append(indicators, yellowStyle.Render(item.Status))
		}

		if item.StaleInfo != "" {
			indicators = append(indicators, dimStyle.Render("stale: "+item.StaleInfo))
		}

		if item.PRInfo != "" {
			indicators = append(indicators, cyanStyle.Render(item.PRInfo))
		}

		if item.CIStatus != "" {
			ciIcon := ""
			switch item.CIStatus {
			case "success":
				ciIcon = greenStyle.Render("●")
			case "failure":
				ciIcon = redStyle.Render("●")
			case "pending":
				ciIcon = yellowStyle.Render("●")
			}
			if ciIcon != "" {
				indicators = append(indicators, ciIcon)
			}
		}

		indicatorStr := ""
		if len(indicators) > 0 {
			indicatorStr = " " + dimStyle.Render("[") + strings.Join(indicators, " ") + dimStyle.Render("]")
		}

		fmt.Printf("%s%s%s\n", prefix, name, indicatorStr)

		// Add path on second line for verbose mode or current worktree
		if item.IsCurrent || i == 0 {
			fmt.Printf("   %s\n", Path(item.Path))
		}
	}
}

// PrintSimpleWorktreeList prints a simple worktree list (for non-verbose output)
func PrintSimpleWorktreeList(items []WorktreeListItem) {
	for _, item := range items {
		prefix := "  "
		if item.IsCurrent {
			prefix = greenStyle.Render("▸ ")
		}

		name := item.Name

		// Add stale info
		staleInfo := ""
		if item.StaleInfo != "" {
			staleInfo = " " + dimStyle.Render("[stale: "+item.StaleInfo+"]")
		}

		// Add CI status
		ciIcon := ""
		switch item.CIStatus {
		case "success":
			ciIcon = " " + greenStyle.Render("●")
		case "failure":
			ciIcon = " " + redStyle.Render("●")
		case "pending":
			ciIcon = " " + yellowStyle.Render("●")
		}

		fmt.Printf("%s%s%s%s\n", prefix, name, staleInfo, ciIcon)
	}
}

// RepoName extracts the repo name from a path
func RepoName(repoPath string) string {
	return filepath.Base(repoPath)
}
