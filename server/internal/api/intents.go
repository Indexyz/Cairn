package api

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/Indexyz/Cairn/server/internal/domain"
)

func (app *App) createIntent(w http.ResponseWriter, r *http.Request) error {
	var body domain.CreateIntentRequest
	if err := decodeBody(r, &body); err != nil {
		return err
	}
	if validation := body.Validate(); validation != nil {
		return validation
	}
	return app.tx(w, r, func(ctx context.Context, conn *sql.Conn) (int, any, error) {
		projectID := r.PathValue("project")
		project, err := domain.CheckProjectRunnable(ctx, conn, projectID)
		if err != nil {
			return 0, nil, err
		}
		exhausted, err := domain.StepBudgetExhausted(ctx, conn, projectID)
		if err != nil {
			return 0, nil, err
		}
		if exhausted {
			blocked, err := domain.EnforceStepLimit(ctx, conn, projectID)
			if err != nil {
				return 0, nil, err
			}
			if blocked {
				app.pendingNotify(ctx, conn, "project_blocked", projectID, project.Title, "")
			}
			return http.StatusForbidden, map[string]any{"detail": "Project intent step limit reached"}, nil
		}
		if err := domain.ValidateFactsExist(ctx, conn, projectID, body.From); err != nil {
			return 0, nil, err
		}
		if err := domain.ValidateGoalNotInSources(body.From); err != nil {
			return 0, nil, err
		}
		if err := domain.ValidateIntentCreatorWorker(body.Creator, body.Worker); err != nil {
			return 0, nil, err
		}
		now := domain.UtcNow()
		intentID, err := domain.NextIntentID(ctx, conn, projectID)
		if err != nil {
			return 0, nil, err
		}
		var heartbeat *string
		if body.Worker != nil {
			heartbeat = &now
		}
		if _, err := conn.ExecContext(ctx,
			"INSERT INTO intents (id, project_id, to_fact_id, description, creator, worker, last_heartbeat_at, created_at, concluded_at) VALUES (?, ?, NULL, ?, ?, ?, ?, ?, NULL)",
			intentID, projectID, body.Description, body.Creator, body.Worker, heartbeat, now); err != nil {
			return 0, nil, err
		}
		for _, factID := range body.From {
			if _, err := conn.ExecContext(ctx, "INSERT INTO intent_sources (intent_id, project_id, fact_id) VALUES (?, ?, ?)",
				intentID, projectID, factID); err != nil {
				return 0, nil, err
			}
		}
		return http.StatusCreated, domain.Intent{
			ID: intentID, From: body.From, Description: body.Description, Creator: body.Creator,
			Worker: body.Worker, LastHeartbeatAt: heartbeat, CreatedAt: now,
		}, nil
	})
}

func (app *App) heartbeatIntent(w http.ResponseWriter, r *http.Request) error {
	var body domain.HeartbeatRequest
	if err := decodeBody(r, &body); err != nil {
		return err
	}
	if validation := body.Validate(); validation != nil {
		return validation
	}
	return app.tx(w, r, func(ctx context.Context, conn *sql.Conn) (int, any, error) {
		projectID := r.PathValue("project")
		intentID := r.PathValue("intent")
		if _, err := domain.CheckProjectRunnable(ctx, conn, projectID); err != nil {
			return 0, nil, err
		}
		if _, err := domain.GetClaimableOpenIntentOr404(ctx, conn, projectID, intentID, body.Worker); err != nil {
			return 0, nil, err
		}
		now := domain.UtcNow()
		if _, err := conn.ExecContext(ctx, "UPDATE intents SET worker = ?, last_heartbeat_at = ? WHERE id = ? AND project_id = ?",
			body.Worker, now, intentID, projectID); err != nil {
			return 0, nil, err
		}
		row, err := domain.GetIntentOr404(ctx, conn, projectID, intentID)
		if err != nil {
			return 0, nil, err
		}
		intent, err := domain.IntentToModel(ctx, conn, row, projectID)
		if err != nil {
			return 0, nil, err
		}
		return http.StatusOK, intent, nil
	})
}

func (app *App) releaseIntent(w http.ResponseWriter, r *http.Request) error {
	var body domain.HeartbeatRequest
	if err := decodeBody(r, &body); err != nil {
		return err
	}
	if validation := body.Validate(); validation != nil {
		return validation
	}
	return app.tx(w, r, func(ctx context.Context, conn *sql.Conn) (int, any, error) {
		projectID := r.PathValue("project")
		intentID := r.PathValue("intent")
		if _, err := domain.CheckProjectRunnable(ctx, conn, projectID); err != nil {
			return 0, nil, err
		}
		row, err := domain.GetReleasableOpenIntentOr404(ctx, conn, projectID, intentID, body.Worker)
		if err != nil {
			return 0, nil, err
		}
		if row.Worker != nil && *row.Worker == body.Worker {
			if _, err := conn.ExecContext(ctx, "UPDATE intents SET worker = NULL WHERE id = ? AND project_id = ?", intentID, projectID); err != nil {
				return 0, nil, err
			}
			row, err = domain.GetIntentOr404(ctx, conn, projectID, intentID)
			if err != nil {
				return 0, nil, err
			}
		}
		intent, err := domain.IntentToModel(ctx, conn, row, projectID)
		if err != nil {
			return 0, nil, err
		}
		return http.StatusOK, intent, nil
	})
}

func (app *App) concludeIntent(w http.ResponseWriter, r *http.Request) error {
	var body domain.ConcludeRequest
	if err := decodeBody(r, &body); err != nil {
		return err
	}
	if validation := body.Validate(); validation != nil {
		return validation
	}
	return app.tx(w, r, func(ctx context.Context, conn *sql.Conn) (int, any, error) {
		projectID := r.PathValue("project")
		intentID := r.PathValue("intent")
		project, err := domain.CheckProjectRunnable(ctx, conn, projectID)
		if err != nil {
			return 0, nil, err
		}
		if _, err := domain.GetClaimableOpenIntentOr404(ctx, conn, projectID, intentID, body.Worker); err != nil {
			return 0, nil, err
		}
		now := domain.UtcNow()
		factID, err := domain.NextFactID(ctx, conn, projectID)
		if err != nil {
			return 0, nil, err
		}
		if _, err := conn.ExecContext(ctx, "INSERT INTO facts (id, project_id, description) VALUES (?, ?, ?)",
			factID, projectID, body.Description); err != nil {
			return 0, nil, err
		}
		if _, err := conn.ExecContext(ctx,
			"UPDATE intents SET to_fact_id = ?, worker = ?, last_heartbeat_at = ?, concluded_at = ? WHERE id = ? AND project_id = ?",
			factID, body.Worker, now, now, intentID, projectID); err != nil {
			return 0, nil, err
		}
		row, err := domain.GetIntentOr404(ctx, conn, projectID, intentID)
		if err != nil {
			return 0, nil, err
		}
		intent, err := domain.IntentToModel(ctx, conn, row, projectID)
		if err != nil {
			return 0, nil, err
		}
		blocked, err := domain.EnforceStepLimit(ctx, conn, projectID)
		if err != nil {
			return 0, nil, err
		}
		if blocked {
			app.pendingNotify(ctx, conn, "project_blocked", projectID, project.Title, "")
		}
		return http.StatusOK, domain.ConcludeResponse{
			Fact: domain.Fact{ID: factID, Description: body.Description}, Intent: *intent,
		}, nil
	})
}
