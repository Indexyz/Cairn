// Package scheduler owns the project-creation schedules and fires them.
package scheduler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/Indexyz/Cairn/server/internal/domain"
	"github.com/Indexyz/Cairn/server/internal/store"
)

// Notify pushes the project_created event from inside the firing transaction.
type Notify func(ctx context.Context, conn *sql.Conn, projectID, title string)

// ErrSkipped reports that the previous run is still active.
var ErrSkipped = errors.New("previous run still active")

// Result describes one firing.
type Result struct {
	ProjectID string `json:"project_id"`
	Skipped   bool   `json:"skipped"`
	Detail    string `json:"detail"`
}

const columns = `id, name, cron, timezone, enabled, title, origin, goal, bootstrap_enabled, max_steps,
	hints, next_run_at, last_run_at, last_status, last_detail, last_project_id, skip_count, created_at, updated_at`

func scan(scanner interface{ Scan(...any) error }) (*domain.Schedule, error) {
	schedule := &domain.Schedule{}
	var enabled, bootstrap int
	var maxSteps sql.NullInt64
	var hints string
	var nextRun, lastRun, lastStatus, lastDetail, lastProject sql.NullString
	if err := scanner.Scan(
		&schedule.ID, &schedule.Name, &schedule.Cron, &schedule.Timezone, &enabled,
		&schedule.Title, &schedule.Origin, &schedule.Goal, &bootstrap, &maxSteps,
		&hints, &nextRun, &lastRun, &lastStatus, &lastDetail, &lastProject,
		&schedule.SkipCount, &schedule.CreatedAt, &schedule.UpdatedAt,
	); err != nil {
		return nil, err
	}
	schedule.Enabled = enabled != 0
	schedule.BootstrapEnabled = bootstrap != 0
	if maxSteps.Valid {
		value := int(maxSteps.Int64)
		schedule.MaxSteps = &value
	}
	if err := json.Unmarshal([]byte(hints), &schedule.Hints); err != nil {
		return nil, err
	}
	for source, target := range map[*sql.NullString]**string{
		&nextRun: &schedule.NextRunAt, &lastRun: &schedule.LastRunAt, &lastStatus: &schedule.LastStatus,
		&lastDetail: &schedule.LastDetail, &lastProject: &schedule.LastProjectID,
	} {
		if source.Valid {
			value := source.String
			*target = &value
		}
	}
	return schedule, nil
}

// List returns every schedule in creation order.
func List(ctx context.Context, db *store.DB) ([]domain.Schedule, error) {
	schedules := []domain.Schedule{}
	err := db.Read(ctx, func(conn *sql.Conn) error {
		rows, err := conn.QueryContext(ctx, "SELECT "+columns+" FROM schedules ORDER BY created_at, id")
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			schedule, err := scan(rows)
			if err != nil {
				return err
			}
			schedules = append(schedules, *schedule)
		}
		return nil
	})
	return schedules, err
}

// Get loads one schedule.
func Get(ctx context.Context, db *store.DB, scheduleID string) (*domain.Schedule, error) {
	schedule := &domain.Schedule{}
	err := db.Read(ctx, func(conn *sql.Conn) error {
		loaded, err := get(ctx, conn, scheduleID)
		schedule = loaded
		return err
	})
	return schedule, err
}

func get(ctx context.Context, conn *sql.Conn, scheduleID string) (*domain.Schedule, error) {
	schedule, err := scan(conn.QueryRowContext(ctx, "SELECT "+columns+" FROM schedules WHERE id = ?", scheduleID))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.NotFound("schedule " + scheduleID + " not found")
		}
		return nil, err
	}
	return schedule, nil
}

