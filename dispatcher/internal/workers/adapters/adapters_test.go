package adapters

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Indexyz/Cairn/dispatcher/internal/config"
)

func worker(workerType config.WorkerType, env map[string]string) *config.WorkerConfig {
	return &config.WorkerConfig{
		Name:       string(workerType),
		Type:       workerType,
		TaskTypes:  []config.TaskType{config.TaskReason},
		MaxRunning: 1,
		Priority:   0,
		Env:        env,
	}
}

func TestPiDriverModelsJSONAndExecuteArgvIncludeContextWindowAndTools(t *testing.T) {
	value := worker(config.WorkerPi, map[string]string{
		"PI_MODEL":                "model",
		"PI_BASE_URL":             "http://api",
		"PI_API_KEY":              "secret",
		"PI_PROVIDER_API":         "openai-completions",
		"PI_MODEL_CONTEXT_WINDOW": "131072",
	})

	result := NewPi(false).BuildExecute(value, "prompt", nil)
	argv := result.Argv
	// sh -lc script -- agentdir modelsjson <pi argv...>
	models := argv[5]
	var payload struct {
		Providers struct {
			Cairn struct {
				Models []struct {
					ContextWindow int `json:"contextWindow"`
				} `json:"models"`
			} `json:"cairn"`
		} `json:"providers"`
	}
	if err := json.Unmarshal([]byte(models), &payload); err != nil {
		t.Fatalf("models json: %v (%s)", err, models)
	}
	if payload.Providers.Cairn.Models[0].ContextWindow != 131072 {
		t.Fatalf("context window = %d", payload.Providers.Cairn.Models[0].ContextWindow)
	}
	if !contains(argv, "--tools") {
		t.Fatal("--tools missing")
	}
	tail := argv[len(argv)-2:]
	if tail[0] != "-p" || tail[1] != "prompt" {
		t.Fatalf("tail = %v", tail)
	}
}

func TestCodexDriverExecuteArgvPassesModelEndpointAndPrompt(t *testing.T) {
	value := worker(config.WorkerCodex, map[string]string{
		"CODEX_MODEL":    "gpt-test",
		"CODEX_BASE_URL": "http://api/v1",
		"OPENAI_API_KEY": "secret",
	})

	argv := NewCodex(false).BuildExecute(value, "prompt", nil).Argv

	if !contains(argv, "--model") || !contains(argv, "gpt-test") {
		t.Fatalf("argv = %v", argv)
	}
	if !contains(argv, `model_providers.cairn.base_url="http://api/v1"`) {
		t.Fatalf("base url config missing: %v", argv)
	}
	tail := argv[len(argv)-2:]
	if tail[0] != "--" || tail[1] != "prompt" {
		t.Fatalf("tail = %v", tail)
	}
}

func TestCodexConcludeKeepsResumeArgvShape(t *testing.T) {
	value := worker(config.WorkerCodex, map[string]string{"CODEX_MODEL": "m", "CODEX_BASE_URL": "http://api", "OPENAI_API_KEY": "k"})

	argv := NewCodex(false).BuildConclude(value, "prompt", "session-1")

	if argv[0] != "codex" || argv[1] != "exec" || argv[2] != "resume" || argv[3] != "session-1" {
		t.Fatalf("argv = %v", argv)
	}
	if argv[4] != "--dangerously-bypass-approvals-and-sandbox" {
		t.Fatalf("missing bypass flag: %v", argv)
	}
}

func TestPiDriverExtractsSessionAndLastAssistantText(t *testing.T) {
	driver := NewPi(false)
	stdout := strings.Join([]string{
		`{"type":"session","id":"session-123"}`,
		`{"type":"turn_end","message":{"role":"assistant","content":[{"type":"text","text":"{\"accepted\":true,\"data\":{}}"}]}}`,
	}, "\n")

	if session := driver.ExtractSession(nil, stdout, ""); session == nil || *session != "session-123" {
		t.Fatalf("session = %v", session)
	}
	if text := driver.ExtractResponseText(stdout, ""); text != `{"accepted":true,"data":{}}` {
		t.Fatalf("text = %q", text)
	}
}

func TestPiAgentEndPrefersLastAssistantMessage(t *testing.T) {
	driver := NewPi(false)
	stdout := strings.Join([]string{
		`{"type":"agent_end","messages":[{"role":"assistant","content":[{"type":"text","text":"first"}]},{"role":"assistant","content":[{"type":"text","text":"second"}]}]}`,
	}, "\n")

	if text := driver.ExtractResponseText(stdout, ""); text != "second" {
		t.Fatalf("text = %q", text)
	}
}

func TestMockDriverHealthReflectsConfiguredOutcome(t *testing.T) {
	ok := worker(config.WorkerMock, map[string]string{"MOCK_HEALTHCHECK": `{"delay":[0,0],"outcomes":{"ok":1.0,"fail":0.0}}`})
	fail := worker(config.WorkerMock, map[string]string{"MOCK_HEALTHCHECK": `{"delay":[0,0],"outcomes":{"ok":0.0,"fail":1.0}}`})

	if !(Mock{}).CheckHealth(ok, time.Second).OK {
		t.Fatal("expected healthy")
	}
	if (Mock{}).CheckHealth(fail, time.Second).OK {
		t.Fatal("expected unhealthy")
	}
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
