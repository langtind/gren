// Package directive provides shell directive writing for navigation and command execution.
// This uses an environment variable (GREN_DIRECTIVE_FILE) to communicate with the shell wrapper,
// which sources the directive file after gren exits.
package directive

import (
	"fmt"
	"os"
	"strings"
)

const (
	// EnvDirectiveFile is the environment variable name for the directive file path.
	// The shell wrapper sets this to a mktemp-created file before invoking gren.
	EnvDirectiveFile = "GREN_DIRECTIVE_FILE"

	// LegacyTempFile is the old fixed temp file path for backward compatibility.
	// Used when GREN_DIRECTIVE_FILE is not set (old shell integration).
	LegacyTempFile = "/tmp/gren_navigate"
)

// WriteDirective writes a shell directive to be executed after gren exits.
// If GREN_DIRECTIVE_FILE is set, writes to that file.
// Otherwise, falls back to the legacy fixed temp file for backward compatibility.
func WriteDirective(directive string) error {
	file := os.Getenv(EnvDirectiveFile)
	if file == "" {
		// Fall back to legacy behavior for users who haven't updated their shell config
		file = LegacyTempFile
	}
	return os.WriteFile(file, []byte(directive+"\n"), 0644)
}

// WriteCD writes a cd directive to change directory after gren exits.
func WriteCD(path string) error {
	return WriteDirective(fmt.Sprintf("cd %q", path))
}

// WriteExec writes an exec directive to run a command after cd.
// The command replaces the current shell process.
func WriteExec(command string) error {
	return WriteDirective(fmt.Sprintf("exec %s", command))
}

// WriteCDAndExec writes both cd and exec directives.
// First changes to the directory, then executes the command.
func WriteCDAndExec(path, command string) error {
	directives := []string{
		fmt.Sprintf("cd %q", path),
		command,
	}
	return WriteDirective(strings.Join(directives, "\n"))
}

// WriteCDAndRun writes a cd directive followed by a command to run (not exec).
// The command runs in a subshell, keeping the shell alive after.
func WriteCDAndRun(path, command string) error {
	directives := []string{
		fmt.Sprintf("cd %q", path),
		command,
	}
	return WriteDirective(strings.Join(directives, "\n"))
}

// IsShellIntegrationActive returns true if the shell wrapper is active.
// This can be used to provide different behavior when running with/without shell integration.
func IsShellIntegrationActive() bool {
	return os.Getenv(EnvDirectiveFile) != ""
}

// Clear removes any existing directive file.
// Useful for cleanup or when canceling an operation.
func Clear() error {
	file := os.Getenv(EnvDirectiveFile)
	if file == "" {
		file = LegacyTempFile
	}
	err := os.Remove(file)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
