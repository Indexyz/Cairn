package domain

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// HTTPError is a status + detail response.
type HTTPError struct {
	Status int
	Detail string
}

func (err *HTTPError) Error() string { return err.Detail }

// NotFound builds a 404.
func NotFound(detail string) *HTTPError { return &HTTPError{Status: 404, Detail: detail} }

// Conflict builds a 409.
func Conflict(detail string) *HTTPError { return &HTTPError{Status: 409, Detail: detail} }

// Forbidden builds a 403.
func Forbidden(detail string) *HTTPError { return &HTTPError{Status: 403, Detail: detail} }

// BadRequest builds a 400.
func BadRequest(detail string) *HTTPError { return &HTTPError{Status: 400, Detail: detail} }

// UtcNow is the lease timestamp format.
func UtcNow() string { return time.Now().UTC().Format("2006-01-02T15:04:05Z") }

// WorkerModelName strips the dispatcher instance suffix.
func WorkerModelName(worker *string) *string {
	if worker == nil {
		return nil
	}
	name := *worker
	for index := 0; index < len(name); index++ {
		if name[index] == '@' {
			trimmed := name[:index]
			return &trimmed
		}
	}
	return &name
}

// BuildLoadSnapshot counts every live lease.
func BuildLoadSnapshot(ctx context.Context, conn *sql.Conn) (*LoadSnapshot, error) {
	if err := ExpireWorkers(ctx, conn, nil); err != nil {
		return nil, err
	}
	if err := ExpireReasonLeases(ctx, conn, nil); err != nil {
		return nil, err
	}
	snapshot := &LoadSnapshot{Projects: map[string]int{}, Workers: map[string]int{}}
	type lease struct {
		project string
		worker  *string
	}
	var leases []lease

	claimed, err := conn.QueryContext(ctx, "SELECT project_id, worker FROM intents WHERE concluded_at IS NULL AND worker IS NOT NULL")
	if err != nil {
		return nil, err
	}
	for claimed.Next() {
		var project string
		var worker sql.NullString
		if err := claimed.Scan(&project, &worker); err != nil {
			claimed.Close()
			return nil, err
		}
		value := worker.String
		leases = append(leases, lease{project, &value})
	}
	claimed.Close()

	reasoning, err := conn.QueryContext(ctx, "SELECT id, reason_worker FROM projects WHERE reason_worker IS NOT NULL")
	if err != nil {
		return nil, err
	}
	for reasoning.Next() {
		var project string
		var worker sql.NullString
		if err := reasoning.Scan(&project, &worker); err != nil {
			reasoning.Close()
			return nil, err
		}
		value := worker.String
		leases = append(leases, lease{project, &value})
	}
	reasoning.Close()

	for _, item := range leases {
		snapshot.Projects[item.project]++
		if model := WorkerModelName(item.worker); model != nil {
			snapshot.Workers[*model]++
		}
	}
	snapshot.RunningTasks = len(leases)
	snapshot.RunningProjects = len(snapshot.Projects)
	return snapshot, nil
}

// NextProjectID reserves a project id.
func NextProjectID(ctx context.Context, conn *sql.Conn) (string, error) {
	return nextCounter(ctx, conn, "project", "proj_")
}

// NextChannelID reserves a channel id.
func NextChannelID(ctx context.Context, conn *sql.Conn) (string, error) {
	return nextCounter(ctx, conn, "notification_channel", "chan_")
}

// NextScheduleID reserves a schedule id.
func NextScheduleID(ctx context.Context, conn *sql.Conn) (string, error) {
	return nextCounter(ctx, conn, "schedule", "sched_")
}

func nextCounter(ctx context.Context, conn *sql.Conn, name, prefix string) (string, error) {
	if _, err := conn.ExecContext(ctx, "UPDATE counters SET value = value + 1 WHERE name = ?", name); err != nil {
		return "", err
	}
	var value int
	if err := conn.QueryRowContext(ctx, "SELECT value FROM counters WHERE name = ?", name).Scan(&value); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%03d", prefix, value), nil
}

func nextScopedID(ctx context.Context, conn *sql.Conn, projectID, kind, prefix string) (string, error) {
	if _, err := conn.ExecContext(ctx, "INSERT OR IGNORE INTO scoped_counters (project_id, kind, value) VALUES (?, ?, 0)", projectID, kind); err != nil {
		return "", err
	}
	if _, err := conn.ExecContext(ctx, "UPDATE scoped_counters SET value = value + 1 WHERE project_id = ? AND kind = ?", projectID, kind); err != nil {
		return "", err
	}
	var value int
	if err := conn.QueryRowContext(ctx, "SELECT value FROM scoped_counters WHERE project_id = ? AND kind = ?", projectID, kind).Scan(&value); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%03d", prefix, value), nil
}

