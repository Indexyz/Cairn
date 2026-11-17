package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func budgetProject(t *testing.T, harness *testApp, maxSteps *int, title string) string {
	t.Helper()
	body := map[string]any{"title": title, "origin": "start", "goal": "finish", "bootstrap_enabled": false}
	if maxSteps != nil {
		body["max_steps"] = *maxSteps
	}
	status, payload := harness.json(t, http.MethodPost, "/projects", body)
	if status != http.StatusCreated {
		t.Fatalf("create status = %d payload=%v", status, payload)
	}
	return payload["project"].(map[string]any)["id"].(string)
}

func stepIntent(t *testing.T, harness *testApp, projectID, description string) int {
	t.Helper()
	status, _ := harness.json(t, http.MethodPost, "/projects/"+projectID+"/intents",
		map[string]any{"from": []string{"origin"}, "description": description, "creator": "reasoner", "worker": nil})
	return status
}

func concludeStep(t *testing.T, harness *testApp, projectID, intentID string) {
	t.Helper()
	status, payload := harness.json(t, http.MethodPost, "/projects/"+projectID+"/intents/"+intentID+"/conclude",
		map[string]any{"worker": "explorer", "description": "fact " + intentID})
	if status != http.StatusOK {
		t.Fatalf("conclude status = %d payload=%v", status, payload)
	}
}

func projectStatus(t *testing.T, harness *testApp, projectID string) string {
	t.Helper()
	_, detail := harness.json(t, http.MethodGet, "/projects/"+projectID, nil)
	return detail["project"].(map[string]any)["status"].(string)
}

func TestMaxStepsIsOptionalAndRoundTrips(t *testing.T) {
	harness := newTestApp(t)
	_, unlimited := harness.json(t, http.MethodPost, "/projects", map[string]any{"title": "free", "origin": "start", "goal": "finish"})
	if unlimited["project"].(map[string]any)["max_steps"] != nil {
		t.Fatalf("max_steps = %v", unlimited["project"].(map[string]any)["max_steps"])
	}

	projectID := budgetProject(t, harness, intPtr(3), "budgeted")
	_, detail := harness.json(t, http.MethodGet, "/projects/"+projectID, nil)
	if detail["project"].(map[string]any)["max_steps"] != float64(3) {
		t.Fatalf("detail = %v", detail["project"])
	}
	_, raw := harness.request(t, http.MethodGet, "/projects", nil)
	var summaries []map[string]any
	_ = json.Unmarshal(raw, &summaries)
	found := false
	for _, summary := range summaries {
		if summary["id"] == projectID && summary["max_steps"] == float64(3) {
			found = true
		}
	}
	if !found {
		t.Fatalf("summary missing max_steps: %s", raw)
	}
	_, exported := harness.request(t, http.MethodGet, "/projects/"+projectID+"/export?format=yaml", nil)
	if !containsString(string(exported), "max_steps: 3") {
		t.Fatalf("export = %s", exported)
	}
}

func TestMaxStepsRejectsNonPositiveValues(t *testing.T) {
	harness := newTestApp(t)
	for _, value := range []int{0, -2} {
		status, _ := harness.json(t, http.MethodPost, "/projects", map[string]any{
			"title": "bad", "origin": "start", "goal": "finish", "max_steps": value,
		})
		if status != http.StatusUnprocessableEntity {
			t.Fatalf("max_steps=%d status = %d", value, status)
		}
	}
}

func TestCapRefusesNewIntentsAndBlocksOnceTheQueueDrains(t *testing.T) {
	harness := newTestApp(t)
	projectID := budgetProject(t, harness, intPtr(2), "budgeted")

	if status := stepIntent(t, harness, projectID, "first"); status != http.StatusCreated {
		t.Fatalf("first status = %d", status)
	}
	if status := stepIntent(t, harness, projectID, "second"); status != http.StatusCreated {
		t.Fatalf("second status = %d", status)
	}
	status, payload := harness.json(t, http.MethodPost, "/projects/"+projectID+"/intents",
		map[string]any{"from": []string{"origin"}, "description": "third", "creator": "reasoner", "worker": nil})
	if status != http.StatusForbidden {
		t.Fatalf("third status = %d", status)
	}
	if detail, _ := payload["detail"].(string); !containsString(detail, "step limit") {
		t.Fatalf("detail = %v", payload["detail"])
	}
	if projectStatus(t, harness, projectID) != "active" {
		t.Fatal("open intent should keep the project active")
	}
	concludeStep(t, harness, projectID, "i001")
	if projectStatus(t, harness, projectID) != "active" {
		t.Fatal("one open intent should keep the project active")
	}
	concludeStep(t, harness, projectID, "i002")
	if projectStatus(t, harness, projectID) != "blocked" {
		t.Fatalf("status = %q", projectStatus(t, harness, projectID))
	}
	if status := stepIntent(t, harness, projectID, "fourth"); status != http.StatusForbidden {
		t.Fatalf("fourth status = %d", status)
	}
	if status, _ := harness.json(t, http.MethodPost, "/projects/"+projectID+"/hints",
		map[string]any{"content": "note", "creator": "human"}); status != http.StatusCreated {
		t.Fatalf("hint status = %d", status)
	}
}

