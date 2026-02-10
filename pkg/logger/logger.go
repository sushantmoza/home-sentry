package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

var levelNames = map[LogLevel]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
}

type Logger struct {
	mu          sync.Mutex
	level       LogLevel
	file        *os.File
	logDir      string
	currentDate string
	writers     io.Writer
	done        chan struct{}
}

var defaultLogger *Logger

// Init initializes the global logger
func Init(logDir string, level LogLevel) error {
	logger, err := NewLogger(logDir, level)
	if err != nil {
		return err
	}
	defaultLogger = logger

	// Redirect standard log package to our logger
	log.SetOutput(logger)
	log.SetFlags(0) // We handle formatting ourselves

	return nil
}

// NewLogger creates a new logger instance
func NewLogger(logDir string, level LogLevel) (*Logger, error) {
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	l := &Logger{
		level:  level,
		logDir: logDir,
		done:   make(chan struct{}),
	}

	if err := l.rotateLogFile(); err != nil {
		return nil, err
	}

	// Cleanup old logs
	go l.cleanupOldLogs(7 * 24 * time.Hour)

	return l, nil
}

func (l *Logger) rotateLogFile() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	if l.currentDate == today && l.file != nil {
		return nil
	}

	// Close old file if exists
	if l.file != nil {
		l.file.Close()
	}

	// Open new log file
	logPath := filepath.Join(l.logDir, fmt.Sprintf("home-sentry-%s.log", today))
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	l.file = file
	l.currentDate = today
	l.writers = io.MultiWriter(os.Stdout, file)

	return nil
}

func (l *Logger) cleanupOldLogs(maxAge time.Duration) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	// Run immediately on startup
	l.doCleanup(maxAge)

	for {
		select {
		case <-ticker.C:
			l.doCleanup(maxAge)
		case <-l.done:
			return
		}
	}
}

func (l *Logger) doCleanup(maxAge time.Duration) {
	files, err := filepath.Glob(filepath.Join(l.logDir, "home-sentry-*.log"))
	if err != nil {
		return
	}

	cutoff := time.Now().Add(-maxAge)
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(file)
		}
	}
}

// sanitizeFormatString removes format specifiers from user input to prevent format string attacks
func sanitizeFormatString(s string) string {
	// Replace % with %% to escape format specifiers
	// This prevents user input like "%s%s%s" from being interpreted as format directives
	return strings.ReplaceAll(s, "%", "%%")
}

// sanitizeLogMessage sanitizes all arguments to prevent format string injection
func sanitizeLogMessage(args []interface{}) []interface{} {
	sanitized := make([]interface{}, len(args))
	for i, arg := range args {
		switch v := arg.(type) {
		case string:
			sanitized[i] = sanitizeFormatString(v)
		default:
			sanitized[i] = arg
		}
	}
	return sanitized
}

func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	// Check for daily rotation
	l.rotateLogFile()

	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	levelStr := levelNames[level]

	// Sanitize arguments to prevent format string injection
	sanitizedArgs := sanitizeLogMessage(args)
	message := fmt.Sprintf(format, sanitizedArgs...)

	// Get caller info
	_, file, line, ok := runtime.Caller(2)
	caller := ""
	if ok {
		caller = fmt.Sprintf("%s:%d", filepath.Base(file), line)
	}

	logLine := fmt.Sprintf("[%s] [%s] [%s] %s\n", timestamp, levelStr, caller, message)

	if l.writers != nil {
		l.writers.Write([]byte(logLine))
	}
}

// Write implements io.Writer for compatibility with standard log package
func (l *Logger) Write(p []byte) (n int, err error) {
	msg := strings.TrimSpace(string(p))
	l.log(INFO, "%s", msg)
	return len(p), nil
}

// Close stops the cleanup goroutine and closes the log file
func (l *Logger) Close() error {
	// Signal cleanup goroutine to stop
	select {
	case <-l.done:
		// Already closed
	default:
		close(l.done)
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// Package-level logging functions
func Debug(format string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.log(DEBUG, format, args...)
	}
}

func Info(format string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.log(INFO, format, args...)
	}
}

func Warn(format string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.log(WARN, format, args...)
	}
}

func Error(format string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.log(ERROR, format, args...)
	}
}

// GetLogDir returns the log directory path
func GetLogDir() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "logs"
	}
	return filepath.Join(appData, "HomeSentry", "logs")
}

// GetRecentLogs returns the most recent log entries
func GetRecentLogs(count int) ([]string, error) {
	logDir := GetLogDir()
	files, err := filepath.Glob(filepath.Join(logDir, "home-sentry-*.log"))
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return []string{}, nil
	}

	// Sort by name (date) descending
	sort.Sort(sort.Reverse(sort.StringSlice(files)))

	// Read from most recent file
	content, err := os.ReadFile(files[0])
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")

	// Return last N lines
	start := len(lines) - count
	if start < 0 {
		start = 0
	}

	return lines[start:], nil
}
