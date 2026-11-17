package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"sort"

	"github.com/Indexyz/Cairn/server/internal/domain"
	"github.com/Indexyz/Cairn/server/internal/mockmodel"
)

func (app *App) listProjects(w http.ResponseWriter, r *http.Request) error {
	return app.tx(w, r, func(ctx context.Context, conn *sql.Conn) (int, any, error) {
		if err := domain.ExpireWorkers(ctx, conn, nil); err != nil {
			return 0, nil, err
		}
		if err := domain.ExpireReasonLeases(ctx, conn, nil); err != nil {
			return 0, nil, err
		}
		rows, err := conn.QueryContext(ctx, `
			SELECT p.id, p.title, p.status, p.bootstrap_enabled, p.max_steps, p.schedule_id, p.created_at,
			       p.reason_worker, p.reason_trigger, p.reason_started_at, p.reason_last_heartbeat_at,
			       (SELECT COUNT(*) FROM facts WHERE project_id = p.id),
			       (SELECT COUNT(*) FROM intents WHERE project_id = p.id),
			       (SELECT COUNT(*) FROM intents WHERE project_id = p.id AND concluded_at IS NULL AND worker IS NOT NULL),
			       (SELECT COUNT(*) FROM intents WHERE project_id = p.id AND concluded_at IS NULL AND worker IS NULL),
			       (SELECT COUNT(*) FROM hints WHERE project_id = p.id)
			FROM projects p ORDER BY p.created_at`)
		if err != nil {
			return 0, nil, err
		}
		defer rows.Close()
		summaries := []domain.ProjectSummary{}
		for rows.Next() {
			row := &domain.ProjectRow{}
			var bootstrap int
			var maxSteps sql.NullInt64
			var scheduleID sql.NullString
			var reasonWorker, reasonTrigger, reasonStarted, reasonHeartbeat sql.NullString
			summary := domain.ProjectSummary{}
			if err := rows.Scan(
				&row.ID, &row.Title, &row.Status, &bootstrap, &maxSteps, &scheduleID, &row.CreatedAt,
				&reasonWorker, &reasonTrigger, &reasonStarted, &reasonHeartbeat,
				&summary.FactCount, &summary.IntentCount, &summary.WorkingIntentCount,
				&summary.UnclaimedIntentCount, &summary.HintCount,
			); err != nil {
				return 0, nil, err
			}
			row.BootstrapEnabled = bootstrap != 0
			if maxSteps.Valid {
				value := int(maxSteps.Int64)
				row.MaxSteps = &value
			}
			if scheduleID.Valid {
				value := scheduleID.String
				row.ScheduleID = &value
			}
			if reasonWorker.Valid {
				value := reasonWorker.String
				row.ReasonWorker = &value
			}
			if reasonTrigger.Valid {
				value := reasonTrigger.String
				row.ReasonTrigger = &value
			}
			if reasonStarted.Valid {
				value := reasonStarted.String
				row.ReasonStartedAt = &value
			}
			if reasonHeartbeat.Valid {
				value := reasonHeartbeat.String
				row.ReasonLastHeartbeat = &value
			}
			summary.ProjectMeta = domain.ProjectMetaFromRow(row)
			summaries = append(summaries, summary)
		}
		return http.StatusOK, summaries, nil
	})
}

func (app *App) createProject(w http.ResponseWriter, r *http.Request) error {
	var body domain.CreateProjectRequest
	if err := decodeBody(r, &body); err != nil {
		return err
	}
	if validation := body.Validate(); validation != nil {
		return validation
	}
	return app.tx(w, r, func(ctx context.Context, conn *sql.Conn) (int, any, error) {
		detail, err := domain.CreateProject(ctx, conn, &body)
		if err != nil {
			return 0, nil, err
		}
		app.NotifyProjectCreated(ctx, conn, detail.Project.ID, body.Title)
		return http.StatusCreated, detail, nil
	})
}

