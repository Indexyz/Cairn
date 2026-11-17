package tasks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Indexyz/Cairn/dispatcher/internal/config"
	"github.com/Indexyz/Cairn/dispatcher/internal/health"
	"github.com/Indexyz/Cairn/dispatcher/internal/protocol"
	"github.com/Indexyz/Cairn/dispatcher/internal/runtime"
	"github.com/Indexyz/Cairn/dispatcher/internal/workers"
)

type fakeProcess struct {
	result runtime.ProcessResult
}

func (process *fakeProcess) Start() error { return nil }
func (process *fakeProcess) Communicate(time.Duration) runtime.ProcessResult {
	return process.result
}
func (process *fakeProcess) Kill()         {}
func (process *fakeProcess) Cancel(string) {}

type fakeBackend struct {
	results []runtime.ProcessResult
	writes  [][3]string
}

func (*fakeBackend) WorkspacePath(projectID string) string { return "/workspace/" + projectID }

func (backend *fakeBackend) EnsureRunning(projectID string) (string, error) {
	return "container-" + projectID, nil
}

func (backend *fakeBackend) BuildExecProcess(_ string, _ map[string]string, _ []string, _ int, _ int) (runtime.ExecProcess, error) {
	result := runtime.ProcessResult{}
	if len(backend.results) > 0 {
		result = backend.results[0]
		backend.results = backend.results[1:]
	}
	return &fakeProcess{result: result}, nil
}

func (backend *fakeBackend) WriteTextFile(container, path, content string) error {
	backend.writes = append(backend.writes, [3]string{container, path, content})
	return nil
}

type fakeClient struct {
	project        *protocol.ProjectDetail
	status         int
	createdIntents []string
	concluded      []string
	completed      []string
	released       []string
	releasedReason []string
}

func (client *fakeClient) GetProject(string) (*protocol.ProjectDetail, error) {
	return client.project, nil
}
func (client *fakeClient) Heartbeat(string, string, string) protocol.ApiResult {
	return protocol.ApiResult{StatusCode: 200}
}
func (client *fakeClient) ClaimReason(string, string, string) protocol.ApiResult {
	return protocol.ApiResult{StatusCode: 200}
}
func (client *fakeClient) ReasonHeartbeat(string, string) protocol.ApiResult {
	return protocol.ApiResult{StatusCode: 200}
}
func (client *fakeClient) Release(_, _, worker string) protocol.ApiResult {
	client.released = append(client.released, worker)
	return protocol.ApiResult{StatusCode: 200}
}
func (client *fakeClient) ReleaseReason(_, worker string) protocol.ApiResult {
	client.releasedReason = append(client.releasedReason, worker)
	return protocol.ApiResult{StatusCode: 200}
}
func (client *fakeClient) Conclude(_, _, _, description string) protocol.ApiResult {
	client.concluded = append(client.concluded, description)
	body, _ := json.Marshal(map[string]any{"fact": map[string]any{"id": "f002"}})
	return protocol.ApiResult{StatusCode: 200, Data: json.RawMessage(body)}
}
func (client *fakeClient) Complete(_ string, from []string, description, _ string) protocol.ApiResult {
	client.completed = append(client.completed, description+" <- "+strings.Join(from, ","))
	return protocol.ApiResult{StatusCode: client.status}
}
func (client *fakeClient) CreateIntent(_ string, _ []string, description, creator string) protocol.ApiResult {
	client.createdIntents = append(client.createdIntents, description+" by "+creator)
	return protocol.ApiResult{StatusCode: 200}
}

type fakeDriver struct {
	workers.SeedSessionDriver
	health         health.Result
	execArgv       []string
	concludePrompt []string
	executePrompt  []string
}

func (driver *fakeDriver) TypeName() string { return "fake" }
func (driver *fakeDriver) CheckHealth(*config.WorkerConfig, time.Duration) health.Result {
	return driver.health
}
func (driver *fakeDriver) BuildExecute(_ *config.WorkerConfig, prompt string, session *string) workers.DriverResult {
	driver.executePrompt = append(driver.executePrompt, prompt)
	return workers.DriverResult{Argv: []string{"fake"}, Session: session}
}
func (driver *fakeDriver) BuildConclude(_ *config.WorkerConfig, prompt, _ string) []string {
	driver.concludePrompt = append(driver.concludePrompt, prompt)
	return []string{"fake-conclude"}
}
func (driver *fakeDriver) LocalBinary() *string { return nil }

