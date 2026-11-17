package adapters

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
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
	argv := []string{
		"codex", "exec", "--dangerously-bypass-approvals-and-sandbox",
		"--model", env["CODEX_MODEL"],
		"-c", `model_provider="cairn"`,
		"-c", `model_providers.cairn.name="cairn"`,
		"-c", `model_providers.cairn.wire_api="responses"`,
		"-c", `model_reasoning_effort="high"`,
		"-c", `model_providers.cairn.base_url="` + env["CODEX_BASE_URL"] + `"`,
		"-c", `model_providers.cairn.env_key="OPENAI_API_KEY"`,
	}
	return append(argv, codexMCPArgs(worker)...)
}

// codexMCPArgs translates MCP_CONFIG into codex `-c` overrides.
func codexMCPArgs(worker *config.WorkerConfig) []string {
	raw := strings.TrimSpace(worker.Env["MCP_CONFIG"])
	if raw == "" {
		return nil
	}
	var document struct {
		Servers map[string]struct {
			Command string            `json:"command"`
			Args    []string          `json:"args"`
			Env     map[string]string `json:"env"`
			URL     string            `json:"url"`
			Headers map[string]string `json:"headers"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(raw), &document); err != nil {
		return nil
	}
	names := make([]string, 0, len(document.Servers))
	for name := range document.Servers {
		names = append(names, name)
	}
	sort.Strings(names)
	var argv []string
	for _, name := range names {
		server := document.Servers[name]
		prefix := "mcp_servers." + name
		if server.Command != "" {
			argv = append(argv, "-c", fmt.Sprintf("%s.command=%q", prefix, server.Command))
			if len(server.Args) > 0 {
				quoted := make([]string, len(server.Args))
				for index, arg := range server.Args {
					quoted[index] = fmt.Sprintf("%q", arg)
				}
				argv = append(argv, "-c", fmt.Sprintf("%s.args=[%s]", prefix, strings.Join(quoted, ", ")))
			}
		}
		if server.URL != "" {
			argv = append(argv, "-c", fmt.Sprintf("%s.url=%q", prefix, server.URL))
		}
		if len(server.Env) > 0 {
			keys := sortedKeys(server.Env)
			pairs := make([]string, len(keys))
			for index, key := range keys {
				pairs[index] = fmt.Sprintf("%s=%q", key, server.Env[key])
			}
			argv = append(argv, "-c", fmt.Sprintf("%s.env={%s}", prefix, strings.Join(pairs, ", ")))
		}
		if len(server.Headers) > 0 {
			keys := sortedKeys(server.Headers)
			pairs := make([]string, len(keys))
			for index, key := range keys {
				pairs[index] = fmt.Sprintf("%s=%q", key, server.Headers[key])
			}
			argv = append(argv, "-c", fmt.Sprintf("%s.http_headers={%s}", prefix, strings.Join(pairs, ", ")))
		}
	}
	return argv
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
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
