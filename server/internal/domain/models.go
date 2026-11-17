// Package domain holds the wire types, validation and graph rules.
package domain

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// FieldError mirrors FastAPI's validation error shape closely enough for the UI.
type FieldError struct {
	Type  string `json:"type"`
	Loc   []any  `json:"loc"`
	Msg   string `json:"msg"`
	Input any    `json:"input,omitempty"`
}

// ValidationError is a 422 payload.
type ValidationError struct {
	Errors []FieldError
}

func (err *ValidationError) Error() string {
	parts := make([]string, 0, len(err.Errors))
	for _, item := range err.Errors {
		parts = append(parts, item.Msg)
	}
	return strings.Join(parts, "; ")
}

func fieldError(field, msg string, input any) *ValidationError {
	return &ValidationError{Errors: []FieldError{{
		Type: "value_error", Loc: []any{"body", field}, Msg: msg, Input: input,
	}}}
}

// Settings are the shared lease timeouts.
type Settings struct {
	IntentTimeout int `json:"intent_timeout"`
	ReasonTimeout int `json:"reason_timeout"`
}

// Validate enforces the five-second floor.
func (settings *Settings) Validate() *ValidationError {
	if settings.IntentTimeout < 5 {
		return fieldError("intent_timeout", "Input should be greater than or equal to 5", settings.IntentTimeout)
	}
	if settings.ReasonTimeout < 5 {
		return fieldError("reason_timeout", "Input should be greater than or equal to 5", settings.ReasonTimeout)
	}
	return nil
}

// Fact is a graph node.
type Fact struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

// Intent is a graph edge.
type Intent struct {
	ID              string   `json:"id"`
	From            []string `json:"from"`
	To              *string  `json:"to"`
	Description     string   `json:"description"`
	Creator         string   `json:"creator"`
	Worker          *string  `json:"worker"`
	LastHeartbeatAt *string  `json:"last_heartbeat_at"`
	CreatedAt       string   `json:"created_at"`
	ConcludedAt     *string  `json:"concluded_at"`
}

// Hint is an out-of-band hint.
type Hint struct {
	ID        string `json:"id"`
	Content   string `json:"content"`
	Creator   string `json:"creator"`
	CreatedAt string `json:"created_at"`
}

// ProjectReason is the project-level reason lease.
type ProjectReason struct {
	Worker          string  `json:"worker"`
	Trigger         *string `json:"trigger"`
	StartedAt       *string `json:"started_at"`
	LastHeartbeatAt *string `json:"last_heartbeat_at"`
}

// ProjectMeta is the project header.
type ProjectMeta struct {
	ID               string         `json:"id"`
	Title            string         `json:"title"`
	Status           string         `json:"status"`
	BootstrapEnabled bool           `json:"bootstrap_enabled"`
	MaxSteps         *int           `json:"max_steps"`
	ScheduleID       *string        `json:"schedule_id"`
	CreatedAt        string         `json:"created_at"`
	Reason           *ProjectReason `json:"reason"`
}

// ProjectSummary adds graph counters.
type ProjectSummary struct {
	ProjectMeta
	FactCount            int `json:"fact_count"`
	IntentCount          int `json:"intent_count"`
	WorkingIntentCount   int `json:"working_intent_count"`
	UnclaimedIntentCount int `json:"unclaimed_intent_count"`
	HintCount            int `json:"hint_count"`
}

// ProjectDetail is a full graph snapshot.
type ProjectDetail struct {
	Project ProjectMeta `json:"project"`
	Facts   []Fact      `json:"facts"`
	Intents []Intent    `json:"intents"`
	Hints   []Hint      `json:"hints"`
}

// LoadSnapshot is the global lease load.
type LoadSnapshot struct {
	RunningTasks    int            `json:"running_tasks"`
	RunningProjects int            `json:"running_projects"`
	Projects        map[string]int `json:"projects"`
	Workers         map[string]int `json:"workers"`
}

// Worker types and task types.
const (
	TypeClaudeCode = "claudecode"
	TypeCodex      = "codex"
	TypePi         = "pi"
	TypeMock       = "mock"
)

