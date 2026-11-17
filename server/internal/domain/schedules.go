package domain

import (
	"encoding/json"
	"strings"
	"time"
)

// Schedule statuses recorded after the last run.
const (
	ScheduleStatusCreated = "created"
	ScheduleStatusSkipped = "skipped"
	ScheduleStatusError   = "error"
)

// SchedulePayload is a schedule write: the trigger plus its project template.
type SchedulePayload struct {
	Name             string   `json:"name"`
	Cron             string   `json:"cron"`
	Timezone         string   `json:"timezone"`
	Enabled          *bool    `json:"enabled"`
	Title            string   `json:"title"`
	Origin           string   `json:"origin"`
	Goal             string   `json:"goal"`
	BootstrapEnabled *bool    `json:"bootstrap_enabled"`
	MaxSteps         *int     `json:"max_steps"`
	Hints            []string `json:"hints"`
}

// EnabledValue defaults a missing flag to true.
func (payload *SchedulePayload) EnabledValue() bool {
	if payload.Enabled == nil {
		return true
	}
	return *payload.Enabled
}

// BootstrapValue defaults a missing bootstrap flag to true.
func (payload *SchedulePayload) BootstrapValue() bool {
	if payload.BootstrapEnabled == nil {
		return true
	}
	return *payload.BootstrapEnabled
}

// ParseSchedule decodes and validates a schedule write.
func ParseSchedule(raw []byte) (*SchedulePayload, error) {
	var payload SchedulePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fieldError("body", "Input should be a valid object", nil)
	}
	if validation := payload.Validate(); validation != nil {
		return nil, validation
	}
	return &payload, nil
}

// Validate trims every field and checks the trigger.
func (payload *SchedulePayload) Validate() *ValidationError {
	payload.Name = strings.TrimSpace(payload.Name)
	payload.Cron = strings.Join(strings.Fields(payload.Cron), " ")
	payload.Timezone = strings.TrimSpace(payload.Timezone)
	if payload.Timezone == "" {
		payload.Timezone = "UTC"
	}
	for field, value := range map[string]string{
		"name": payload.Name, "title": payload.Title, "origin": payload.Origin, "goal": payload.Goal,
	} {
		if value == "" {
			return fieldError(field, "must not be empty", value)
		}
	}
	payload.Title = strings.TrimSpace(payload.Title)
	payload.Origin = strings.TrimSpace(payload.Origin)
	payload.Goal = strings.TrimSpace(payload.Goal)
	if _, err := ParseCron(payload.Cron); err != nil {
		return err.(*ValidationError)
	}
	if _, err := ParseTimezone(payload.Timezone); err != nil {
		return err.(*ValidationError)
	}
	if payload.MaxSteps != nil && *payload.MaxSteps < 1 {
		return fieldError("max_steps", "Input should be greater than or equal to 1", *payload.MaxSteps)
	}
	trimmed := make([]string, 0, len(payload.Hints))
	for _, hint := range payload.Hints {
		if text := strings.TrimSpace(hint); text != "" {
			trimmed = append(trimmed, text)
		}
	}
	payload.Hints = trimmed
	return nil
}

// Schedule is a stored row plus its next firing.
type Schedule struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Cron             string   `json:"cron"`
	Timezone         string   `json:"timezone"`
	Enabled          bool     `json:"enabled"`
	Title            string   `json:"title"`
	Origin           string   `json:"origin"`
	Goal             string   `json:"goal"`
	BootstrapEnabled bool     `json:"bootstrap_enabled"`
	MaxSteps         *int     `json:"max_steps"`
	Hints            []string `json:"hints"`
	NextRunAt        *string  `json:"next_run_at"`
	LastRunAt        *string  `json:"last_run_at"`
	LastStatus       *string  `json:"last_status"`
	LastDetail       *string  `json:"last_detail"`
	LastProjectID    *string  `json:"last_project_id"`
	SkipCount        int      `json:"skip_count"`
	CreatedAt        string   `json:"created_at"`
	UpdatedAt        string   `json:"updated_at"`
}

// Template renders the project request a firing creates.
func (schedule *Schedule) Template() *CreateProjectRequest {
	bootstrap := schedule.BootstrapEnabled
	request := &CreateProjectRequest{
		Title:            schedule.Title,
		Origin:           schedule.Origin,
		Goal:             schedule.Goal,
		BootstrapEnabled: &bootstrap,
		MaxSteps:         schedule.MaxSteps,
	}
	if len(schedule.Hints) > 0 {
		hints := make([]CreateHintInline, 0, len(schedule.Hints))
		for _, hint := range schedule.Hints {
			hints = append(hints, CreateHintInline{Content: hint, Creator: "schedule:" + schedule.ID})
		}
		request.Hints = &hints
	}
	return request
}

// NextRun computes the firing after `after` in the schedule's own zone.
func (schedule *Schedule) NextRun(after time.Time) (string, error) {
	loc, err := ParseTimezone(schedule.Timezone)
	if err != nil {
		return "", err
	}
	cron, err := ParseCron(schedule.Cron)
	if err != nil {
		return "", err
	}
	next, err := cron.Next(after, loc)
	if err != nil {
		return "", err
	}
	return next.UTC().Format("2006-01-02T15:04:05Z"), nil
}
