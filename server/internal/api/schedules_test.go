package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/Indexyz/Cairn/server/internal/scheduler"
)

func scheduleBody() map[string]any {
	return map[string]any{
		"name": "nightly", "cron": "0 3 * * *", "timezone": "UTC", "enabled": true,
		"title": "nightly scan", "origin": "10.0.0.1", "goal": "get a shell",
		"bootstrap_enabled": true, "hints": []string{"start with nmap"},
	}
}

func createSchedule(t *testing.T, harness *testApp) (string, map[string]any) {
	t.Helper()
	status, payload := harness.json(t, http.MethodPost, "/schedules", scheduleBody())
	if status != http.StatusCreated {
		t.Fatalf("create schedule: %d %v", status, payload)
	}
	scheduleID, _ := payload["id"].(string)
	if scheduleID == "" {
		t.Fatalf("missing schedule id: %v", payload)
	}
	return scheduleID, payload
}

func (harness *testApp) dueSchedule(t *testing.T, scheduleID string) {
	t.Helper()
	harness.sqlExec(t, "UPDATE schedules SET next_run_at = ? WHERE id = ?",
		time.Now().Add(-time.Minute).UTC().Format("2006-01-02T15:04:05Z"), scheduleID)
}

func (harness *testApp) tick(t *testing.T) {
	t.Helper()
	runner := &scheduler.Runner{DB: harness.app.DB, Now: time.Now}
	if err := runner.Tick(t.Context()); err != nil {
		t.Fatalf("tick: %v", err)
	}
}

func (harness *testApp) scheduleRow(t *testing.T, scheduleID string) map[string]any {
	t.Helper()
	status, payload := harness.json(t, http.MethodGet, "/schedules", nil)
	if status != http.StatusOK {
		t.Fatalf("list schedules: %d", status)
	}
	schedules, _ := payload["schedules"].([]any)
	_ = schedules
	return payload
}

func scheduleField(t *testing.T, harness *testApp, scheduleID, field string) string {
	t.Helper()
	var value sql.NullString
	if err := harness.app.DB.Read(t.Context(), func(conn *sql.Conn) error {
		return conn.QueryRowContext(t.Context(), "SELECT "+field+" FROM schedules WHERE id = ?", scheduleID).Scan(&value)
	}); err != nil {
		t.Fatalf("read %s: %v", field, err)
	}
	return value.String
}

func TestCreateScheduleAnchorsNextRun(t *testing.T) {
	harness := newTestApp(t)
	scheduleID, payload := createSchedule(t, harness)
	if payload["next_run_at"] == nil {
		t.Fatalf("next_run_at missing: %v", payload)
	}
	if payload["timezone"] != "UTC" || payload["skip_count"] != float64(0) {
		t.Fatalf("unexpected payload: %v", payload)
	}
	if payload["last_status"] != nil {
		t.Fatalf("a fresh schedule has no run: %v", payload)
	}

	status, raw := harness.request(t, http.MethodGet, "/schedules", nil)
	if status != http.StatusOK {
		t.Fatalf("list: %d", status)
	}
	var schedules []map[string]any
	if err := json.Unmarshal(raw, &schedules); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(schedules) != 1 || schedules[0]["id"] != scheduleID {
		t.Fatalf("unexpected list: %v", schedules)
	}
}

func TestCreateScheduleValidation(t *testing.T) {
	harness := newTestApp(t)
	body := scheduleBody()
	body["cron"] = "every minute"
	status, payload := harness.json(t, http.MethodPost, "/schedules", body)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d %v", status, payload)
	}

	body = scheduleBody()
	body["timezone"] = "Mars/Base"
	if status, _ := harness.json(t, http.MethodPost, "/schedules", body); status != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for the timezone, got %d", status)
	}

	if status, _ := harness.json(t, http.MethodPut, "/schedules/sched_999", scheduleBody()); status != http.StatusNotFound {
		t.Fatalf("expected 404 for an unknown schedule, got %d", status)
	}
}