// NextFactID reserves a fact id.
func NextFactID(ctx context.Context, conn *sql.Conn, projectID string) (string, error) {
	return nextScopedID(ctx, conn, projectID, "fact", "f")
}

// NextIntentID reserves an intent id.
func NextIntentID(ctx context.Context, conn *sql.Conn, projectID string) (string, error) {
	return nextScopedID(ctx, conn, projectID, "intent", "i")
}

// NextHintID reserves a hint id.
func NextHintID(ctx context.Context, conn *sql.Conn, projectID string) (string, error) {
	return nextScopedID(ctx, conn, projectID, "hint", "h")
}

// ProjectRow is a projects table row.
type ProjectRow struct {
	ID                  string
	Title               string
	Status              string
	BootstrapEnabled    bool
	MaxSteps            *int
	CreatedAt           string
	ReasonWorker        *string
	ReasonTrigger       *string
	ReasonStartedAt     *string
	ReasonLastHeartbeat *string
}

const projectColumns = "id, title, status, bootstrap_enabled, max_steps, created_at, reason_worker, reason_trigger, reason_started_at, reason_last_heartbeat_at"

func scanProject(scanner interface{ Scan(...any) error }) (*ProjectRow, error) {
	row := &ProjectRow{}
	var bootstrap int
	var maxSteps sql.NullInt64
	var reasonWorker, reasonTrigger, reasonStarted, reasonHeartbeat sql.NullString
	if err := scanner.Scan(
		&row.ID, &row.Title, &row.Status, &bootstrap, &maxSteps, &row.CreatedAt,
		&reasonWorker, &reasonTrigger, &reasonStarted, &reasonHeartbeat,
	); err != nil {
		return nil, err
	}
	row.BootstrapEnabled = bootstrap != 0
	if maxSteps.Valid {
		value := int(maxSteps.Int64)
		row.MaxSteps = &value
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
	return row, nil
}

// GetProjectOr404 loads one project.
func GetProjectOr404(ctx context.Context, conn *sql.Conn, projectID string) (*ProjectRow, error) {
	row, err := scanProject(conn.QueryRowContext(ctx, "SELECT "+projectColumns+" FROM projects WHERE id = ?", projectID))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, NotFound("Project not found")
		}
		return nil, err
	}
	return row, nil
}

// CheckProjectRunnable allows active and blocked projects.
func CheckProjectRunnable(ctx context.Context, conn *sql.Conn, projectID string) (*ProjectRow, error) {
	row, err := GetProjectOr404(ctx, conn, projectID)
	if err != nil {
		return nil, err
	}
	if row.Status != "active" && row.Status != "blocked" {
		return nil, Forbidden("Project is " + row.Status)
	}
	return row, nil
}

// CheckProjectHintWritable allows every status.
func CheckProjectHintWritable(ctx context.Context, conn *sql.Conn, projectID string) (*ProjectRow, error) {
	return GetProjectOr404(ctx, conn, projectID)
}

// CheckProjectCompleted requires the completed status.
func CheckProjectCompleted(ctx context.Context, conn *sql.Conn, projectID string) (*ProjectRow, error) {
	row, err := GetProjectOr404(ctx, conn, projectID)
	if err != nil {
		return nil, err
	}
	if row.Status != "completed" {
		return nil, Forbidden("Project is " + row.Status)
	}
	return row, nil
}

// ValidateFactsExist checks each fact id.
func ValidateFactsExist(ctx context.Context, conn *sql.Conn, projectID string, factIDs []string) error {
	for _, factID := range factIDs {
		var one int
		if err := conn.QueryRowContext(ctx, "SELECT 1 FROM facts WHERE id = ? AND project_id = ?", factID, projectID).Scan(&one); err != nil {
			if err == sql.ErrNoRows {
				return NotFound("Fact " + factID + " not found")
			}
			return err
		}
	}
	return nil
}

// ValidateGoalNotInSources rejects `goal` as a source.
func ValidateGoalNotInSources(factIDs []string) error {
	for _, factID := range factIDs {
		if factID == "goal" {
			return BadRequest("goal cannot be used in from")
		}
	}
	return nil
}

// ValidateIntentCreatorWorker keeps creator and worker aligned.
func ValidateIntentCreatorWorker(creator string, worker *string) error {
	if worker != nil && *worker != creator {
		return BadRequest("worker must be null or equal to creator")
	}
	return nil
}

// IntentRow is an intents table row.
type IntentRow struct {
	ID              string
	ToFactID        *string
	Description     string
	Creator         string
	Worker          *string
	LastHeartbeatAt *string
	CreatedAt       string
	ConcludedAt     *string
}

const intentColumns = "id, to_fact_id, description, creator, worker, last_heartbeat_at, created_at, concluded_at"

