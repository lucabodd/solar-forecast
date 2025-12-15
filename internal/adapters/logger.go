package adapters

import (
	"fmt"
	"log"
)

// SimpleLogger implements ports.Logger using standard log package
type SimpleLogger struct {
	debugEnabled bool
}

// NewSimpleLogger creates a new simple logger
func NewSimpleLogger(debugEnabled bool) *SimpleLogger {
	return &SimpleLogger{
		debugEnabled: debugEnabled,
	}
}

// Info logs an info level message
func (l *SimpleLogger) Info(msg string, args ...interface{}) {
	prefix := "[INFO] "
	l.logMessage(prefix, msg, args...)
}

// Error logs an error level message
func (l *SimpleLogger) Error(msg string, args ...interface{}) {
	prefix := "[ERROR] "
	l.logMessage(prefix, msg, args...)
}

// Debug logs a debug level message (only if enabled)
func (l *SimpleLogger) Debug(msg string, args ...interface{}) {
	if l.debugEnabled {
		prefix := "[DEBUG] "
		l.logMessage(prefix, msg, args...)
	}
}

// Warn logs a warning level message
func (l *SimpleLogger) Warn(msg string, args ...interface{}) {
	prefix := "[WARN] "
	l.logMessage(prefix, msg, args...)
}

// logMessage formats and logs a message with key-value pairs
func (l *SimpleLogger) logMessage(prefix, msg string, args ...interface{}) {
	message := msg
	if len(args) > 0 {
		// Format key-value pairs
		if len(args)%2 == 0 {
			for i := 0; i < len(args); i += 2 {
				message += fmt.Sprintf(" %v=%v", args[i], args[i+1])
			}
		} else {
			// Odd number of args, just append them
			message = fmt.Sprintf("%s %v", msg, args)
		}
	}
	log.Printf("%s%s\n", prefix, message)
}