func TestUpdateAndDeleteSchedule(t *testing.T) {
	harness := newTestApp(t)
	scheduleID, before := createSchedule(t, harness)

	body := scheduleBody()
	body["cron"] = "*/5 * * * *"
	body["title"] = "renamed scan"
	body["enabled"] = false
	status, payload := harness.json(t, http.MethodPut, "/schedules/"+scheduleID, body)
	if status != http.StatusOK {
		t.Fatalf("update: %d %v", status, payload)
	}
	if payload["title"] != "renamed scan" || payload["enabled"] != false || payload["next_run_at"] != nil {
		t.Fatalf("unexpected update: %v", payload)
	}
	if payload["cron"] != "*/5 * * * *" {
		t.Fatalf("cron not stored: %v", payload["cron"])
	}
	if before["next_run_at"] == payload["next_run_at"] {
		t.Fatal("the anchor should have moved or cleared")
	}

	if status, _ := harness.request(t, http.MethodDelete, "/schedules/"+scheduleID, nil); status != http.StatusNoContent {
		t.Fatalf("delete: %d", status)
	}
	status, raw := harness.request(t, http.MethodGet, "/schedules", nil)
	var remaining []map[string]any
	if err := json.Unmarshal(raw, &remaining); err != nil || status != http.StatusOK {
		t.Fatalf("list after delete: %d %v", status, err)
	}
	if len(remaining) != 0 {
		t.Fatalf("schedule survived the delete: %v", remaining)
	}
	if status, _ := harness.request(t, http.MethodDelete, "/schedules/"+scheduleID, nil); status != http.StatusNotFound {
		t.Fatalf("second delete: %d", status)
	}
}

func TestRunScheduleCreatesProject(t *testing.T) {
	harness := newTestApp(t)
	harness.json(t, http.MethodPost, "/notifications", map[string]any{
		"name": "hook", "kind": "webhook", "enabled": true,
		"events": []string{"project_created"}, "config": map[string]string{"url": "https://example.test/hook"},
	})
	scheduleID, _ := createSchedule(t, harness)

	status, payload := harness.json(t, http.MethodPost, "/schedules/"+scheduleID+"/run", nil)
	if status != http.StatusOK {
		t.Fatalf("run: %d %v", status, payload)
	}
	projectID, _ := payload["project_id"].(string)
	if projectID == "" || payload["skipped"] != false {
		t.Fatalf("unexpected run result: %v", payload)
	}

	status, project := harness.json(t, http.MethodGet, "/projects/"+projectID, nil)
	if status != http.StatusOK {
		t.Fatalf("project: %d", status)
	}
	header, _ := project["project"].(map[string]any)
	if header["title"] != "nightly scan" || header["status"] != "active" {
		t.Fatalf("project not built from the template: %v", project)
	}
	if hints, _ := project["hints"].([]any); len(hints) != 1 {
		t.Fatalf("template hint missing: %v", project["hints"])
	}

	if value := scheduleField(t, harness, scheduleID, "last_status"); value != "created" {
		t.Fatalf("last_status = %v", value)
	}
	if value := scheduleField(t, harness, scheduleID, "last_project_id"); value != projectID {
		t.Fatalf("last_project_id = %v", value)
	}

	harness.waitDeliveries()
	calls := harness.recorder.snapshot()
	if len(calls) != 1 {
		t.Fatalf("expected one notification, got %d", len(calls))
	}
}

