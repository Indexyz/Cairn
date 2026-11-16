// Package workers defines the driver surface every agent CLI implementation fills.
package workers

import (
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"

	"github.com/Indexyz/Cairn/dispatcher/internal/config"
	"github.com/Indexyz/Cairn/dispatcher/internal/health"
)

// DriverResult is the argv for one worker invocation.
type DriverResult struct {
	Argv    []string
	Session *string
}

// Driver abstracts one agent CLI.
type Driver interface {
	TypeName() string
	SupportsConclude() bool
	// LocalBinary is the host executable to verify in local mode; nil when not applicable.
	LocalBinary() *string
	PrepareSession() *string
	CheckHealth(worker *config.WorkerConfig, timeout time.Duration) health.Result
	DescribeHealth(worker *config.WorkerConfig) string
	BuildExecute(worker *config.WorkerConfig, prompt string, session *string) DriverResult
	BuildConclude(worker *config.WorkerConfig, prompt, session string) []string
	ExtractSession(session *string, stdout, stderr string) *string
	ExtractResponseText(stdout, stderr string) string
}

// Base provides the default driver behavior.
type Base struct{}

// SupportsConclude reports that the driver can resume a session.
func (Base) SupportsConclude() bool { return true }

// LocalBinary reports no host binary by default.
func (Base) LocalBinary() *string { return nil }

// PrepareSession generates no session by default.
func (Base) PrepareSession() *string { return nil }

// DescribeHealth labels the in-process ping.
func (Base) DescribeHealth(*config.WorkerConfig) string { return "in-process API ping" }

// ExtractSession keeps the current session by default.
func (Base) ExtractSession(session *string, _, _ string) *string { return session }

// ExtractResponseText returns stdout by default.
func (Base) ExtractResponseText(stdout, _ string) string { return stdout }

// SeedSessionDriver seeds a random session id before the first run.
type SeedSessionDriver struct{ Base }

// PrepareSession returns a fresh uuid.
func (SeedSessionDriver) PrepareSession() *string {
	id := uuid.NewString()
	return &id
}

// RegexSessionDriver reads a session id out of stderr when none is known yet.
type RegexSessionDriver struct {
	Base
	Pattern *regexp.Regexp
}

// ExtractSession falls back to the stderr pattern.
func (driver RegexSessionDriver) ExtractSession(session *string, _, stderr string) *string {
	if session != nil && *session != "" {
		return session
	}
	match := driver.Pattern.FindStringSubmatch(stderr)
	if len(match) < 2 {
		return nil
	}
	id := match[1]
	return &id
}

// StringPtr is a small helper for optional string fields.
func StringPtr(value string) *string { return &value }

// MissingEnvError reports an absent required env key.
func MissingEnvError(key string) error {
	return fmt.Errorf("missing env key %s", key)
}