// Create stores a schedule and anchors its first firing.
func Create(ctx context.Context, db *store.DB, payload *domain.SchedulePayload, now time.Time) (*domain.Schedule, error) {
	var schedule *domain.Schedule
	err := db.WithTx(ctx, func(conn *sql.Conn) error {
		scheduleID, err := domain.NextScheduleID(ctx, conn)
		if err != nil {
			return err
		}
		next, err := nextRun(payload, now, payload.EnabledValue())
		if err != nil {
			return err
		}
		stamp := now.UTC().Format("2006-01-02T15:04:05Z")
		hints, _ := json.Marshal(payload.Hints)
		if _, err := conn.ExecContext(ctx,
			"INSERT INTO schedules (id, name, cron, timezone, enabled, title, origin, goal, bootstrap_enabled, max_steps, hints, next_run_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			scheduleID, payload.Name, payload.Cron, payload.Timezone, flag(payload.EnabledValue()), payload.Title,
			payload.Origin, payload.Goal, flag(payload.BootstrapValue()), payload.MaxSteps, string(hints), next, stamp, stamp); err != nil {
			return err
		}
		schedule, err = get(ctx, conn, scheduleID)
		return err
	})
	return schedule, err
}

// Update rewrites a schedule and re-anchors its next firing.
func Update(ctx context.Context, db *store.DB, scheduleID string, payload *domain.SchedulePayload, now time.Time) (*domain.Schedule, error) {
	var schedule *domain.Schedule
	err := db.WithTx(ctx, func(conn *sql.Conn) error {
		if _, err := get(ctx, conn, scheduleID); err != nil {
			return err
		}
		next, err := nextRun(payload, now, payload.EnabledValue())
		if err != nil {
			return err
		}
		hints, _ := json.Marshal(payload.Hints)
		if _, err := conn.ExecContext(ctx,
			"UPDATE schedules SET name = ?, cron = ?, timezone = ?, enabled = ?, title = ?, origin = ?, goal = ?, bootstrap_enabled = ?, max_steps = ?, hints = ?, next_run_at = ?, updated_at = ? WHERE id = ?",
			payload.Name, payload.Cron, payload.Timezone, flag(payload.EnabledValue()), payload.Title, payload.Origin,
			payload.Goal, flag(payload.BootstrapValue()), payload.MaxSteps, string(hints), next,
			now.UTC().Format("2006-01-02T15:04:05Z"), scheduleID); err != nil {
			return err
		}
		schedule, err = get(ctx, conn, scheduleID)
		return err
	})
	return schedule, err
}

// Delete drops a schedule; projects it created keep running.
func Delete(ctx context.Context, db *store.DB, scheduleID string) error {
	return db.WithTx(ctx, func(conn *sql.Conn) error {
		if _, err := get(ctx, conn, scheduleID); err != nil {
			return err
		}
		_, err := conn.ExecContext(ctx, "DELETE FROM schedules WHERE id = ?", scheduleID)
		return err
	})
}

func nextRun(payload *domain.SchedulePayload, now time.Time, enabled bool) (*string, error) {
	if !enabled {
		return nil, nil
	}
	schedule := &domain.Schedule{Cron: payload.Cron, Timezone: payload.Timezone}
	next, err := schedule.NextRun(now)
	if err != nil {
		return nil, err
	}
	return &next, nil
}

