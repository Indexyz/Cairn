// Package logging renders dispatcher logs in the same shape as the Python
// implementation: `[time] LEVEL shortname message`, or bare messages for the
// startup healthcheck CLI.
package logging

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

type Level int

const (
	Debug Level = iota
	Info
	Warning
	Error
)

const prefix = "cairn.dispatcher."

var (
	mu     sync.Mutex
	level  = Info
	bare   bool
	output io.Writer = os.Stderr
)

// Configure sets the global threshold and output format.
func Configure(l Level, bareOutput bool) {
	mu.Lock()
	defer mu.Unlock()
	level = l
	bare = bareOutput
}

// ParseLevel maps a CLI flag to a threshold, defaulting to info.
func ParseLevel(raw string) Level {
	switch strings.ToLower(raw) {
	case "debug":
		return Debug
	case "warning", "warn":
		return Warning
	case "error":
		return Error
	default:
		return Info
	}
}

// SetOutput redirects log output (tests).
func SetOutput(w io.Writer) {
	mu.Lock()
	defer mu.Unlock()
	output = w
}

// Logger is a named logger; the name is shortened by dropping the package prefix.
type Logger struct {
	short string
}

// New builds a logger for a module name such as "cairn.dispatcher.scheduler.loop".
func New(name string) *Logger {
	return &Logger{short: strings.TrimPrefix(name, prefix)}
}

func (l *Logger) logf(lv Level, label, format string, args ...any) {
	mu.Lock()
	threshold, bareOutput, w := level, bare, output
	mu.Unlock()
	if lv < threshold {
		return
	}
	message := fmt.Sprintf(format, args...)
	if bareOutput {
		fmt.Fprintln(w, message)
		return
	}
	fmt.Fprintf(w, "[%s] %s %s %s\n", time.Now().Format("2006-01-02 15:04:05"), label, l.short, message)
}

func (l *Logger) Debugf(format string, args ...any) { l.logf(Debug, "DEBUG", format, args...) }
func (l *Logger) Infof(format string, args ...any)  { l.logf(Info, "INFO", format, args...) }
func (l *Logger) Warnf(format string, args ...any)  { l.logf(Warning, "WARNING", format, args...) }
func (l *Logger) Errorf(format string, args ...any) { l.logf(Error, "ERROR", format, args...) }
