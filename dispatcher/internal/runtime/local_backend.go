package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Indexyz/Cairn/dispatcher/internal/config"
	"github.com/Indexyz/Cairn/dispatcher/internal/logging"
)

var localLog = logging.New("cairn.dispatcher.runtime.local_backend")

// LocalBackend runs workers directly on the dispatcher host. Each project gets an
// isolated working directory; worker processes inherit the host environment so the
// already-configured CLIs and credentials are used as-is.
type LocalBackend struct {
	completedAction config.LocalCompletedAction
	root            string
}

// NewLocalBackend resolves the workspace root.
func NewLocalBackend(local *config.LocalConfig) *LocalBackend {
	root := ""
	if local != nil && local.WorkspaceRoot != nil {
		root = os.ExpandEnv(*local.WorkspaceRoot)
	}
	if root == "" {
		root, _ = os.Getwd()
	}
	action := config.LocalKeep
	if local != nil {
		action = local.CompletedAction
	}
	return &LocalBackend{completedAction: action, root: root}
}

// Close releases nothing for local processes.
func (*LocalBackend) Close() {}

// ContainerName is the project working directory.
func (backend *LocalBackend) ContainerName(projectID string) string {
	return backend.projectDir(projectID)
}

// EnsureRunning creates the project directory.
func (backend *LocalBackend) EnsureRunning(projectID string) (string, error) {
	directory := backend.projectDir(projectID)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", err
	}
	localLog.Debugf("local project workdir ready project=%s dir=%s", projectID, directory)
	return directory, nil
}

// BuildExecProcess prepares a host subprocess with the host environment.
func (backend *LocalBackend) BuildExecProcess(containerName string, env map[string]string, command []string, timeoutSeconds int, killAfterSeconds int) (ExecProcess, error) {
	var timeout *int
	if timeoutSeconds > 0 {
		timeout = &timeoutSeconds
	}
	return NewLocalProcess(command, containerName, env, timeout, killAfterSeconds), nil
}

// WriteTextFile writes an absolute host path.
func (*LocalBackend) WriteTextFile(_ string, path string, content string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("local file path must be absolute: %s", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// NeedsCompletedCleanup reports whether the completed workspace should be removed.
func (backend *LocalBackend) NeedsCompletedCleanup(projectID string) bool {
	if backend.completedAction != config.LocalRemove {
		return false
	}
	_, err := os.Stat(backend.projectDir(projectID))
	return err == nil
}

// NeedsStoppedCleanup is always false locally.
func (*LocalBackend) NeedsStoppedCleanup(string) bool { return false }

// CleanupCompleted removes the workspace when configured to.
func (backend *LocalBackend) CleanupCompleted(projectID string) bool {
	if backend.completedAction == config.LocalRemove {
		directory := backend.projectDir(projectID)
		localLog.Infof("removing completed project workdir project=%s dir=%s", projectID, directory)
		_ = os.RemoveAll(directory)
	}
	return true
}

// CleanupStopped is always a no-op locally.
func (*LocalBackend) CleanupStopped(string) bool { return true }

// ManagedContainerNames is empty locally.
func (*LocalBackend) ManagedContainerNames() []string { return nil }

func (backend *LocalBackend) projectDir(projectID string) string {
	return filepath.Join(backend.root, strings.ReplaceAll(projectID, "/", "-"))
}
