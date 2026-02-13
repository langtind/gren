package cli

import (
	"embed"
	"os"
	"path/filepath"
	"testing"
)

//go:embed testdata/test-skill/*
var testSkillFS embed.FS

func TestInstallSkill(t *testing.T) {
	// Save original skill settings
	origFS := skillFS
	origDirName := skillDirName
	origName := skillName
	defer func() {
		skillFS = origFS
		skillDirName = origDirName
		skillName = origName
	}()

	// Set up test skill
	SetSkillFS(testSkillFS, "testdata/test-skill", "test-skill")

	// Create temp directory for testing
	tmpDir := t.TempDir()
	destDir := filepath.Join(tmpDir, "test-skill")

	t.Run("installs skill files to destination", func(t *testing.T) {
		// Create installer and run with force flag
		err := installSkillToPath(destDir, true)
		if err != nil {
			t.Fatalf("installSkillToPath failed: %v", err)
		}

		// Verify SKILL.md was created
		skillPath := filepath.Join(destDir, "SKILL.md")
		if _, err := os.Stat(skillPath); os.IsNotExist(err) {
			t.Errorf("SKILL.md was not created at %s", skillPath)
		}

		// Verify content is correct
		content, err := os.ReadFile(skillPath)
		if err != nil {
			t.Fatalf("failed to read SKILL.md: %v", err)
		}
		if len(content) == 0 {
			t.Error("SKILL.md is empty")
		}
	})

	t.Run("detects existing files", func(t *testing.T) {
		tmpDir2 := t.TempDir()
		destDir2 := filepath.Join(tmpDir2, "test-skill")

		// Create destination directory with existing file
		os.MkdirAll(destDir2, 0755)
		existingFile := filepath.Join(destDir2, "SKILL.md")
		os.WriteFile(existingFile, []byte("existing content"), 0644)

		// Install should detect existing file
		existing := detectExistingFiles(destDir2)
		if len(existing) == 0 {
			t.Error("should detect existing SKILL.md file")
		}
	})

	t.Run("overwrites with force flag", func(t *testing.T) {
		tmpDir3 := t.TempDir()
		destDir3 := filepath.Join(tmpDir3, "test-skill")

		// Create existing file
		os.MkdirAll(destDir3, 0755)
		existingFile := filepath.Join(destDir3, "SKILL.md")
		os.WriteFile(existingFile, []byte("old content"), 0644)

		// Install with force
		err := installSkillToPath(destDir3, true)
		if err != nil {
			t.Fatalf("installSkillToPath with force failed: %v", err)
		}

		// Verify file was overwritten
		content, err := os.ReadFile(existingFile)
		if err != nil {
			t.Fatalf("failed to read overwritten file: %v", err)
		}
		if string(content) == "old content" {
			t.Error("file was not overwritten")
		}
	})

	t.Run("creates nested directories", func(t *testing.T) {
		tmpDir4 := t.TempDir()
		destDir4 := filepath.Join(tmpDir4, "test-skill")

		err := installSkillToPath(destDir4, true)
		if err != nil {
			t.Fatalf("installSkillToPath failed: %v", err)
		}

		// Verify directory was created
		if _, err := os.Stat(destDir4); os.IsNotExist(err) {
			t.Error("destination directory was not created")
		}
	})
}

func TestSetSkillFS(t *testing.T) {
	// Test that SetSkillFS properly sets the module-level variables
	testFS := testSkillFS
	SetSkillFS(testFS, "test-dir", "test-name")

	if skillFS == nil {
		t.Error("skillFS was not set")
	}
	if skillDirName != "test-dir" {
		t.Errorf("skillDirName = %q, want %q", skillDirName, "test-dir")
	}
	if skillName != "test-name" {
		t.Errorf("skillName = %q, want %q", skillName, "test-name")
	}
}

func TestWalkSkillFiles(t *testing.T) {
	SetSkillFS(testSkillFS, "testdata/test-skill", "test-skill")

	files, err := walkSkillFiles()
	if err != nil {
		t.Fatalf("walkSkillFiles failed: %v", err)
	}

	if len(files) == 0 {
		t.Error("no files found in skill directory")
	}

	// Should find at least SKILL.md
	foundSkillMd := false
	for _, f := range files {
		if f.relPath == "SKILL.md" {
			foundSkillMd = true
			break
		}
	}
	if !foundSkillMd {
		t.Error("SKILL.md not found in walked files")
	}
}