var workerTypes = map[string]bool{TypeClaudeCode: true, TypeCodex: true, TypePi: true, TypeMock: true}

var taskTypes = map[string]bool{"reason": true, "explore": true, "bootstrap": true}

// WorkerModel is a model row.
type WorkerModel struct {
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	TaskTypes  []string          `json:"task_types"`
	MaxRunning int               `json:"max_running"`
	Priority   int               `json:"priority"`
	Env        map[string]string `json:"env"`
	Used       int               `json:"used"`
}

// ParseWorkerModel decodes a strict model payload (unknown keys rejected).
func ParseWorkerModel(raw []byte) (*WorkerModel, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, fieldError("body", "Input should be a valid object", nil)
	}
	allowed := map[string]bool{
		"name": true, "type": true, "task_types": true, "max_running": true,
		"priority": true, "env": true, "used": true,
	}
	for key := range fields {
		if !allowed[key] {
			return nil, fieldError(key, "Extra inputs are not permitted", key)
		}
	}
	for _, required := range []string{"name", "type", "task_types", "max_running", "priority"} {
		if _, present := fields[required]; !present {
			return nil, fieldError(required, "Field required", nil)
		}
	}
	model := &WorkerModel{Env: map[string]string{}}
	if err := decodeField(fields, "name", &model.Name); err != nil {
		return nil, err
	}
	if err := decodeField(fields, "type", &model.Type); err != nil {
		return nil, err
	}
	if err := decodeField(fields, "task_types", &model.TaskTypes); err != nil {
		return nil, err
	}
	if err := decodeField(fields, "max_running", &model.MaxRunning); err != nil {
		return nil, err
	}
	if err := decodeField(fields, "priority", &model.Priority); err != nil {
		return nil, err
	}
	if err := decodeField(fields, "env", &model.Env); err != nil {
		return nil, err
	}
	if model.Env == nil {
		model.Env = map[string]string{}
	}
	if validation := model.Validate(); validation != nil {
		return nil, validation
	}
	return model, nil
}

func decodeField(fields map[string]json.RawMessage, key string, target any) *ValidationError {
	raw, present := fields[key]
	if !present {
		return nil
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fieldError(key, "Input should be a valid value", string(raw))
	}
	return nil
}

// Validate checks the model payload.
func (model *WorkerModel) Validate() *ValidationError {
	if !workerTypes[model.Type] {
		return fieldError("type", "Input should be 'claudecode', 'codex', 'pi' or 'mock'", model.Type)
	}
	if len(model.TaskTypes) == 0 {
		return fieldError("task_types", "task_types must not be empty", model.TaskTypes)
	}
	seen := map[string]bool{}
	for _, taskType := range model.TaskTypes {
		if !taskTypes[taskType] {
			return fieldError("task_types", "Input should be 'reason', 'explore' or 'bootstrap'", taskType)
		}
		if seen[taskType] {
			return fieldError("task_types", "task_types must be unique", model.TaskTypes)
		}
		seen[taskType] = true
	}
	if model.MaxRunning <= 0 {
		return fieldError("max_running", "Input should be greater than 0", model.MaxRunning)
	}
	if model.Priority < 0 {
		return fieldError("priority", "Input should be greater than or equal to 0", model.Priority)
	}
	if model.Type == TypePi {
		if value := strings.TrimSpace(model.Env["PI_MODEL_CONTEXT_WINDOW"]); value != "" {
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return fieldError("env", fmt.Sprintf("worker %s env PI_MODEL_CONTEXT_WINDOW must be an integer", model.Name), value)
			}
			if parsed <= 0 {
				return fieldError("env", fmt.Sprintf("worker %s env PI_MODEL_CONTEXT_WINDOW must be greater than 0", model.Name), value)
			}
		}
	}
	return nil
}

// ReplaceModelsRequest is a snapshot write.
type ReplaceModelsRequest struct {
	Models []WorkerModel `json:"models"`
}

