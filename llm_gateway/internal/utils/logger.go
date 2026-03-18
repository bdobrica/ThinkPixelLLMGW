package utils

import (
	"fmt"
	"log/slog"
	"os"
	"sync"
)

// LogLevel represents an enumeration of log levels
type LogLevel int

const (
	Critical LogLevel = 50
	Fatal    LogLevel = Critical
	Error    LogLevel = 40
	Warning  LogLevel = 30
	Info     LogLevel = 20
	Debug    LogLevel = 10
	NotSet   LogLevel = 0
)

// Logger provides structured logging with context
type Logger struct {
	prefix        string
	logger        *slog.Logger
	logLevel      LogLevel
	logLevelMutex sync.Mutex
}

func toSlogLevel(level LogLevel) slog.Level {
	switch {
	case level <= Debug:
		return slog.LevelDebug
	case level <= Info:
		return slog.LevelInfo
	case level <= Warning:
		return slog.LevelWarn
	default:
		return slog.LevelError
	}
}

// NewLogger creates a new logger with a given prefix
func NewLogger(prefix string, logLevel ...LogLevel) *Logger {
	logLevelValue := Warning
	if len(logLevel) > 0 {
		logLevelValue = logLevel[0]
	}
	return &Logger{
		prefix:   prefix,
		logger:   slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: toSlogLevel(logLevelValue)})).With("component", prefix),
		logLevel: logLevelValue,
	}
}

// SetLogLevel sets the logging level
func (l *Logger) SetLogLevel(logLevel LogLevel) {
	l.logLevelMutex.Lock()
	defer l.logLevelMutex.Unlock()
	l.logLevel = logLevel
	l.logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: toSlogLevel(logLevel)})).With("component", l.prefix)
}

// Info logs an informational message
func (l *Logger) Info(msg string, keyvals ...interface{}) {
	l.logLevelMutex.Lock()
	defer l.logLevelMutex.Unlock()
	if l.logLevel > Info {
		return
	}
	l.logger.Info(msg, keyvals...)
}

// Error logs an error message
func (l *Logger) Error(msg string, keyvals ...interface{}) {
	l.logLevelMutex.Lock()
	defer l.logLevelMutex.Unlock()
	if l.logLevel > Error {
		return
	}
	l.logger.Error(msg, keyvals...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, keyvals ...interface{}) {
	l.logLevelMutex.Lock()
	defer l.logLevelMutex.Unlock()
	if l.logLevel > Warning {
		return
	}
	l.logger.Warn(msg, keyvals...)
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, keyvals ...interface{}) {
	l.logLevelMutex.Lock()
	defer l.logLevelMutex.Unlock()
	if l.logLevel > Debug {
		return
	}
	l.logger.Debug(msg, keyvals...)
}

// formatMessage formats a message with key-value pairs
func (l *Logger) formatMessage(level, msg string, keyvals ...interface{}) string {
	formatted := fmt.Sprintf("[%s] %s", level, msg)
	for i := 0; i < len(keyvals); i += 2 {
		if i+1 < len(keyvals) {
			formatted += fmt.Sprintf(" %v=%v", keyvals[i], keyvals[i+1])
		}
	}
	return formatted
}

// LogError logs an error message
func LogError(err error) {
	if err != nil {
		slog.Error("operation failed", "error", err)
	}
}
