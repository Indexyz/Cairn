package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func modelPayload(name string) map[string]any {
	return map[string]any{
		"name":        name,
		"type":        "claudecode",
		"task_types":  []string{"bootstrap", "reason", "explore"},
		"max_running": 2,
		"priority":    0,
		"env": map[string]string{
			"ANTHROPIC_MODEL":      "deepseek-v4-pro",
			"ANTHROPIC_BASE_URL":   "https://api.example.com/anthropic",
			"ANTHROPIC_AUTH_TOKEN": "token",
		},
	}
}

func withOverrides(base map[string]any, overrides map[string]any) map[string]any {
	for key, value := range overrides {
		base[key] = value
	}
	return base
}

func mockModel(name, behavior string) map[string]any {
	return withOverrides(modelPayload(name), map[string]any{
		"type": "mock", "env": map[string]string{"MOCK_HEALTHCHECK": behavior},
	})
}

func TestModelCRUDRoundTrip(t *testing.T) {
	harness := newTestApp(t)
	if status, _ := harness.json(t, http.MethodPost, "/models", modelPayload("claude-1")); status != http.StatusCreated {
		t.Fatalf("create status = %d", status)
	}
	if status, _ := harness.json(t, http.MethodPost, "/models", modelPayload("claude-1")); status != http.StatusConflict {
		t.Fatalf("duplicate status = %d", status)
	}
	status, raw := harness.request(t, http.MethodGet, "/models", nil)
	if status != http.StatusOK {
		t.Fatalf("list status = %d", status)
	}
	var models []map[string]any
	if err := json.Unmarshal(raw, &models); err != nil || len(models) != 1 || models[0]["name"] != "claude-1" {
		t.Fatalf("models = %s err=%v", raw, err)
	}

	if status, renamed := harness.json(t, http.MethodPut, "/models/claude-1", modelPayload("claude-2")); status != http.StatusOK || renamed["name"] != "claude-2" {
		t.Fatalf("rename status=%d payload=%v", status, renamed)
	}
	if status, _ := harness.json(t, http.MethodPost, "/models", modelPayload("claude-1")); status != http.StatusCreated {
		t.Fatalf("recreate status = %d", status)
	}
	if status, _ := harness.json(t, http.MethodPut, "/models/claude-2", modelPayload("claude-1")); status != http.StatusConflict {
		t.Fatalf("conflict status = %d", status)
	}
	if status, _ := harness.request(t, http.MethodDelete, "/models/claude-2", nil); status != http.StatusNoContent {
		t.Fatalf("delete status = %d", status)
	}
	if status, _ := harness.request(t, http.MethodDelete, "/models/claude-2", nil); status != http.StatusNotFound {
		t.Fatalf("second delete status = %d", status)
	}
}

func TestReplaceModelsIsAFullSnapshot(t *testing.T) {
	harness := newTestApp(t)
	harness.json(t, http.MethodPost, "/models", modelPayload("keep"))
	harness.json(t, http.MethodPost, "/models", modelPayload("drop"))

	status, raw := harness.request(t, http.MethodPut, "/models", map[string]any{
		"models": []any{withOverrides(modelPayload("keep"), map[string]any{"priority": 3}), modelPayload("added")},
	})
	if status != http.StatusOK {
		t.Fatalf("replace status = %d body=%s", status, raw)
	}
	var models []map[string]any
	_ = json.Unmarshal(raw, &models)
	if len(models) != 2 || models[0]["name"] != "added" || models[1]["priority"] != float64(3) {
		t.Fatalf("models = %v", models)
	}

	status, raw = harness.request(t, http.MethodPut, "/models", map[string]any{"models": []any{}})
	if status != http.StatusOK || string(raw) != "[]\n" {
		t.Fatalf("clear status=%d body=%s", status, raw)
	}
}

func TestReplaceModelsRejectsDuplicateNames(t *testing.T) {
	harness := newTestApp(t)
	status, _ := harness.json(t, http.MethodPut, "/models", map[string]any{
		"models": []any{modelPayload("dup"), modelPayload("dup")},
	})
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d", status)
	}
}