// Validate checks duplicate names.
func (request *ReplaceModelsRequest) Validate() *ValidationError {
	duplicates := map[string]bool{}
	seen := map[string]bool{}
	for _, model := range request.Models {
		if seen[model.Name] {
			duplicates[model.Name] = true
		}
		seen[model.Name] = true
	}
	if len(duplicates) == 0 {
		return nil
	}
	names := make([]string, 0, len(duplicates))
	for name := range duplicates {
		names = append(names, name)
	}
	sort.Strings(names)
	return fieldError("models", "duplicate model names: "+strings.Join(names, ", "), nil)
}

// ModelTestResult reports a probe.
type ModelTestResult struct {
	OK         bool   `json:"ok"`
	Status     *int   `json:"status"`
	Detail     string `json:"detail"`
	Endpoint   string `json:"endpoint"`
	DurationMs int    `json:"duration_ms"`
}

// Notification kinds and events.
const (
	KindDiscord  = "discord"
	KindTelegram = "telegram"
	KindWebhook  = "webhook"
)

var NotificationEvents = []string{
	"project_created", "project_stopped", "project_resumed",
	"project_blocked", "project_completed", "project_reopened",
}

// NotificationChannelPayload is a channel write.
type NotificationChannelPayload struct {
	Name    string            `json:"name"`
	Kind    string            `json:"kind"`
	Enabled *bool             `json:"enabled"`
	Events  []string          `json:"events"`
	Config  map[string]string `json:"config"`
}

// EnabledValue defaults a missing flag to true.
func (payload *NotificationChannelPayload) EnabledValue() bool {
	if payload.Enabled == nil {
		return true
	}
	return *payload.Enabled
}

// NotificationChannel adds identity.
type NotificationChannel struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Kind      string            `json:"kind"`
	Enabled   bool              `json:"enabled"`
	Events    []string          `json:"events"`
	Config    map[string]string `json:"config"`
	CreatedAt string            `json:"created_at"`
	UpdatedAt string            `json:"updated_at"`
}

// NotificationTestResult reports a test delivery.
type NotificationTestResult struct {
	OK         bool   `json:"ok"`
	Status     *int   `json:"status"`
	Detail     string `json:"detail"`
	DurationMs int    `json:"duration_ms"`
}

// Fact payloads.
type FactRequest struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

// CreateHintInline is a project-creation hint.
type CreateHintInline struct {
	Content string `json:"content"`
	Creator string `json:"creator"`
}

// CreateProjectRequest creates a project.
type CreateProjectRequest struct {
	Title            string              `json:"title"`
	Origin           string              `json:"origin"`
	Goal             string              `json:"goal"`
	BootstrapEnabled *bool               `json:"bootstrap_enabled"`
	MaxSteps         *int                `json:"max_steps"`
	Hints            *[]CreateHintInline `json:"hints"`
}

// Validate trims and checks the project payload.
func (request *CreateProjectRequest) Validate() *ValidationError {
	for field, value := range map[string]string{"title": request.Title, "origin": request.Origin, "goal": request.Goal} {
		if strings.TrimSpace(value) == "" {
			return fieldError(field, "must not be empty", value)
		}
	}
	request.Title = strings.TrimSpace(request.Title)
	request.Origin = strings.TrimSpace(request.Origin)
	request.Goal = strings.TrimSpace(request.Goal)
	if request.MaxSteps != nil && *request.MaxSteps < 1 {
		return fieldError("max_steps", "Input should be greater than or equal to 1", *request.MaxSteps)
	}
	if request.Hints != nil {
		for index := range *request.Hints {
			hint := &(*request.Hints)[index]
			if strings.TrimSpace(hint.Content) == "" {
				return fieldError("hints", "must not be empty", hint.Content)
			}
			if strings.TrimSpace(hint.Creator) == "" {
				return fieldError("hints", "must not be empty", hint.Creator)
			}
			hint.Content = strings.TrimSpace(hint.Content)
			hint.Creator = strings.TrimSpace(hint.Creator)
		}
	}
	return nil
}

