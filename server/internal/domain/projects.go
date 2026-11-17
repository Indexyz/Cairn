package domain

import (
	"context"
	"database/sql"
)

// CreateProject writes a project graph head and returns its detail.
func CreateProject(ctx context.Context, conn *sql.Conn, body *CreateProjectRequest) (*ProjectDetail, error) {
	return CreateProjectFor(ctx, conn, body, nil)
}

// CreateProjectFor also tags the project with the schedule that fired it.
func CreateProjectFor(ctx context.Context, conn *sql.Conn, body *CreateProjectRequest, scheduleID *string) (*ProjectDetail, error) {
	projectID, err := NextProjectID(ctx, conn)
	if err != nil {
		return nil, err
	}
	now := UtcNow()
	bootstrap := body.BootstrapValue()
	if _, err := conn.ExecContext(ctx,
		"INSERT INTO projects (id, title, status, bootstrap_enabled, max_steps, schedule_id, created_at) VALUES (?, ?, 'active', ?, ?, ?, ?)",
		projectID, body.Title, boolToInt(bootstrap), body.MaxSteps, scheduleID, now); err != nil {
		return nil, err
	}
	facts := []Fact{{ID: "origin", Description: body.Origin}, {ID: "goal", Description: body.Goal}}
	for _, fact := range facts {
		if _, err := conn.ExecContext(ctx, "INSERT INTO facts (id, project_id, description) VALUES (?, ?, ?)",
			fact.ID, projectID, fact.Description); err != nil {
			return nil, err
		}
	}
	hints := []Hint{}
	if body.Hints != nil {
		for _, hint := range *body.Hints {
			hintID, err := NextHintID(ctx, conn, projectID)
			if err != nil {
				return nil, err
			}
			if _, err := conn.ExecContext(ctx, "INSERT INTO hints (id, project_id, content, creator, created_at) VALUES (?, ?, ?, ?, ?)",
				hintID, projectID, hint.Content, hint.Creator, now); err != nil {
				return nil, err
			}
			hints = append(hints, Hint{ID: hintID, Content: hint.Content, Creator: hint.Creator, CreatedAt: now})
		}
	}
	return &ProjectDetail{
		Project: ProjectMeta{
			ID: projectID, Title: body.Title, Status: "active", BootstrapEnabled: bootstrap,
			MaxSteps: body.MaxSteps, ScheduleID: scheduleID, CreatedAt: now,
		},
		Facts:   facts,
		Intents: []Intent{},
		Hints:   hints,
	}, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
