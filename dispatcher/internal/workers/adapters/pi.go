package adapters

import (
	"encoding/json"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/Indexyz/Cairn/dispatcher/internal/config"
	"github.com/Indexyz/Cairn/dispatcher/internal/health"
	"github.com/Indexyz/Cairn/dispatcher/internal/workers"
)

// Pi runs the `pi` CLI.
type Pi struct {
	workers.Base
	local bool
}

// NewPi builds a driver; local mode reuses the host CLI configuration.
func NewPi(local bool) *Pi { return &Pi{local: local} }

// TypeName returns the config discriminator.
func (*Pi) TypeName() string { return "pi" }

// LocalBinary reports the host CLI.
func (*Pi) LocalBinary() *string { return workers.StringPtr("pi") }

// CheckHealth pings the configured provider API shape.
func (*Pi) CheckHealth(worker *config.WorkerConfig, timeout time.Duration) health.Result {
	env := worker.Env
	base := strings.TrimRight(env["PI_BASE_URL"], "/")
	model := env["PI_MODEL"]
	api := env["PI_PROVIDER_API"]
	proxy := health.ProxyFromEnv(env)
	headers := map[string]string{
		"authorization": "Bearer " + env["PI_API_KEY"],
		"content-type":  "application/json",
	}
	switch {
	case strings.Contains(api, "anthropic"):
		headers["anthropic-version"] = anthropicVersion
		return health.HTTPPing(base+"/v1/messages", headers, map[string]any{
			"model": model, "max_tokens": 10,
			"messages": []any{map[string]any{"role": "user", "content": "ping"}},
		}, timeout, proxy)
	case strings.Contains(api, "responses"):
		return health.HTTPPing(base+"/responses", headers, map[string]any{
			"model": model, "input": []any{map[string]any{"role": "user", "content": "ping"}}, "stream": false,
		}, timeout, proxy)
	default:
		// openai-completions and anything else: OpenAI-compatible chat/completions
		return health.HTTPPing(base+"/chat/completions", headers, map[string]any{
			"model": model, "max_tokens": 10,
			"messages": []any{map[string]any{"role": "user", "content": "ping"}},
		}, timeout, proxy)
	}
}

// DescribeHealth labels the probe.
func (*Pi) DescribeHealth(worker *config.WorkerConfig) string {
	return "POST " + worker.Env["PI_BASE_URL"] + " (api=" + worker.Env["PI_PROVIDER_API"] + ", model=" + worker.Env["PI_MODEL"] + ")"
}

// BuildExecute builds the execute argv.
func (driver *Pi) BuildExecute(worker *config.WorkerConfig, prompt string, session *string) workers.DriverResult {
	if driver.local {
		return workers.DriverResult{Argv: driver.localArgv(worker, prompt, session), Session: session}
	}
	env := worker.Env
	argv := []string{
		"--provider", "cairn",
		"--model", env["PI_MODEL"],
		"--mode", "json",
		"--session-dir", sessionDir(worker),
	}
	if session != nil && *session != "" {
		argv = append(argv, "--session", *session)
	}
	argv = append(argv, "-p", prompt)
	return workers.DriverResult{Argv: driver.wrapWithModels(worker, argv), Session: session}
}

// BuildConclude resumes the session with the conclude prompt.
func (driver *Pi) BuildConclude(worker *config.WorkerConfig, prompt, session string) []string {
	if driver.local {
		return driver.localArgv(worker, prompt, &session)
	}
	env := worker.Env
	argv := []string{
		"--provider", "cairn",
		"--model", env["PI_MODEL"],
		"--mode", "json",
		"--session-dir", sessionDir(worker),
		"--session", session,
		"-p", prompt,
	}
	return driver.wrapWithModels(worker, argv)
}

// ExtractSession reads the session id from the JSON event stream.
func (driver *Pi) ExtractSession(session *string, stdout, _ string) *string {
	if session != nil && *session != "" {
		return session
	}
	for _, event := range iterEvents(stdout) {
		if event["type"] != "session" {
			continue
		}
		if id, ok := event["id"].(string); ok && id != "" {
			return &id
		}
	}
	return nil
}

