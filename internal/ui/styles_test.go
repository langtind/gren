package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestHelpItem(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		desc     string
		wantKey  string
		wantDesc string
	}{
		{
			name:     "simple help item",
			key:      "q",
			desc:     "quit",
			wantKey:  "q",
			wantDesc: "quit",
		},
		{
			name:     "compound key",
			key:      "↑/k",
			desc:     "move up",
			wantKey:  "↑/k",
			wantDesc: "move up",
		},
		{
			name:     "enter key",
			key:      "enter",
			desc:     "select",
			wantKey:  "enter",
			wantDesc: "select",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HelpItem(tt.key, tt.desc)

			// The result should contain both key and description
			if !strings.Contains(result, tt.wantKey) {
				t.Errorf("HelpItem() result should contain key %q", tt.wantKey)
			}
			if !strings.Contains(result, tt.wantDesc) {
				t.Errorf("HelpItem() result should contain desc %q", tt.wantDesc)
			}
		})
	}
}

func TestHelpBar(t *testing.T) {
	t.Run("single item", func(t *testing.T) {
		item := HelpItem("q", "quit")
		result := HelpBar(item)

		if result == "" {
			t.Error("HelpBar() returned empty string")
		}
		if !strings.Contains(result, "quit") {
			t.Error("HelpBar() should contain the item")
		}
	})

	t.Run("multiple items", func(t *testing.T) {
		items := []string{
			HelpItem("q", "quit"),
			HelpItem("n", "new"),
			HelpItem("d", "delete"),
		}
		result := HelpBar(items...)

		// Should contain separator
		if !strings.Contains(result, "│") {
			t.Error("HelpBar() should contain separator │")
		}

		// Should contain all items
		for _, content := range []string{"quit", "new", "delete"} {
			if !strings.Contains(result, content) {
				t.Errorf("HelpBar() should contain %q", content)
			}
		}
	})

	t.Run("empty items", func(t *testing.T) {
		result := HelpBar()
		if result != "" {
			t.Errorf("HelpBar() with no items should return empty string, got %q", result)
		}
	})
}

func TestStatusBadge(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   string
	}{
		{
			name:   "clean status",
			status: "clean",
			want:   "✓",
		},
		{
			name:   "modified status",
			status: "modified",
			want:   "M",
		},
		{
			name:   "untracked status",
			status: "untracked",
			want:   "?",
		},
		{
			name:   "mixed status",
			status: "mixed",
			want:   "M?",
		},
		{
			name:   "unpushed status",
			status: "unpushed",
			want:   "↑",
		},
		{
			name:   "missing status",
			status: "missing",
			want:   "✗",
		},
		{
			name:   "unknown status defaults to clean",
			status: "unknown",
			want:   "✓",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StatusBadge(tt.status)
			if !strings.Contains(result, tt.want) {
				t.Errorf("StatusBadge(%q) should contain %q, got %q", tt.status, tt.want, result)
			}
		})
	}
}

