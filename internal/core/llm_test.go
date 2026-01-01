package core

import (
	"strings"
	"testing"

	"github.com/langtind/gren/internal/config"
)

func TestNewLLMGenerator(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.Config
		wantNil bool
	}{
		{
			name:    "nil config",
			cfg:     nil,
			wantNil: true,
		},
		{
			name: "empty command",
			cfg: &config.Config{
				CommitGenerator: config.CommitGenerator{
					Command: "",
				},
			},
			wantNil: true,
		},
		{
			name: "valid config",
			cfg: &config.Config{
				CommitGenerator: config.CommitGenerator{
					Command: "llm",
					Args:    []string{"-m", "gpt-4"},
				},
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := NewLLMGenerator(tt.cfg)
			if (gen == nil) != tt.wantNil {
				t.Errorf("NewLLMGenerator() nil = %v, want nil = %v", gen == nil, tt.wantNil)
			}
			if gen != nil {
				if gen.Command != tt.cfg.CommitGenerator.Command {
					t.Errorf("Command = %q, want %q", gen.Command, tt.cfg.CommitGenerator.Command)
				}
			}
		})
	}
}

func TestFilterLockFiles(t *testing.T) {
	tests := []struct {
		name      string
		diff      string
		wantFiles []string // files that should remain
		skipFiles []string // files that should be filtered
	}{
		{
			name: "filters package-lock.json",
			diff: `diff --git a/package.json b/package.json
--- a/package.json
+++ b/package.json
@@ -1,5 +1,6 @@
 {
   "name": "test",
+  "version": "1.0.1",
   "dependencies": {}
 }
diff --git a/package-lock.json b/package-lock.json
--- a/package-lock.json
+++ b/package-lock.json
@@ -1,100 +1,200 @@
 {
   "lockfile content": "lots of changes"
 }`,
			wantFiles: []string{"package.json"},
			skipFiles: []string{"package-lock.json"},
		},
		{
			name: "filters yarn.lock",
			diff: `diff --git a/src/index.ts b/src/index.ts
--- a/src/index.ts
+++ b/src/index.ts
@@ -1 +1 @@
-console.log("hello")
+console.log("world")
diff --git a/yarn.lock b/yarn.lock
--- a/yarn.lock
+++ b/yarn.lock
@@ -1,50 +1,100 @@
 # yarn lockfile`,
			wantFiles: []string{"src/index.ts"},
			skipFiles: []string{"yarn.lock"},
		},
		{
			name: "filters go.sum",
			diff: `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1 +1 @@
-package main
+package cmd
diff --git a/go.sum b/go.sum
--- a/go.sum
+++ b/go.sum
@@ -1,20 +1,40 @@
 github.com/pkg v1.0.0 h1:hash`,
			wantFiles: []string{"main.go"},
			skipFiles: []string{"go.sum"},
		},
		{
			name: "filters multiple lock files",
			diff: `diff --git a/index.js b/index.js
--- a/index.js
+++ b/index.js
@@ -1 +1 @@
-const x = 1
+const x = 2
diff --git a/package-lock.json b/package-lock.json
--- a/package-lock.json
+++ b/package-lock.json
@@ -1 +1 @@
-old lock
+new lock
diff --git a/pnpm-lock.yaml b/pnpm-lock.yaml
--- a/pnpm-lock.yaml
+++ b/pnpm-lock.yaml
@@ -1 +1 @@
-old pnpm
+new pnpm`,
			wantFiles: []string{"index.js"},
			skipFiles: []string{"package-lock.json", "pnpm-lock.yaml"},
		},
		{
			name:      "no lock files",
			diff:      `diff --git a/src/main.rs b/src/main.rs`,
			wantFiles: []string{"src/main.rs"},
			skipFiles: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterLockFiles(tt.diff)

			for _, want := range tt.wantFiles {
				if !strings.Contains(result, want) {
					t.Errorf("filterLockFiles() should contain %q", want)
				}
			}

			for _, skip := range tt.skipFiles {
				if strings.Contains(result, "diff --git a/"+skip) || strings.Contains(result, "diff --git b/"+skip) {
					t.Errorf("filterLockFiles() should not contain %q", skip)
				}
			}
		})
	}
}

