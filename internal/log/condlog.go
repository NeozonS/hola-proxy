// Package log provides a verbosity-gated logger and a non-blocking writer
// suitable for funnelling logs from multiple goroutines.
package log

import (
	"fmt"
	"log"
)

const (
	CRITICAL = 50
	ERROR    = 40
	WARNING  = 30
	INFO     = 20
	DEBUG    = 10
	NOTSET   = 0
)

// CondLogger writes a message only if its level is >= the configured verbosity.
type CondLogger struct {
	logger    *log.Logger
	verbosity int
}

// NewCondLogger wraps a *log.Logger with a verbosity threshold.
func NewCondLogger(logger *log.Logger, verbosity int) *CondLogger {
	return &CondLogger{verbosity: verbosity, logger: logger}
}

// Log emits at an arbitrary level; mostly used by tests.
func (cl *CondLogger) Log(verb int, format string, v ...interface{}) error {
	if verb >= cl.verbosity {
		return cl.logger.Output(2, fmt.Sprintf(format, v...))
	}
	return nil
}

func (cl *CondLogger) log(verb int, format string, v ...interface{}) error {
	if verb >= cl.verbosity {
		return cl.logger.Output(3, fmt.Sprintf(format, v...))
	}
	return nil
}

func (cl *CondLogger) Critical(s string, v ...interface{}) error {
	return cl.log(CRITICAL, "CRITICAL "+s, v...)
}
func (cl *CondLogger) Error(s string, v ...interface{}) error {
	return cl.log(ERROR, "ERROR    "+s, v...)
}
func (cl *CondLogger) Warning(s string, v ...interface{}) error {
	return cl.log(WARNING, "WARNING  "+s, v...)
}
func (cl *CondLogger) Info(s string, v ...interface{}) error {
	return cl.log(INFO, "INFO     "+s, v...)
}
func (cl *CondLogger) Debug(s string, v ...interface{}) error {
	return cl.log(DEBUG, "DEBUG    "+s, v...)
}