func TestStatusBadgeDetailed(t *testing.T) {
	// Use empty AdaptiveColor for tests (no background)
	noBg := lipgloss.AdaptiveColor{}

	t.Run("all zeros returns clean", func(t *testing.T) {
		result := StatusBadgeDetailed("clean", "active", 0, 0, 0, 0, 0, "", noBg)
		if !strings.Contains(result, "✓") {
			t.Errorf("StatusBadgeDetailed with all zeros should show ✓, got %q", result)
		}
	})

	t.Run("staged only", func(t *testing.T) {
		result := StatusBadgeDetailed("modified", "active", 3, 0, 0, 0, 0, "", noBg)
		if !strings.Contains(result, "+3") {
			t.Errorf("StatusBadgeDetailed should show +3 for staged, got %q", result)
		}
	})

	t.Run("modified only", func(t *testing.T) {
		result := StatusBadgeDetailed("modified", "active", 0, 2, 0, 0, 0, "", noBg)
		if !strings.Contains(result, "~2") {
			t.Errorf("StatusBadgeDetailed should show ~2 for modified, got %q", result)
		}
	})

	t.Run("untracked only", func(t *testing.T) {
		result := StatusBadgeDetailed("modified", "active", 0, 0, 5, 0, 0, "", noBg)
		if !strings.Contains(result, "?5") {
			t.Errorf("StatusBadgeDetailed should show ?5 for untracked, got %q", result)
		}
	})

	t.Run("unpushed only", func(t *testing.T) {
		result := StatusBadgeDetailed("modified", "active", 0, 0, 0, 4, 0, "", noBg)
		if !strings.Contains(result, "↑4") {
			t.Errorf("StatusBadgeDetailed should show ↑4 for unpushed, got %q", result)
		}
	})

	t.Run("mixed status", func(t *testing.T) {
		result := StatusBadgeDetailed("mixed", "active", 1, 2, 3, 4, 0, "", noBg)
		if !strings.Contains(result, "+1") {
			t.Error("StatusBadgeDetailed should show +1 for staged")
		}
		if !strings.Contains(result, "~2") {
			t.Error("StatusBadgeDetailed should show ~2 for modified")
		}
		if !strings.Contains(result, "?3") {
			t.Error("StatusBadgeDetailed should show ?3 for untracked")
		}
		if !strings.Contains(result, "↑4") {
			t.Error("StatusBadgeDetailed should show ↑4 for unpushed")
		}
	})

	t.Run("stale branch shows sleep emoji", func(t *testing.T) {
		result := StatusBadgeDetailed("clean", "stale", 0, 0, 0, 0, 0, "", noBg)
		if !strings.Contains(result, "💤") {
			t.Errorf("StatusBadgeDetailed with stale branch should show 💤, got %q", result)
		}
	})

	t.Run("PR open shows green badge", func(t *testing.T) {
		result := StatusBadgeDetailed("clean", "active", 0, 0, 0, 0, 110, "OPEN", noBg)
		if !strings.Contains(result, "#110") {
			t.Errorf("StatusBadgeDetailed should show #110 for PR, got %q", result)
		}
	})

	t.Run("PR merged shows badge", func(t *testing.T) {
		result := StatusBadgeDetailed("clean", "active", 0, 0, 0, 0, 95, "MERGED", noBg)
		if !strings.Contains(result, "#95") {
			t.Errorf("StatusBadgeDetailed should show #95 for merged PR, got %q", result)
		}
	})

	t.Run("stale with PR shows both", func(t *testing.T) {
		result := StatusBadgeDetailed("clean", "stale", 0, 0, 0, 0, 95, "MERGED", noBg)
		if !strings.Contains(result, "💤") {
			t.Errorf("StatusBadgeDetailed should show 💤 for stale, got %q", result)
		}
		if !strings.Contains(result, "#95") {
			t.Errorf("StatusBadgeDetailed should show #95 for PR, got %q", result)
		}
	})

	t.Run("stale with uncommitted changes shows both", func(t *testing.T) {
		// This is the key test - stale worktrees can still have uncommitted changes
		result := StatusBadgeDetailed("modified", "stale", 0, 2, 1, 0, 93, "MERGED", noBg)
		if !strings.Contains(result, "💤") {
			t.Errorf("StatusBadgeDetailed should show 💤 for stale, got %q", result)
		}
		if !strings.Contains(result, "~2") {
			t.Errorf("StatusBadgeDetailed should show ~2 for modified files, got %q", result)
		}
		if !strings.Contains(result, "?1") {
			t.Errorf("StatusBadgeDetailed should show ?1 for untracked files, got %q", result)
		}
		if !strings.Contains(result, "#93") {
			t.Errorf("StatusBadgeDetailed should show #93 for PR, got %q", result)
		}
	})
}

func TestWizardHeader(t *testing.T) {
	result := WizardHeader("Test Title")
	if !strings.Contains(result, "Test Title") {
		t.Errorf("WizardHeader() should contain the title, got %q", result)
	}
}

func TestWizardOption(t *testing.T) {
	t.Run("selected option", func(t *testing.T) {
		result := WizardOption("Option 1", true)
		if !strings.Contains(result, "▶") {
			t.Error("Selected option should have ▶ prefix")
		}
		if !strings.Contains(result, "Option 1") {
			t.Error("Option should contain label")
		}
	})

	t.Run("unselected option", func(t *testing.T) {
		result := WizardOption("Option 2", false)
		if strings.Contains(result, "▶") {
			t.Error("Unselected option should not have ▶ prefix")
		}
		if !strings.Contains(result, "Option 2") {
			t.Error("Option should contain label")
		}
	})
}

func TestWizardHelpBar(t *testing.T) {
	t.Run("single item", func(t *testing.T) {
		result := WizardHelpBar("↑↓ navigate")
		if !strings.Contains(result, "navigate") {
			t.Error("WizardHelpBar should contain the item")
		}
	})

	t.Run("multiple items", func(t *testing.T) {
		result := WizardHelpBar("↑↓ navigate", "enter select", "esc back")
		if !strings.Contains(result, "•") {
			t.Error("WizardHelpBar should contain separator •")
		}
		if !strings.Contains(result, "navigate") {
			t.Error("WizardHelpBar should contain first item")
		}
		if !strings.Contains(result, "select") {
			t.Error("WizardHelpBar should contain second item")
		}
		if !strings.Contains(result, "back") {
			t.Error("WizardHelpBar should contain third item")
		}
	})
}
