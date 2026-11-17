package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Indexyz/Cairn/server/internal/notify"
	"github.com/Indexyz/Cairn/server/internal/store"
)

type recordedCall struct {
	URL  string
	Body map[string]any
}

type recorder struct {
	mu    sync.Mutex
	calls []recordedCall
}

func (recorder *recorder) RoundTrip(request *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(request.Body)
	record := recordedCall{URL: request.URL.String()}
	if len(body) > 0 {
		_ = json.Unmarshal(body, &record.Body)
	}
	recorder.mu.Lock()
	recorder.calls = append(recorder.calls, record)
	recorder.mu.Unlock()
	return &http.Response{
		StatusCode: http.StatusNoContent,
		Body:       io.NopCloser(bytes.NewReader(nil)),
		Header:     http.Header{},
	}, nil
}

func (recorder *recorder) snapshot() []recordedCall {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return append([]recordedCall(nil), recorder.calls...)
}

type testApp struct {
	app      *App
	server   *httptest.Server
	recorder *recorder
	dbPath   string
}

func newTestApp(t *testing.T) *testApp {
	t.Helper()
	database, err := store.Open(t.TempDir() + "/cairn.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	rec := &recorder{}
	app := &App{
		DB:         database,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		Deliverer:  &notify.Deliverer{Client: &http.Client{Transport: rec}},
	}
	server := httptest.NewServer(app.Handler())
	t.Cleanup(server.Close)
	return &testApp{app: app, server: server, recorder: rec}
}

func (harness *testApp) request(t *testing.T, method, path string, body any) (int, []byte) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequest(method, harness.server.URL+path, reader)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if body != nil {
		request.Header.Set("content-type", "application/json")
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(response.Body)
	return response.StatusCode, raw
}

func (harness *testApp) json(t *testing.T, method, path string, body any) (int, map[string]any) {
	t.Helper()
	status, raw := harness.request(t, method, path, body)
	payload := map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Fatalf("decode %s %s: %v (%s)", method, path, err, raw)
		}
	}
	return status, payload
}

func (harness *testApp) waitDeliveries() {
	harness.app.Deliverer.Wait()
}

func (harness *testApp) sqlExec(t *testing.T, statement string, args ...any) {
	t.Helper()
	if err := harness.app.DB.WithTx(t.Context(), func(conn *sql.Conn) error {
		_, err := conn.ExecContext(t.Context(), statement, args...)
		return err
	}); err != nil {
		t.Fatalf("exec %q: %v", statement, err)
	}
}

func (harness *testApp) createProject(t *testing.T) string {
	t.Helper()
	status, payload := harness.json(t, http.MethodPost, "/projects", map[string]any{
		"title": "test", "origin": "starting point", "goal": "finish",
		"hints": []map[string]string{{"content": "initial clue", "creator": "human"}},
	})
	if status != http.StatusCreated {
		t.Fatalf("create project status %d: %v", status, payload)
	}
	project := payload["project"].(map[string]any)
	if project["bootstrap_enabled"] != true {
		t.Fatalf("bootstrap enabled = %v", project["bootstrap_enabled"])
	}
	return project["id"].(string)
}

