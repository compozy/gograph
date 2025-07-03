package logger

import (
	"io"
	"os"
	"sync"

	"github.com/charmbracelet/log"
)

var (
	// Default logger instance
	logger *log.Logger

	// Initialize logger once
	initLoggerOnce sync.Once
)

// InitLogger initializes the default logger
func InitLogger() {
	initLoggerOnce.Do(func() {
		// Initialize default logger
		logger = log.New(os.Stderr)
		logger.SetLevel(log.InfoLevel)
	})
}

// ensureInitialized ensures the logger is initialized before use
func ensureInitialized() {
	InitLogger()
}

// SetLevel sets the logging level
func SetLevel(level log.Level) {
	ensureInitialized()
	logger.SetLevel(level)
}

// SetDebug enables debug logging
func SetDebug(debug bool) {
	ensureInitialized()
	if debug {
		logger.SetLevel(log.DebugLevel)
	} else {
		logger.SetLevel(log.InfoLevel)
	}
}

// Info logs an info message
func Info(msg string, keyvals ...any) {
	ensureInitialized()
	logger.Info(msg, keyvals...)
}

// Debug logs a debug message
func Debug(msg string, keyvals ...any) {
	ensureInitialized()
	logger.Debug(msg, keyvals...)
}

// Error logs an error message
func Error(msg string, keyvals ...any) {
	ensureInitialized()
	logger.Error(msg, keyvals...)
}

// Warn logs a warning message
func Warn(msg string, keyvals ...any) {
	ensureInitialized()
	logger.Warn(msg, keyvals...)
}

// Fatal logs a fatal message and exits
func Fatal(msg string, keyvals ...any) {
	ensureInitialized()
	logger.Fatal(msg, keyvals...)
}

// With returns a new logger with additional context
func With(keyvals ...any) *log.Logger {
	ensureInitialized()
	return logger.With(keyvals...)
}

// Disable completely disables logging output
func Disable() {
	ensureInitialized()
	logger.SetOutput(io.Discard)
}

// Enable re-enables logging output to stderr
func Enable() {
	ensureInitialized()
	logger.SetOutput(os.Stderr)
}