func (app *App) forkProject(w http.ResponseWriter, r *http.Request) error {
	var body domain.ForkProjectRequest
	if err := decodeOptional(r, &body); err != nil {
		return err
	}
	body.NormalizeTitle()
	return app.tx(w, r, func(ctx context.Context, conn *sql.Conn) (int, any, error) {
		source, err := domain.GetProjectOr404(ctx, conn, r.PathValue("project"))
		if err != nil {
			return 0, nil, err
		}
		facts := map[string]string{}
		rows, err := conn.QueryContext(ctx, "SELECT id, description FROM facts WHERE project_id = ? AND id IN ('origin', 'goal')", source.ID)
		if err != nil {
			return 0, nil, err
		}
		for rows.Next() {
			var id, description string
			if err := rows.Scan(&id, &description); err != nil {
				rows.Close()
				return 0, nil, err
			}
			facts[id] = description
		}
		rows.Close()
		hintRows, err := conn.QueryContext(ctx, "SELECT content, creator FROM hints WHERE project_id = ? ORDER BY created_at", source.ID)
		if err != nil {
			return 0, nil, err
		}
		type sourceHint struct{ content, creator string }
		var sourceHints []sourceHint
		for hintRows.Next() {
			var hint sourceHint
			if err := hintRows.Scan(&hint.content, &hint.creator); err != nil {
				hintRows.Close()
				return 0, nil, err
			}
			sourceHints = append(sourceHints, hint)
		}
		hintRows.Close()

		projectID, err := domain.NextProjectID(ctx, conn)
		if err != nil {
			return 0, nil, err
		}
		now := domain.UtcNow()
		title := source.Title + " (fork)"
		if body.Title != nil {
			title = *body.Title
		}
		if _, err := conn.ExecContext(ctx,
			"INSERT INTO projects (id, title, status, bootstrap_enabled, max_steps, created_at) VALUES (?, ?, 'active', ?, ?, ?)",
			projectID, title, boolInt(source.BootstrapEnabled), source.MaxSteps, now); err != nil {
			return 0, nil, err
		}
		for _, id := range []string{"origin", "goal"} {
			if _, err := conn.ExecContext(ctx, "INSERT INTO facts (id, project_id, description) VALUES (?, ?, ?)", id, projectID, facts[id]); err != nil {
				return 0, nil, err
			}
		}
		hints := []domain.Hint{}
		for _, hint := range sourceHints {
			hintID, err := domain.NextHintID(ctx, conn, projectID)
			if err != nil {
				return 0, nil, err
			}
			if _, err := conn.ExecContext(ctx, "INSERT INTO hints (id, project_id, content, creator, created_at) VALUES (?, ?, ?, ?, ?)",
				hintID, projectID, hint.content, hint.creator, now); err != nil {
				return 0, nil, err
			}
			hints = append(hints, domain.Hint{ID: hintID, Content: hint.content, Creator: hint.creator, CreatedAt: now})
		}
		app.pendingNotify(ctx, conn, "project_created", projectID, title, "")
		return http.StatusCreated, domain.ProjectDetail{
			Project: domain.ProjectMeta{
				ID: projectID, Title: title, Status: "active", BootstrapEnabled: source.BootstrapEnabled,
				MaxSteps: source.MaxSteps, CreatedAt: now,
			},
			Facts:   []domain.Fact{{ID: "origin", Description: facts["origin"]}, {ID: "goal", Description: facts["goal"]}},
			Intents: []domain.Intent{},
			Hints:   hints,
		}, nil
	})
}