func testConfig() *config.DispatchConfig {
	dispatch, err := config.Parse([]byte(`
server: http://127.0.0.1:8000
runtime:
  interval: 60
  max_workers: 2
  max_running_projects: 1
  max_project_workers: 2
  healthcheck_timeout: 5
  prompt_group: default
tasks:
  bootstrap: {timeout: 10, conclude_timeout: 5}
  reason: {timeout: 10, max_intents: 3}
  explore: {timeout: 10, conclude_timeout: 5}
container: {image: test-image, network_mode: host, completed_action: stop}
workers:
  - {name: test-worker, type: mock, task_types: [bootstrap, reason, explore], max_running: 1, priority: 0}
`))
	if err != nil {
		panic(err)
	}
	return dispatch
}

func testProject() *protocol.ProjectDetail {
	empty := ""
	return &protocol.ProjectDetail{
		Project: protocol.ProjectMeta{ID: "proj_001", Title: "t", Status: "active"},
		Facts: []protocol.Fact{
			{ID: "origin", Description: "start"},
			{ID: "goal", Description: "finish"},
			{ID: "f001", Description: "known"},
		},
		Intents: []protocol.Intent{{
			ID: "i001", From: []string{"f001"}, Description: "investigate", Creator: "reasoner",
			Worker: &empty, CreatedAt: "2026-01-01T00:00:00Z",
		}},
	}
}

func useDriver(t *testing.T, driver workers.Driver) {
	t.Helper()
	previous := driverFor
	driverFor = func(config.WorkerType, config.ExecutionMode) (workers.Driver, error) { return driver, nil }
	t.Cleanup(func() { driverFor = previous })
}

func TestReasonWritesGraphSnapshotAndCreatesIntent(t *testing.T) {
	dispatch := testConfig()
	project := testProject()
	project.Intents = nil
	client := &fakeClient{project: project}
	backend := &fakeBackend{results: []runtime.ProcessResult{{
		Stdout: `{"accepted":true,"data":{"intents":[{"from":["f001"],"description":"next step"}]}}`,
	}}}
	driver := &fakeDriver{}
	useDriver(t, driver)
	graphYAML := "project:\n  title: huge\n" + strings.Repeat("x", 100000)

	outcome := RunReasonTask(dispatch, client, backend, project, graphYAML, &dispatch.Workers[0], &runtime.TaskCancellation{})

	if outcome != "success" {
		t.Fatalf("outcome = %q", outcome)
	}
	if len(client.createdIntents) != 1 || client.createdIntents[0] != "next step by test-worker" {
		t.Fatalf("created intents = %v", client.createdIntents)
	}
	if len(backend.writes) != 1 {
		t.Fatalf("writes = %v", backend.writes)
	}
	write := backend.writes[0]
	if write[0] != "container-proj_001" || !strings.Contains(write[1], "/tmp/cairn-prompts/reason_execute-") || !strings.HasSuffix(write[1], "/graph.yaml") {
		t.Fatalf("write = %v", write)
	}
	if write[2] != graphYAML {
		t.Fatal("snapshot content mismatch")
	}
	if strings.Contains(driver.executePrompt[0], graphYAML) {
		t.Fatal("graph yaml should not be inlined in the prompt")
	}
	if !strings.Contains(driver.executePrompt[0], write[1]) {
		t.Fatal("prompt should point at the snapshot path")
	}
}

func TestExplorePlainTextExitUsesConcludeFallback(t *testing.T) {
	dispatch := testConfig()
	project := testProject()
	client := &fakeClient{project: project}
	backend := &fakeBackend{results: []runtime.ProcessResult{
		{Stdout: "Need inspect files and keep working."},
		{Stdout: `{"accepted":true,"data":{"description":"confirmed fact"}}`},
	}}
	driver := &fakeDriver{}
	useDriver(t, driver)

	outcome := RunExploreTask(dispatch, client, backend, project, "facts:\n- id: f001\n", &project.Intents[0], &dispatch.Workers[0], &runtime.TaskCancellation{})

	if outcome != "success" {
		t.Fatalf("outcome = %q", outcome)
	}
	if len(client.concluded) != 1 || client.concluded[0] != "confirmed fact" {
		t.Fatalf("concluded = %v", client.concluded)
	}
	if len(backend.writes) != 2 || !strings.Contains(backend.writes[0][1], "/explore_execute-") || !strings.Contains(backend.writes[1][1], "/explore_conclude-") {
		t.Fatalf("writes = %v", backend.writes)
	}
	if len(driver.executePrompt) != 1 || len(driver.concludePrompt) != 1 {
		t.Fatalf("prompts %d/%d", len(driver.executePrompt), len(driver.concludePrompt))
	}
}

