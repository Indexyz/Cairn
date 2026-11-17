package runtime

import (
	"time"
)

// ProcessResult is the outcome of one worker invocation.
type ProcessResult struct {
	ReturnCode   int
	Stdout       string
	Stderr       string
	TimedOut     bool
	Cancelled    bool
	CancelReason *string
}

// ExecProcess is a worker process, inside a container or on the host.
type ExecProcess interface {
	Start() error
	Communicate(timeout time.Duration) ProcessResult
	Kill()
	Cancel(reason string)
}

// Backend is the execution substrate for worker processes.
//
// Two implementations exist: ContainerManager (one Docker container per project)
// and LocalBackend (host subprocesses, one working directory per project).
type Backend interface {
	ContainerName(projectID string) string
	EnsureRunning(projectID string) (string, error)
	WorkspacePath(projectID string) string
	BuildExecProcess(containerName string, env map[string]string, command []string, timeoutSeconds int, killAfterSeconds int) (ExecProcess, error)
	WriteTextFile(containerName, path, content string) error
	NeedsCompletedCleanup(projectID string) bool
	NeedsStoppedCleanup(projectID string) bool
	CleanupCompleted(projectID string) bool
	CleanupStopped(projectID string) bool
	Close()
}