func (app *App) getProject(w http.ResponseWriter, r *http.Request) error {
	return app.tx(w, r, func(ctx context.Context, conn *sql.Conn) (int, any, error) {
		projectID := r.PathValue("project")
		if err := domain.ExpireWorkers(ctx, conn, &projectID); err != nil {
			return 0, nil, err
		}
		if err := domain.ExpireReasonLeases(ctx, conn, &projectID); err != nil {
			return 0, nil, err
		}
		project, err := domain.GetProjectOr404(ctx, conn, projectID)
		if err != nil {
			return 0, nil, err
		}
		factRows, err := conn.QueryContext(ctx, "SELECT id, description FROM facts WHERE project_id = ?", projectID)
		if err != nil {
			return 0, nil, err
		}
		facts := []domain.Fact{}
		for factRows.Next() {
			var fact domain.Fact
			if err := factRows.Scan(&fact.ID, &fact.Description); err != nil {
				factRows.Close()
				return 0, nil, err
			}
			facts = append(facts, fact)
		}
		factRows.Close()
		hintRows, err := conn.QueryContext(ctx, "SELECT id, content, creator, created_at FROM hints WHERE project_id = ? ORDER BY created_at", projectID)
		if err != nil {
			return 0, nil, err
		}
		hints := []domain.Hint{}
		for hintRows.Next() {
			var hint domain.Hint
			if err := hintRows.Scan(&hint.ID, &hint.Content, &hint.Creator, &hint.CreatedAt); err != nil {
				hintRows.Close()
				return 0, nil, err
			}
			hints = append(hints, hint)
		}
		hintRows.Close()
		intents, err := domain.BuildIntents(ctx, conn, projectID)
		if err != nil {
			return 0, nil, err
		}
		return http.StatusOK, domain.ProjectDetail{
			Project: domain.ProjectMetaFromRow(project), Facts: facts, Intents: intents, Hints: hints,
		}, nil
	})
}

func (app *App) deleteProject(w http.ResponseWriter, r *http.Request) error {
	return app.tx(w, r, func(ctx context.Context, conn *sql.Conn) (int, any, error) {
		projectID := r.PathValue("project")
		if _, err := domain.GetProjectOr404(ctx, conn, projectID); err != nil {
			return 0, nil, err
		}
		if _, err := conn.ExecContext(ctx, "DELETE FROM projects WHERE id = ?", projectID); err != nil {
			return 0, nil, err
		}
		return http.StatusNoContent, nil, nil
	})
}

func (app *App) updateProjectTitle(w http.ResponseWriter, r *http.Request) error {
	var body domain.UpdateProjectTitleRequest
	if err := decodeBody(r, &body); err != nil {
		return err
	}
	if validation := body.Validate(); validation != nil {
		return validation
	}
	return app.tx(w, r, func(ctx context.Context, conn *sql.Conn) (int, any, error) {
		projectID := r.PathValue("project")
		if _, err := domain.GetProjectOr404(ctx, conn, projectID); err != nil {
			return 0, nil, err
		}
		if _, err := conn.ExecContext(ctx, "UPDATE projects SET title = ? WHERE id = ?", body.Title, projectID); err != nil {
			return 0, nil, err
		}
		updated, err := domain.GetProjectOr404(ctx, conn, projectID)
		if err != nil {
			return 0, nil, err
		}
		return http.StatusOK, domain.ProjectMetaFromRow(updated), nil
	})
}

func (app *App) updateProjectMaxSteps(w http.ResponseWriter, r *http.Request) error {
	var body domain.UpdateProjectMaxStepsRequest
	if err := decodeBody(r, &body); err != nil {
		return err
	}
	if validation := body.Validate(); validation != nil {
		return validation
	}
	return app.tx(w, r, func(ctx context.Context, conn *sql.Conn) (int, any, error) {
		projectID := r.PathValue("project")
		row, err := domain.GetProjectOr404(ctx, conn, projectID)
		if err != nil {
			return 0, nil, err
		}
		if row.Status == "completed" {
			return 0, nil, domain.Conflict("Completed projects cannot change max_steps")
		}
		if _, err := conn.ExecContext(ctx, "UPDATE projects SET max_steps = ? WHERE id = ?", body.MaxSteps, projectID); err != nil {
			return 0, nil, err
		}
		if row.Status == "blocked" {
			exhausted, err := domain.StepBudgetExhausted(ctx, conn, projectID)
			if err != nil {
				return 0, nil, err
			}
			if !exhausted {
				if _, err := conn.ExecContext(ctx, "UPDATE projects SET status = 'active' WHERE id = ?", projectID); err != nil {
					return 0, nil, err
				}
			}
		} else if row.Status == "active" {
			if _, err := domain.EnforceStepLimit(ctx, conn, projectID); err != nil {
				return 0, nil, err
			}
		}
		updated, err := domain.GetProjectOr404(ctx, conn, projectID)
		if err != nil {
			return 0, nil, err
		}
		if updated.Status != row.Status && (updated.Status == "active" || updated.Status == "blocked") {
			event := "project_blocked"
			if updated.Status == "active" {
				event = "project_resumed"
			}
			app.pendingNotify(ctx, conn, event, projectID, updated.Title, "")
		}
		return http.StatusOK, domain.ProjectMetaFromRow(updated), nil
	})
}

