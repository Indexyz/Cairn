package api

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/Indexyz/Cairn/server/internal/domain"
)

func (app *App) getSettings(w http.ResponseWriter, r *http.Request) error {
	return app.tx(w, r, func(ctx context.Context, conn *sql.Conn) (int, any, error) {
		settings := domain.Settings{}
		if err := conn.QueryRowContext(ctx, "SELECT intent_timeout, reason_timeout, max_workers, max_running_projects, max_project_workers FROM settings WHERE rowid = 1").
			Scan(&settings.IntentTimeout, &settings.ReasonTimeout, &settings.MaxWorkers, &settings.MaxRunningProjects, &settings.MaxProjectWorkers); err != nil {
			return 0, nil, err
		}
		return http.StatusOK, settings, nil
	})
}

func (app *App) updateSettings(w http.ResponseWriter, r *http.Request) error {
	var body domain.Settings
	if err := decodeBody(r, &body); err != nil {
		return err
	}
	if validation := body.Validate(); validation != nil {
		return validation
	}
	return app.tx(w, r, func(ctx context.Context, conn *sql.Conn) (int, any, error) {
		if _, err := conn.ExecContext(ctx, "UPDATE settings SET intent_timeout = ?, reason_timeout = ?, max_workers = ?, max_running_projects = ?, max_project_workers = ? WHERE rowid = 1",
			body.IntentTimeout, body.ReasonTimeout, body.MaxWorkers, body.MaxRunningProjects, body.MaxProjectWorkers); err != nil {
			return 0, nil, err
		}
		return http.StatusOK, body, nil
	})
}

func (app *App) createHint(w http.ResponseWriter, r *http.Request) error {
	var body domain.CreateHintRequest
	if err := decodeBody(r, &body); err != nil {
		return err
	}
	if validation := body.Validate(); validation != nil {
		return validation
	}
	return app.tx(w, r, func(ctx context.Context, conn *sql.Conn) (int, any, error) {
		projectID := r.PathValue("project")
		if _, err := domain.CheckProjectHintWritable(ctx, conn, projectID); err != nil {
			return 0, nil, err
		}
		now := domain.UtcNow()
		hintID, err := domain.NextHintID(ctx, conn, projectID)
		if err != nil {
			return 0, nil, err
		}
		if _, err := conn.ExecContext(ctx,
			"INSERT INTO hints (id, project_id, content, creator, created_at) VALUES (?, ?, ?, ?, ?)",
			hintID, projectID, body.Content, body.Creator, now); err != nil {
			return 0, nil, err
		}
		return http.StatusCreated, domain.Hint{ID: hintID, Content: body.Content, Creator: body.Creator, CreatedAt: now}, nil
	})
}
