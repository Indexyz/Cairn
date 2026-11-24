// Package config loads and validates dispatch.yaml with the same rules as the
// Python implementation.
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Indexyz/Cairn/dispatcher/internal/prompting"
)

type TaskType string

const (
	TaskReason    TaskType = "reason"
	TaskExplore   TaskType = "explore"
	TaskBootstrap TaskType = "bootstrap"
)

type WorkerType string

const (
	WorkerClaudeCode WorkerType = "claudecode"
	WorkerCodex      WorkerType = "codex"
	WorkerPi         WorkerType = "pi"
	WorkerMock       WorkerType = "mock"
)

type CompletedAction string

const (
	CompletedRemove CompletedAction = "remove"
	CompletedStop   CompletedAction = "stop"
)

type HealthcheckMode string

const (
	HealthcheckStartupAndTask HealthcheckMode = "startup_and_task"
	HealthcheckStartupOnly    HealthcheckMode = "startup_only"
	HealthcheckDisabled       HealthcheckMode = "disabled"
)

type ExecutionMode string

const (
	ExecutionContainer ExecutionMode = "container"
	ExecutionLocal     ExecutionMode = "local"
)

type LocalCompletedAction string

const (
	LocalKeep   LocalCompletedAction = "keep"
	LocalRemove LocalCompletedAction = "remove"
)

// WorkerEnvKeys are the LLM settings container mode requires per worker type.
var WorkerEnvKeys = map[WorkerType][]string{
	WorkerClaudeCode: {"ANTHROPIC_MODEL", "ANTHROPIC_BASE_URL", "ANTHROPIC_AUTH_TOKEN"},
	WorkerCodex:      {"CODEX_MODEL", "CODEX_BASE_URL", "OPENAI_API_KEY"},
	WorkerPi:         {"PI_MODEL", "PI_BASE_URL", "PI_API_KEY", "PI_PROVIDER_API"},
	WorkerMock:       {},
}

var defaultPromptRequiredTokens = map[string][]string{
	"reason.md":             {"{graph_yaml}", "{fact_ids}", "{open_intents}", "{max_intents}"},
	"explore.md":            {"{graph_yaml}", "{intent_id}", "{intent_description}"},
	"explore_conclude.md":   {"{graph_yaml}", "{intent_id}", "{intent_description}"},
	"bootstrap.md":          {"{origin}", "{goal}", "{hints}"},
	"bootstrap_conclude.md": {"{origin}", "{goal}", "{hints}"},
}

var promptRequiredTokensByGroup = map[string]map[string][]string{
	"mock": {
		"reason.md":             {"{fact_ids}", "{open_intents}", "{max_intents}"},
		"explore.md":            {"{intent_id}"},
		"explore_conclude.md":   {"{intent_id}"},
		"bootstrap.md":          {"{origin}", "{goal}", "{hints}"},
		"bootstrap_conclude.md": {"{origin}", "{goal}", "{hints}"},
	},
}

// MockPhases are the mock adapter phases in fixed evaluation order.
var MockPhases = []string{
	"healthcheck",
	"reason",
	"explore_execute",
	"explore_conclude",
	"bootstrap",
	"bootstrap_conclude",
}

var mockAllowedOutcomes = map[string][]string{
	"healthcheck":        {"ok", "fail"},
	"reason":             {"complete", "intent", "noop", "rejected", "invalid_json", "invalid_payload", "command_fail"},
	"explore_execute":    {"fact", "rejected", "invalid_json", "invalid_payload", "command_fail"},
	"explore_conclude":   {"fact", "rejected", "invalid_json", "invalid_payload", "command_fail"},
	"bootstrap":          {"complete", "fact", "rejected", "invalid_json", "invalid_payload", "command_fail"},
	"bootstrap_conclude": {"fact", "rejected", "invalid_json", "invalid_payload", "command_fail"},
}

type mockDefaults struct {
	Delay    []float64
	Outcomes map[string]string
}

