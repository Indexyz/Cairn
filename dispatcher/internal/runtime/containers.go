package runtime

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/Indexyz/Cairn/dispatcher/internal/config"
	"github.com/Indexyz/Cairn/dispatcher/internal/logging"
)

var containerLog = logging.New("cairn.dispatcher.runtime.containers")

const containerPrefix = "cairn-dispatch-"

// ContainerManager runs one container per project.
type ContainerManager struct {
	config     *config.ContainerConfig
	instanceID string
	api        dockerAPI

	locksMu sync.Mutex
	locks   map[string]*sync.Mutex
}

// NewContainerManager connects to the local Docker daemon.
func NewContainerManager(containerConfig *config.ContainerConfig, instanceID string) (*ContainerManager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &ContainerManager{config: containerConfig, instanceID: instanceID, api: cli, locks: map[string]*sync.Mutex{}}, nil
}

// Close releases the Docker client.
func (manager *ContainerManager) Close() { _ = manager.api.Close() }

// workspaceDir is the agent working directory baked into the worker image.
const workspaceDir = "/home/kali/workspace"

// WorkspacePath is the in-container working directory.
func (*ContainerManager) WorkspacePath(string) string { return workspaceDir }

// ContainerName is instance scoped so dispatchers on one host never collide.
func (manager *ContainerManager) ContainerName(projectID string) string {
	sanitized := strings.ReplaceAll(projectID, "/", "-")
	if manager.instanceID != "" {
		return containerPrefix + manager.instanceID + "-" + sanitized
	}
	return containerPrefix + sanitized
}

// EnsureRunning creates or starts the project container.
func (manager *ContainerManager) EnsureRunning(projectID string) (string, error) {
	name := manager.ContainerName(projectID)
	lock := manager.lock(name)
	lock.Lock()
	defer lock.Unlock()
	return manager.ensureRunningLocked(projectID, name)
}

func (manager *ContainerManager) ensureRunningLocked(projectID, name string) (string, error) {
	state, err := manager.InspectState(name)
	if err != nil {
		return "", err
	}
	if state != nil && *state == "running" {
		containerLog.Debugf("container already running project=%s container=%s", projectID, name)
		return name, nil
	}
	if state != nil {
		containerLog.Infof("starting existing container project=%s container=%s state=%s", projectID, name, *state)
		if err := manager.startExisting(name); err != nil {
			return "", err
		}
		return name, nil
	}

	containerLog.Infof("creating container project=%s container=%s image=%s platform=%s", projectID, name, manager.config.Image, platformLabel(manager.config.Platform))
	hostConfig := &container.HostConfig{}
	if manager.config.NetworkMode != "" {
		hostConfig.NetworkMode = container.NetworkMode(manager.config.NetworkMode)
	}
	if len(manager.config.CapAdd) > 0 {
		hostConfig.CapAdd = manager.config.CapAdd
	}
	var platform *ocispec.Platform
	if manager.config.Platform != nil && *manager.config.Platform != "" {
		platform = &ocispec.Platform{OS: "linux", Architecture: *manager.config.Platform}
		if parts := strings.SplitN(*manager.config.Platform, "/", 2); len(parts) == 2 {
			platform.OS, platform.Architecture = parts[0], parts[1]
		}
	}
	_, err = manager.api.ContainerCreate(context.Background(), &container.Config{
		Image: manager.config.Image,
		Cmd:   []string{"sleep", "infinity"},
	}, hostConfig, &network.NetworkingConfig{}, platform, name)
	if err == nil {
		containerLog.Infof("created container project=%s container=%s", projectID, name)
		if startErr := manager.api.ContainerStart(context.Background(), name, container.StartOptions{}); startErr != nil {
			return "", startErr
		}
		return name, nil
	}
	if !isNameConflict(err) {
		return "", fmt.Errorf("failed to create container %s: %w", name, err)
	}

	containerLog.Infof("container name conflict, reusing existing container project=%s container=%s", projectID, name)
	state, inspectErr := manager.InspectState(name)
	if inspectErr != nil {
		return "", inspectErr
	}
	if state != nil && *state == "running" {
		return name, nil
	}
	if state != nil {
		containerLog.Infof("starting conflicted existing container project=%s container=%s state=%s", projectID, name, *state)
		if startErr := manager.startExisting(name); startErr != nil {
			return "", startErr
		}
		return name, nil
	}
	return "", fmt.Errorf("failed to create container %s", name)
}