func TestProjectWorkflowCreateConcludeCompleteAndReopen(t *testing.T) {
	harness := newTestApp(t)
	projectID := harness.createProject(t)

	status, intent := harness.json(t, http.MethodPost, "/projects/"+projectID+"/intents",
		map[string]any{"from": []string{"origin"}, "description": "investigate", "creator": "reasoner", "worker": nil})
	if status != http.StatusCreated || intent["id"] != "i001" {
		t.Fatalf("intent status=%d payload=%v", status, intent)
	}

	status, heartbeat := harness.json(t, http.MethodPost, "/projects/"+projectID+"/intents/i001/heartbeat",
		map[string]any{"worker": "explorer"})
	if status != http.StatusOK || heartbeat["worker"] != "explorer" {
		t.Fatalf("heartbeat status=%d payload=%v", status, heartbeat)
	}

	status, concluded := harness.json(t, http.MethodPost, "/projects/"+projectID+"/intents/i001/conclude",
		map[string]any{"worker": "explorer", "description": "new fact"})
	if status != http.StatusOK {
		t.Fatalf("conclude status=%d payload=%v", status, concluded)
	}
	fact := concluded["fact"].(map[string]any)
	if fact["id"] != "f001" || fact["description"] != "new fact" {
		t.Fatalf("fact = %v", fact)
	}

	status, completed := harness.json(t, http.MethodPost, "/projects/"+projectID+"/complete",
		map[string]any{"from": []string{"f001"}, "description": "solved", "worker": "reasoner"})
	if status != http.StatusOK || completed["to"] != "goal" {
		t.Fatalf("complete status=%d payload=%v", status, completed)
	}

	status, reopened := harness.json(t, http.MethodPost, "/projects/"+projectID+"/reopen",
		map[string]any{"description": "human correction", "creator": "human"})
	if status != http.StatusOK {
		t.Fatalf("reopen status=%d payload=%v", status, reopened)
	}
	if reopened["project"].(map[string]any)["status"] != "active" {
		t.Fatalf("project = %v", reopened["project"])
	}
	if reopened["fact"].(map[string]any)["id"] != "f002" {
		t.Fatalf("fact = %v", reopened["fact"])
	}
	resent := reopened["intent"].(map[string]any)
	if resent["to"] != "f002" || resent["from"].([]any)[0] != "f001" {
		t.Fatalf("intent = %v", resent)
	}
}

func TestStoppingProjectReleasesClaimsAndReasonButKeepsHintsWritable(t *testing.T) {
	harness := newTestApp(t)
	projectID := harness.createProject(t)
	harness.json(t, http.MethodPost, "/projects/"+projectID+"/intents",
		map[string]any{"from": []string{"origin"}, "description": "work", "creator": "worker-a", "worker": "worker-a"})
	harness.json(t, http.MethodPost, "/projects/"+projectID+"/reason/claim",
		map[string]any{"worker": "worker-b", "trigger": "facts:2->3"})

	status, payload := harness.json(t, http.MethodPut, "/projects/"+projectID+"/status", map[string]any{"status": "stopped"})
	if status != http.StatusOK || payload["reason"] != nil {
		t.Fatalf("status=%d payload=%v", status, payload)
	}
	_, detail := harness.json(t, http.MethodGet, "/projects/"+projectID, nil)
	intents := detail["intents"].([]any)
	if intents[0].(map[string]any)["worker"] != nil {
		t.Fatalf("intent worker = %v", intents[0])
	}
	if status, _ := harness.json(t, http.MethodPost, "/projects/"+projectID+"/hints",
		map[string]any{"content": "manual note", "creator": "human"}); status != http.StatusCreated {
		t.Fatalf("hint status = %d", status)
	}
	if status, _ := harness.json(t, http.MethodPost, "/projects/"+projectID+"/intents",
		map[string]any{"from": []string{"origin"}, "description": "blocked", "creator": "reasoner", "worker": nil}); status != http.StatusForbidden {
		t.Fatalf("intent status = %d", status)
	}
}

func TestIntentCreationRejectsGoalSourceAndMismatchedInitialWorker(t *testing.T) {
	harness := newTestApp(t)
	projectID := harness.createProject(t)

	if status, _ := harness.json(t, http.MethodPost, "/projects/"+projectID+"/intents",
		map[string]any{"from": []string{"goal"}, "description": "invalid", "creator": "reasoner", "worker": nil}); status != http.StatusBadRequest {
		t.Fatalf("goal source status = %d", status)
	}
	if status, _ := harness.json(t, http.MethodPost, "/projects/"+projectID+"/intents",
		map[string]any{"from": []string{"origin"}, "description": "invalid", "creator": "reasoner", "worker": "explorer"}); status != http.StatusBadRequest {
		t.Fatalf("worker mismatch status = %d", status)
	}
}