func TestModelValidationRejectsBadPayloads(t *testing.T) {
	harness := newTestApp(t)
	cases := []map[string]any{
		modelPayload("a1"),
	}
	_ = cases
	if status, _ := harness.json(t, http.MethodPost, "/models", withOverrides(modelPayload("a"), map[string]any{"type": "nope"})); status != http.StatusUnprocessableEntity {
		t.Fatalf("type status = %d", status)
	}
	if status, _ := harness.json(t, http.MethodPost, "/models", withOverrides(modelPayload("b"), map[string]any{"task_types": []string{}})); status != http.StatusUnprocessableEntity {
		t.Fatalf("task types status = %d", status)
	}
	if status, _ := harness.json(t, http.MethodPost, "/models", withOverrides(modelPayload("c"), map[string]any{"task_types": []string{"reason", "reason"}})); status != http.StatusUnprocessableEntity {
		t.Fatalf("duplicate task status = %d", status)
	}
	if status, _ := harness.json(t, http.MethodPost, "/models", withOverrides(modelPayload("d"), map[string]any{"max_running": 0})); status != http.StatusUnprocessableEntity {
		t.Fatalf("max running status = %d", status)
	}
	if status, _ := harness.json(t, http.MethodPost, "/models", withOverrides(modelPayload("e"), map[string]any{"extra": 1})); status != http.StatusUnprocessableEntity {
		t.Fatalf("extra key status = %d", status)
	}

	badMock := withOverrides(modelPayload("mock-1"), map[string]any{
		"type": "mock", "env": map[string]string{"MOCK_REASON": `{"delay":[0,1],"outcomes":{"intent":0.5}}`},
	})
	if status, _ := harness.json(t, http.MethodPost, "/models", badMock); status != http.StatusUnprocessableEntity {
		t.Fatalf("bad mock status = %d", status)
	}
	goodMock := withOverrides(modelPayload("mock-2"), map[string]any{
		"type": "mock", "env": map[string]string{"MOCK_REASON": `{"delay":[0,1],"outcomes":{"intent":1.0}}`},
	})
	if status, _ := harness.json(t, http.MethodPost, "/models", goodMock); status != http.StatusCreated {
		t.Fatalf("good mock status = %d", status)
	}
}

func TestModelTestReportsHealthyAndUnhealthy(t *testing.T) {
	harness := newTestApp(t)
	harness.json(t, http.MethodPost, "/models", mockModel("ok", `{"delay":[0,0],"outcomes":{"ok":1.0,"fail":0.0}}`))
	status, payload := harness.json(t, http.MethodPost, "/models/ok/test", nil)
	if status != http.StatusOK {
		t.Fatalf("status = %d", status)
	}
	if payload["ok"] != true || payload["status"] != float64(200) || payload["endpoint"] != "mock in-process healthcheck" {
		t.Fatalf("payload = %v", payload)
	}

	harness.json(t, http.MethodPost, "/models", mockModel("bad", `{"delay":[0,0],"outcomes":{"ok":0.0,"fail":1.0}}`))
	_, unhealthy := harness.json(t, http.MethodPost, "/models/bad/test", nil)
	if unhealthy["ok"] != false || unhealthy["status"] != float64(503) || unhealthy["detail"] != "mock healthcheck fail" {
		t.Fatalf("unhealthy = %v", unhealthy)
	}
}

func TestModelTestUnknownModelAndMissingEnv(t *testing.T) {
	harness := newTestApp(t)
	if status, _ := harness.json(t, http.MethodPost, "/models/nope/test", nil); status != http.StatusNotFound {
		t.Fatalf("unknown status = %d", status)
	}
	harness.json(t, http.MethodPost, "/models", withOverrides(modelPayload("empty-claude"), map[string]any{"env": map[string]string{}}))
	status, payload := harness.json(t, http.MethodPost, "/models/empty-claude/test", nil)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("missing env status = %d", status)
	}
	if detail, _ := payload["detail"].(string); detail != "model empty-claude is missing env key ANTHROPIC_BASE_URL" {
		t.Fatalf("detail = %v", payload["detail"])
	}
}