func scanIntent(scanner interface{ Scan(...any) error }) (*IntentRow, error) {
	row := &IntentRow{}
	var toFact, worker, heartbeat, concluded sql.NullString
	if err := scanner.Scan(&row.ID, &toFact, &row.Description, &row.Creator, &worker, &heartbeat, &row.CreatedAt, &concluded); err != nil {
		return nil, err
	}
	for source, target := range map[*sql.NullString]**string{
		&toFact: &row.ToFactID, &worker: &row.Worker, &heartbeat: &row.LastHeartbeatAt, &concluded: &row.ConcludedAt,
	} {
		if source.Valid {
			value := source.String
			*target = &value
		}
	}
	return row, nil
}

// GetIntentOr404 loads an intent.
func GetIntentOr404(ctx context.Context, conn *sql.Conn, projectID, intentID string) (*IntentRow, error) {
	row, err := scanIntent(conn.QueryRowContext(ctx, "SELECT "+intentColumns+" FROM intents WHERE id = ? AND project_id = ?", intentID, projectID))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, NotFound("Intent not found")
		}
		return nil, err
	}
	return row, nil
}

// GetClaimableOpenIntentOr404 loads an intent the worker may claim or conclude.
func GetClaimableOpenIntentOr404(ctx context.Context, conn *sql.Conn, projectID, intentID, worker string) (*IntentRow, error) {
	if err := ExpireWorkers(ctx, conn, &projectID); err != nil {
		return nil, err
	}
	row, err := GetIntentOr404(ctx, conn, projectID, intentID)
	if err != nil {
		return nil, err
	}
	if row.ToFactID != nil {
		return nil, Conflict("Intent already concluded")
	}
	if row.Worker != nil && *row.Worker != worker {
		return nil, Conflict("Intent is currently claimed by " + *row.Worker)
	}
	return row, nil
}

// GetReleasableOpenIntentOr404 loads an intent the worker may release.
func GetReleasableOpenIntentOr404(ctx context.Context, conn *sql.Conn, projectID, intentID, worker string) (*IntentRow, error) {
	if err := ExpireWorkers(ctx, conn, &projectID); err != nil {
		return nil, err
	}
	row, err := GetIntentOr404(ctx, conn, projectID, intentID)
	if err != nil {
		return nil, err
	}
	if row.ToFactID != nil {
		return nil, Conflict("Intent already concluded")
	}
	if row.Worker == nil {
		return row, nil
	}
	if *row.Worker != worker {
		return nil, Conflict("Intent is currently claimed by " + *row.Worker)
	}
	return row, nil
}

