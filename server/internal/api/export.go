package api

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/Indexyz/Cairn/server/internal/domain"
)

type exportProject struct {
	Title            string `yaml:"title"`
	Origin           string `yaml:"origin"`
	Goal             string `yaml:"goal"`
	BootstrapEnabled bool   `yaml:"bootstrap_enabled"`
	MaxSteps         *int   `yaml:"max_steps"`
}

type exportHint struct {
	Content   string  `yaml:"content"`
	Creator   string  `yaml:"creator"`
	CreatedAt *string `yaml:"created_at"`
}

type exportFact struct {
	ID          string `yaml:"id"`
	Description string `yaml:"description"`
}

type exportIntent struct {
	From        []string `yaml:"from"`
	To          *string  `yaml:"to"`
	Description string   `yaml:"description"`
	Creator     string   `yaml:"creator"`
	Worker      *string  `yaml:"worker"`
	CreatedAt   *string  `yaml:"created_at"`
	ConcludedAt *string  `yaml:"concluded_at"`
}

type exportDocument struct {
	Project exportProject  `yaml:"project"`
	Hints   []exportHint   `yaml:"hints,omitempty"`
	Facts   []exportFact   `yaml:"facts"`
	Intents []exportIntent `yaml:"intents,omitempty"`
}

func formatExportTimestamp(value *string) *string {
	if value == nil || *value == "" {
		return value
	}
	parsed, err := time.Parse(time.RFC3339, strings.Replace(*value, "Z", "+00:00", 1))
	if err != nil {
		return value
	}
	formatted := parsed.Local().Format("2006-01-02 15:04:05")
	return &formatted
}

type exportData struct {
	project *domain.ProjectRow
	facts   []domain.Fact
	hints   []domain.Hint
	intents []domain.Intent
}

func loadExport(ctx context.Context, conn *sql.Conn, projectID string) (*exportData, error) {
	if err := domain.ExpireWorkers(ctx, conn, &projectID); err != nil {
		return nil, err
	}
	if err := domain.ExpireReasonLeases(ctx, conn, &projectID); err != nil {
		return nil, err
	}
	project, err := domain.GetProjectOr404(ctx, conn, projectID)
	if err != nil {
		return nil, err
	}
	data := &exportData{project: project}

	rows, err := conn.QueryContext(ctx, "SELECT id, description FROM facts WHERE project_id = ?", projectID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var fact domain.Fact
		if err := rows.Scan(&fact.ID, &fact.Description); err != nil {
			rows.Close()
			return nil, err
		}
		data.facts = append(data.facts, fact)
	}
	rows.Close()

	hintRows, err := conn.QueryContext(ctx, "SELECT id, content, creator, created_at FROM hints WHERE project_id = ? ORDER BY created_at", projectID)
	if err != nil {
		return nil, err
	}
	for hintRows.Next() {
		var hint domain.Hint
		if err := hintRows.Scan(&hint.ID, &hint.Content, &hint.Creator, &hint.CreatedAt); err != nil {
			hintRows.Close()
			return nil, err
		}
		data.hints = append(data.hints, hint)
	}
	hintRows.Close()

	intents, err := domain.BuildIntents(ctx, conn, projectID)
	if err != nil {
		return nil, err
	}
	data.intents = intents
	return data, nil
}

func (app *App) exportProject(w http.ResponseWriter, r *http.Request) error {
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "yaml"
	}
	if format != "yaml" && format != "timeline" {
		return domain.BadRequest("Supported formats: yaml, timeline")
	}
	var text string
	if err := app.DB.Read(r.Context(), func(conn *sql.Conn) error {
		data, err := loadExport(r.Context(), conn, r.PathValue("project"))
		if err != nil {
			return err
		}
		if format == "timeline" {
			text = renderTimeline(data)
			return nil
		}
		encoded, err := renderYAML(data)
		if err != nil {
			return err
		}
		text = encoded
		return nil
	}); err != nil {
		return err
	}
	w.Header().Set("content-type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(text))
	return nil
}

