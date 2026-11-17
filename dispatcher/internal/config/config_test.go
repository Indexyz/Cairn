package config

import (
	"encoding/json"
	"strings"
	"testing"
)

const baseYAML = `
server: http://127.0.0.1:8000
runtime:
  interval: 60
  max_workers: 2
  max_running_projects: 1
  max_project_workers: 2
  healthcheck_timeout: 5
  prompt_group: default
tasks:
  bootstrap: {timeout: 10, conclude_timeout: 5}
  reason: {timeout: 10, max_intents: 3}
  explore: {timeout: 10, conclude_timeout: 5}
container:
  image: test-image
  network_mode: host
  completed_action: stop
workers:
  - name: test-worker
    type: mock
    task_types: [bootstrap, reason, explore]
    max_running: 1
    priority: 0
`

func parseYAML(t *testing.T, yamlText string) (*DispatchConfig, error) {
	t.Helper()
	return Parse([]byte(yamlText))
}

func mustParse(t *testing.T, yamlText string) *DispatchConfig {
	t.Helper()
	config, err := parseYAML(t, yamlText)
	if err != nil {
		t.Fatalf("unexpected config error: %v", err)
	}
	return config
}

func TestCommonEnvMergesWithWorkerOverride(t *testing.T) {
	config := mustParse(t, baseYAML+`
common_env:
  SHARED: common
  OVERRIDE: common
`)
	config.Workers[0].Env = map[string]string{"OVERRIDE": "worker"}
	config.mergeCommonEnv()

	if got := config.Workers[0].Env["SHARED"]; got != "common" {
		t.Fatalf("shared env = %q, want common", got)
	}
	if got := config.Workers[0].Env["OVERRIDE"]; got != "worker" {
		t.Fatalf("override env = %q, want worker", got)
	}
}

func TestWorkerHealthcheckDefaultsToStartupOnlyAndRejectsUnknownMode(t *testing.T) {
	config := mustParse(t, baseYAML)
	if config.Runtime.WorkerHealthcheck != HealthcheckStartupOnly {
		t.Fatalf("default healthcheck = %q", config.Runtime.WorkerHealthcheck)
	}

	bad := strings.Replace(baseYAML, "  prompt_group: default", "  prompt_group: default\n  worker_healthcheck: sometimes", 1)
	if _, err := parseYAML(t, bad); err == nil || !strings.Contains(err.Error(), "worker_healthcheck") {
		t.Fatalf("unknown mode error = %v", err)
	}
}

func TestRejectsDuplicateWorkersAndExcessProjectParallelism(t *testing.T) {
	duplicate := baseYAML + `
  - name: test-worker
    type: mock
    task_types: [reason]
    max_running: 1
    priority: 0
`
	if _, err := parseYAML(t, duplicate); err == nil || !strings.Contains(err.Error(), "worker names must be unique") {
		t.Fatalf("duplicate error = %v", err)
	}

	excess := strings.Replace(baseYAML, "max_project_workers: 2", "max_project_workers: 3", 1)
	if _, err := parseYAML(t, excess); err == nil || !strings.Contains(err.Error(), "max_project_workers cannot exceed max_workers") {
		t.Fatalf("parallelism error = %v", err)
	}
}

func TestPiWorkerRejectsInvalidContextWindow(t *testing.T) {
	body := `
server: http://127.0.0.1:8000
runtime: {interval: 1, max_workers: 1, max_running_projects: 1, max_project_workers: 1, healthcheck_timeout: 1, prompt_group: default}
tasks:
  bootstrap: {timeout: 1, conclude_timeout: 1}
  reason: {timeout: 1}
  explore: {timeout: 1, conclude_timeout: 1}
container: {image: i, network_mode: host, completed_action: stop}
workers:
  - name: pi
    type: pi
    task_types: [explore]
    max_running: 1
    priority: 0
    env:
      PI_MODEL: model
      PI_BASE_URL: http://api
      PI_API_KEY: secret
      PI_PROVIDER_API: openai-completions
      PI_MODEL_CONTEXT_WINDOW: "0"
`
	_, err := parseYAML(t, body)
	if err == nil || !strings.Contains(err.Error(), "PI_MODEL_CONTEXT_WINDOW must be greater than 0") {
		t.Fatalf("context window error = %v", err)
	}
}

