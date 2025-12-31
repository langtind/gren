package directive

import (
	"os"
	"strings"
	"testing"
)

func TestWriteDirective(t *testing.T) {
	// Create temp file for directive
	tmpFile, err := os.CreateTemp("", "directive-test-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// Set env var
	os.Setenv(EnvDirectiveFile, tmpFile.Name())
	defer os.Unsetenv(EnvDirectiveFile)

	// Write directive
	err = WriteDirective("echo hello")
	if err != nil {
		t.Fatalf("WriteDirective failed: %v", err)
	}

	// Verify content
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to read directive file: %v", err)
	}

	if string(content) != "echo hello\n" {
		t.Errorf("unexpected content: %q, want %q", content, "echo hello\n")
	}
}

func TestWriteCD(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "directive-test-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	os.Setenv(EnvDirectiveFile, tmpFile.Name())
	defer os.Unsetenv(EnvDirectiveFile)

	err = WriteCD("/path/to/dir")
	if err != nil {
		t.Fatalf("WriteCD failed: %v", err)
	}

	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to read directive file: %v", err)
	}

	if !strings.Contains(string(content), "cd") || !strings.Contains(string(content), "/path/to/dir") {
		t.Errorf("unexpected content: %q", content)
	}
}

func TestWriteCDWithSpaces(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "directive-test-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	os.Setenv(EnvDirectiveFile, tmpFile.Name())
	defer os.Unsetenv(EnvDirectiveFile)

	err = WriteCD("/path/with spaces/dir")
	if err != nil {
		t.Fatalf("WriteCD failed: %v", err)
	}

	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to read directive file: %v", err)
	}

	// Should be properly quoted
	expected := `cd "/path/with spaces/dir"`
	if !strings.Contains(string(content), expected) {
		t.Errorf("path with spaces not properly quoted: %q", content)
	}
}

func TestWriteCDAndRun(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "directive-test-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	os.Setenv(EnvDirectiveFile, tmpFile.Name())
	defer os.Unsetenv(EnvDirectiveFile)

	err = WriteCDAndRun("/path/to/dir", "claude")
	if err != nil {
		t.Fatalf("WriteCDAndRun failed: %v", err)
	}

	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to read directive file: %v", err)
	}

	if !strings.Contains(string(content), "cd") || !strings.Contains(string(content), "claude") {
		t.Errorf("unexpected content: %q", content)
	}
}

func TestIsShellIntegrationActive(t *testing.T) {
	// Without env var
	os.Unsetenv(EnvDirectiveFile)
	if IsShellIntegrationActive() {
		t.Error("should be false when env var not set")
	}

	// With env var
	os.Setenv(EnvDirectiveFile, "/tmp/test")
	defer os.Unsetenv(EnvDirectiveFile)
	if !IsShellIntegrationActive() {
		t.Error("should be true when env var is set")
	}
}

func TestLegacyFallback(t *testing.T) {
	// Ensure env var is not set
	os.Unsetenv(EnvDirectiveFile)

	// Clean up any existing legacy file
	os.Remove(LegacyTempFile)
	defer os.Remove(LegacyTempFile)

	// Write directive without env var (should use legacy path)
	err := WriteDirective("test command")
	if err != nil {
		t.Fatalf("WriteDirective with legacy fallback failed: %v", err)
	}

	// Verify legacy file was created
	content, err := os.ReadFile(LegacyTempFile)
	if err != nil {
		t.Fatalf("failed to read legacy file: %v", err)
	}

	if string(content) != "test command\n" {
		t.Errorf("unexpected content in legacy file: %q", content)
	}
}

func TestClear(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "directive-test-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpFile.Close()

	os.Setenv(EnvDirectiveFile, tmpFile.Name())
	defer os.Unsetenv(EnvDirectiveFile)

	// Write something first
	WriteDirective("test")

	// Clear should remove the file
	err = Clear()
	if err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	// File should not exist
	if _, err := os.Stat(tmpFile.Name()); !os.IsNotExist(err) {
		t.Error("file should have been removed")
	}

	// Clearing non-existent file should not error
	err = Clear()
	if err != nil {
		t.Errorf("Clear on non-existent file should not error: %v", err)
	}
}
