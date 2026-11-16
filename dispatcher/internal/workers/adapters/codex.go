package adapters

import (
	"regexp"
	"time"

	"github.com/Indexyz/Cairn/dispatcher/internal/config"
	"github.com/Indexyz/Cairn/dispatcher/internal/health"
	"github.com/Indexyz/Cairn/dispatcher/internal/workers"
)

// Codex runs the `codex` CLI.
type Codex struct {
	workers.RegexSessionDriver
	local bool
}

// NewCodex builds a driver; local mode reuses the host CLI configuration.
func NewCodex(local bool) *Codex {
	return &Codex{
		RegexSessionDriver: workers.RegexSessionDriver{
			Pattern: regexp.MustCompile(`session id:\s*([0-9a-fA-F-]+)`),
		},
		local: local,
	}
}

// TypeName returns the config discriminator.
func (*Codex) TypeName() string { return "codex" }

// LocalBinary reports the host CLI.
func (*Codex) LocalBinary() *string { return workers.StringPtr("codex") }

// CheckHealth pings the responses endpoint.
func (*Codex) CheckHealth(worker *config.WorkerConfig, timeout time.Duration) health.Result {
	env := worker.Env
	return health.HTTPPing(env["CODEX_BASE_URL"]+"/responses", map[string]string{
		"authorization": "Bearer " + env["OPENAI_API_KEY"],
		"content-type":  "application/json",
	}, map[string]any{
		"model":  env["CODEX_MODEL"],
		"input":  []any{map[string]any{"role": "user", "content": "ping"}},
		"stream": false,
	}, timeout, health.ProxyFromEnv(env))
}

// DescribeHealth labels the probe.
func (*Codex) DescribeHealth(worker *config.WorkerConfig) string {
	return "POST " + worker.Env["CODEX_BASE_URL"] + "/responses (model=" + worker.Env["CODEX_MODEL"] + ")"
}

func (driver *Codex) executePrefix(worker *config.WorkerConfig) []string {
	if driver.local {
		return []string{"codex", "exec", "--dangerously-bypass-approvals-and-sandbox"}
	}
	env := worker.Env
	return []string{
		"codex", "exec", "--dangerously-bypass-approvals-and-sandbox",
		"--model", env["CODEX_MODEL"],
		"-c", `model_provider="cairn"`,
		"-c", `model_providers.cairn.name="cairn"`,
		"-c", `model_providers.cairn.wire_api="responses"`,
		"-c", `model_reasoning_effort="high"`,
		"-c", `model_providers.cairn.base_url="` + env["CODEX_BASE_URL"] + `"`,
		"-c", `model_providers.cairn.env_key="OPENAI_API_KEY"`,
	}
}

// BuildExecute builds the execute argv.
func (driver *Codex) BuildExecute(worker *config.WorkerConfig, prompt string, session *string) workers.DriverResult {
	argv := append(driver.executePrefix(worker), "--", prompt)
	return workers.DriverResult{Argv: argv, Session: session}
}

// BuildConclude resumes the session with the conclude prompt.
func (driver *Codex) BuildConclude(worker *config.WorkerConfig, prompt, session string) []string {
	if driver.local {
		return []string{"codex", "exec", "resume", session, "--dangerously-bypass-approvals-and-sandbox", "--", prompt}
	}
	argv := append([]string{"codex", "exec", "resume", session}, driver.executePrefix(worker)[2:]...)
	return append(argv, "--", prompt)
}
