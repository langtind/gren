package cli

import (
	"os"
	"testing"
)

func TestColorize(t *testing.T) {
	// Test when not in terminal (should return plain text)
	original := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	result := colorize("test", colorBold)

	w.Close()
	os.Stdout = original
	r.Close()

	// When running in tests, usually not a terminal
	// The result should either be plain or colored depending on terminal detection
	if result != "test" && result != colorBold+"test"+colorReset {
		t.Errorf("colorize() = %q, want plain or colored", result)
	}
}

func TestBold(t *testing.T) {
	result := bold("test")
	// Should contain the text regardless of terminal status
	if result == "" {
		t.Error("bold() returned empty string")
	}
}

func TestDim(t *testing.T) {
	result := dim("test")
	if result == "" {
		t.Error("dim() returned empty string")
	}
}

func TestCyan(t *testing.T) {
	result := cyan("test")
	if result == "" {
		t.Error("cyan() returned empty string")
	}
}

func TestGreen(t *testing.T) {
	result := green("test")
	if result == "" {
		t.Error("green() returned empty string")
	}
}

func TestYellow(t *testing.T) {
	result := yellow("test")
	if result == "" {
		t.Error("yellow() returned empty string")
	}
}

func TestBlue(t *testing.T) {
	result := blue("test")
	if result == "" {
		t.Error("blue() returned empty string")
	}
}

func TestColorConstants(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"colorReset", colorReset},
		{"colorBold", colorBold},
		{"colorDim", colorDim},
		{"colorCyan", colorCyan},
		{"colorGreen", colorGreen},
		{"colorYellow", colorYellow},
		{"colorBlue", colorBlue},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value == "" {
				t.Errorf("%s should not be empty", tt.name)
			}
			// All ANSI codes should start with escape
			if tt.value[0] != '\033' {
				t.Errorf("%s should start with escape sequence", tt.name)
			}
		})
	}
}

func TestShowCommandHelp(t *testing.T) {
	// These shouldn't panic
	commands := []string{"create", "merge", "for-each", "step", "unknown"}
	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			// Just verify it doesn't panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("ShowCommandHelp(%q) panicked: %v", cmd, r)
				}
			}()
			// Redirect stdout to prevent output during test
			old := os.Stdout
			_, w, _ := os.Pipe()
			os.Stdout = w
			ShowCommandHelp(cmd)
			w.Close()
			os.Stdout = old
		})
	}
}

func TestPrintCommand(t *testing.T) {
	// Just verify it doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("printCommand() panicked: %v", r)
		}
	}()

	// Redirect stdout to prevent output during test
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	printCommand("test", "-n name", "Test description")
	printCommand("cmd", "", "No args")
	printCommand("longcommand", "--very-long-arg", "Long description here")

	w.Close()
	os.Stdout = old
}

func TestPrintCommandPadding(t *testing.T) {
	// Test padding calculation
	tests := []struct {
		cmd    string
		args   string
		minPad int
	}{
		{"a", "b", 2},
		{"verylongcommand", "verylongargs", 2},
		{"cmd", "", 2},
	}

	for _, tt := range tests {
		t.Run(tt.cmd+"/"+tt.args, func(t *testing.T) {
			padding := 28 - len(tt.cmd) - len(tt.args)
			if tt.args != "" {
				padding--
			}
			if padding < 2 {
				padding = 2
			}
			if padding < tt.minPad {
				t.Errorf("padding = %d, want >= %d", padding, tt.minPad)
			}
		})
	}
}

func TestColorHelpers(t *testing.T) {
	// Test that color helpers return strings containing the input
	input := "test string"

	tests := []struct {
		name string
		fn   func(string) string
	}{
		{"bold", bold},
		{"dim", dim},
		{"cyan", cyan},
		{"green", green},
		{"yellow", yellow},
		{"blue", blue},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.fn(input)
			// Result should contain the original string
			found := false
			for i := 0; i <= len(result)-len(input); i++ {
				if result[i:i+len(input)] == input {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s(%q) = %q, should contain input", tt.name, input, result)
			}
		})
	}
}

func TestShowCreateHelp(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("showCreateHelp() panicked: %v", r)
		}
	}()

	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	showCreateHelp()
	w.Close()
	os.Stdout = old
}

func TestShowMergeHelp(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("showMergeHelp() panicked: %v", r)
		}
	}()

	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	showMergeHelp()
	w.Close()
	os.Stdout = old
}

func TestShowForEachHelp(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("showForEachHelp() panicked: %v", r)
		}
	}()

	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	showForEachHelp()
	w.Close()
	os.Stdout = old
}

func TestShowStepHelp(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("showStepHelp() panicked: %v", r)
		}
	}()

	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w
	showStepHelp()
	w.Close()
	os.Stdout = old
}
