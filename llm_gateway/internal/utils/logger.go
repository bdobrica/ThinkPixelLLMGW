package utils

import (
	"fmt"
	"log"
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
	logger        *log.Logger
	logLevel      LogLevel
	logLevelMutex sync.Mutex
}

// NewLogger creates a new logger with a given prefix
func NewLogger(prefix string, logLevel ...LogLevel) *Logger {
	logLevelValue := Warning
	if len(logLevel) > 0 {
		logLevelValue = logLevel[0]
	}
	return &Logger{
		prefix:   prefix,
		logger:   log.New(os.Stdout, fmt.Sprintf("[%s] ", prefix), log.LstdFlags),
		logLevel: logLevelValue,
	}
}

// SetLogLevel sets the logging level
func (l *Logger) SetLogLevel(logLevel LogLevel) {
	l.logLevelMutex.Lock()
	defer l.logLevelMutex.Unlock()
	l.logLevel = logLevel
}

// Info logs an informational message
func (l *Logger) Info(msg string, keyvals ...interface{}) {
	l.logLevelMutex.Lock()
	defer l.logLevelMutex.Unlock()
	if l.logLevel > Info {
		return
	}
	l.logger.Println(l.formatMessage("INFO", msg, keyvals...))
}

// Error logs an error message
func (l *Logger) Error(msg string, keyvals ...interface{}) {
	l.logLevelMutex.Lock()
	defer l.logLevelMutex.Unlock()
	if l.logLevel > Error {
		return
	}
	l.logger.Println(l.formatMessage("ERROR", msg, keyvals...))
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, keyvals ...interface{}) {
	l.logLevelMutex.Lock()
	defer l.logLevelMutex.Unlock()
	if l.logLevel > Warning {
		return
	}
	l.logger.Println(l.formatMessage("WARN", msg, keyvals...))
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, keyvals ...interface{}) {
	l.logLevelMutex.Lock()
	defer l.logLevelMutex.Unlock()
	if l.logLevel > Debug {
		return
	}
	l.logger.Println(l.formatMessage("DEBUG", msg, keyvals...))
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
		log.Println("Error:", err)
	}
}