func TestSettingsAndExportShareTheDatabase(t *testing.T) {
	harness := newTestApp(t)
	projectID := harness.createProject(t)

	if status, payload := harness.json(t, http.MethodPut, "/settings",
		map[string]any{"intent_timeout": 30, "reason_timeout": 45}); status != http.StatusOK || payload["intent_timeout"] != float64(30) {
		t.Fatalf("settings status=%d payload=%v", status, payload)
	}
	if _, payload := harness.json(t, http.MethodGet, "/settings", nil); payload["reason_timeout"] != float64(45) {
		t.Fatalf("settings = %v", payload)
	}

	status, exported := harness.request(t, http.MethodGet, "/projects/"+projectID+"/export?format=yaml", nil)
	if status != http.StatusOK || !bytes.Contains(exported, []byte("origin: starting point")) || !bytes.Contains(exported, []byte("goal: finish")) {
		t.Fatalf("export status=%d body=%s", status, exported)
	}
	if status, _ := harness.request(t, http.MethodGet, "/projects/"+projectID+"/export?format=invalid", nil); status != http.StatusBadRequest {
		t.Fatalf("invalid export status = %d", status)
	}
}

func TestExpiredIntentAndReasonLeasesCanBeReclaimed(t *testing.T) {
	harness := newTestApp(t)
	projectID := harness.createProject(t)
	harness.json(t, http.MethodPost, "/projects/"+projectID+"/intents",
		map[string]any{"from": []string{"origin"}, "description": "work", "creator": "worker-a", "worker": "worker-a"})
	harness.json(t, http.MethodPost, "/projects/"+projectID+"/reason/claim",
		map[string]any{"worker": "worker-a", "trigger": "bootstrap"})
	harness.sqlExec(t, "UPDATE intents SET last_heartbeat_at = '2000-01-01T00:00:00Z' WHERE project_id = ?", projectID)
	harness.sqlExec(t, "UPDATE projects SET reason_last_heartbeat_at = '2000-01-01T00:00:00Z' WHERE id = ?", projectID)

	status, heartbeat := harness.json(t, http.MethodPost, "/projects/"+projectID+"/intents/i001/heartbeat",
		map[string]any{"worker": "worker-b"})
	if status != http.StatusOK || heartbeat["worker"] != "worker-b" {
		t.Fatalf("heartbeat status=%d payload=%v", status, heartbeat)
	}
	status, claim := harness.json(t, http.MethodPost, "/projects/"+projectID+"/reason/claim",
		map[string]any{"worker": "worker-b", "trigger": "facts:2->3"})
	if status != http.StatusOK || claim["reason"].(map[string]any)["worker"] != "worker-b" {
		t.Fatalf("claim status=%d payload=%v", status, claim)
	}
}

func TestLiveReasonLeaseRejectsCompetingWorker(t *testing.T) {
	harness := newTestApp(t)
	projectID := harness.createProject(t)
	harness.json(t, http.MethodPost, "/projects/"+projectID+"/reason/claim",
		map[string]any{"worker": "worker-a", "trigger": "bootstrap"})

	status, payload := harness.json(t, http.MethodPost, "/projects/"+projectID+"/reason/claim",
		map[string]any{"worker": "worker-b", "trigger": "facts:2->3"})
	if status != http.StatusConflict {
		t.Fatalf("status = %d", status)
	}
	if detail, _ := payload["detail"].(string); !bytes.Contains([]byte(detail), []byte("worker-a")) {
		t.Fatalf("detail = %v", payload["detail"])
	}
}

func TestProjectCreationPersistsDisabledBootstrapAndExportsIt(t *testing.T) {
	harness := newTestApp(t)
	status, payload := harness.json(t, http.MethodPost, "/projects", map[string]any{
		"title": "no bootstrap", "origin": "start", "goal": "finish", "bootstrap_enabled": false,
	})
	if status != http.StatusCreated {
		t.Fatalf("status = %d", status)
	}
	projectID := payload["project"].(map[string]any)["id"].(string)
	_, detail := harness.json(t, http.MethodGet, "/projects/"+projectID, nil)
	if detail["project"].(map[string]any)["bootstrap_enabled"] != false {
		t.Fatalf("bootstrap = %v", detail["project"])
	}
	_, exported := harness.request(t, http.MethodGet, "/projects/"+projectID+"/export?format=yaml", nil)
	if !bytes.Contains(exported, []byte("bootstrap_enabled: false")) {
		t.Fatalf("export = %s", exported)
	}
}

