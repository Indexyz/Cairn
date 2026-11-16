package registry

import (
	"strings"
	"testing"

	"github.com/Indexyz/Cairn/dispatcher/internal/config"
)

func TestGetSelectsLocalOrContainerVariant(t *testing.T) {
	worker := &config.WorkerConfig{Name: "w", Type: config.WorkerCodex, TaskTypes: []config.TaskType{config.TaskExplore}, MaxRunning: 1, Env: map[string]string{
		"CODEX_MODEL": "m", "CODEX_BASE_URL": "http://api", "OPENAI_API_KEY": "k",
	}}

	local, err := Get(config.WorkerCodex, config.ExecutionLocal)
	if err != nil {
		t.Fatalf("local: %v", err)
	}
	localArgv := local.BuildExecute(worker, "p", nil).Argv
	for _, part := range localArgv {
		if part == "--model" {
			t.Fatalf("local driver injected provider flags: %v", localArgv)
		}
	}

	container, err := Get(config.WorkerCodex, config.ExecutionContainer)
	if err != nil {
		t.Fatalf("container: %v", err)
	}
	containerArgv := container.BuildExecute(worker, "p", nil).Argv
	if !strings.Contains(strings.Join(containerArgv, "\x00"), "model_providers") {
		t.Fatalf("container driver missing provider flags: %v", containerArgv)
	}
}

func TestClaudeCodeAndMockDriversAreSharedAcrossModes(t *testing.T) {
	worker := &config.WorkerConfig{Name: "w", Type: config.WorkerClaudeCode, TaskTypes: []config.TaskType{config.TaskExplore}, MaxRunning: 1}

	for _, workerType := range []config.WorkerType{config.WorkerClaudeCode, config.WorkerMock} {
		local, _ := Get(workerType, config.ExecutionLocal)
		container, _ := Get(workerType, config.ExecutionContainer)
		localArgv := strings.Join(local.BuildExecute(worker, "p", nil).Argv, "\x00")
		containerArgv := strings.Join(container.BuildExecute(worker, "p", nil).Argv, "\x00")
		if localArgv != containerArgv {
			t.Fatalf("%s driver differs per mode", workerType)
		}
	}
}

func TestGetRejectsUnknownType(t *testing.T) {
	if _, err := Get("nope", config.ExecutionLocal); err == nil {
		t.Fatal("expected unknown worker type error")
	}
}