func renderYAML(data *exportData) (string, error) {
	origin, goal := "", ""
	for _, fact := range data.facts {
		switch fact.ID {
		case "origin":
			origin = fact.Description
		case "goal":
			goal = fact.Description
		}
	}
	document := exportDocument{
		Project: exportProject{
			Title: data.project.Title, Origin: origin, Goal: goal,
			BootstrapEnabled: data.project.BootstrapEnabled, MaxSteps: data.project.MaxSteps,
		},
	}
	for _, hint := range data.hints {
		created := hint.CreatedAt
		document.Hints = append(document.Hints, exportHint{Content: hint.Content, Creator: hint.Creator, CreatedAt: formatExportTimestamp(&created)})
	}
	for _, fact := range data.facts {
		document.Facts = append(document.Facts, exportFact{ID: fact.ID, Description: fact.Description})
	}
	for _, intent := range data.intents {
		created := intent.CreatedAt
		concluded := intent.ConcludedAt
		document.Intents = append(document.Intents, exportIntent{
			From: intent.From, To: intent.To, Description: intent.Description, Creator: intent.Creator,
			Worker: intent.Worker, CreatedAt: formatExportTimestamp(&created), ConcludedAt: formatExportTimestamp(concluded),
		})
	}
	encoded, err := yaml.Marshal(document)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

type timelineEvent struct {
	timestamp string
	order     int
	text      string
}

func renderTimeline(data *exportData) string {
	facts := map[string]string{}
	for _, fact := range data.facts {
		facts[fact.ID] = fact.Description
	}
	events := []timelineEvent{}
	order := 0
	created := data.project.CreatedAt
	events = append(events, timelineEvent{created, order, fmt.Sprintf("[%s] PROJECT CREATED\n  origin: %s\n  goal: %s",
		derefString(formatExportTimestamp(&created)), facts["origin"], facts["goal"])})
	order++

	for _, hint := range data.hints {
		stamp := hint.CreatedAt
		events = append(events, timelineEvent{hint.CreatedAt, order, fmt.Sprintf("[%s] HINT by %s\n  %s",
			derefString(formatExportTimestamp(&stamp)), hint.Creator, hint.Content)})
		order++
	}

	for _, intent := range data.intents {
		from := strings.Join(intent.From, ", ")
		stamp := intent.CreatedAt
		meta := "  from: " + from
		if intent.Worker != nil && intent.ConcludedAt == nil {
			meta += "\n  worker: " + *intent.Worker + " (in progress)"
		}
		events = append(events, timelineEvent{intent.CreatedAt, order, fmt.Sprintf("[%s] INTENT DECLARED %s by %s\n%s\n  %s",
			derefString(formatExportTimestamp(&stamp)), intent.ID, intent.Creator, meta, intent.Description)})
		order++

		if intent.ConcludedAt == nil || intent.To == nil {
			continue
		}
		concluded := intent.ConcludedAt
		actor := intent.Creator
		if intent.Worker != nil {
			actor = *intent.Worker
		}
		var block string
		if *intent.To == "goal" {
			block = fmt.Sprintf("[%s] PROJECT COMPLETED by %s\n  via: %s from %s",
				derefString(formatExportTimestamp(concluded)), actor, intent.ID, from)
		} else {
			block = fmt.Sprintf("[%s] INTENT CONCLUDED %s by %s\n  from: %s\n  produced: %s\n  %s",
				derefString(formatExportTimestamp(concluded)), intent.ID, actor, from, *intent.To, facts[*intent.To])
		}
		events = append(events, timelineEvent{*intent.ConcludedAt, order, block})
		order++
	}

	sort.SliceStable(events, func(left, right int) bool {
		if events[left].timestamp != events[right].timestamp {
			return events[left].timestamp < events[right].timestamp
		}
		return events[left].order < events[right].order
	})
	blocks := make([]string, 0, len(events))
	for _, event := range events {
		blocks = append(blocks, event.text)
	}
	return strings.Join(blocks, "\n\n") + "\n"
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