func TestIsLockFile(t *testing.T) {
	tests := []struct {
		path   string
		isLock bool
	}{
		{"package-lock.json", true},
		{"yarn.lock", true},
		{"pnpm-lock.yaml", true},
		{"bun.lockb", true},
		{"Cargo.lock", true},
		{"go.sum", true},
		{"Gemfile.lock", true},
		{"poetry.lock", true},
		{"Pipfile.lock", true},
		{"composer.lock", true},
		{"flake.lock", true},
		{"package.json", false},
		{"go.mod", false},
		{"src/main.go", false},
		{"nested/package-lock.json", true},
		{"some/path/yarn.lock", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := isLockFile(tt.path)
			if got != tt.isLock {
				t.Errorf("isLockFile(%q) = %v, want %v", tt.path, got, tt.isLock)
			}
		})
	}
}

func TestTruncateDiff(t *testing.T) {
	tests := []struct {
		name       string
		diffLen    int
		wantTrunc  bool
		wantMaxLen int
	}{
		{
			name:       "small diff",
			diffLen:    1000,
			wantTrunc:  false,
			wantMaxLen: 1000,
		},
		{
			name:       "exact threshold",
			diffLen:    DiffSizeThreshold,
			wantTrunc:  false,
			wantMaxLen: DiffSizeThreshold,
		},
		{
			name:       "over threshold",
			diffLen:    DiffSizeThreshold + 1000,
			wantTrunc:  true,
			wantMaxLen: DiffSizeThreshold + 100, // account for truncation message
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diff := strings.Repeat("a", tt.diffLen)
			result, truncated := truncateDiff(diff)

			if truncated != tt.wantTrunc {
				t.Errorf("truncateDiff() truncated = %v, want %v", truncated, tt.wantTrunc)
			}

			if truncated && !strings.Contains(result, "truncated") {
				t.Error("truncateDiff() should contain truncation message")
			}

			if len(result) > tt.wantMaxLen {
				t.Errorf("truncateDiff() length = %d, want <= %d", len(result), tt.wantMaxLen)
			}
		})
	}
}

func TestCleanCommitMessage(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		output string
	}{
		{
			name:   "clean message",
			input:  "feat: add new feature",
			output: "feat: add new feature",
		},
		{
			name:   "with whitespace",
			input:  "  fix: bug fix  \n\n",
			output: "fix: bug fix",
		},
		{
			name:   "with markdown code block",
			input:  "```\nfeat: add feature\n```",
			output: "feat: add feature",
		},
		{
			name:   "with common prefix",
			input:  "Commit message: feat: add feature",
			output: "feat: add feature",
		},
		{
			name:   "with quotes",
			input:  `"feat: add feature"`,
			output: "feat: add feature",
		},
		{
			name:   "with single quotes",
			input:  `'fix: bug fix'`,
			output: "fix: bug fix",
		},
		{
			name:   "multiline takes first",
			input:  "feat: first line\nSecond line\nThird line",
			output: "feat: first line",
		},
		{
			name:   "truncates long message",
			input:  strings.Repeat("a", 100),
			output: strings.Repeat("a", 69) + "...",
		},
		{
			name:   "here's the commit message prefix",
			input:  "Here's the commit message: feat: new feature",
			output: "feat: new feature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanCommitMessage(tt.input)
			if got != tt.output {
				t.Errorf("cleanCommitMessage() = %q, want %q", got, tt.output)
			}
		})
	}
}

func TestExpandPromptTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		diff     string
		context  string
		want     string
	}{
		{
			name:     "basic template",
			template: "Diff:\n{{ diff }}",
			diff:     "file changes",
			context:  "",
			want:     "Diff:\nfile changes",
		},
		{
			name:     "template with context",
			template: "Context: {{ context }}\nChanges: {{ diff }}",
			diff:     "diff content",
			context:  "branch: main",
			want:     "Context: branch: main\nChanges: diff content",
		},
		{
			name:     "no spaces in placeholders",
			template: "{{diff}} and {{context}}",
			diff:     "diff",
			context:  "ctx",
			want:     "diff and ctx",
		},
		{
			name:     "no placeholders",
			template: "Just plain text",
			diff:     "ignored",
			context:  "also ignored",
			want:     "Just plain text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandPromptTemplate(tt.template, tt.diff, tt.context)
			if got != tt.want {
				t.Errorf("expandPromptTemplate() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateConventionalCommit(t *testing.T) {
	tests := []struct {
		message string
		valid   bool
	}{
		{"feat: add new feature", true},
		{"fix: resolve bug", true},
		{"docs: update readme", true},
		{"style: format code", true},
		{"refactor: restructure module", true},
		{"perf: improve performance", true},
		{"test: add tests", true},
		{"build: update deps", true},
		{"ci: fix workflow", true},
		{"chore: cleanup", true},
		{"revert: undo change", true},
		{"feat(scope): scoped feature", true},
		{"fix(auth): fix login", true},
		{"Add new feature", false},
		{"fixed bug", false},
		{"", false},
		{"feat:", false},
		{"feat", false},
		{"FEAT: uppercase", false},
	}

	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			got := ValidateConventionalCommit(tt.message)
			if got != tt.valid {
				t.Errorf("ValidateConventionalCommit(%q) = %v, want %v", tt.message, got, tt.valid)
			}
		})
	}
}

func TestBuildPrompt(t *testing.T) {
	gen := &LLMGenerator{
		Command: "echo",
	}

	diff := "test diff"
	context := "test context"

	prompt := gen.buildPrompt(diff, context)

	// Should contain conventional commits instruction
	if !strings.Contains(prompt, "conventional commits") {
		t.Error("buildPrompt() should mention conventional commits")
	}

	// Should contain the diff
	if !strings.Contains(prompt, diff) {
		t.Error("buildPrompt() should contain the diff")
	}

	// Should contain the context
	if !strings.Contains(prompt, context) {
		t.Error("buildPrompt() should contain the context")
	}

	// Should have 72 character limit instruction
	if !strings.Contains(prompt, "72") {
		t.Error("buildPrompt() should mention 72 character limit")
	}
}

func TestBuildPromptWithTemplate(t *testing.T) {
	gen := &LLMGenerator{
		Command:  "echo",
		Template: "Custom: {{ diff }}\nInfo: {{ context }}",
	}

	diff := "my diff"
	context := "my context"

	prompt := gen.buildPrompt(diff, context)

	if prompt != "Custom: my diff\nInfo: my context" {
		t.Errorf("buildPrompt() with template = %q", prompt)
	}
}

func TestLockFilesConstant(t *testing.T) {
	// Verify all expected lock files are in the list
	expected := []string{
		"package-lock.json",
		"yarn.lock",
		"pnpm-lock.yaml",
		"bun.lockb",
		"Cargo.lock",
		"go.sum",
		"Gemfile.lock",
		"poetry.lock",
		"Pipfile.lock",
		"composer.lock",
		"flake.lock",
	}

	for _, lock := range expected {
		found := false
		for _, l := range LockFiles {
			if l == lock {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("LockFiles should contain %q", lock)
		}
	}
}

func TestDiffSizeThreshold(t *testing.T) {
	// Verify the threshold is reasonable (400k characters)
	if DiffSizeThreshold != 400000 {
		t.Errorf("DiffSizeThreshold = %d, want 400000", DiffSizeThreshold)
	}
}
