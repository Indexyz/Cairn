package e2e

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Indexyz/Cairn/dispatcher/internal/scheduler"
)

const parityImage = "docker.io/library/python:3.13-alpine"

type workloadResult struct {
	status   string
	factIDs  []string
	targets  []string
	evidence string
}

// workloadConfig renders the same mock workload for local or container execution.
func workloadConfig(t *testing.T, mode, serverURL, workspace string) string {
	t.Helper()
	execution := "local"
	executionBlock := "local:\n  workspace_root: " + workspace + "\n  completed_action: keep\n"
	if mode == "container" {
		execution = "container"
		executionBlock = "container:\n  image: " + parityImage + "\n  network_mode: host\n  completed_action: stop\n"
	}
	content := fmt.Sprintf(`
server: %s
runtime:
  interval: 1
  max_workers: 1
  max_running_projects: 1
  max_project_workers: 1
  healthcheck_timeout: 10
  prompt_group: mock
  execution: %s
tasks:
  bootstrap: {timeout: 60, conclude_timeout: 60}
  reason: {timeout: 60, max_intents: 1}
  explore: {timeout: 60, conclude_timeout: 60}
%s
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
`, serverURL, execution, executionBlock)
	path := filepath.Join(t.TempDir(), "dispatch.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func runWorkload(t *testing.T, mode string) workloadResult {
	t.Helper()
	root := repoRoot(t)
	instance := startServer(t, root)
	projectID := instance.createProject(t)
	workspace := t.TempDir()
	configPath := workloadConfig(t, mode, instance.baseURL, workspace)

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

	deadline := time.Now().Add(180 * time.Second)
	var detail projectDetail
	for time.Now().Before(deadline) {
		detail = instance.project(t, projectID)
		if detail.Project.Status != "active" {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	result := workloadResult{status: detail.Project.Status}
	for _, fact := range detail.Facts {
		result.factIDs = append(result.factIDs, fact.ID)
	}
	for _, intent := range detail.Intents {
		target := "<open>"
		if intent.To != nil {
			target = *intent.To
		}
		result.targets = append(result.targets, target)
	}
	return result
}

func TestParityLocalAndContainerModes(t *testing.T) {
	if os.Getenv("CAIRN_E2E") == "0" {
		t.Skip("CAIRN_E2E=0")
	}
	local := runWorkload(t, "local")

	ensurePodman(t)
	container := runWorkload(t, "container")

	for name, result := range map[string]workloadResult{"local": local, "container": container} {
		if result.status != "completed" {
			t.Fatalf("%s status = %q", name, result.status)
		}
	}
	if strings.Join(local.factIDs, ",") != strings.Join(container.factIDs, ",") {
		t.Fatalf("facts differ: local=%v container=%v", local.factIDs, container.factIDs)
	}
	if strings.Join(local.targets, ",") != strings.Join(container.targets, ",") {
		t.Fatalf("intent targets differ: local=%v container=%v", local.targets[:], container.targets[:])
	}
	if fmt.Sprint(local.factIDs) != "[origin goal f001]" {
		t.Fatalf("unexpected facts: %v", local.factIDs)
	}
}

// ensurePodman starts a Docker-compatible podman service and points the
// dispatcher at it through DOCKER_HOST.
func ensurePodman(t *testing.T) {
	t.Helper()
	if os.Getenv("CAIRN_E2E_SKIP_CONTAINER") == "1" {
		t.Skip("CAIRN_E2E_SKIP_CONTAINER=1")
	}
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not installed")
	}
	hasImage := exec.Command("podman", "image", "exists", parityImage).Run() == nil
	if !hasImage {
		pull := exec.Command("podman", "pull", "-q", parityImage)
		pull.Stdout = io.Discard
		pull.Stderr = os.Stderr
		if err := pull.Run(); err != nil {
			t.Skipf("cannot pull %s: %v", parityImage, err)
		}
	}

	socket := filepath.Join(t.TempDir(), "podman.sock")
	service := exec.Command("podman", "system", "service", "--time=0", "unix://"+socket)
	service.Stdout = io.Discard
	service.Stderr = os.Stderr
	if err := service.Start(); err != nil {
		t.Fatalf("start podman service: %v", err)
	}
	t.Cleanup(func() {
		cleanupContainers(t, socket)
		_ = service.Process.Signal(os.Interrupt)
		done := make(chan struct{})
		go func() { _ = service.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = service.Process.Kill()
		}
	})

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(socket); err == nil {
			if exec.Command("podman", "--url", "unix://"+socket, "info").Run() == nil {
				t.Setenv("DOCKER_HOST", "unix://"+socket)
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("podman service did not become ready")
}

func cleanupContainers(t *testing.T, socket string) {
	t.Helper()
	out, err := exec.Command("podman", "--url", "unix://"+socket, "ps", "-aq", "--filter", "name=cairn-dispatch").Output()
	if err != nil {
		return
	}
	ids := strings.Fields(string(bytes.TrimSpace(out)))
	if len(ids) == 0 {
		return
	}
	args := append([]string{"--url", "unix://" + socket, "rm", "-f"}, ids...)
	_ = exec.Command("podman", args...).Run()
}