var mockDefaultBehavior = map[string]mockDefaults{
	"healthcheck": {
		Delay:    []float64{0.05, 0.15},
		Outcomes: map[string]string{"ok": "1.0", "fail": "0.0"},
	},
	"reason": {
		Delay: []float64{0.05, 0.3},
		Outcomes: map[string]string{
			"complete": "0.0", "intent": "1.0", "noop": "0.0", "rejected": "0.0",
			"invalid_json": "0.0", "invalid_payload": "0.0", "command_fail": "0.0",
		},
	},
	"explore_execute": {
		Delay: []float64{0.05, 0.3},
		Outcomes: map[string]string{
			"fact": "1.0", "rejected": "0.0", "invalid_json": "0.0",
			"invalid_payload": "0.0", "command_fail": "0.0",
		},
	},
	"explore_conclude": {
		Delay: []float64{0.05, 0.3},
		Outcomes: map[string]string{
			"fact": "1.0", "rejected": "0.0", "invalid_json": "0.0",
			"invalid_payload": "0.0", "command_fail": "0.0",
		},
	},
	"bootstrap": {
		Delay: []float64{0.05, 0.3},
		Outcomes: map[string]string{
			"complete": "1.0", "fact": "0.0", "rejected": "0.0", "invalid_json": "0.0",
			"invalid_payload": "0.0", "command_fail": "0.0",
		},
	},
	"bootstrap_conclude": {
		Delay: []float64{0.05, 0.3},
		Outcomes: map[string]string{
			"fact": "1.0", "rejected": "0.0", "invalid_json": "0.0",
			"invalid_payload": "0.0", "command_fail": "0.0",
		},
	},
}

// MockRule forces an outcome when all of its conditions match.
type MockRule struct {
	Force            string
	FactIDsGTE       *int
	FactIDsLTE       *int
	OpenIntentsEmpty *bool
}

// MockPhase is the resolved behavior of one mock phase.
type MockPhase struct {
	MinDelay float64
	MaxDelay float64
	Outcomes map[string]float64
	Rules    []MockRule
}

// MockBehavior maps a phase name to its resolved behavior.
type MockBehavior map[string]MockPhase

type mockPhasePayload struct {
	Delay    []float64              `json:"delay"`
	Outcomes map[string]json.Number `json:"outcomes"`
	Rules    []mockRulePayload      `json:"rules"`
}

type mockRulePayload struct {
	Force            string `json:"force"`
	FactIDsGTE       *int   `json:"fact_ids_gte"`
	FactIDsLTE       *int   `json:"fact_ids_lte"`
	OpenIntentsEmpty *bool  `json:"open_intents_empty"`
}

// ReasonTaskConfig holds reason task timings.
type ReasonTaskConfig struct {
	Timeout    int `yaml:"timeout"`
	MaxIntents int `yaml:"max_intents"`
}

// ExploreTaskConfig holds explore task timings.
type ExploreTaskConfig struct {
	Timeout         int `yaml:"timeout"`
	ConcludeTimeout int `yaml:"conclude_timeout"`
}

// BootstrapTaskConfig holds bootstrap task timings.
type BootstrapTaskConfig struct {
	Timeout         int `yaml:"timeout"`
	ConcludeTimeout int `yaml:"conclude_timeout"`
}

// TasksConfig groups the per-task timings.
type TasksConfig struct {
	Bootstrap BootstrapTaskConfig `yaml:"bootstrap"`
	Reason    ReasonTaskConfig    `yaml:"reason"`
	Explore   ExploreTaskConfig   `yaml:"explore"`
}

// ContainerConfig describes the worker container image.
type ContainerConfig struct {
	Image           string          `yaml:"image"`
	NetworkMode     string          `yaml:"network_mode"`
	CompletedAction CompletedAction `yaml:"completed_action"`
	Platform        *string         `yaml:"platform"`
	CapAdd          []string        `yaml:"cap_add"`
}

// LocalConfig describes local execution.
type LocalConfig struct {
	WorkspaceRoot   *string              `yaml:"workspace_root"`
	CompletedAction LocalCompletedAction `yaml:"completed_action"`
}