func TestProjectCreationRejectsInvalidBootstrapEnabled(t *testing.T) {
	harness := newTestApp(t)
	status, _ := harness.json(t, http.MethodPost, "/projects", map[string]any{
		"title": "invalid bootstrap", "origin": "start", "goal": "finish", "bootstrap_enabled": "sometimes",
	})
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d", status)
	}
}

func TestForkCopiesOriginGoalAndHints(t *testing.T) {
	harness := newTestApp(t)
	projectID := harness.createProject(t)

	status, forked := harness.json(t, http.MethodPost, "/projects/"+projectID+"/fork", map[string]any{})
	if status != http.StatusCreated {
		t.Fatalf("status = %d", status)
	}
	project := forked["project"].(map[string]any)
	if project["id"] == projectID || project["title"] != "test (fork)" || project["status"] != "active" || project["bootstrap_enabled"] != true {
		t.Fatalf("project = %v", project)
	}
	facts := forked["facts"].([]any)
	if facts[0].(map[string]any)["description"] != "starting point" || facts[1].(map[string]any)["description"] != "finish" {
		t.Fatalf("facts = %v", facts)
	}
	hints := forked["hints"].([]any)
	hint := hints[0].(map[string]any)
	if hint["content"] != "initial clue" || hint["creator"] != "human" || hint["id"] != "h001" || hint["created_at"] != project["created_at"] {
		t.Fatalf("hint = %v", hint)
	}
	if len(forked["intents"].([]any)) != 0 {
		t.Fatalf("intents = %v", forked["intents"])
	}

	_, source := harness.json(t, http.MethodGet, "/projects/"+projectID, nil)
	if source["project"].(map[string]any)["title"] != "test" {
		t.Fatalf("source title = %v", source["project"])
	}
}

func TestForkAcceptsTitleAndEmptyBody(t *testing.T) {
	harness := newTestApp(t)
	projectID := harness.createProject(t)

	_, named := harness.json(t, http.MethodPost, "/projects/"+projectID+"/fork", map[string]any{"title": "retry"})
	_, blank := harness.json(t, http.MethodPost, "/projects/"+projectID+"/fork", map[string]any{"title": "   "})
	if named["project"].(map[string]any)["title"] != "retry" {
		t.Fatalf("named = %v", named["project"])
	}
	if blank["project"].(map[string]any)["title"] != "test (fork)" {
		t.Fatalf("blank = %v", blank["project"])
	}
}

func TestForkReturns404ForUnknownProject(t *testing.T) {
	harness := newTestApp(t)
	if status, _ := harness.json(t, http.MethodPost, "/projects/proj_999/fork", map[string]any{}); status != http.StatusNotFound {
		t.Fatalf("status = %d", status)
	}
}

func TestOpenAPIAndDocsAreServed(t *testing.T) {
	harness := newTestApp(t)
	status, raw := harness.request(t, http.MethodGet, "/openapi.json", nil)
	if status != http.StatusOK || !containsString(string(raw), "\"title\":\"Cairn\"") || !containsString(string(raw), "/projects/{project}/complete") {
		t.Fatalf("openapi status=%d body=%s", status, raw)
	}
	status, raw = harness.request(t, http.MethodGet, "/docs", nil)
	if status != http.StatusOK || !containsString(string(raw), "swagger-ui") {
		t.Fatalf("docs status=%d body=%s", status, raw)
	}
	if status, _ := harness.request(t, http.MethodGet, "/nope", nil); status != http.StatusNotFound {
		t.Fatalf("unknown path status = %d", status)
	}
}