func claimedIntent(t *testing.T, harness *testApp, worker string) string {
	t.Helper()
	status, payload := harness.json(t, http.MethodPost, "/projects", map[string]any{"title": "t", "origin": "o", "goal": "g"})
	if status != http.StatusCreated {
		t.Fatalf("project status = %d", status)
	}
	projectID := payload["project"].(map[string]any)["id"].(string)
	if status, _ := harness.json(t, http.MethodPost, "/projects/"+projectID+"/intents",
		map[string]any{"from": []string{"origin"}, "description": "busy", "creator": worker, "worker": worker}); status != http.StatusCreated {
		t.Fatalf("intent status = %d", status)
	}
	return projectID
}

func TestModelUsageIsDerivedFromLiveLeases(t *testing.T) {
	harness := newTestApp(t)
	harness.json(t, http.MethodPost, "/models", modelPayload("busy"))
	harness.json(t, http.MethodPost, "/models", modelPayload("idle"))
	claimedIntent(t, harness, "busy@inst1")

	_, raw := harness.request(t, http.MethodGet, "/models", nil)
	var models []map[string]any
	_ = json.Unmarshal(raw, &models)
	usage := map[string]int{}
	for _, model := range models {
		usage[model["name"].(string)] = int(model["used"].(float64))
	}
	if usage["busy"] != 1 || usage["idle"] != 0 {
		t.Fatalf("usage = %v", usage)
	}
}

func TestModelUsageDropsWithLeaseAndModel(t *testing.T) {
	harness := newTestApp(t)
	harness.json(t, http.MethodPost, "/models", modelPayload("gone"))
	claimedIntent(t, harness, "gone@inst1")

	_, raw := harness.request(t, http.MethodGet, "/models", nil)
	var models []map[string]any
	_ = json.Unmarshal(raw, &models)
	if models[0]["used"] != float64(1) {
		t.Fatalf("used = %v", models[0]["used"])
	}
	harness.sqlExec(t, "UPDATE intents SET last_heartbeat_at = '2000-01-01T00:00:00Z'")
	_, raw = harness.request(t, http.MethodGet, "/models", nil)
	_ = json.Unmarshal(raw, &models)
	if models[0]["used"] != float64(0) {
		t.Fatalf("used after expiry = %v", models[0]["used"])
	}
	harness.request(t, http.MethodDelete, "/models/gone", nil)
	_, raw = harness.request(t, http.MethodGet, "/models", nil)
	if string(raw) != "[]\n" {
		t.Fatalf("models = %s", raw)
	}
}

func TestLoadSnapshotCountsTasksProjectsAndWorkers(t *testing.T) {
	harness := newTestApp(t)
	harness.json(t, http.MethodPost, "/models", modelPayload("busy"))
	claimedIntent(t, harness, "busy@inst1")

	_, load := harness.json(t, http.MethodGet, "/load", nil)
	if load["running_tasks"] != float64(1) || load["running_projects"] != float64(1) {
		t.Fatalf("load = %v", load)
	}
	if load["workers"].(map[string]any)["busy"] != float64(1) {
		t.Fatalf("workers = %v", load["workers"])
	}
}

func TestReplaceModelsRequiresModelsKeyAndValidTimeout(t *testing.T) {
	harness := newTestApp(t)
	if status, _ := harness.json(t, http.MethodPut, "/models", map[string]any{}); status != http.StatusUnprocessableEntity {
		t.Fatalf("missing models status = %d", status)
	}
	harness.json(t, http.MethodPost, "/models", mockModel("m", `{"delay":[0,0],"outcomes":{"ok":1.0,"fail":0.0}}`))
	for _, timeout := range []string{"abc", "0", "61"} {
		if status, _ := harness.request(t, http.MethodPost, "/models/m/test?timeout="+timeout, nil); status != http.StatusUnprocessableEntity {
			t.Fatalf("timeout %q status = %d", timeout, status)
		}
	}
}