func TestTickSkipsWhilePreviousRunIsActive(t *testing.T) {
	harness := newTestApp(t)
	scheduleID, _ := createSchedule(t, harness)

	harness.dueSchedule(t, scheduleID)
	harness.tick(t)
	first := scheduleField(t, harness, scheduleID, "last_project_id")
	if first == "" {
		t.Fatal("the first tick should have created a project")
	}

	harness.dueSchedule(t, scheduleID)
	harness.tick(t)
	if value := scheduleField(t, harness, scheduleID, "last_status"); value != "skipped" {
		t.Fatalf("last_status = %v", value)
	}
	if value := scheduleField(t, harness, scheduleID, "skip_count"); value != "1" {
		t.Fatalf("skip_count = %v", value)
	}
	if value := scheduleField(t, harness, scheduleID, "last_project_id"); value != first {
		t.Fatalf("last_project_id changed to %v", value)
	}
	if value := scheduleField(t, harness, scheduleID, "next_run_at"); value == "" {
		t.Fatal("a skipped run must re-anchor the next firing")
	}

	// a drained project lets the next tick fire again
	harness.sqlExec(t, "UPDATE projects SET status = 'completed' WHERE id = ?", first)
	harness.dueSchedule(t, scheduleID)
	harness.tick(t)
	second := scheduleField(t, harness, scheduleID, "last_project_id")
	if second == first || second == "" {
		t.Fatalf("expected a second project, got %v", second)
	}
}

func TestTickIgnoresDisabledAndFutureSchedules(t *testing.T) {
	harness := newTestApp(t)
	scheduleID, _ := createSchedule(t, harness)
	body := scheduleBody()
	body["enabled"] = false
	if status, payload := harness.json(t, http.MethodPut, "/schedules/"+scheduleID, body); status != http.StatusOK {
		t.Fatalf("disable: %d %v", status, payload)
	}
	harness.sqlExec(t, "UPDATE schedules SET next_run_at = ? WHERE id = ?",
		time.Now().Add(-time.Hour).UTC().Format("2006-01-02T15:04:05Z"), scheduleID)

	harness.tick(t)
	if value := scheduleField(t, harness, scheduleID, "last_status"); value != "" {
		t.Fatalf("disabled schedule fired: %v", value)
	}

	status, raw := harness.request(t, http.MethodGet, "/projects", nil)
	var projects []map[string]any
	if err := json.Unmarshal(raw, &projects); err != nil || status != http.StatusOK {
		t.Fatalf("projects: %d %v", status, err)
	}
	if len(projects) != 0 {
		t.Fatalf("disabled schedule created projects: %v", projects)
	}
}

func TestRunScheduleRecordsFailures(t *testing.T) {
	harness := newTestApp(t)
	scheduleID, _ := createSchedule(t, harness)
	// occupy the id the next creation will claim
	harness.sqlExec(t, "INSERT INTO projects (id, title, status, created_at) VALUES ('proj_001', 'squatter', 'active', ?)",
		time.Now().UTC().Format("2006-01-02T15:04:05Z"))

	status, payload := harness.json(t, http.MethodPost, "/schedules/"+scheduleID+"/run", nil)
	if status != http.StatusInternalServerError {
		t.Fatalf("run: %d %v", status, payload)
	}
	if value := scheduleField(t, harness, scheduleID, "last_status"); value != "error" {
		t.Fatalf("last_status = %v", value)
	}
	if value := scheduleField(t, harness, scheduleID, "last_detail"); value == "" {
		t.Fatal("the failure should be recorded")
	}
}

func TestDeleteScheduleKeepsProjects(t *testing.T) {
	harness := newTestApp(t)
	scheduleID, _ := createSchedule(t, harness)
	harness.json(t, http.MethodPost, "/schedules/"+scheduleID+"/run", nil)
	projectID := scheduleField(t, harness, scheduleID, "last_project_id")

	if status, _ := harness.request(t, http.MethodDelete, "/schedules/"+scheduleID, nil); status != http.StatusNoContent {
		t.Fatalf("delete: %d", status)
	}
	if status, _ := harness.json(t, http.MethodGet, "/projects/"+projectID, nil); status != http.StatusOK {
		t.Fatalf("project vanished with the schedule: %d", status)
	}
	var scheduleIDColumn sql.NullString
	if err := harness.app.DB.Read(context.Background(), func(conn *sql.Conn) error {
		return conn.QueryRowContext(context.Background(), "SELECT schedule_id FROM projects WHERE id = ?", projectID).Scan(&scheduleIDColumn)
	}); err != nil {
		t.Fatalf("read project: %v", err)
	}
	if scheduleIDColumn.Valid {
		t.Fatalf("schedule_id should be cleared, got %v", scheduleIDColumn.String)
	}
}