// Execute fires one schedule.
//
// A forced run (the UI button) always creates a project and leaves the cadence
// anchors alone; a scheduled run skips while the previous project is still
// active and re-anchors the next firing.
func Execute(ctx context.Context, db *store.DB, scheduleID string, now time.Time, notify Notify, force bool) (*Result, error) {
	result := &Result{}
	var createErr error
	err := db.WithTx(ctx, func(conn *sql.Conn) error {
		schedule, err := get(ctx, conn, scheduleID)
		if err != nil {
			return err
		}
		stamp := now.UTC().Format("2006-01-02T15:04:05Z")
		anchor := (*string)(nil)
		if !force {
			next, err := schedule.NextRun(now)
			if err != nil {
				return err
			}
			anchor = &next
			active, err := activeProject(ctx, conn, scheduleID)
			if err != nil {
				return err
			}
			if active != "" {
				result.Skipped = true
				result.Detail = "project " + active + " is still active"
				_, err = conn.ExecContext(ctx,
					"UPDATE schedules SET last_run_at = ?, last_status = ?, last_detail = ?, skip_count = skip_count + 1, next_run_at = ? WHERE id = ?",
					stamp, domain.ScheduleStatusSkipped, result.Detail, anchor, scheduleID)
				return err
			}
		}
		detail, err := domain.CreateProjectFor(ctx, conn, schedule.Template(), &schedule.ID)
		if err != nil {
			// keep the transaction so the failure is recorded
			createErr = err
			_, recordErr := conn.ExecContext(ctx,
				"UPDATE schedules SET last_run_at = ?, last_status = ?, last_detail = ?, next_run_at = COALESCE(?, next_run_at) WHERE id = ?",
				stamp, domain.ScheduleStatusError, err.Error(), anchor, scheduleID)
			return recordErr
		}
		result.ProjectID = detail.Project.ID
		result.Detail = "created " + detail.Project.ID
		if _, err := conn.ExecContext(ctx,
			"UPDATE schedules SET last_run_at = ?, last_status = ?, last_detail = NULL, last_project_id = ?, next_run_at = COALESCE(?, next_run_at) WHERE id = ?",
			stamp, domain.ScheduleStatusCreated, detail.Project.ID, anchor, scheduleID); err != nil {
			return err
		}
		if notify != nil {
			notify(ctx, conn, detail.Project.ID, detail.Project.Title)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if createErr != nil {
		return nil, createErr
	}
	return result, nil
}

func activeProject(ctx context.Context, conn *sql.Conn, scheduleID string) (string, error) {
	var projectID string
	err := conn.QueryRowContext(ctx,
		"SELECT id FROM projects WHERE schedule_id = ? AND status = 'active' ORDER BY created_at DESC LIMIT 1", scheduleID).Scan(&projectID)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return projectID, err
}

// Runner fires due schedules on an interval.
type Runner struct {
	DB       *store.DB
	Interval time.Duration
	Now      func() time.Time
	Notify   Notify
	Log      func(format string, args ...any)
}

// Run ticks until the context is cancelled.
func (runner *Runner) Run(ctx context.Context) {
	interval := runner.Interval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := runner.Tick(ctx); err != nil && ctx.Err() == nil {
			runner.logf("scheduler tick failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Tick fires every schedule that is due.
func (runner *Runner) Tick(ctx context.Context) error {
	now := runner.now()
	var due []string
	if err := runner.DB.Read(ctx, func(conn *sql.Conn) error {
		rows, err := conn.QueryContext(ctx,
			"SELECT id FROM schedules WHERE enabled = 1 AND next_run_at IS NOT NULL AND next_run_at <= ? ORDER BY next_run_at",
			now.UTC().Format("2006-01-02T15:04:05Z"))
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var scheduleID string
			if err := rows.Scan(&scheduleID); err != nil {
				return err
			}
			due = append(due, scheduleID)
		}
		return nil
	}); err != nil {
		return err
	}
	for _, scheduleID := range due {
		result, err := Execute(ctx, runner.DB, scheduleID, now, runner.Notify, false)
		switch {
		case err != nil:
			runner.logf("schedule %s failed: %v", scheduleID, err)
		case result.Skipped:
			runner.logf("schedule %s skipped: %s", scheduleID, result.Detail)
		default:
			runner.logf("schedule %s created %s", scheduleID, result.ProjectID)
		}
	}
	return nil
}

func (runner *Runner) now() time.Time {
	if runner.Now != nil {
		return runner.Now()
	}
	return time.Now()
}

func (runner *Runner) logf(format string, args ...any) {
	if runner.Log != nil {
		runner.Log(format, args...)
	}
}

func flag(value bool) int {
	if value {
		return 1
	}
	return 0
}