// InspectState returns the container state, or nil when it does not exist.
func (manager *ContainerManager) InspectState(name string) (*string, error) {
	inspect, err := manager.api.ContainerInspect(context.Background(), name)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to inspect container %s: %w", name, err)
	}
	state := inspect.State.Status
	return &state, nil
}

// CleanupCompleted removes or stops the container, per configuration.
func (manager *ContainerManager) CleanupCompleted(projectID string) bool {
	name := manager.ContainerName(projectID)
	state, err := manager.InspectState(name)
	if err != nil || state == nil {
		return err == nil
	}
	if manager.config.CompletedAction == config.CompletedRemove {
		containerLog.Infof("removing completed project container project=%s container=%s", projectID, name)
		if err := manager.api.ContainerRemove(context.Background(), name, container.RemoveOptions{Force: true}); err != nil && !errdefs.IsNotFound(err) {
			containerLog.Warnf("failed to remove container=%s error=%s", name, err)
			return false
		}
		after, err := manager.InspectState(name)
		return err == nil && after == nil
	}
	if *state == "running" {
		containerLog.Infof("stopping completed project container project=%s container=%s", projectID, name)
		if err := manager.stop(name); err != nil {
			return false
		}
		after, err := manager.InspectState(name)
		return err == nil && (after == nil || *after != "running")
	}
	return true
}

// CleanupStopped stops a running container.
func (manager *ContainerManager) CleanupStopped(projectID string) bool {
	name := manager.ContainerName(projectID)
	state, err := manager.InspectState(name)
	if err != nil || state == nil || *state != "running" {
		return err == nil
	}
	containerLog.Infof("stopping stopped project container project=%s container=%s", projectID, name)
	if err := manager.stop(name); err != nil {
		return false
	}
	after, err := manager.InspectState(name)
	return err == nil && (after == nil || *after != "running")
}

// CleanupOrphan removes a leftover container by name.
func (manager *ContainerManager) CleanupOrphan(name string) bool {
	state, err := manager.InspectState(name)
	if err != nil || state == nil {
		return err == nil
	}
	containerLog.Infof("removing orphan project container container=%s state=%s", name, *state)
	if err := manager.api.ContainerRemove(context.Background(), name, container.RemoveOptions{Force: true}); err != nil && !errdefs.IsNotFound(err) {
		containerLog.Warnf("failed to remove orphan container=%s error=%s", name, err)
		return false
	}
	after, err := manager.InspectState(name)
	return err == nil && after == nil
}

// ManagedContainerNames lists every cairn container on the host.
func (manager *ContainerManager) ManagedContainerNames() []string {
	containers, err := manager.api.ContainerList(context.Background(), container.ListOptions{All: true})
	if err != nil {
		containerLog.Warnf("failed to list managed containers error=%s", err)
		return nil
	}
	names := make([]string, 0, len(containers))
	for _, item := range containers {
		for _, name := range item.Names {
			trimmed := strings.TrimPrefix(name, "/")
			if strings.HasPrefix(trimmed, containerPrefix) {
				names = append(names, trimmed)
			}
		}
	}
	sort.Strings(names)
	return names
}

// NeedsCompletedCleanup reports whether completed cleanup has work to do.
func (manager *ContainerManager) NeedsCompletedCleanup(projectID string) bool {
	state, err := manager.InspectState(manager.ContainerName(projectID))
	if err != nil || state == nil {
		return false
	}
	if manager.config.CompletedAction == config.CompletedRemove {
		return true
	}
	return *state == "running"
}

// NeedsOrphanCleanup reports whether the orphan container still exists.
func (manager *ContainerManager) NeedsOrphanCleanup(name string) bool {
	state, err := manager.InspectState(name)
	return err == nil && state != nil
}

// NeedsStoppedCleanup reports whether the stopped container still runs.
func (manager *ContainerManager) NeedsStoppedCleanup(projectID string) bool {
	state, err := manager.InspectState(manager.ContainerName(projectID))
	return err == nil && state != nil && *state == "running"
}