func TestExploreHealthcheckFailureReleasesClaim(t *testing.T) {
	dispatch := testConfig()
	dispatch.Runtime.WorkerHealthcheck = config.HealthcheckStartupAndTask
	project := testProject()
	client := &fakeClient{project: project}
	backend := &fakeBackend{}
	status := 401
	useDriver(t, &fakeDriver{health: health.Result{Status: &status, Detail: "unauthorized"}})

	outcome := RunExploreTask(dispatch, client, backend, project, "graph", &project.Intents[0], &dispatch.Workers[0], &runtime.TaskCancellation{})

	if outcome != "unhealthy" {
		t.Fatalf("outcome = %q", outcome)
	}
	if len(client.released) != 1 {
		t.Fatalf("released = %v", client.released)
	}
	if len(backend.writes) != 0 {
		t.Fatalf("writes = %v", backend.writes)
	}
}

func TestBootstrapCompletesWithProducedFact(t *testing.T) {
	dispatch := testConfig()
	project := testProject()
	project.Intents = nil
	client := &fakeClient{project: project, status: 200}
	backend := &fakeBackend{results: []runtime.ProcessResult{{
		Stdout: `{"accepted":true,"data":{"fact":{"description":"flag found"},"complete":{"description":"solved"}}}`,
	}}}
	useDriver(t, &fakeDriver{})
	intent := &protocol.Intent{ID: "i001", From: []string{"origin"}, Description: "bootstrap"}

	outcome := RunBootstrapTask(dispatch, client, backend, project, intent, &dispatch.Workers[0], &runtime.TaskCancellation{})

	if outcome != "success" {
		t.Fatalf("outcome = %q", outcome)
	}
	if len(client.concluded) != 1 || client.concluded[0] != "flag found" {
		t.Fatalf("concluded = %v", client.concluded)
	}
	if len(client.completed) != 1 || client.completed[0] != "solved <- f002" {
		t.Fatalf("completed = %v", client.completed)
	}
}

func TestBootstrapRejectionReleasesClaim(t *testing.T) {
	dispatch := testConfig()
	project := testProject()
	project.Intents = nil
	client := &fakeClient{project: project}
	backend := &fakeBackend{results: []runtime.ProcessResult{{Stdout: `{"accepted":false}`}}}
	useDriver(t, &fakeDriver{})
	intent := &protocol.Intent{ID: "i001", From: []string{"origin"}, Description: "bootstrap"}

	outcome := RunBootstrapTask(dispatch, client, backend, project, intent, &dispatch.Workers[0], &runtime.TaskCancellation{})

	if outcome != "rejected" {
		t.Fatalf("outcome = %q", outcome)
	}
	if len(client.released) != 1 {
		t.Fatalf("released = %v", client.released)
	}
}

func TestReasonCompletePathMarksProject(t *testing.T) {
	dispatch := testConfig()
	project := testProject()
	client := &fakeClient{project: project, status: 200}
	backend := &fakeBackend{results: []runtime.ProcessResult{{
		Stdout: `{"accepted":true,"data":{"complete":{"from":["f001"],"description":"done"}}}`,
	}}}
	useDriver(t, &fakeDriver{})

	outcome := RunReasonTask(dispatch, client, backend, project, "graph", &dispatch.Workers[0], &runtime.TaskCancellation{})

	if outcome != "success" || len(client.completed) != 1 || client.completed[0] != "done <- f001" {
		t.Fatalf("outcome=%q completed=%v", outcome, client.completed)
	}
}

func localClientProject() (*fakeClient, *protocol.ProjectDetail) {
	project := testProject()
	return &fakeClient{project: project}, project
}