// RuntimeConfig holds the scheduling knobs.
type RuntimeConfig struct {
	MaxWorkers         int             `yaml:"max_workers"`
	MaxRunningProjects int             `yaml:"max_running_projects"`
	MaxProjectWorkers  int             `yaml:"max_project_workers"`
	Interval           int             `yaml:"interval"`
	HealthcheckTimeout int             `yaml:"healthcheck_timeout"`
	WorkerHealthcheck  HealthcheckMode `yaml:"worker_healthcheck"`
	Execution          ExecutionMode   `yaml:"execution"`
	PromptGroup        string          `yaml:"prompt_group"`
}

// WorkerConfig is a worker model entry.
type WorkerConfig struct {
	Name       string            `yaml:"name" json:"name"`
	Type       WorkerType        `yaml:"type" json:"type"`
	TaskTypes  []TaskType        `yaml:"task_types" json:"task_types"`
	MaxRunning int               `yaml:"max_running" json:"max_running"`
	Priority   int               `yaml:"priority" json:"priority"`
	Env        map[string]string `yaml:"env" json:"env"`
}

// MCPServer is one MCP server, in the shape the Claude CLI accepts.
type MCPServer struct {
	Type    string            `yaml:"type" json:"type,omitempty"`
	Command string            `yaml:"command" json:"command,omitempty"`
	Args    []string          `yaml:"args" json:"args,omitempty"`
	Env     map[string]string `yaml:"env" json:"env,omitempty"`
	URL     string            `yaml:"url" json:"url,omitempty"`
	Headers map[string]string `yaml:"headers" json:"headers,omitempty"`
}

// MCPConfig lists the MCP servers passed to every agent.
type MCPConfig struct {
	Servers map[string]MCPServer `yaml:"servers"`
}

// DispatchConfig is the whole dispatch.yaml document.
type DispatchConfig struct {
	Server    string            `yaml:"server"`
	Runtime   RuntimeConfig     `yaml:"runtime"`
	Tasks     TasksConfig       `yaml:"tasks"`
	Container *ContainerConfig  `yaml:"container"`
	Local     *LocalConfig      `yaml:"local"`
	CommonEnv map[string]string `yaml:"common_env"`
	MCP       *MCPConfig        `yaml:"mcp"`
	Workers   []WorkerConfig    `yaml:"workers"`
}

// Load reads, validates and returns dispatch.yaml.
func Load(path string) (*DispatchConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(raw)
}

// Parse validates a YAML document.
func Parse(raw []byte) (*DispatchConfig, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	decoder.KnownFields(true)
	var config DispatchConfig
	if err := decoder.Decode(&config); err != nil {
		return nil, err
	}
	if config.Runtime.WorkerHealthcheck == "" {
		config.Runtime.WorkerHealthcheck = HealthcheckStartupOnly
	}
	if config.Runtime.Execution == "" {
		config.Runtime.Execution = ExecutionContainer
	}
	if config.Local != nil && config.Local.CompletedAction == "" {
		config.Local.CompletedAction = LocalKeep
	}
	config.mergeCommonEnv()
	if err := config.mergeMCP(); err != nil {
		return nil, err
	}
	if err := config.validate(); err != nil {
		return nil, err
	}
	return &config, nil
}

func (config *DispatchConfig) mergeCommonEnv() {
	for index := range config.Workers {
		merged := map[string]string{}
		for key, value := range config.CommonEnv {
			merged[key] = value
		}
		for key, value := range config.Workers[index].Env {
			merged[key] = value
		}
		config.Workers[index].Env = merged
	}
}

// mergeMCP normalizes and validates the MCP block. The dispatcher pushes it to the
// server and only falls back to it as a seed, so workers get MCP_CONFIG at sync time.
func (config *DispatchConfig) mergeMCP() error {
	if config.MCP == nil || len(config.MCP.Servers) == 0 {
		return nil
	}
	for name, server := range config.MCP.Servers {
		if server.Command == "" && server.URL == "" {
			return fmt.Errorf("mcp.servers.%s needs command or url", name)
		}
		if server.Command == "" {
			if server.Type == "" {
				server.Type = "http"
			}
		} else if server.Type == "" {
			server.Type = "stdio"
		}
		if server.Type != "stdio" && server.Type != "http" && server.Type != "sse" {
			return fmt.Errorf("mcp.servers.%s type must be stdio, http or sse", name)
		}
		config.MCP.Servers[name] = server
	}
	return nil
}

