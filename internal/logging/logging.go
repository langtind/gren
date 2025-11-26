package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

var (
	logger  *log.Logger
	logFile *os.File
	logPath string
	enabled bool
)

// Init initializes the logger with the default log path for the OS
func Init() error {
	logDir := getLogDir()
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	logPath = filepath.Join(logDir, "gren.log")

	// Open log file in append mode
	var err error
	logFile, err = os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	logger = log.New(logFile, "", 0)
	enabled = true

	// Log startup
	Info("gren started")

	return nil
}

// getLogDir returns the appropriate log directory for the OS
func getLogDir() string {
	switch runtime.GOOS {
	case "darwin":
		// macOS: ~/Library/Logs/gren/
		home, err := os.UserHomeDir()
		if err != nil {
			return "/tmp/gren/logs"
		}
		return filepath.Join(home, "Library", "Logs", "gren")
	case "linux":
		// Linux: ~/.local/state/gren/logs/ or /tmp
		home, err := os.UserHomeDir()
		if err != nil {
			return "/tmp/gren/logs"
		}
		return filepath.Join(home, ".local", "state", "gren", "logs")
	default:
		return "/tmp/gren/logs"
	}
}

// Close closes the log file
func Close() {
	if logFile != nil {
		Info("gren shutting down")
		logFile.Close()
	}
}

// GetLogPath returns the path to the log file
func GetLogPath() string {
	return logPath
}

// formatMessage formats a log message with timestamp and level
func formatMessage(level, format string, args ...interface{}) string {
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	message := fmt.Sprintf(format, args...)
	return fmt.Sprintf("[%s] [%s] %s", timestamp, level, message)
}

// Debug logs a debug message
func Debug(format string, args ...interface{}) {
	if enabled && logger != nil {
		logger.Println(formatMessage("DEBUG", format, args...))
	}
}

// Info logs an info message
func Info(format string, args ...interface{}) {
	if enabled && logger != nil {
		logger.Println(formatMessage("INFO", format, args...))
	}
}

// Warn logs a warning message
func Warn(format string, args ...interface{}) {
	if enabled && logger != nil {
		logger.Println(formatMessage("WARN", format, args...))
	}
}

// Error logs an error message
func Error(format string, args ...interface{}) {
	if enabled && logger != nil {
		logger.Println(formatMessage("ERROR", format, args...))
	}
}

// SetOutput sets additional output (e.g., for debugging to stderr)
func SetOutput(w io.Writer) {
	if logger != nil {
		logger.SetOutput(io.MultiWriter(logFile, w))
	}
}