// BootstrapValue defaults a missing bootstrap flag to true.
func (request *CreateProjectRequest) BootstrapValue() bool {
	return request.BootstrapEnabled == nil || *request.BootstrapEnabled
}

// ForkProjectRequest optionally renames the fork.
type ForkProjectRequest struct {
	Title *string `json:"title"`
}

// NormalizeTitle trims and drops blank titles.
func (request *ForkProjectRequest) NormalizeTitle() {
	if request.Title == nil {
		return
	}
	text := strings.TrimSpace(*request.Title)
	if text == "" {
		request.Title = nil
		return
	}
	request.Title = &text
}

// CreateHintRequest adds a hint.
type CreateHintRequest struct {
	Content string `json:"content"`
	Creator string `json:"creator"`
}

// Validate trims both fields.
func (request *CreateHintRequest) Validate() *ValidationError {
	return trimFields("content", &request.Content, "creator", &request.Creator)
}

// CreateIntentRequest declares an intent.
type CreateIntentRequest struct {
	From        []string `json:"from"`
	Description string   `json:"description"`
	Creator     string   `json:"creator"`
	Worker      *string  `json:"worker"`
}

// Validate trims and checks the intent payload.
func (request *CreateIntentRequest) Validate() *ValidationError {
	if len(request.From) == 0 {
		return fieldError("from", "List should have at least 1 item after validation, not 0", request.From)
	}
	for index, item := range request.From {
		text := strings.TrimSpace(item)
		if text == "" {
			return fieldError("from", "fact ids must not be empty", item)
		}
		request.From[index] = text
	}
	return trimFields("description", &request.Description, "creator", &request.Creator, "worker", request.Worker)
}

// HeartbeatRequest carries a worker.
type HeartbeatRequest struct {
	Worker string `json:"worker"`
}

// Validate trims the worker.
func (request *HeartbeatRequest) Validate() *ValidationError {
	if strings.TrimSpace(request.Worker) == "" {
		return fieldError("worker", "must not be empty", request.Worker)
	}
	request.Worker = strings.TrimSpace(request.Worker)
	return nil
}

// ReasonClaimRequest claims the reason lease.
type ReasonClaimRequest struct {
	Worker  string `json:"worker"`
	Trigger string `json:"trigger"`
}

// Validate trims both fields.
func (request *ReasonClaimRequest) Validate() *ValidationError {
	return trimFields("worker", &request.Worker, "trigger", &request.Trigger)
}

// ConcludeRequest concludes an intent.
type ConcludeRequest struct {
	Worker      string `json:"worker"`
	Description string `json:"description"`
}

// Validate trims both fields.
func (request *ConcludeRequest) Validate() *ValidationError {
	return trimFields("worker", &request.Worker, "description", &request.Description)
}

// ConcludeResponse is a concluded intent and its fact.
type ConcludeResponse struct {
	Fact   Fact   `json:"fact"`
	Intent Intent `json:"intent"`
}

// CompleteRequest completes a project.
type CompleteRequest struct {
	From        []string `json:"from"`
	Description string   `json:"description"`
	Worker      string   `json:"worker"`
}

// Validate trims and checks the completion payload.
func (request *CompleteRequest) Validate() *ValidationError {
	if len(request.From) == 0 {
		return fieldError("from", "List should have at least 1 item after validation, not 0", request.From)
	}
	for index, item := range request.From {
		text := strings.TrimSpace(item)
		if text == "" {
			return fieldError("from", "fact ids must not be empty", item)
		}
		request.From[index] = text
	}
	return trimFields("description", &request.Description, "worker", &request.Worker)
}

// UpdateProjectStatusRequest switches between active and stopped.
type UpdateProjectStatusRequest struct {
	Status string `json:"status"`
}

// Validate checks the status literal.
func (request *UpdateProjectStatusRequest) Validate() *ValidationError {
	if request.Status != "active" && request.Status != "stopped" {
		return fieldError("status", "Input should be 'active' or 'stopped'", request.Status)
	}
	return nil
}

// UpdateProjectTitleRequest renames a project.
type UpdateProjectTitleRequest struct {
	Title string `json:"title"`
}