func (app *App) updateProjectStatus(w http.ResponseWriter, r *http.Request) error {
	var body domain.UpdateProjectStatusRequest
	if err := decodeBody(r, &body); err != nil {
		return err
	}
	if validation := body.Validate(); validation != nil {
		return validation
	}
	return app.tx(w, r, func(ctx context.Context, conn *sql.Conn) (int, any, error) {
		projectID := r.PathValue("project")
		if err := domain.ExpireReasonLeases(ctx, conn, &projectID); err != nil {
			return 0, nil, err
		}
		row, err := domain.GetProjectOr404(ctx, conn, projectID)
		if err != nil {
			return 0, nil, err
		}
		if row.Status == "completed" {
			return 0, nil, domain.Conflict("Completed projects cannot change status")
		}
		if row.Status == body.Status {
			return http.StatusOK, domain.ProjectMetaFromRow(row), nil
		}
		if _, err := conn.ExecContext(ctx, "UPDATE projects SET status = ? WHERE id = ?", body.Status, projectID); err != nil {
			return 0, nil, err
		}
		if body.Status == "stopped" {
			if _, err := conn.ExecContext(ctx, "UPDATE intents SET worker = NULL WHERE project_id = ? AND concluded_at IS NULL", projectID); err != nil {
				return 0, nil, err
			}
			if err := domain.ClearProjectReason(ctx, conn, projectID); err != nil {
				return 0, nil, err
			}
		}
		updated, err := domain.GetProjectOr404(ctx, conn, projectID)
		if err != nil {
			return 0, nil, err
		}
		event := "project_resumed"
		if body.Status == "stopped" {
			event = "project_stopped"
		}
		app.pendingNotify(ctx, conn, event, projectID, updated.Title, "")
		return http.StatusOK, domain.ProjectMetaFromRow(updated), nil
	})
}

func (app *App) claimReason(w http.ResponseWriter, r *http.Request) error {
	var body domain.ReasonClaimRequest
	if err := decodeBody(r, &body); err != nil {
		return err
	}
	if validation := body.Validate(); validation != nil {
		return validation
	}
	return app.tx(w, r, func(ctx context.Context, conn *sql.Conn) (int, any, error) {
		projectID := r.PathValue("project")
		if _, err := domain.CheckProjectRunnable(ctx, conn, projectID); err != nil {
			return 0, nil, err
		}
		if err := domain.ExpireReasonLeases(ctx, conn, &projectID); err != nil {
			return 0, nil, err
		}
		row, err := domain.GetProjectOr404(ctx, conn, projectID)
		if err != nil {
			return 0, nil, err
		}
		if row.ReasonWorker != nil && *row.ReasonWorker != body.Worker {
			return 0, nil, domain.Conflict("Project reason is currently claimed by " + *row.ReasonWorker)
		}
		if row.ReasonWorker != nil && *row.ReasonWorker == body.Worker {
			return http.StatusOK, domain.ProjectMetaFromRow(row), nil
		}
		now := domain.UtcNow()
		if _, err := conn.ExecContext(ctx, `
			UPDATE projects SET reason_worker = ?, reason_trigger = ?, reason_started_at = ?, reason_last_heartbeat_at = ? WHERE id = ?`,
			body.Worker, body.Trigger, now, now, projectID); err != nil {
			return 0, nil, err
		}
		updated, err := domain.GetProjectOr404(ctx, conn, projectID)
		if err != nil {
			return 0, nil, err
		}
		return http.StatusOK, domain.ProjectMetaFromRow(updated), nil
	})
}

