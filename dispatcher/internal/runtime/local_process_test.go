package runtime

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func timeoutPtr(value int) *int { return &value }

func TestLocalProcessCapturesOutput(t *testing.T) {
	process := NewLocalProcess([]string{"/bin/sh", "-c", "echo hi; echo err >&2"}, "", nil, nil, 5)
	if err := process.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	result := process.Communicate(5 * time.Second)

	if result.ReturnCode != 0 {
		t.Fatalf("returncode = %d", result.ReturnCode)
	}
	if strings.TrimSpace(result.Stdout) != "hi" || strings.TrimSpace(result.Stderr) != "err" {
		t.Fatalf("stdout=%q stderr=%q", result.Stdout, result.Stderr)
	}
}

func TestLocalProcessTimesOutAndKills(t *testing.T) {
	process := NewLocalProcess([]string{"/bin/sh", "-c", "sleep 10"}, "", nil, timeoutPtr(1), 1)
	if err := process.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	started := time.Now()
	result := process.Communicate(30 * time.Second)

	if !result.TimedOut {
		t.Fatal("expected a timeout")
	}
	if elapsed := time.Since(started); elapsed > 8*time.Second {
		t.Fatalf("timeout took %s", elapsed)
	}
}

func TestLocalProcessCancelRecordsReason(t *testing.T) {
	process := NewLocalProcess([]string{"/bin/sh", "-c", "sleep 10"}, "", nil, timeoutPtr(30), 1)
	if err := process.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	process.Cancel("project stopped")
	result := process.Communicate(20 * time.Second)

	if !result.Cancelled || result.CancelReason == nil || *result.CancelReason != "project stopped" {
		t.Fatalf("result = %+v", result)
	}
}

func TestLocalBackendWriteTextFileRejectsRelativePath(t *testing.T) {
	backend := NewLocalBackend(nil)

	if err := backend.WriteTextFile("", "relative/path.yaml", "x"); err == nil {
		t.Fatal("expected relative path error")
	}
}

func TestLocalProcessExitCodeAndCwd(t *testing.T) {
	directory := t.TempDir()
	process := NewLocalProcess([]string{"/bin/sh", "-c", "pwd; exit 3"}, directory, nil, timeoutPtr(10), 2)
	if err := process.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	result := process.Communicate(20 * time.Second)

	if result.ReturnCode != 3 {
		t.Fatalf("returncode = %d", result.ReturnCode)
	}
	if strings.TrimSpace(result.Stdout) != directory {
		t.Fatalf("cwd = %q want %q", strings.TrimSpace(result.Stdout), directory)
	}
}

func TestLocalProcessKillTerminatesChildProcessGroup(t *testing.T) {
	directory := t.TempDir()
	pidFile := filepath.Join(directory, "child.pid")
	process := NewLocalProcess([]string{"/bin/sh", "-c", "sleep 30 & echo $! > " + pidFile + "; wait"}, directory, nil, timeoutPtr(1), 1)
	if err := process.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	result := process.Communicate(20 * time.Second)

	if !result.TimedOut {
		t.Fatal("expected timeout")
	}
	raw, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("read pid file: %v", err)
	}
	childPID, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("parse pid: %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(childPID, 0); err != nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("child process %d survived the group kill", childPID)
}