// Validate trims the title.
func (request *UpdateProjectTitleRequest) Validate() *ValidationError {
	if strings.TrimSpace(request.Title) == "" {
		return fieldError("title", "must not be empty", request.Title)
	}
	request.Title = strings.TrimSpace(request.Title)
	return nil
}

// UpdateProjectMaxStepsRequest sets or clears the budget.
type UpdateProjectMaxStepsRequest struct {
	MaxSteps *int `json:"max_steps"`
}

// Validate checks the floor.
func (request *UpdateProjectMaxStepsRequest) Validate() *ValidationError {
	if request.MaxSteps != nil && *request.MaxSteps < 1 {
		return fieldError("max_steps", "Input should be greater than or equal to 1", *request.MaxSteps)
	}
	return nil
}

// ReopenRequest reopens a completed project.
type ReopenRequest struct {
	Description string `json:"description"`
	Creator     string `json:"creator"`
}

// Validate trims both fields.
func (request *ReopenRequest) Validate() *ValidationError {
	return trimFields("description", &request.Description, "creator", &request.Creator)
}

// ReopenResponse is the rewritten graph head.
type ReopenResponse struct {
	Project ProjectMeta `json:"project"`
	Fact    Fact        `json:"fact"`
	Intent  Intent      `json:"intent"`
}

func trimFields(field string, value *string, rest ...any) *ValidationError {
	if trimmed := strings.TrimSpace(*value); trimmed == "" {
		return fieldError(field, "must not be empty", *value)
	} else {
		*value = trimmed
	}
	for index := 0; index+1 < len(rest); index += 2 {
		name, ok := rest[index].(string)
		pointer, okPointer := rest[index+1].(*string)
		if !ok || !okPointer || pointer == nil {
			continue
		}
		trimmed := strings.TrimSpace(*pointer)
		if trimmed == "" {
			return fieldError(name, "must not be empty", *pointer)
		}
		*pointer = trimmed
	}
	return nil
}

// NotificationKinds and their required config fields.
var notificationKinds = map[string][]string{
	KindDiscord:  {"webhook_url"},
	KindTelegram: {"bot_token", "chat_id"},
	KindWebhook:  {"url"},
}

var notificationURLFields = map[string]bool{"webhook_url": true, "url": true}

// ParseNotificationChannel validates a channel write.
func ParseNotificationChannel(raw []byte) (*NotificationChannelPayload, error) {
	var payload NotificationChannelPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fieldError("body", "Input should be a valid object", nil)
	}
	if payload.Events == nil {
		payload.Events = append([]string(nil), NotificationEvents...)
	}
	if validation := payload.Validate(); validation != nil {
		return nil, validation
	}
	return &payload, nil
}

// Validate normalizes a channel payload.
func (payload *NotificationChannelPayload) Validate() *ValidationError {
	payload.Name = strings.TrimSpace(payload.Name)
	if payload.Name == "" {
		return fieldError("name", "must not be empty", payload.Name)
	}
	fields, known := notificationKinds[payload.Kind]
	if !known {
		return fieldError("kind", "Input should be 'discord', 'telegram' or 'webhook'", payload.Kind)
	}
	if len(payload.Events) == 0 {
		return fieldError("events", "at least one event is required", payload.Events)
	}
	selected := map[string]bool{}
	for _, event := range payload.Events {
		selected[event] = true
	}
	ordered := make([]string, 0, len(selected))
	for _, event := range NotificationEvents {
		if selected[event] {
			ordered = append(ordered, event)
		}
	}
	if len(ordered) == 0 {
		return fieldError("events", "at least one event is required", payload.Events)
	}
	payload.Events = ordered

	cleaned := map[string]string{}
	for _, key := range fields {
		value := strings.TrimSpace(payload.Config[key])
		if value == "" {
			return fieldError("config."+key, key+" must not be empty", value)
		}
		if notificationURLFields[key] && !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
			return fieldError("config."+key, key+" must be an http(s) URL", value)
		}
		cleaned[key] = value
	}
	payload.Config = cleaned
	return nil
}
