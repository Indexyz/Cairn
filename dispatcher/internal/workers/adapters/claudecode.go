// Package adapters implements the agent CLI drivers.
package adapters

import (
	"time"

	"github.com/Indexyz/Cairn/dispatcher/internal/config"
	"github.com/Indexyz/Cairn/dispatcher/internal/health"
	"github.com/Indexyz/Cairn/dispatcher/internal/workers"
)

const anthropicVersion = "2023-06-01"

// ClaudeCode runs the `claude` CLI.
type ClaudeCode struct{ workers.SeedSessionDriver }

// TypeName returns the config discriminator.
func (ClaudeCode) TypeName() string { return "claudecode" }

// LocalBinary reports the host CLI.
func (ClaudeCode) LocalBinary() *string { return workers.StringPtr("claude") }

// CheckHealth pings the Anthropic messages endpoint.
func (ClaudeCode) CheckHealth(worker *config.WorkerConfig, timeout time.Duration) health.Result {
	env := worker.Env
	return health.HTTPPing(env["ANTHROPIC_BASE_URL"]+"/v1/messages", map[string]string{
		"authorization":     "Bearer " + env["ANTHROPIC_AUTH_TOKEN"],
		"anthropic-version": anthropicVersion,
		"content-type":      "application/json",
	}, map[string]any{
		"model":      env["ANTHROPIC_MODEL"],
		"max_tokens": 10,
		"messages":   []any{map[string]any{"role": "user", "content": "ping"}},
	}, timeout, health.ProxyFromEnv(env))
}

// DescribeHealth labels the probe.
func (ClaudeCode) DescribeHealth(worker *config.WorkerConfig) string {
	return "POST " + worker.Env["ANTHROPIC_BASE_URL"] + "/v1/messages (model=" + worker.Env["ANTHROPIC_MODEL"] + ")"
}

// BuildExecute builds the execute argv.
func (ClaudeCode) BuildExecute(_ *config.WorkerConfig, prompt string, session *string) workers.DriverResult {
	argv := []string{"claude", "--session-id", deref(session), "--dangerously-skip-permissions", "-p", "--", prompt}
	return workers.DriverResult{Argv: argv, Session: session}
}

// BuildConclude resumes the session with the conclude prompt.
func (ClaudeCode) BuildConclude(_ *config.WorkerConfig, prompt, session string) []string {
	return []string{"claude", "-r", session, "--dangerously-skip-permissions", "-p", "--", prompt}
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