func (config *DispatchConfig) validate() error {
	if config.Server == "" {
		return fmt.Errorf("server is required")
	}
	if config.Runtime.MaxWorkers <= 0 {
		return fmt.Errorf("runtime.max_workers must be greater than 0")
	}
	if config.Runtime.MaxRunningProjects <= 0 {
		return fmt.Errorf("runtime.max_running_projects must be greater than 0")
	}
	if config.Runtime.MaxProjectWorkers <= 0 {
		return fmt.Errorf("runtime.max_project_workers must be greater than 0")
	}
	if config.Runtime.MaxProjectWorkers > config.Runtime.MaxWorkers {
		return fmt.Errorf("max_project_workers cannot exceed max_workers")
	}
	if config.Runtime.Interval <= 0 {
		return fmt.Errorf("runtime.interval must be greater than 0")
	}
	if config.Runtime.HealthcheckTimeout <= 0 {
		return fmt.Errorf("runtime.healthcheck_timeout must be greater than 0")
	}
	if config.Runtime.PromptGroup == "" {
		return fmt.Errorf("runtime.prompt_group is required")
	}
	switch config.Runtime.WorkerHealthcheck {
	case HealthcheckStartupAndTask, HealthcheckStartupOnly, HealthcheckDisabled:
	default:
		return fmt.Errorf("runtime.worker_healthcheck must be one of startup_and_task, startup_only, disabled")
	}
	switch config.Runtime.Execution {
	case ExecutionContainer, ExecutionLocal:
	default:
		return fmt.Errorf("runtime.execution must be one of container, local")
	}
	if config.Tasks.Reason.Timeout <= 0 {
		return fmt.Errorf("tasks.reason.timeout must be greater than 0")
	}
	if config.Tasks.Reason.MaxIntents == 0 {
		config.Tasks.Reason.MaxIntents = 3
	}
	if config.Tasks.Reason.MaxIntents <= 0 {
		return fmt.Errorf("tasks.reason.max_intents must be greater than 0")
	}
	if config.Tasks.Explore.Timeout <= 0 || config.Tasks.Explore.ConcludeTimeout <= 0 {
		return fmt.Errorf("explore task timeouts must be greater than 0")
	}
	if config.Tasks.Bootstrap.Timeout <= 0 || config.Tasks.Bootstrap.ConcludeTimeout <= 0 {
		return fmt.Errorf("bootstrap task timeouts must be greater than 0")
	}

	names := map[string]bool{}
	for index := range config.Workers {
		worker := &config.Workers[index]
		if names[worker.Name] {
			return fmt.Errorf("worker names must be unique")
		}
		names[worker.Name] = true
		if err := validateWorker(worker); err != nil {
			return err
		}
	}

	if config.Runtime.Execution == ExecutionContainer {
		if config.Container == nil {
			return fmt.Errorf("container config is required when runtime.execution is container")
		}
		if config.Container.Image == "" {
			return fmt.Errorf("container.image is required")
		}
		if config.Container.NetworkMode == "" {
			return fmt.Errorf("container.network_mode is required")
		}
		switch config.Container.CompletedAction {
		case CompletedRemove, CompletedStop:
		default:
			return fmt.Errorf("container.completed_action must be one of remove, stop")
		}
		for _, worker := range config.Workers {
			var missing []string
			for _, key := range WorkerEnvKeys[worker.Type] {
				if worker.Env[key] == "" {
					missing = append(missing, key)
				}
			}
			if len(missing) > 0 {
				return fmt.Errorf("worker %s missing env keys: %s", worker.Name, strings.Join(missing, ", "))
			}
		}
	} else if config.Local == nil {
		config.Local = &LocalConfig{CompletedAction: LocalKeep}
	}

	return validatePromptResources(config.Runtime.PromptGroup)
}