// BuildExecProcess wraps the command in the timeout coreutil.
func (manager *ContainerManager) BuildExecProcess(containerName string, env map[string]string, command []string, timeoutSeconds int, killAfterSeconds int) (ExecProcess, error) {
	inspect, err := manager.api.ContainerInspect(context.Background(), containerName)
	if err != nil {
		return nil, fmt.Errorf("container not found: %s", containerName)
	}
	argv := []string{}
	if timeoutSeconds > 0 {
		if killAfterSeconds <= 0 {
			killAfterSeconds = 5
		}
		argv = append(argv, "timeout", "-k", fmt.Sprintf("%ds", killAfterSeconds), fmt.Sprintf("%ds", timeoutSeconds))
	}
	argv = append(argv, command...)
	return NewManagedProcess(manager.api, inspect.ID, containerName, argv, env), nil
}

// WriteTextFile copies content into the container through a tar archive.
func (manager *ContainerManager) WriteTextFile(containerName, target, content string) error {
	archivePath, payload, err := textFileArchive(target, content)
	if err != nil {
		return err
	}
	inspect, err := manager.api.ContainerInspect(context.Background(), containerName)
	if err != nil {
		return fmt.Errorf("container not found: %s", containerName)
	}
	if err := manager.api.CopyToContainer(context.Background(), inspect.ID, archivePath, bytes.NewReader(payload), container.CopyToContainerOptions{}); err != nil {
		return fmt.Errorf("failed to write container file %s: %w", target, err)
	}
	return nil
}

func (manager *ContainerManager) startExisting(name string) error {
	if err := manager.api.ContainerStart(context.Background(), name, container.StartOptions{}); err != nil {
		state, inspectErr := manager.InspectState(name)
		if inspectErr == nil && state != nil && *state == "running" {
			return nil
		}
		return fmt.Errorf("failed to start container %s: %w", name, err)
	}
	return nil
}

func (manager *ContainerManager) stop(name string) error {
	timeout := 1
	err := manager.api.ContainerStop(context.Background(), name, container.StopOptions{Timeout: &timeout})
	if err != nil && !errdefs.IsNotFound(err) {
		containerLog.Warnf("failed to stop container=%s error=%s", name, err)
		return err
	}
	return nil
}

func (manager *ContainerManager) lock(name string) *sync.Mutex {
	manager.locksMu.Lock()
	defer manager.locksMu.Unlock()
	lock, ok := manager.locks[name]
	if !ok {
		lock = &sync.Mutex{}
		manager.locks[name] = lock
	}
	return lock
}

func isNameConflict(err error) bool {
	if errdefs.IsConflict(err) {
		return true
	}
	return strings.Contains(err.Error(), "is already in use")
}

func platformLabel(platform *string) string {
	if platform == nil || *platform == "" {
		return "auto"
	}
	return *platform
}

func textFileArchive(target, content string) (string, []byte, error) {
	clean := path.Clean(target)
	if !path.IsAbs(clean) || clean == "/" {
		return "", nil, fmt.Errorf("container file path must be absolute: %s", target)
	}
	parts := strings.Split(strings.TrimPrefix(clean, "/"), "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", nil, fmt.Errorf("invalid container file path: %s", target)
		}
	}
	archivePath := "/"
	archiveParts := parts
	if len(parts) > 1 {
		archivePath = "/" + parts[0]
		archiveParts = parts[1:]
	}

	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	parent := ""
	for _, part := range archiveParts[:len(archiveParts)-1] {
		if parent == "" {
			parent = part
		} else {
			parent = parent + "/" + part
		}
		if err := writer.WriteHeader(&tar.Header{Name: parent, Typeflag: tar.TypeDir, Mode: 0o755}); err != nil {
			return "", nil, err
		}
	}
	payload := []byte(content)
	name := strings.Join(archiveParts, "/")
	if err := writer.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(payload))}); err != nil {
		return "", nil, err
	}
	if _, err := writer.Write(payload); err != nil {
		return "", nil, err
	}
	if err := writer.Close(); err != nil {
		return "", nil, err
	}
	return archivePath, buffer.Bytes(), nil
}
