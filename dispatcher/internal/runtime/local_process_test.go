package runtime

import (
	"strings"
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
