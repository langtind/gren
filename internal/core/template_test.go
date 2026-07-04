package core

import (
	"os"
	"strconv"
	"testing"
)

func TestHashPort(t *testing.T) {
	// Deterministic: same branch always maps to the same port.
	a := hashPort("feat/parallel-agents")
	b := hashPort("feat/parallel-agents")
	if a != b {
		t.Errorf("hashPort not deterministic: %d != %d", a, b)
	}

	// Range: always within 10000-19999 for a variety of inputs.
	for _, branch := range []string{
		"main", "feat/foo", "bugfix/JIRA-123", "", "a", "release/2024.10.01",
		"very/long/branch/name/with/many/segments",
	} {
		p := hashPort(branch)
		if p < 10000 || p > 19999 {
			t.Errorf("hashPort(%q) = %d, out of range 10000-19999", branch, p)
		}
	}

	// Distinct branches should (typically) map to distinct ports.
	if hashPort("feat/a") == hashPort("feat/b") {
		t.Errorf("hashPort collision for feat/a and feat/b")
	}
}

func TestSanitizeDB(t *testing.T) {
	tests := []struct {
		branch   string
		expected string
	}{
		{"main", "main"},
		{"feat/Foo-Bar", "feat_foo_bar"},
		{"feature/JIRA-42", "feature_jira_42"},
		{"123-start", "_123_start"},
		{"UPPER", "upper"},
		{"", ""},
		{"a.b.c", "a_b_c"},
	}
	for _, tt := range tests {
		got := sanitizeDB(tt.branch)
		if got != tt.expected {
			t.Errorf("sanitizeDB(%q) = %q, want %q", tt.branch, got, tt.expected)
		}
	}
}

// EvalTemplate exposes the template engine to scripts (via `gren step eval`),
// resolving variables against the current worktree.
func TestEvalTemplate(t *testing.T) {
	repo := mkRepo(t) // fresh repo on branch main

	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origWd)
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}

	wm := &WorktreeManager{}
	got, err := wm.EvalTemplate("b={{ branch }} port={{ branch | hash_port }} db={{ branch | sanitize_db }}")
	if err != nil {
		t.Fatalf("EvalTemplate: %v", err)
	}

	want := "b=main port=" + strconv.Itoa(hashPort("main")) + " db=" + sanitizeDB("main")
	if got != want {
		t.Errorf("EvalTemplate = %q, want %q", got, want)
	}
}

func TestExpandTemplateFilters(t *testing.T) {
	ctx := TemplateContext{
		Branch:          "feat/My-Thing",
		BranchSanitized: sanitizeBranch("feat/My-Thing"),
	}

	wantPort := strconv.Itoa(hashPort("feat/My-Thing"))
	wantDB := sanitizeDB("feat/My-Thing")

	tests := []struct {
		template string
		expected string
	}{
		{"{{ branch | hash_port }}", wantPort},
		{"{{branch|hash_port}}", wantPort},
		{"{{ branch | sanitize_db }}", wantDB},
		{"{{branch|sanitize_db}}", wantDB},
		{"port {{ branch | hash_port }} db {{ branch | sanitize_db }}", "port " + wantPort + " db " + wantDB},
	}
	for _, tt := range tests {
		got := expandTemplate(tt.template, ctx)
		if got != tt.expected {
			t.Errorf("expandTemplate(%q) = %q, want %q", tt.template, got, tt.expected)
		}
	}
}