func (app *App) heartbeatReason(w http.ResponseWriter, r *http.Request) error {
	var body domain.HeartbeatRequest
	if err := decodeBody(r, &body); err != nil {
		return err
	}
	if validation := body.Validate(); validation != nil {
		return validation
	}
	return app.tx(w, r, func(ctx context.Context, conn *sql.Conn) (int, any, error) {
		projectID := r.PathValue("project")
		if _, err := domain.CheckProjectRunnable(ctx, conn, projectID); err != nil {
			return 0, nil, err
		}
		if err := domain.ExpireReasonLeases(ctx, conn, &projectID); err != nil {
			return 0, nil, err
		}
		row, err := domain.GetProjectOr404(ctx, conn, projectID)
		if err != nil {
			return 0, nil, err
		}
		if row.ReasonWorker == nil {
			return 0, nil, domain.Conflict("Project reason is not currently claimed")
		}
		if *row.ReasonWorker != body.Worker {
			return 0, nil, domain.Conflict("Project reason is currently claimed by " + *row.ReasonWorker)
		}
		if _, err := conn.ExecContext(ctx, "UPDATE projects SET reason_last_heartbeat_at = ? WHERE id = ?", domain.UtcNow(), projectID); err != nil {
			return 0, nil, err
		}
		updated, err := domain.GetProjectOr404(ctx, conn, projectID)
		if err != nil {
			return 0, nil, err
		}
		return http.StatusOK, domain.ProjectMetaFromRow(updated), nil
	})
}

func (app *App) releaseReason(w http.ResponseWriter, r *http.Request) error {
	var body domain.HeartbeatRequest
	if err := decodeBody(r, &body); err != nil {
		return err
	}
	if validation := body.Validate(); validation != nil {
		return validation
	}
	return app.tx(w, r, func(ctx context.Context, conn *sql.Conn) (int, any, error) {
		projectID := r.PathValue("project")
		if _, err := domain.CheckProjectRunnable(ctx, conn, projectID); err != nil {
			return 0, nil, err
		}
		if err := domain.ExpireReasonLeases(ctx, conn, &projectID); err != nil {
			return 0, nil, err
		}
		row, err := domain.GetProjectOr404(ctx, conn, projectID)
		if err != nil {
			return 0, nil, err
		}
		if row.ReasonWorker == nil {
			return http.StatusOK, domain.ProjectMetaFromRow(row), nil
		}
		if *row.ReasonWorker != body.Worker {
			return 0, nil, domain.Conflict("Project reason is currently claimed by " + *row.ReasonWorker)
		}
		if err := domain.ClearProjectReason(ctx, conn, projectID); err != nil {
			return 0, nil, err
		}
		updated, err := domain.GetProjectOr404(ctx, conn, projectID)
		if err != nil {
			return 0, nil, err
		}
		return http.StatusOK, domain.ProjectMetaFromRow(updated), nil
	})
}

func (app *App) completeProject(w http.ResponseWriter, r *http.Request) error {
	var body domain.CompleteRequest
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
		if err := domain.ExpireReasonLeases(ctx, conn, &projectID); err != nil {
			return 0, nil, err
		}
		if err := domain.ValidateFactsExist(ctx, conn, projectID, body.From); err != nil {
			return 0, nil, err
		}
		if err := domain.ValidateGoalNotInSources(body.From); err != nil {
			return 0, nil, err
		}
		now := domain.UtcNow()
		intentID, err := domain.NextIntentID(ctx, conn, projectID)
		if err != nil {
			return 0, nil, err
		}
		if _, err := conn.ExecContext(ctx,
			"INSERT INTO intents (id, project_id, to_fact_id, description, creator, worker, last_heartbeat_at, created_at, concluded_at) VALUES (?, ?, 'goal', ?, ?, ?, ?, ?, ?)",
			intentID, projectID, body.Description, body.Worker, body.Worker, now, now, now); err != nil {
			return 0, nil, err
		}
		for _, factID := range body.From {
			if _, err := conn.ExecContext(ctx, "INSERT INTO intent_sources (intent_id, project_id, fact_id) VALUES (?, ?, ?)",
				intentID, projectID, factID); err != nil {
				return 0, nil, err
			}
		}
		if _, err := conn.ExecContext(ctx, `
			UPDATE projects SET status = 'completed', reason_worker = NULL, reason_trigger = NULL,
				reason_started_at = NULL, reason_last_heartbeat_at = NULL WHERE id = ?`, projectID); err != nil {
			return 0, nil, err
		}
		app.pendingNotify(ctx, conn, "project_completed", projectID, project.Title, body.Description)
		worker := body.Worker
		return http.StatusOK, domain.Intent{
			ID: intentID, From: body.From, To: stringPointer("goal"), Description: body.Description,
			Creator: body.Worker, Worker: &worker, LastHeartbeatAt: &now, CreatedAt: now, ConcludedAt: &now,
		}, nil
	})
}