func TestMockWorkerRejectsUnknownPhaseConfiguration(t *testing.T) {
	body := strings.Replace(baseYAML, "    priority: 0", "    priority: 0\n    env: {MOCK_UNKNOWN: \"{}\"}", 1)
	_, err := parseYAML(t, body)
	if err == nil || !strings.Contains(err.Error(), "unsupported mock env keys") {
		t.Fatalf("mock env error = %v", err)
	}
}

func TestMockOutcomeProbabilitiesMustSumToOne(t *testing.T) {
	env := `env: {MOCK_REASON: '{"delay":[0,0],"outcomes":{"complete":0.5,"intent":0.2}}'}`
	body := strings.Replace(baseYAML, "    priority: 0", "    priority: 0\n    "+env, 1)
	_, err := parseYAML(t, body)
	if err == nil || !strings.Contains(err.Error(), "probabilities must sum to 1.0") {
		t.Fatalf("probability error = %v", err)
	}
}

func TestMockRulesAreNormalized(t *testing.T) {
	env := `env: {MOCK_REASON: '{"delay":[0,0],"outcomes":{"complete":1.0,"intent":0.0,"noop":0.0,"rejected":0.0,"invalid_json":0.0,"invalid_payload":0.0,"command_fail":0.0},"rules":[{"force":"intent","fact_ids_gte":3,"open_intents_empty":true}]}'}`
	body := strings.Replace(baseYAML, "    priority: 0", "    priority: 0\n    "+env, 1)
	config := mustParse(t, body)

	behavior, err := ResolveMockBehavior("test-worker", config.Workers[0].Env)
	if err != nil {
		t.Fatalf("resolve behavior: %v", err)
	}
	rule := behavior["reason"].Rules[0]
	if rule.Force != "intent" || rule.FactIDsGTE == nil || *rule.FactIDsGTE != 3 {
		t.Fatalf("unexpected rule: %+v", rule)
	}
	if rule.OpenIntentsEmpty == nil || !*rule.OpenIntentsEmpty {
		t.Fatalf("open_intents_empty not captured: %+v", rule)
	}
}

func TestContainerModeRequiresWorkerEnvKeysButLocalModeDoesNot(t *testing.T) {
	claude := `
server: http://127.0.0.1:8000
runtime: {interval: 1, max_workers: 1, max_running_projects: 1, max_project_workers: 1, healthcheck_timeout: 1, prompt_group: default}
tasks:
  bootstrap: {timeout: 1, conclude_timeout: 1}
  reason: {timeout: 1}
  explore: {timeout: 1, conclude_timeout: 1}
container: {image: i, network_mode: host, completed_action: stop}
workers:
  - name: claude
    type: claudecode
    task_types: [reason]
    max_running: 1
    priority: 0
`
	_, err := parseYAML(t, claude)
	if err == nil || !strings.Contains(err.Error(), "missing env keys") {
		t.Fatalf("container env error = %v", err)
	}

	local := `
server: http://127.0.0.1:8000
runtime: {interval: 1, max_workers: 1, max_running_projects: 1, max_project_workers: 1, healthcheck_timeout: 1, prompt_group: default, execution: local}
tasks:
  bootstrap: {timeout: 1, conclude_timeout: 1}
  reason: {timeout: 1}
  explore: {timeout: 1, conclude_timeout: 1}
workers:
  - name: claude
    type: claudecode
    task_types: [reason]
    max_running: 1
    priority: 0
`
	config := mustParse(t, local)
	if config.Local == nil || config.Local.CompletedAction != LocalKeep {
		t.Fatalf("local defaults = %+v", config.Local)
	}
}

func TestContainerPlatformIsOptional(t *testing.T) {
	config := mustParse(t, baseYAML)
	if config.Container.Platform != nil {
		t.Fatalf("platform default = %v", *config.Container.Platform)
	}

	withPlatform := strings.Replace(baseYAML, "  completed_action: stop", "  completed_action: stop\n  platform: linux/arm64", 1)
	parsed := mustParse(t, withPlatform)
	if parsed.Container.Platform == nil || *parsed.Container.Platform != "linux/arm64" {
		t.Fatalf("platform = %v", parsed.Container.Platform)
	}
}

