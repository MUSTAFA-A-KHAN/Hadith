package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"sync"
	"time"
)

// Level represents log level
type Level int

const (
	DebugLevel Level = iota
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
)

// String returns string representation of level
func (l Level) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	case FatalLevel:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// Logger provides structured logging
type Logger struct {
	mu       sync.Mutex
	level    Level
	output   io.Writer
	prefix   string
	flags    int
	callers  bool
}

// New creates a new logger
func New(output io.Writer, level Level, callers bool) *Logger {
	return &Logger{
		level:   level,
		output:  output,
		callers: callers,
		flags:   log.LstdFlags,
	}
}

// Default creates a default logger
func Default() *Logger {
	return New(os.Stdout, InfoLevel, true)
}

// WithLevel creates a logger with specific level
func (l *Logger) WithLevel(level Level) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	newLogger := *l
	newLogger.level = level
	return &newLogger
}

// log writes a log entry
func (l *Logger) log(level Level, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	message := fmt.Sprintf(format, args...)
	
	var callerInfo string
	if l.callers {
		_, file, line, ok := runtime.Caller(2)
		if ok {
			callerInfo = fmt.Sprintf(" (%s:%d)", file, line)
		}
	}

	logLine := fmt.Sprintf("[%s] [%s] %s%s", timestamp, level.String(), message, callerInfo)
	
	if l.prefix != "" {
		logLine = fmt.Sprintf("[%s] %s", l.prefix, logLine)
	}

	fmt.Fprintln(l.output, logLine)

	if level == FatalLevel {
		os.Exit(1)
	}
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DebugLevel, format, args...)
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(InfoLevel, format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(WarnLevel, format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ErrorLevel, format, args...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(format string, args ...interface{}) {
	l.log(FatalLevel, format, args...)
}

// WithPrefix creates a logger with prefix
func (l *Logger) WithPrefix(prefix string) *Logger {
	l.mu.Lock()
	defer l.mu.Unlock()

	newLogger := *l
	newLogger.prefix = prefix
	return &newLogger
}

// Helper functions for common logging patterns

// LogRequest logs an incoming request
func (l *Logger) LogRequest(updateID int64, chatID int64, command string) {
	l.Debug("Request: update_id=%d chat_id=%d command=%s", updateID, chatID, command)
}

// LogResponse logs a response
func (l *Logger) LogResponse(chatID int64, messageType string, success bool) {
	if success {
		l.Debug("Response: chat_id=%d type=%s status=success", chatID, messageType)
	} else {
		l.Warn("Response: chat_id=%d type=%s status=failed", chatID, messageType)
	}
}

// LogError logs an error with context
func (l *Logger) LogError(err error, context string) {
	l.Error("Error: %v (context: %s)", err, context)
}

// LogCallback logs a callback query
func (l *Logger) LogCallback(callbackID string, chatID int64, data string) {
	l.Debug("Callback: callback_id=%s chat_id=%d data=%s", callbackID, chatID, data)
}