func (app *App) reopenProject(w http.ResponseWriter, r *http.Request) error {
	var body domain.ReopenRequest
	if err := decodeBody(r, &body); err != nil {
		return err
	}
	if validation := body.Validate(); validation != nil {
		return validation
	}
	return app.tx(w, r, func(ctx context.Context, conn *sql.Conn) (int, any, error) {
		projectID := r.PathValue("project")
		if err := domain.ExpireReasonLeases(ctx, conn, &projectID); err != nil {
			return 0, nil, err
		}
		if _, err := domain.CheckProjectCompleted(ctx, conn, projectID); err != nil {
			return 0, nil, err
		}
		completion, err := domain.GetCompletionIntentOr409(ctx, conn, projectID)
		if err != nil {
			return 0, nil, err
		}
		sourceRows, err := conn.QueryContext(ctx, "SELECT fact_id FROM intent_sources WHERE intent_id = ? AND project_id = ? ORDER BY rowid", completion.ID, projectID)
		if err != nil {
			return 0, nil, err
		}
		sourceIDs := []string{}
		for sourceRows.Next() {
			var factID string
			if err := sourceRows.Scan(&factID); err != nil {
				sourceRows.Close()
				return 0, nil, err
			}
			sourceIDs = append(sourceIDs, factID)
		}
		sourceRows.Close()
		if len(sourceIDs) == 0 {
			return 0, nil, domain.Conflict("Completion intent is missing its source facts")
		}

		now := domain.UtcNow()
		factID, err := domain.NextFactID(ctx, conn, projectID)
		if err != nil {
			return 0, nil, err
		}
		intentID, err := domain.NextIntentID(ctx, conn, projectID)
		if err != nil {
			return 0, nil, err
		}
		if _, err := conn.ExecContext(ctx, "DELETE FROM intents WHERE id = ? AND project_id = ?", completion.ID, projectID); err != nil {
			return 0, nil, err
		}
		if _, err := conn.ExecContext(ctx, "INSERT INTO facts (id, project_id, description) VALUES (?, ?, ?)", factID, projectID, body.Description); err != nil {
			return 0, nil, err
		}
		if _, err := conn.ExecContext(ctx,
			"INSERT INTO intents (id, project_id, to_fact_id, description, creator, worker, last_heartbeat_at, created_at, concluded_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
			intentID, projectID, factID, "external_feedback", body.Creator, body.Creator, now, now, now); err != nil {
			return 0, nil, err
		}
		for _, sourceID := range sourceIDs {
			if _, err := conn.ExecContext(ctx, "INSERT INTO intent_sources (intent_id, project_id, fact_id) VALUES (?, ?, ?)",
				intentID, projectID, sourceID); err != nil {
				return 0, nil, err
			}
		}
		if err := domain.ClearProjectReason(ctx, conn, projectID); err != nil {
			return 0, nil, err
		}
		if _, err := conn.ExecContext(ctx, "UPDATE projects SET status = 'active' WHERE id = ?", projectID); err != nil {
			return 0, nil, err
		}
		updatedProject, err := domain.GetProjectOr404(ctx, conn, projectID)
		if err != nil {
			return 0, nil, err
		}
		intentRow, err := domain.GetIntentOr404(ctx, conn, projectID, intentID)
		if err != nil {
			return 0, nil, err
		}
		intent, err := domain.IntentToModel(ctx, conn, intentRow, projectID)
		if err != nil {
			return 0, nil, err
		}
		app.pendingNotify(ctx, conn, "project_reopened", projectID, updatedProject.Title, body.Description)
		return http.StatusOK, domain.ReopenResponse{
			Project: domain.ProjectMetaFromRow(updatedProject),
			Fact:    domain.Fact{ID: factID, Description: body.Description},
			Intent:  *intent,
		}, nil
	})
}

func stringPointer(value string) *string { return &value }

var _ = sort.Strings
var _ = mockmodel.Resolve
var _ = json.Marshal