func TestBlockedProjectStillCompletesAndNotifies(t *testing.T) {
	harness := newTestApp(t)
	harness.json(t, http.MethodPost, "/notifications", map[string]any{
		"name": "blocked", "kind": "webhook", "enabled": true,
		"events": []string{"project_blocked"}, "config": map[string]string{"url": "https://x.test/hook"},
	})
	projectID := budgetProject(t, harness, intPtr(1), "budgeted")
	stepIntent(t, harness, projectID, "only")
	concludeStep(t, harness, projectID, "i001")
	if projectStatus(t, harness, projectID) != "blocked" {
		t.Fatal("project should be blocked")
	}
	harness.waitDeliveries()
	calls := harness.recorder.snapshot()
	if len(calls) != 1 || calls[0].Body["event"] != "project_blocked" {
		t.Fatalf("calls = %v", calls)
	}

	status, _ := harness.json(t, http.MethodPost, "/projects/"+projectID+"/complete",
		map[string]any{"from": []string{"f001"}, "description": "solved", "worker": "reasoner"})
	if status != http.StatusOK || projectStatus(t, harness, projectID) != "completed" {
		t.Fatalf("complete status = %d", status)
	}
}

func TestResumingSpentProjectBlocksAgainOnNextIntent(t *testing.T) {
	harness := newTestApp(t)
	harness.json(t, http.MethodPost, "/notifications", map[string]any{
		"name": "blocked", "kind": "webhook", "enabled": true,
		"events": []string{"project_blocked"}, "config": map[string]string{"url": "https://x.test/hook"},
	})
	projectID := budgetProject(t, harness, intPtr(1), "budgeted")
	stepIntent(t, harness, projectID, "only")
	concludeStep(t, harness, projectID, "i001")

	status, _ := harness.json(t, http.MethodPut, "/projects/"+projectID+"/status", map[string]any{"status": "active"})
	if status != http.StatusOK {
		t.Fatalf("resume status = %d", status)
	}
	if status := stepIntent(t, harness, projectID, "over budget"); status != http.StatusForbidden {
		t.Fatalf("intent status = %d", status)
	}
	if projectStatus(t, harness, projectID) != "blocked" {
		t.Fatal("project should be blocked again")
	}
	harness.waitDeliveries()
	if calls := harness.recorder.snapshot(); len(calls) != 2 {
		t.Fatalf("calls = %v", calls)
	}
	if status, _ := harness.json(t, http.MethodPut, "/projects/"+projectID+"/status", map[string]any{"status": "stopped"}); status != http.StatusOK {
		t.Fatalf("stop status = %d", status)
	}
}

func TestRaisingOrClearingMaxStepsUnblocks(t *testing.T) {
	harness := newTestApp(t)
	projectID := budgetProject(t, harness, intPtr(1), "budgeted")
	stepIntent(t, harness, projectID, "only")
	concludeStep(t, harness, projectID, "i001")

	status, raised := harness.json(t, http.MethodPut, "/projects/"+projectID+"/max_steps", map[string]any{"max_steps": 5})
	if status != http.StatusOK || raised["status"] != "active" {
		t.Fatalf("raised status=%d payload=%v", status, raised)
	}
	if status := stepIntent(t, harness, projectID, "again"); status != http.StatusCreated {
		t.Fatalf("intent status = %d", status)
	}

	_, lowered := harness.json(t, http.MethodPut, "/projects/"+projectID+"/max_steps", map[string]any{"max_steps": 2})
	if lowered["status"] != "active" {
		t.Fatalf("lowered = %v", lowered)
	}
	concludeStep(t, harness, projectID, "i002")
	if projectStatus(t, harness, projectID) != "blocked" {
		t.Fatal("project should be blocked")
	}

	_, cleared := harness.json(t, http.MethodPut, "/projects/"+projectID+"/max_steps", map[string]any{"max_steps": nil})
	if cleared["status"] != "active" || cleared["max_steps"] != nil {
		t.Fatalf("cleared = %v", cleared)
	}
	if status := stepIntent(t, harness, projectID, "unbounded"); status != http.StatusCreated {
		t.Fatalf("intent status = %d", status)
	}
}

func TestLoweringCapDoesNotBlockWhileIntentsOpen(t *testing.T) {
	harness := newTestApp(t)
	projectID := budgetProject(t, harness, nil, "free")
	stepIntent(t, harness, projectID, "first")
	stepIntent(t, harness, projectID, "second")

	_, payload := harness.json(t, http.MethodPut, "/projects/"+projectID+"/max_steps", map[string]any{"max_steps": 1})
	if payload["status"] != "active" {
		t.Fatalf("payload = %v", payload)
	}
	concludeStep(t, harness, projectID, "i001")
	concludeStep(t, harness, projectID, "i002")
	if projectStatus(t, harness, projectID) != "blocked" {
		t.Fatal("project should be blocked")
	}
}

func TestCompletedProjectRejectsMaxStepsChanges(t *testing.T) {
	harness := newTestApp(t)
	projectID := budgetProject(t, harness, nil, "free")
	harness.json(t, http.MethodPost, "/projects/"+projectID+"/complete",
		map[string]any{"from": []string{"origin"}, "description": "solved", "worker": "reasoner"})
	if status, _ := harness.json(t, http.MethodPut, "/projects/"+projectID+"/max_steps", map[string]any{"max_steps": 4}); status != http.StatusConflict {
		t.Fatalf("status = %d", status)
	}
}

func TestForkCopiesMaxSteps(t *testing.T) {
	harness := newTestApp(t)
	projectID := budgetProject(t, harness, intPtr(7), "budgeted")
	status, forked := harness.json(t, http.MethodPost, "/projects/"+projectID+"/fork", map[string]any{})
	if status != http.StatusCreated || forked["project"].(map[string]any)["max_steps"] != float64(7) {
		t.Fatalf("status=%d payload=%v", status, forked)
	}
}

func intPtr(value int) *int { return &value }

func containsString(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for index := 0; index+len(needle) <= len(haystack); index++ {
			if haystack[index:index+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