func validateWorker(worker *WorkerConfig) error {
	if worker.Name == "" {
		return fmt.Errorf("worker name is required")
	}
	switch worker.Type {
	case WorkerClaudeCode, WorkerCodex, WorkerPi, WorkerMock:
	default:
		return fmt.Errorf("worker %s has unsupported type: %s", worker.Name, worker.Type)
	}
	if len(worker.TaskTypes) == 0 {
		return fmt.Errorf("task_types must not be empty")
	}
	seen := map[TaskType]bool{}
	for _, taskType := range worker.TaskTypes {
		switch taskType {
		case TaskReason, TaskExplore, TaskBootstrap:
		default:
			return fmt.Errorf("worker %s has unsupported task type: %s", worker.Name, taskType)
		}
		if seen[taskType] {
			return fmt.Errorf("task_types must be unique")
		}
		seen[taskType] = true
	}
	if worker.MaxRunning <= 0 {
		return fmt.Errorf("worker %s max_running must be greater than 0", worker.Name)
	}
	if worker.Priority < 0 {
		return fmt.Errorf("worker %s priority must be greater than or equal to 0", worker.Name)
	}
	if worker.Type == WorkerPi {
		if err := validateOptionalPositiveIntEnv(worker.Name, worker.Env, "PI_MODEL_CONTEXT_WINDOW"); err != nil {
			return err
		}
	}
	if worker.Type == WorkerMock {
		if _, err := ResolveMockBehavior(worker.Name, worker.Env); err != nil {
			return err
		}
	}
	return nil
}

func validateOptionalPositiveIntEnv(workerName string, env map[string]string, key string) error {
	value, present := env[key]
	if !present || strings.TrimSpace(value) == "" {
		return nil
	}
	var parsed int
	value = strings.TrimSpace(value)
	if converted, err := strconv.Atoi(value); err == nil {
		parsed = converted
	} else {
		return fmt.Errorf("worker %s env %s must be an integer", workerName, key)
	}
	if parsed <= 0 {
		return fmt.Errorf("worker %s env %s must be greater than 0", workerName, key)
	}
	return nil
}

func validatePromptResources(group string) error {
	if !prompting.GroupExists(group) {
		return fmt.Errorf("missing prompt group: %s", group)
	}
	required, ok := promptRequiredTokensByGroup[group]
	if !ok {
		required = defaultPromptRequiredTokens
	}
	names := make([]string, 0, len(required))
	for name := range required {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		content, err := prompting.Load(group, name)
		if err != nil {
			return fmt.Errorf("prompt group %s missing resource: %s", group, name)
		}
		var missing []string
		for _, token := range required[name] {
			if !strings.Contains(content, token) {
				missing = append(missing, token)
			}
		}
		if len(missing) > 0 {
			return fmt.Errorf("prompt group %s resource %s missing placeholders: %s", group, name, strings.Join(missing, ", "))
		}
	}
	return nil
}