func localConfig() *config.DispatchConfig {
	dispatch := testConfig()
	dispatch.Runtime.Execution = config.ExecutionLocal
	dispatch.Runtime.WorkerHealthcheck = config.HealthcheckDisabled
	dispatch.Workers[0].Type = config.WorkerClaudeCode
	return dispatch
}

func installFakeCLI(t *testing.T, name, body string) {
	t.Helper()
	binDir := t.TempDir()
	script := "#!/bin/sh\n" + body + "\n"
	path := binDir + "/" + name
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake cli: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestExploreRunsRealLocalCLIEndToEnd(t *testing.T) {
	installFakeCLI(t, "claude", `echo '{"accepted":true,"data":{"description":"local fake fact"}}'`)
	root := t.TempDir()
	GraphSnapshotRoot = root
	t.Cleanup(func() { GraphSnapshotRoot = "/tmp/cairn-prompts" })

	dispatch := localConfig()
	client, project := localClientProject()
	workspace := t.TempDir()
	backend := runtime.NewLocalBackend(&config.LocalConfig{WorkspaceRoot: &workspace})

	outcome := RunExploreTask(dispatch, client, backend, project, "facts:\n- id: f001\n", &project.Intents[0], &dispatch.Workers[0], &runtime.TaskCancellation{})

	if outcome != "success" {
		t.Fatalf("outcome = %q", outcome)
	}
	if len(client.concluded) != 1 || client.concluded[0] != "local fake fact" {
		t.Fatalf("concluded = %v", client.concluded)
	}
	found := false
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && info != nil && info.Name() == "graph.yaml" {
			found = true
		}
		return nil
	})
	if !found {
		t.Fatalf("graph snapshot not materialised under %s", root)
	}
}

func TestExploreLocalCLIRejectionReleasesIntent(t *testing.T) {
	installFakeCLI(t, "claude", `echo '{"accepted":false,"reason":"policy_refusal"}'`)
	root := t.TempDir()
	GraphSnapshotRoot = root
	t.Cleanup(func() { GraphSnapshotRoot = "/tmp/cairn-prompts" })

	dispatch := localConfig()
	client, project := localClientProject()
	workspace := t.TempDir()
	backend := runtime.NewLocalBackend(&config.LocalConfig{WorkspaceRoot: &workspace})

	outcome := RunExploreTask(dispatch, client, backend, project, "facts:\n- id: f001\n", &project.Intents[0], &dispatch.Workers[0], &runtime.TaskCancellation{})

	if outcome != "rejected" {
		t.Fatalf("outcome = %q", outcome)
	}
	if len(client.concluded) != 0 || len(client.released) != 1 {
		t.Fatalf("concluded=%v released=%v", client.concluded, client.released)
	}
}

func TestExploreWritesProjectScopedMCPConfig(t *testing.T) {
	dispatch := testConfig()
	if dispatch.Workers[0].Env == nil {
		dispatch.Workers[0].Env = map[string]string{}
	}
	payload := `{"mcpServers":{"fs":{"command":"mcp-fs"}}}`
	dispatch.Workers[0].Env["MCP_CONFIG"] = payload
	project := testProject()
	client := &fakeClient{project: project}
	backend := &fakeBackend{results: []runtime.ProcessResult{
		{Stdout: `{"accepted":true,"data":{"description":"confirmed fact"}}`},
	}}
	useDriver(t, &fakeDriver{})

	RunExploreTask(dispatch, client, backend, project, "facts: []\n", &project.Intents[0], &dispatch.Workers[0], &runtime.TaskCancellation{})

	wanted := map[string]bool{"/.mcp.json": false, "/.pi/mcp.json": false}
	for _, write := range backend.writes {
		for suffix := range wanted {
			if strings.HasSuffix(write[1], suffix) {
				wanted[suffix] = true
				if write[2] != payload {
					t.Fatalf("mcp content = %q", write[2])
				}
				if !strings.HasPrefix(write[1], "/workspace/") {
					t.Fatalf("mcp path = %q", write[1])
				}
			}
		}
	}
	for suffix, seen := range wanted {
		if !seen {
			t.Fatalf("no %s written: %v", suffix, backend.writes)
		}
	}
}
