// Package e2e runs the Go dispatcher against a real Python server.
package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Indexyz/Cairn/dispatcher/internal/scheduler"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "server", "go.mod")); err != nil {
		t.Skip("server module not available")
	}
	return root
}

var (
	buildOnce    sync.Once
	serverBinary string
	buildError   error
)

// serverExecutable builds the Go server once per test binary.
func serverExecutable(t *testing.T, root string) string {
	t.Helper()
	buildOnce.Do(func() {
		serverBinary = filepath.Join(os.TempDir(), fmt.Sprintf("cairn-server-%d", os.Getpid()))
		command := exec.Command("go", "build", "-o", serverBinary, "./cmd/cairn")
		command.Dir = filepath.Join(root, "server")
		output, err := command.CombinedOutput()
		if err != nil {
			buildError = fmt.Errorf("build server: %v: %s", err, output)
		}
	})
	if buildError != nil {
		t.Fatalf("%v", buildError)
	}
	return serverBinary
}

// ensurePython3 makes python3 resolvable for the mock worker, using the
// uv-managed interpreter when the host has no python3 on PATH.
func ensurePython3(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("python3"); err == nil {
		return
	}
	interpreter, err := exec.Command("uv", "python", "find").Output()
	if err != nil {
		t.Skipf("no python3 and uv cannot provide one: %v", err)
	}
	binDir := t.TempDir()
	shim := filepath.Join(binDir, "python3")
	script := fmt.Sprintf("#!/bin/sh\nexec %s \"$@\"\n", string(bytes.TrimSpace(interpreter)))
	if err := os.WriteFile(shim, []byte(script), 0o755); err != nil {
		t.Fatalf("write python3 shim: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

type server struct {
	baseURL string
	command *exec.Cmd
}

func startServer(t *testing.T, root string) *server {
	t.Helper()
	ensurePython3(t)
	port := freePort(t)
	database := filepath.Join(t.TempDir(), "cairn.db")
	command := exec.Command(
		serverExecutable(t, root),
		"serve", "--db-path", database, "--port", fmt.Sprint(port), "--no-access-log",
	)
	command.Dir = root
	command.Stdout = os.Stderr
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	instance := &server{baseURL: fmt.Sprintf("http://127.0.0.1:%d", port), command: command}
	t.Cleanup(func() {
		_ = command.Process.Signal(os.Interrupt)
		done := make(chan struct{})
		go func() { _ = command.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = command.Process.Kill()
		}
	})

	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		response, err := http.Get(instance.baseURL + "/settings")
		if err == nil {
			response.Body.Close()
			if response.StatusCode == 200 {
				return instance
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("server did not become ready")
	return nil
}

func (instance *server) createProject(t *testing.T) string {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"title": "integration", "origin": "start", "goal": "finish"})
	response, err := http.Post(instance.baseURL+"/projects", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	defer response.Body.Close()
	payload, _ := io.ReadAll(response.Body)
	if response.StatusCode != 201 {
		t.Fatalf("create project status %d: %s", response.StatusCode, payload)
	}
	var created struct {
		Project struct {
			ID string `json:"id"`
		} `json:"project"`
	}
	if err := json.Unmarshal(payload, &created); err != nil {
		t.Fatalf("decode project: %v", err)
	}
	return created.Project.ID
}

type projectDetail struct {
	Project struct {
		Status string `json:"status"`
	} `json:"project"`
	Facts []struct {
		ID string `json:"id"`
	} `json:"facts"`
	Intents []struct {
		ID string  `json:"id"`
		To *string `json:"to"`
	} `json:"intents"`
}

func (instance *server) project(t *testing.T, projectID string) projectDetail {
	t.Helper()
	response, err := http.Get(instance.baseURL + "/projects/" + projectID)
	if err != nil {
		t.Fatalf("get project: %v", err)
	}
	defer response.Body.Close()
	var detail projectDetail
	if err := json.NewDecoder(response.Body).Decode(&detail); err != nil {
		t.Fatalf("decode project: %v", err)
	}
	return detail
}

func progressConfig(t *testing.T, serverURL, workspace string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "dispatch.yaml")
	content := fmt.Sprintf(`
server: %s
runtime:
  interval: 1
  max_workers: 1
  max_running_projects: 1
  max_project_workers: 1
  healthcheck_timeout: 5
  prompt_group: mock
  execution: local
tasks:
  bootstrap: {timeout: 20, conclude_timeout: 20}
  reason: {timeout: 20, max_intents: 1}
  explore: {timeout: 20, conclude_timeout: 20}
local:
  workspace_root: %s
  completed_action: keep
workers:
  - name: mock-worker
    type: mock
    task_types: [bootstrap, reason, explore]
    max_running: 1
    priority: 0
    env:
      MOCK_HEALTHCHECK: '{"delay":[0,0],"outcomes":{"ok":1.0,"fail":0.0}}'
      MOCK_BOOTSTRAP: '{"delay":[0,0],"outcomes":{"complete":1.0,"fact":0.0,"rejected":0.0,"invalid_json":0.0,"invalid_payload":0.0,"command_fail":0.0}}'
      MOCK_REASON: '{"delay":[0,0],"outcomes":{"complete":1.0,"intent":0.0,"noop":0.0,"rejected":0.0,"invalid_json":0.0,"invalid_payload":0.0,"command_fail":0.0}}'
      MOCK_EXPLORE_EXECUTE: '{"delay":[0,0],"outcomes":{"fact":1.0,"rejected":0.0,"invalid_json":0.0,"invalid_payload":0.0,"command_fail":0.0}}'
`, serverURL, workspace)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestMockSchedulerBootstrapCompletesProjectEndToEnd(t *testing.T) {
	if os.Getenv("CAIRN_E2E") == "0" {
		t.Skip("CAIRN_E2E=0")
	}
	root := repoRoot(t)
	instance := startServer(t, root)
	projectID := instance.createProject(t)
	workspace := t.TempDir()
	configPath := progressConfig(t, instance.baseURL, workspace)

	loop, err := scheduler.New(configPath)
	if err != nil {
		t.Fatalf("loop: %v", err)
	}
	go func() {
		if runErr := loop.Run(false); runErr != nil {
			t.Logf("loop stopped: %v", runErr)
		}
	}()
	t.Cleanup(func() {
		loop.Stop()
		loop.Wait()
	})

	deadline := time.Now().Add(90 * time.Second)
	var detail projectDetail
	for time.Now().Before(deadline) {
		detail = instance.project(t, projectID)
		if detail.Project.Status != "active" {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}

	if detail.Project.Status != "completed" {
		t.Fatalf("status = %q", detail.Project.Status)
	}
	factIDs := make([]string, 0, len(detail.Facts))
	for _, fact := range detail.Facts {
		factIDs = append(factIDs, fact.ID)
	}
	if fmt.Sprint(factIDs) != "[origin goal f001]" {
		t.Fatalf("facts = %v", factIDs)
	}
	if len(detail.Intents) != 2 || detail.Intents[0].To == nil || *detail.Intents[0].To != "f001" {
		t.Fatalf("intents = %+v", detail.Intents)
	}
	if detail.Intents[1].To == nil || *detail.Intents[1].To != "goal" {
		t.Fatalf("completion intent = %+v", detail.Intents[1])
	}
}
