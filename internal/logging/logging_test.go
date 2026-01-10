package logging

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGetLogDir(t *testing.T) {
	logDir := getLogDir()

	switch runtime.GOOS {
	case "darwin":
		if !strings.Contains(logDir, "Library/Logs/gren") {
			t.Errorf("macOS log dir should contain Library/Logs/gren, got: %s", logDir)
		}
	case "linux":
		if !strings.Contains(logDir, ".local/state/gren/logs") {
			t.Errorf("Linux log dir should contain .local/state/gren/logs, got: %s", logDir)
		}
	default:
		if logDir != "/tmp/gren/logs" {
			t.Errorf("default log dir should be /tmp/gren/logs, got: %s", logDir)
		}
	}
}

func TestFormatMessage(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		format   string
		args     []interface{}
		contains []string
	}{
		{
			name:     "simple message",
			level:    "INFO",
			format:   "test message",
			contains: []string{"[INFO]", "test message"},
		},
		{
			name:     "formatted message",
			level:    "ERROR",
			format:   "error: %s",
			args:     []interface{}{"something went wrong"},
			contains: []string{"[ERROR]", "error: something went wrong"},
		},
		{
			name:     "multiple args",
			level:    "DEBUG",
			format:   "value=%d, name=%s",
			args:     []interface{}{42, "test"},
			contains: []string{"[DEBUG]", "value=42", "name=test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatMessage(tt.level, tt.format, tt.args...)

			for _, s := range tt.contains {
				if !strings.Contains(result, s) {
					t.Errorf("formatMessage() result should contain %q, got: %s", s, result)
				}
			}

			// Should contain timestamp pattern
			if !strings.Contains(result, "[20") { // year starts with 20
				t.Errorf("formatMessage() should contain timestamp, got: %s", result)
			}
		})
	}
}

func TestLoggingFunctionsWhenDisabled(t *testing.T) {
	// Store original state
	origLogger := logger
	origLogFile := logFile
	origEnabled := enabled

	// Restore state after test
	defer func() {
		logger = origLogger
		logFile = origLogFile
		enabled = origEnabled
	}()

	// Setup disabled state
	logger = nil
	logFile = nil
	enabled = false

	// Test when disabled - should not panic
	Debug("test debug")
	Info("test info")
	Warn("test warn")
	Error("test error")
	// Should not panic - test passes if we get here
}

func TestLoggingFunctionsWhenEnabled(t *testing.T) {
	// Create temp log file
	tmpDir, err := os.MkdirTemp("", "gren-log-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logFilePath := filepath.Join(tmpDir, "test.log")
	file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("failed to create log file: %v", err)
	}
	defer file.Close()

	// Store original state
	origLogger := logger
	origLogFile := logFile
	origEnabled := enabled

	// Restore state after test
	defer func() {
		logger = origLogger
		logFile = origLogFile
		enabled = origEnabled
	}()

	// Setup test logger
	logFile = file
	logger = log.New(file, "", 0)
	enabled = true

	// Log some messages
	Debug("test debug message")
	Info("test info message")
	Warn("test warn message")
	Error("test error message")

	// Close and read the file
	file.Close()
	content, err := os.ReadFile(logFilePath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	logContent := string(content)

	// Verify all log levels appear
	levels := []string{"DEBUG", "INFO", "WARN", "ERROR"}
	for _, level := range levels {
		if !strings.Contains(logContent, level) {
			t.Errorf("log file should contain %s level, got: %s", level, logContent)
		}
	}

	// Verify messages appear
	messages := []string{"test debug message", "test info message", "test warn message", "test error message"}
	for _, msg := range messages {
		if !strings.Contains(logContent, msg) {
			t.Errorf("log file should contain %q, got: %s", msg, logContent)
		}
	}
}

func TestGetLogPath(t *testing.T) {
	// Store original state
	origLogPath := logPath
	defer func() {
		logPath = origLogPath
	}()

	logPath = "/test/path/gren.log"
	result := GetLogPath()
	if result != "/test/path/gren.log" {
		t.Errorf("GetLogPath() = %q, want /test/path/gren.log", result)
	}
}

func TestSetOutput(t *testing.T) {
	// Test that SetOutput doesn't panic when logger is nil
	origLogger := logger
	origLogFile := logFile
	defer func() {
		logger = origLogger
		logFile = origLogFile
	}()

	logger = nil
	var buf bytes.Buffer
	SetOutput(&buf) // Should not panic when logger is nil
}

func TestSetOutputWithLogger(t *testing.T) {
	// Create temp log file
	tmpDir, err := os.MkdirTemp("", "gren-log-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logFilePath := filepath.Join(tmpDir, "test.log")
	file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("failed to create log file: %v", err)
	}
	defer file.Close()

	// Store original state
	origLogger := logger
	origLogFile := logFile
	origEnabled := enabled

	// Restore state after test
	defer func() {
		logger = origLogger
		logFile = origLogFile
		enabled = origEnabled
	}()

	// Setup test logger
	logFile = file
	logger = log.New(file, "", 0)
	enabled = true

	// Add additional output
	var buf bytes.Buffer
	SetOutput(&buf)

	// Log a message
	logger.Println("test message")

	// Both outputs should have the message
	if !strings.Contains(buf.String(), "test message") {
		t.Errorf("buffer should contain message, got: %s", buf.String())
	}
}

func TestClose(t *testing.T) {
	// Create temp log file
	tmpDir, err := os.MkdirTemp("", "gren-log-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logFilePath := filepath.Join(tmpDir, "test.log")
	file, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("failed to create log file: %v", err)
	}

	// Store original state
	origLogger := logger
	origLogFile := logFile
	origEnabled := enabled

	// Restore state after test
	defer func() {
		logger = origLogger
		logFile = origLogFile
		enabled = origEnabled
	}()

	// Setup test logger
	logFile = file
	logger = log.New(file, "", 0)
	enabled = true

	// Close should not panic
	Close()
}

func TestCloseWithNilFile(t *testing.T) {
	// Store original state
	origLogFile := logFile

	// Restore state after test
	defer func() {
		logFile = origLogFile
	}()

	logFile = nil
	Close() // Should not panic
}
