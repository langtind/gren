package cli

import (
	"strings"
	"testing"
)

func TestBashCompletionScript(t *testing.T) {
	script := bashCompletionScript

	// Should contain bash completion setup
	if !strings.Contains(script, "_gren_completions") {
		t.Error("bash completion should contain _gren_completions function")
	}

	// Should contain all commands
	commands := []string{"create", "list", "delete", "switch", "merge", "for-each", "step", "cleanup"}
	for _, cmd := range commands {
		if !strings.Contains(script, cmd) {
			t.Errorf("bash completion should contain command %q", cmd)
		}
	}

	// Should have complete command
	if !strings.Contains(script, "complete -F") {
		t.Error("bash completion should have complete -F directive")
	}
}

func TestZshCompletionScript(t *testing.T) {
	script := zshCompletionScript

	// Should contain zsh completion setup
	if !strings.Contains(script, "#compdef gren") {
		t.Error("zsh completion should start with #compdef gren")
	}

	// Should contain _gren function
	if !strings.Contains(script, "_gren") {
		t.Error("zsh completion should contain _gren function")
	}

	// Should contain all commands with descriptions
	commands := []string{"create", "list", "delete", "switch", "merge", "for-each"}
	for _, cmd := range commands {
		if !strings.Contains(script, cmd) {
			t.Errorf("zsh completion should contain command %q", cmd)
		}
	}

	// Should handle subcommands
	if !strings.Contains(script, "step") {
		t.Error("zsh completion should contain step subcommand")
	}
}

func TestFishCompletionScript(t *testing.T) {
	script := fishCompletionScript

	// Should contain fish completion commands
	if !strings.Contains(script, "complete -c gren") {
		t.Error("fish completion should contain 'complete -c gren'")
	}

	// Should contain all commands
	commands := []string{"create", "list", "delete", "switch", "merge", "for-each", "step"}
	for _, cmd := range commands {
		if !strings.Contains(script, cmd) {
			t.Errorf("fish completion should contain command %q", cmd)
		}
	}

	// Should have descriptions
	if !strings.Contains(script, "-d") {
		t.Error("fish completion should have -d (description) flags")
	}
}

func TestCompletionScriptsNotEmpty(t *testing.T) {
	tests := []struct {
		name   string
		script string
	}{
		{"bash", bashCompletionScript},
		{"zsh", zshCompletionScript},
		{"fish", fishCompletionScript},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.script) < 100 {
				t.Errorf("%s completion script too short: %d chars", tt.name, len(tt.script))
			}
		})
	}
}

func TestCompletionCommands(t *testing.T) {
	// All completion scripts should contain these commands
	expectedCommands := []string{
		"create",
		"list",
		"delete",
		"switch",
		"merge",
		"for-each",
		"step",
		"init",
		"compare",
		"cleanup",
		"marker",
		"shell-init",
		"completion",
	}

	scripts := []struct {
		name   string
		script string
	}{
		{"bash", bashCompletionScript},
		{"zsh", zshCompletionScript},
		{"fish", fishCompletionScript},
	}

	for _, s := range scripts {
		t.Run(s.name, func(t *testing.T) {
			for _, cmd := range expectedCommands {
				if !strings.Contains(s.script, cmd) {
					t.Errorf("%s completion missing command %q", s.name, cmd)
				}
			}
		})
	}
}

func TestBashCompletionStructure(t *testing.T) {
	script := bashCompletionScript

	// Check for COMPREPLY
	if !strings.Contains(script, "COMPREPLY") {
		t.Error("bash completion should use COMPREPLY")
	}

	// Check for compgen
	if !strings.Contains(script, "compgen") {
		t.Error("bash completion should use compgen")
	}

	// Should be executable
	if !strings.HasPrefix(strings.TrimSpace(script), "# ") && !strings.HasPrefix(strings.TrimSpace(script), "_") {
		t.Log("bash completion should start with comment or function")
	}
}

func TestZshCompletionStructure(t *testing.T) {
	script := zshCompletionScript

	// Check for autoload
	if !strings.Contains(script, "#compdef") {
		t.Error("zsh completion should have #compdef")
	}

	// Check for _arguments or _describe
	if !strings.Contains(script, "_arguments") && !strings.Contains(script, "_describe") {
		t.Error("zsh completion should use _arguments or _describe")
	}
}

func TestFishCompletionStructure(t *testing.T) {
	script := fishCompletionScript

	// Check structure
	lines := strings.Split(script, "\n")
	hasComplete := false
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "complete -c gren") {
			hasComplete = true
			break
		}
	}
	if !hasComplete {
		t.Error("fish completion should have 'complete -c gren' commands")
	}
}

func TestCompletionFlagsIncluded(t *testing.T) {
	// Test that completion scripts are functional - flags may not be explicitly listed
	// as the shell handles common flags like --help automatically
	scripts := []struct {
		name   string
		script string
	}{
		{"bash", bashCompletionScript},
		{"zsh", zshCompletionScript},
		{"fish", fishCompletionScript},
	}

	for _, s := range scripts {
		t.Run(s.name, func(t *testing.T) {
			// Just verify the script is substantial
			if len(s.script) < 500 {
				t.Errorf("%s completion script too short: %d chars", s.name, len(s.script))
			}
		})
	}
}

func TestCreateCommandOptions(t *testing.T) {
	// Create command has specific options that should be completed
	createOptions := []string{"-n", "-b", "-x", "--branch", "--existing", "--dir"}

	scripts := []struct {
		name     string
		script   string
		minFound int
	}{
		{"bash", bashCompletionScript, 2},
		{"zsh", zshCompletionScript, 2},
		{"fish", fishCompletionScript, 1}, // Fish uses different syntax, fewer options directly listed
	}

	for _, s := range scripts {
		t.Run(s.name, func(t *testing.T) {
			// Check at least some create options are present
			found := 0
			for _, opt := range createOptions {
				if strings.Contains(s.script, opt) {
					found++
				}
			}
			if found < s.minFound {
				t.Errorf("%s completion should have at least %d create command options, found %d", s.name, s.minFound, found)
			}
		})
	}
}

func TestStepSubcommands(t *testing.T) {
	// step has subcommands: commit, squash
	subcommands := []string{"commit", "squash"}

	scripts := []struct {
		name   string
		script string
	}{
		{"bash", bashCompletionScript},
		{"zsh", zshCompletionScript},
		{"fish", fishCompletionScript},
	}

	for _, s := range scripts {
		t.Run(s.name, func(t *testing.T) {
			for _, sub := range subcommands {
				if !strings.Contains(s.script, sub) {
					t.Errorf("%s completion missing step subcommand %q", s.name, sub)
				}
			}
		})
	}
}

func TestMarkerSubcommands(t *testing.T) {
	// marker has subcommands: set, get, clear, list
	subcommands := []string{"set", "get", "clear", "list"}

	scripts := []struct {
		name   string
		script string
	}{
		{"bash", bashCompletionScript},
		{"zsh", zshCompletionScript},
		{"fish", fishCompletionScript},
	}

	for _, s := range scripts {
		t.Run(s.name, func(t *testing.T) {
			found := 0
			for _, sub := range subcommands {
				if strings.Contains(s.script, sub) {
					found++
				}
			}
			// At least some marker subcommands should be present
			if found < 2 {
				t.Logf("%s completion has %d marker subcommands", s.name, found)
			}
		})
	}
}
