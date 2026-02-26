package logger

import (
	"fmt"
	"log"
)

// StdLogger wraps Go's standard library logger to implement the Logger interface.
type StdLogger struct {
	stdLog   *log.Logger
	quietLog *log.Logger // For logging that should be suppressed outside windows
}

// NewStdLogger creates a logger that writes important messages to stdout and all messages to file.
func NewStdLogger(stdLog, quietLog *log.Logger) *StdLogger {
	return &StdLogger{
		stdLog:   stdLog,
		quietLog: quietLog,
	}
}

// Infof logs an informational message (written to quiet logger only).
func (l *StdLogger) Infof(msg string, args ...interface{}) {
	if l.quietLog != nil {
		l.quietLog.Printf(msg, args...)
	}
}

// Warnf logs a warning message (written to both stdout and quiet logger).
func (l *StdLogger) Warnf(msg string, args ...interface{}) {
	formatted := fmt.Sprintf(msg, args...)
	if l.stdLog != nil {
		l.stdLog.Printf("⚠️  %s", formatted)
	}
	if l.quietLog != nil {
		l.quietLog.Printf("⚠️  %s", formatted)
	}
}

// Errorf logs an error message (written to both stdout and quiet logger).
func (l *StdLogger) Errorf(msg string, args ...interface{}) {
	formatted := fmt.Sprintf(msg, args...)
	if l.stdLog != nil {
		l.stdLog.Printf("❌ %s", formatted)
	}
	if l.quietLog != nil {
		l.quietLog.Printf("❌ %s", formatted)
	}
}

// Debugf logs a debug message (written to quiet logger only).
func (l *StdLogger) Debugf(msg string, args ...interface{}) {
	if l.quietLog != nil {
		l.quietLog.Printf("[DEBUG] "+msg, args...)
	}
}

// QuietInfof logs an informational message that should be suppressed outside sign windows.
func (l *StdLogger) QuietInfof(msg string, args ...interface{}) {
	if l.quietLog != nil {
		l.quietLog.Printf(msg, args...)
	}
}