// ResolveMockBehavior parses and validates MOCK_* env configuration.
func ResolveMockBehavior(workerName string, env map[string]string) (MockBehavior, error) {
	allowedEnvKeys := map[string]bool{}
	for _, phase := range MockPhases {
		allowedEnvKeys["MOCK_"+strings.ToUpper(phase)] = true
	}
	var unknown []string
	for key := range env {
		if strings.HasPrefix(key, "MOCK_") && !allowedEnvKeys[key] {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, fmt.Errorf("worker %s has unsupported mock env keys: %s", workerName, strings.Join(unknown, ", "))
	}

	behavior := MockBehavior{}
	for _, phase := range MockPhases {
		prefix := "MOCK_" + strings.ToUpper(phase)
		payload, err := parseMockPhasePayload(workerName, env, prefix, mockDefaultBehavior[phase])
		if err != nil {
			return nil, err
		}
		if len(payload.Delay) != 2 {
			return nil, fmt.Errorf("worker %s %s.delay must be a two-element number array", workerName, prefix)
		}
		minDelay, maxDelay := payload.Delay[0], payload.Delay[1]
		if minDelay < 0 || maxDelay < 0 {
			return nil, fmt.Errorf("worker %s %s.delay must be non-negative", workerName, prefix)
		}
		if maxDelay < minDelay {
			return nil, fmt.Errorf("worker %s %s.delay[1] must be greater than or equal to delay[0]", workerName, prefix)
		}
		if payload.Outcomes == nil {
			return nil, fmt.Errorf("worker %s %s.outcomes must be an object", workerName, prefix)
		}
		allowed := mockAllowedOutcomes[phase]
		allowedSet := map[string]bool{}
		for _, outcome := range allowed {
			allowedSet[outcome] = true
		}
		var unknownOutcomes []string
		for outcome := range payload.Outcomes {
			if !allowedSet[outcome] {
				unknownOutcomes = append(unknownOutcomes, outcome)
			}
		}
		if len(unknownOutcomes) > 0 {
			sort.Strings(unknownOutcomes)
			return nil, fmt.Errorf("worker %s %s.outcomes has unsupported keys: %s", workerName, prefix, strings.Join(unknownOutcomes, ", "))
		}

		outcomes := map[string]float64{}
		total := new(big.Rat)
		sortedAllowed := append([]string(nil), allowed...)
		sort.Strings(sortedAllowed)
		for _, outcome := range sortedAllowed {
			weight, err := parseMockProbability(workerName, prefix, payload.Outcomes, phase, outcome)
			if err != nil {
				return nil, err
			}
			asFloat, _ := weight.Float64()
			outcomes[outcome] = asFloat
			total.Add(total, weight)
		}
		if total.Cmp(big.NewRat(1, 1)) != 0 {
			return nil, fmt.Errorf("worker %s %s.outcomes probabilities must sum to 1.0, got %s", workerName, prefix, total.FloatString(4))
		}

		rules := make([]MockRule, 0, len(payload.Rules))
		for index, rule := range payload.Rules {
			if !allowedSet[rule.Force] {
				return nil, fmt.Errorf(
					"worker %s %s.rules[%d].force must be one of: %s",
					workerName, prefix, index, strings.Join(sortedAllowed, ", "),
				)
			}
			if rule.FactIDsGTE != nil && *rule.FactIDsGTE < 0 {
				return nil, fmt.Errorf("worker %s %s.rules[%d].fact_ids_gte must be a non-negative integer", workerName, prefix, index)
			}
			if rule.FactIDsLTE != nil && *rule.FactIDsLTE < 0 {
				return nil, fmt.Errorf("worker %s %s.rules[%d].fact_ids_lte must be a non-negative integer", workerName, prefix, index)
			}
			rules = append(rules, MockRule{
				Force:            rule.Force,
				FactIDsGTE:       rule.FactIDsGTE,
				FactIDsLTE:       rule.FactIDsLTE,
				OpenIntentsEmpty: rule.OpenIntentsEmpty,
			})
		}

		behavior[phase] = MockPhase{MinDelay: minDelay, MaxDelay: maxDelay, Outcomes: outcomes, Rules: rules}
	}
	return behavior, nil
}

func parseMockPhasePayload(workerName string, env map[string]string, key string, defaults mockDefaults) (mockPhasePayload, error) {
	raw, present := env[key]
	if !present {
		outcomes := make(map[string]json.Number, len(defaults.Outcomes))
		for outcome, weight := range defaults.Outcomes {
			outcomes[outcome] = json.Number(weight)
		}
		return mockPhasePayload{Delay: defaults.Delay, Outcomes: outcomes}, nil
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	var payload mockPhasePayload
	if err := decoder.Decode(&payload); err != nil {
		return mockPhasePayload{}, fmt.Errorf("worker %s %s must be a JSON object", workerName, key)
	}
	return payload, nil
}

func parseMockProbability(workerName, phaseKey string, outcomes map[string]json.Number, phase, outcome string) (*big.Rat, error) {
	literal, present := "", false
	if number, ok := outcomes[outcome]; ok {
		literal, present = number.String(), true
	}
	if !present {
		literal = mockDefaultBehavior[phase].Outcomes[outcome]
	}
	weight, ok := new(big.Rat).SetString(literal)
	if !ok {
		return nil, fmt.Errorf("worker %s %s.outcomes.%s must be a decimal probability", workerName, phaseKey, outcome)
	}
	if weight.Sign() < 0 || weight.Cmp(big.NewRat(1, 1)) > 0 {
		return nil, fmt.Errorf("worker %s %s.outcomes.%s must be between 0 and 1", workerName, phaseKey, outcome)
	}
	return weight, nil
}