// GetCompletionIntentOr409 loads the goal edge.
func GetCompletionIntentOr409(ctx context.Context, conn *sql.Conn, projectID string) (*IntentRow, error) {
	rows, err := conn.QueryContext(ctx, "SELECT "+intentColumns+" FROM intents WHERE project_id = ? AND to_fact_id = 'goal'", projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var found []*IntentRow
	for rows.Next() {
		row, err := scanIntent(rows)
		if err != nil {
			return nil, err
		}
		found = append(found, row)
	}
	if len(found) == 0 {
		return nil, Conflict("Completed project is missing its completion intent")
	}
	if len(found) > 1 {
		return nil, Conflict("Completed project has multiple completion intents")
	}
	return found[0], nil
}

// IntentToModel renders an intent with its sources.
func IntentToModel(ctx context.Context, conn *sql.Conn, row *IntentRow, projectID string) (*Intent, error) {
	sources, err := conn.QueryContext(ctx, "SELECT fact_id FROM intent_sources WHERE intent_id = ? AND project_id = ? ORDER BY rowid", row.ID, projectID)
	if err != nil {
		return nil, err
	}
	defer sources.Close()
	from := []string{}
	for sources.Next() {
		var factID string
		if err := sources.Scan(&factID); err != nil {
			return nil, err
		}
		from = append(from, factID)
	}
	return &Intent{
		ID: row.ID, From: from, To: row.ToFactID, Description: row.Description, Creator: row.Creator,
		Worker: row.Worker, LastHeartbeatAt: row.LastHeartbeatAt, CreatedAt: row.CreatedAt, ConcludedAt: row.ConcludedAt,
	}, nil
}

// BuildIntents renders every intent of a project.
func BuildIntents(ctx context.Context, conn *sql.Conn, projectID string) ([]Intent, error) {
	rows, err := conn.QueryContext(ctx, "SELECT "+intentColumns+" FROM intents WHERE project_id = ? ORDER BY created_at", projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	intents := []Intent{}
	for rows.Next() {
		row, err := scanIntent(rows)
		if err != nil {
			return nil, err
		}
		intent, err := IntentToModel(ctx, conn, row, projectID)
		if err != nil {
			return nil, err
		}
		intents = append(intents, *intent)
	}
	return intents, nil
}

func getTimeout(ctx context.Context, conn *sql.Conn, column string) (int, error) {
	var value int
	err := conn.QueryRowContext(ctx, "SELECT "+column+" FROM settings WHERE rowid = 1").Scan(&value)
	return value, err
}

// ExpireWorkers releases stale intent claims.
func ExpireWorkers(ctx context.Context, conn *sql.Conn, projectID *string) error {
	timeout, err := getTimeout(ctx, conn, "intent_timeout")
	if err != nil {
		return err
	}
	query := `UPDATE intents SET worker = NULL
		WHERE to_fact_id IS NULL AND worker IS NOT NULL AND last_heartbeat_at IS NOT NULL
		  AND (julianday(?) - julianday(last_heartbeat_at)) * 86400 > ?`
	params := []any{UtcNow(), timeout}
	if projectID != nil {
		query = query + " AND project_id = ?"
		params = append(params, *projectID)
	}
	_, err = conn.ExecContext(ctx, query, params...)
	return err
}

// ExpireReasonLeases releases stale reason leases.
func ExpireReasonLeases(ctx context.Context, conn *sql.Conn, projectID *string) error {
	timeout, err := getTimeout(ctx, conn, "reason_timeout")
	if err != nil {
		return err
	}
	query := `UPDATE projects
		SET reason_worker = NULL, reason_trigger = NULL, reason_started_at = NULL, reason_last_heartbeat_at = NULL
		WHERE reason_worker IS NOT NULL AND reason_last_heartbeat_at IS NOT NULL
		  AND (julianday(?) - julianday(reason_last_heartbeat_at)) * 86400 > ?`
	params := []any{UtcNow(), timeout}
	if projectID != nil {
		query = query + " AND id = ?"
		params = append(params, *projectID)
	}
	_, err = conn.ExecContext(ctx, query, params...)
	return err
}

// ProjectReasonFromRow renders the reason lease.
func ProjectReasonFromRow(row *ProjectRow) *ProjectReason {
	if row.ReasonWorker == nil {
		return nil
	}
	return &ProjectReason{
		Worker: *row.ReasonWorker, Trigger: row.ReasonTrigger,
		StartedAt: row.ReasonStartedAt, LastHeartbeatAt: row.ReasonLastHeartbeat,
	}
}

// ProjectMetaFromRow renders the project header.
func ProjectMetaFromRow(row *ProjectRow) ProjectMeta {
	return ProjectMeta{
		ID: row.ID, Title: row.Title, Status: row.Status, BootstrapEnabled: row.BootstrapEnabled,
		MaxSteps: row.MaxSteps, CreatedAt: row.CreatedAt, Reason: ProjectReasonFromRow(row),
	}
}

// ClearProjectReason drops the reason lease.
func ClearProjectReason(ctx context.Context, conn *sql.Conn, projectID string) error {
	_, err := conn.ExecContext(ctx, `UPDATE projects
		SET reason_worker = NULL, reason_trigger = NULL, reason_started_at = NULL, reason_last_heartbeat_at = NULL
		WHERE id = ?`, projectID)
	return err
}

func countIntents(ctx context.Context, conn *sql.Conn, projectID string) (declared, open int, err error) {
	err = conn.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(CASE WHEN to_fact_id IS NULL THEN 1 ELSE 0 END), 0)
		FROM intents WHERE project_id = ?`, projectID).Scan(&declared, &open)
	return declared, open, err
}

// StepBudgetExhausted reports a spent intent budget.
func StepBudgetExhausted(ctx context.Context, conn *sql.Conn, projectID string) (bool, error) {
	var maxSteps sql.NullInt64
	if err := conn.QueryRowContext(ctx, "SELECT max_steps FROM projects WHERE id = ?", projectID).Scan(&maxSteps); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	if !maxSteps.Valid {
		return false, nil
	}
	declared, _, err := countIntents(ctx, conn, projectID)
	if err != nil {
		return false, err
	}
	return declared >= int(maxSteps.Int64), nil
}

// EnforceStepLimit blocks a drained project whose budget is spent.
func EnforceStepLimit(ctx context.Context, conn *sql.Conn, projectID string) (bool, error) {
	var status string
	var maxSteps sql.NullInt64
	if err := conn.QueryRowContext(ctx, "SELECT status, max_steps FROM projects WHERE id = ?", projectID).Scan(&status, &maxSteps); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	if status != "active" || !maxSteps.Valid {
		return false, nil
	}
	declared, open, err := countIntents(ctx, conn, projectID)
	if err != nil {
		return false, err
	}
	if declared < int(maxSteps.Int64) || open > 0 {
		return false, nil
	}
	if _, err := conn.ExecContext(ctx, "UPDATE projects SET status = 'blocked' WHERE id = ?", projectID); err != nil {
		return false, err
	}
	if err := ClearProjectReason(ctx, conn, projectID); err != nil {
		return false, err
	}
	return true, nil
}
