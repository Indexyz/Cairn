package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Indexyz/Cairn/dispatcher/internal/config"
)

func TestLocalBackendCreatesIsolatedProjectDir(t *testing.T) {
	root := t.TempDir()
	backend := NewLocalBackend(&config.LocalConfig{WorkspaceRoot: &root})

	handle, err := backend.EnsureRunning("proj_001")
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}

	want := filepath.Join(root, "proj_001")
	if handle != want {
		t.Fatalf("handle = %q want %q", handle, want)
	}
	if info, err := os.Stat(handle); err != nil || !info.IsDir() {
		t.Fatalf("project dir missing: %v", err)
	}
	if backend.ContainerName("proj_001") != want {
		t.Fatalf("container name = %q", backend.ContainerName("proj_001"))
	}
}

func TestLocalBackendMergesHostEnvWithWorkerEnv(t *testing.T) {
	t.Setenv("CAIRN_HOST_VAR", "host")
	root := t.TempDir()
	backend := NewLocalBackend(&config.LocalConfig{WorkspaceRoot: &root})
	handle, _ := backend.EnsureRunning("proj_001")

	process, err := backend.BuildExecProcess(handle, map[string]string{"CAIRN_WORKER_VAR": "worker"},
		[]string{"/bin/sh", "-c", `printf "%s-%s" "$CAIRN_HOST_VAR" "$CAIRN_WORKER_VAR"`}, 10, 5)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := process.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	result := process.Communicate(20 * time.Second)

	if result.Stdout != "host-worker" {
		t.Fatalf("stdout = %q", result.Stdout)
	}
}

func TestLocalBackendWriteTextFileWritesToHost(t *testing.T) {
	root := t.TempDir()
	backend := NewLocalBackend(&config.LocalConfig{WorkspaceRoot: &root})
	target := filepath.Join(root, "snapshots", "graph.yaml")

	if err := backend.WriteTextFile("ignored", target, "facts: []\n"); err != nil {
		t.Fatalf("write: %v", err)
	}

	content, err := os.ReadFile(target)
	if err != nil || string(content) != "facts: []\n" {
		t.Fatalf("content = %q err = %v", content, err)
	}
}

func TestLocalBackendKeepLeavesDirAndReportsNoCleanup(t *testing.T) {
	root := t.TempDir()
	backend := NewLocalBackend(&config.LocalConfig{WorkspaceRoot: &root, CompletedAction: config.LocalKeep})
	handle, _ := backend.EnsureRunning("proj_001")

	if backend.NeedsCompletedCleanup("proj_001") {
		t.Fatal("keep should not need cleanup")
	}
	if !backend.CleanupCompleted("proj_001") {
		t.Fatal("cleanup should succeed")
	}
	if _, err := os.Stat(handle); err != nil {
		t.Fatalf("dir removed: %v", err)
	}
}

func TestLocalBackendRemoveDeletesDirOnCompletion(t *testing.T) {
	root := t.TempDir()
	backend := NewLocalBackend(&config.LocalConfig{WorkspaceRoot: &root, CompletedAction: config.LocalRemove})
	handle, _ := backend.EnsureRunning("proj_001")

	if !backend.NeedsCompletedCleanup("proj_001") {
		t.Fatal("remove should need cleanup")
	}
	if !backend.CleanupCompleted("proj_001") {
		t.Fatal("cleanup should succeed")
	}
	if _, err := os.Stat(handle); err == nil {
		t.Fatal("dir still exists")
	}
}

func TestLocalBackendStoppedCleanupIsNoop(t *testing.T) {
	root := t.TempDir()
	backend := NewLocalBackend(&config.LocalConfig{WorkspaceRoot: &root})
	if _, err := backend.EnsureRunning("proj_001"); err != nil {
		t.Fatalf("ensure: %v", err)
	}

	if backend.NeedsStoppedCleanup("proj_001") {
		t.Fatal("stopped cleanup should be a noop")
	}
	if !backend.CleanupStopped("proj_001") {
		t.Fatal("stopped cleanup should succeed")
	}
}

func TestLocalBackendDefaultsWorkspaceToCwd(t *testing.T) {
	backend := NewLocalBackend(nil)
	cwd, _ := os.Getwd()
	handle, err := backend.EnsureRunning("proj_default")
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	defer os.RemoveAll(handle)

	if !strings.HasPrefix(handle, cwd) {
		t.Fatalf("handle = %q not under cwd %q", handle, cwd)
	}
}

func TestLocalCommonEnvReachesWorkerSubprocess(t *testing.T) {
	root := t.TempDir()
	proxy := "http://127.0.0.1:7897"
	body := `
server: http://127.0.0.1:8000
runtime: {interval: 1, max_workers: 1, max_running_projects: 1, max_project_workers: 1, healthcheck_timeout: 1, prompt_group: default, execution: local, worker_healthcheck: disabled}
tasks:
  bootstrap: {timeout: 1, conclude_timeout: 1}
  reason: {timeout: 1}
  explore: {timeout: 1, conclude_timeout: 1}
local: {workspace_root: ` + root + `}
common_env:
  https_proxy: ` + proxy + `
  http_proxy: ` + proxy + `
  all_proxy: ` + proxy + `
workers:
  - {name: c, type: claudecode, task_types: [explore], max_running: 1, priority: 0}
`
	dispatch, err := config.Parse([]byte(body))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	worker := dispatch.Workers[0]
	if worker.Env["https_proxy"] != proxy {
		t.Fatalf("common_env not merged: %v", worker.Env)
	}
	backend := NewLocalBackend(dispatch.Local)
	handle, err := backend.EnsureRunning("proj_001")
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	process, err := backend.BuildExecProcess(handle, worker.Env,
		[]string{"/bin/sh", "-c", `printf "%s|%s|%s" "$https_proxy" "$http_proxy" "$all_proxy"`}, 10, 5)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := process.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	result := process.Communicate(20 * time.Second)
	want := proxy + "|" + proxy + "|" + proxy
	if result.Stdout != want {
		t.Fatalf("stdout = %q want %q", result.Stdout, want)
	}
}