// ExtractResponseText returns the last assistant text from the event stream.
func (driver *Pi) ExtractResponseText(stdout, _ string) string {
	var assistant map[string]any
	for _, event := range iterEvents(stdout) {
		switch event["type"] {
		case "turn_end":
			if message, ok := event["message"].(map[string]any); ok && message["role"] == "assistant" {
				assistant = message
			}
		case "agent_end":
			messages, ok := event["messages"].([]any)
			if !ok {
				continue
			}
			for index := len(messages) - 1; index >= 0; index-- {
				message, ok := messages[index].(map[string]any)
				if ok && message["role"] == "assistant" {
					assistant = message
					break
				}
			}
		}
	}
	if assistant == nil {
		return stdout
	}
	content, ok := assistant["content"].([]any)
	if !ok {
		return stdout
	}
	parts := make([]string, 0, len(content))
	for _, item := range content {
		object, ok := item.(map[string]any)
		if !ok || object["type"] != "text" {
			continue
		}
		if text, ok := object["text"].(string); ok && text != "" {
			parts = append(parts, text)
		}
	}
	joined := strings.TrimSpace(strings.Join(parts, "\n"))
	if joined == "" {
		return stdout
	}
	return joined
}

func (driver *Pi) localArgv(worker *config.WorkerConfig, prompt string, session *string) []string {
	sessionDirectory := sessionDir(worker)
	piArgv := []string{
		"--mode", "json",
		"--session-dir", sessionDirectory,
		"--no-extensions",
		"--no-skills",
		"--no-prompt-templates",
		"--no-themes",
		"--no-context-files",
		"--tools", "read,write,edit,bash,grep,find,ls",
	}
	if session != nil && *session != "" {
		piArgv = append(piArgv, "--session", *session)
	}
	piArgv = append(piArgv, "-p", prompt)
	script := "sdir=\"$1\"\nshift\nmkdir -p \"$sdir\"\nexec pi \"$@\"\n"
	return append([]string{"/bin/sh", "-lc", script, "--", sessionDirectory}, piArgv...)
}

func (driver *Pi) wrapWithModels(worker *config.WorkerConfig, piArgv []string) []string {
	script := "agent_dir=\"$1\"\n" +
		"models_json=\"$2\"\n" +
		"shift 2\n" +
		"mkdir -p \"$agent_dir\"\n" +
		"mkdir -p \"$agent_dir/sessions\"\n" +
		"printf \"%s\" \"$models_json\" > \"$agent_dir/models.json\"\n" +
		"exec env PI_CODING_AGENT_DIR=\"$agent_dir\" pi \"$@\"\n"
	argv := []string{
		"--no-extensions",
		"--no-skills",
		"--no-prompt-templates",
		"--no-themes",
		"--no-context-files",
		"--tools", "read,write,edit,bash,grep,find,ls",
	}
	prefix := []string{"/bin/sh", "-lc", script, "--", agentDir(worker), modelsJSON(worker)}
	prefix = append(prefix, argv...)
	return append(prefix, piArgv...)
}

func agentDir(worker *config.WorkerConfig) string {
	return path.Join("/tmp/cairn-pi", worker.Name)
}

func sessionDir(worker *config.WorkerConfig) string {
	return path.Join(agentDir(worker), "sessions")
}

func iterEvents(stdout string) []map[string]any {
	events := []map[string]any{}
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			continue
		}
		events = append(events, payload)
	}
	return events
}

func modelsJSON(worker *config.WorkerConfig) string {
	env := worker.Env
	model := map[string]any{"id": env["PI_MODEL"], "name": env["PI_MODEL"]}
	if window := env["PI_MODEL_CONTEXT_WINDOW"]; window != "" {
		if parsed, err := strconv.Atoi(window); err == nil {
			model["contextWindow"] = parsed
		}
	}
	provider := map[string]any{
		"baseUrl": env["PI_BASE_URL"],
		"api":     env["PI_PROVIDER_API"],
		"apiKey":  env["PI_API_KEY"],
		"models":  []any{model},
	}
	payload := map[string]any{"providers": map[string]any{"cairn": provider}}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}
