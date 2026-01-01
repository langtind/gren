package core

import (
	"testing"
)

func TestParseMarkerType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected MarkerType
		wantErr  bool
	}{
		{
			name:     "working keyword",
			input:    "working",
			expected: MarkerWorking,
		},
		{
			name:     "work shorthand",
			input:    "work",
			expected: MarkerWorking,
		},
		{
			name:     "working emoji",
			input:    "🤖",
			expected: MarkerWorking,
		},
		{
			name:     "waiting keyword",
			input:    "waiting",
			expected: MarkerWaiting,
		},
		{
			name:     "wait shorthand",
			input:    "wait",
			expected: MarkerWaiting,
		},
		{
			name:     "waiting emoji",
			input:    "💬",
			expected: MarkerWaiting,
		},
		{
			name:     "idle keyword",
			input:    "idle",
			expected: MarkerIdle,
		},
		{
			name:     "idle emoji",
			input:    "💤",
			expected: MarkerIdle,
		},
		{
			name:     "custom emoji",
			input:    "🔥",
			expected: MarkerType("🔥"),
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMarkerType(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMarkerType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("ParseMarkerType() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSanitizeBranchForConfig(t *testing.T) {
	tests := []struct {
		branch   string
		expected string
	}{
		{"main", "main"},
		{"feature/auth", "feature%2Fauth"},                 // URL encoding for slashes
		{"feat/user/login", "feat%2Fuser%2Flogin"},         // Multiple slashes
		{"simple-branch", "simple-branch"},                 // Hyphens unchanged
		{"feat_underscore", "feat_underscore"},             // Underscores preserved
		{"feat/with_underscore", "feat%2Fwith_underscore"}, // Mixed: slash encoded, underscore preserved
	}

	for _, tt := range tests {
		t.Run(tt.branch, func(t *testing.T) {
			got := sanitizeBranchForConfig(tt.branch)
			if got != tt.expected {
				t.Errorf("sanitizeBranchForConfig(%q) = %q, want %q", tt.branch, got, tt.expected)
			}
		})
	}
}

func TestRestoreBranchFromConfig(t *testing.T) {
	tests := []struct {
		key      string
		expected string
	}{
		{"main", "main"},
		{"feature%2Fauth", "feature/auth"},                 // URL decoding for slashes
		{"feat%2Fuser%2Flogin", "feat/user/login"},         // Multiple slashes
		{"simple-branch", "simple-branch"},                 // Hyphens unchanged
		{"feat_underscore", "feat_underscore"},             // Underscores preserved (lossless)
		{"feat%2Fwith_underscore", "feat/with_underscore"}, // Mixed: slash decoded, underscore preserved
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := restoreBranchFromConfig(tt.key)
			if got != tt.expected {
				t.Errorf("restoreBranchFromConfig(%q) = %q, want %q", tt.key, got, tt.expected)
			}
		})
	}
}

func TestBranchEncodingRoundTrip(t *testing.T) {
	// Test that encode->decode is lossless for various branch names
	branches := []string{
		"main",
		"feature/auth",
		"feat/user/login",
		"simple-branch",
		"feat_underscore",
		"feat/with_underscore",
		"fix/bug_123",
		"release/v1.0.0",
	}

	for _, branch := range branches {
		t.Run(branch, func(t *testing.T) {
			encoded := sanitizeBranchForConfig(branch)
			decoded := restoreBranchFromConfig(encoded)
			if decoded != branch {
				t.Errorf("Round-trip failed: %q -> %q -> %q", branch, encoded, decoded)
			}
		})
	}
}