func TestBundledPromptGroupsHaveRequiredPlaceholders(t *testing.T) {
	if err := validatePromptResources("default"); err != nil {
		t.Fatalf("default prompts: %v", err)
	}
	if err := validatePromptResources("mock"); err != nil {
		t.Fatalf("mock prompts: %v", err)
	}
	if err := validatePromptResources("missing"); err == nil {
		t.Fatal("expected missing prompt group error")
	}
}

func TestUnknownKeysAreRejected(t *testing.T) {
	body := baseYAML + "unknown_key: 1\n"
	if _, err := parseYAML(t, body); err == nil {
		t.Fatal("expected unknown key error")
	}
}

func TestLocalExecutionNeedsNoContainerOrWorkerEnv(t *testing.T) {
	body := `
server: http://127.0.0.1:8000
runtime: {interval: 1, max_workers: 1, max_running_projects: 1, max_project_workers: 1, healthcheck_timeout: 1, prompt_group: default, execution: local}
tasks:
  bootstrap: {timeout: 1, conclude_timeout: 1}
  reason: {timeout: 1}
  explore: {timeout: 1, conclude_timeout: 1}
workers:
  - {name: c, type: claudecode, task_types: [explore], max_running: 1, priority: 0}
`
	config := mustParse(t, body)

	if config.Container != nil {
		t.Fatal("local mode should not need a container block")
	}
	if config.Local == nil || config.Local.CompletedAction != LocalKeep {
		t.Fatalf("local defaults = %+v", config.Local)
	}
	if config.Local.WorkspaceRoot != nil {
		t.Fatalf("workspace root = %v", config.Local.WorkspaceRoot)
	}
	if len(config.Workers[0].Env) != 0 {
		t.Fatalf("worker env = %v", config.Workers[0].Env)
	}
}

func TestContainerExecutionRequiresContainerBlock(t *testing.T) {
	body := strings.Replace(baseYAML, "container:\n  image: test-image\n  network_mode: host\n  completed_action: stop\n", "", 1)
	if _, err := parseYAML(t, body); err == nil || !strings.Contains(err.Error(), "container config is required") {
		t.Fatalf("error = %v", err)
	}
}

func TestShippedLocalExampleConfigIsValid(t *testing.T) {
	config, err := Load("../../../dispatch.local.example.yaml")
	if err != nil {
		t.Fatalf("load example: %v", err)
	}
	if config.Runtime.Execution != ExecutionLocal || config.Container != nil || config.Local == nil {
		t.Fatalf("example config = %+v", config.Runtime)
	}
	if config.Local.CompletedAction != LocalKeep {
		t.Fatalf("completed action = %q", config.Local.CompletedAction)
	}
}

func TestMCPBlockReachesWorkerEnv(t *testing.T) {
	document := baseYAML + `
mcp:
  servers:
    fs:
      command: mcp-fs
      args: ["--root", "/work"]
      env:
        TOKEN: secret
    remote:
      url: https://mcp.test/sse
`
	config := mustParse(t, document)
	var payload struct {
		Servers map[string]map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(config.Workers[0].Env["MCP_CONFIG"]), &payload); err != nil {
		t.Fatalf("mcp env: %v", err)
	}
	if payload.Servers["fs"]["command"] != "mcp-fs" || payload.Servers["fs"]["type"] != "stdio" {
		t.Fatalf("stdio server = %v", payload.Servers["fs"])
	}
	if payload.Servers["remote"]["url"] != "https://mcp.test/sse" || payload.Servers["remote"]["type"] != "http" {
		t.Fatalf("http server = %v", payload.Servers["remote"])
	}
}

func TestMCPWorkerOverrideWins(t *testing.T) {
	override := `    priority: 0
    env:
      MCP_CONFIG: '{"mcpServers":{"own":{"command":"x"}}}'`
	document := strings.Replace(baseYAML, "    priority: 0", override, 1) + `
mcp:
  servers:
    fs:
      command: mcp-fs
`
	config := mustParse(t, document)
	if config.Workers[0].Env["MCP_CONFIG"] != `{"mcpServers":{"own":{"command":"x"}}}` {
		t.Fatalf("env = %v", config.Workers[0].Env["MCP_CONFIG"])
	}
}

func TestMCPValidationRejectsIncompleteServers(t *testing.T) {
	document := baseYAML + `
mcp:
  servers:
    broken: {}
`
	if _, err := parseYAML(t, document); err == nil || !strings.Contains(err.Error(), "mcp.servers.broken") {
		t.Fatalf("error = %v", err)
	}
}
