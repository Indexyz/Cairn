package runtime

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/errdefs"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/Indexyz/Cairn/dispatcher/internal/config"
	"github.com/Indexyz/Cairn/dispatcher/internal/protocol"
)

type fakeProcess struct {
	cancelled []string
	kills     int
}

func (process *fakeProcess) Start() error { return nil }
func (process *fakeProcess) Communicate(time.Duration) ProcessResult {
	return ProcessResult{}
}
func (process *fakeProcess) Kill() { process.kills++ }
func (process *fakeProcess) Cancel(reason string) {
	process.cancelled = append(process.cancelled, reason)
}

func TestTaskCancellationKeepsFirstReasonAndCancelsLateProcess(t *testing.T) {
	cancellation := &TaskCancellation{}

	if !cancellation.Cancel("project stopped") {
		t.Fatal("first cancel should report a change")
	}
	if cancellation.Cancel("second reason") {
		t.Fatal("second cancel should be a no-op")
	}
	if reason := cancellation.Reason(); reason == nil || *reason != "project stopped" {
		t.Fatalf("reason = %v", reason)
	}

	process := &fakeProcess{}
	cancellation.AttachProcess(process)
	if len(process.cancelled) != 1 || process.cancelled[0] != "project stopped" {
		t.Fatalf("cancelled = %v", process.cancelled)
	}
}

func TestHeartbeatFailureKillsAttachedProcess(t *testing.T) {
	process := &fakeProcess{}
	lease := NewHeartbeatLease(func() protocol.ApiResult { return protocol.ApiResult{StatusCode: 409} }, "intent", "worker", time.Minute)
	lease.AttachProcess(process)

	lease.fail(409, "lost")

	failure := lease.Failure()
	if failure == nil || failure.StatusCode == nil || *failure.StatusCode != 409 {
		t.Fatalf("failure = %+v", failure)
	}
	if process.kills != 1 {
		t.Fatalf("kills = %d", process.kills)
	}
}

type fakeAPI struct {
	states      []string
	inspectPos  int
	startCalls  int
	stopCalls   int
	removeCalls int
	archivePath string
	archiveBody []byte
	createdName string
	platform    *ocispec.Platform
	createErr   error
	execCmd     []string
	execPid     int
	execRunning bool
}

func (api *fakeAPI) ContainerInspect(context.Context, string) (container.InspectResponse, error) {
	if len(api.states) == 0 {
		return container.InspectResponse{}, errdefs.NotFound(errors.New("not found"))
	}
	index := api.inspectPos
	if index >= len(api.states) {
		index = len(api.states) - 1
	}
	api.inspectPos++
	state := api.states[index]
	return container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{ID: "cid", State: &container.State{Status: state}},
	}, nil
}

func (api *fakeAPI) ContainerCreate(_ context.Context, _ *container.Config, _ *container.HostConfig, _ *network.NetworkingConfig, platform *ocispec.Platform, name string) (container.CreateResponse, error) {
	api.createdName = name
	api.platform = platform
	if api.createErr != nil {
		return container.CreateResponse{}, api.createErr
	}
	return container.CreateResponse{ID: "cid"}, nil
}

func (api *fakeAPI) ContainerStart(context.Context, string, container.StartOptions) error {
	api.startCalls++
	return nil
}

func (api *fakeAPI) ContainerStop(context.Context, string, container.StopOptions) error {
	api.stopCalls++
	return nil
}

func (api *fakeAPI) ContainerRemove(context.Context, string, container.RemoveOptions) error {
	api.removeCalls++
	return nil
}

func (api *fakeAPI) ContainerList(context.Context, container.ListOptions) ([]container.Summary, error) {
	return nil, nil
}

func (api *fakeAPI) ContainerExecCreate(_ context.Context, _ string, options container.ExecOptions) (container.ExecCreateResponse, error) {
	api.execCmd = options.Cmd
	return container.ExecCreateResponse{ID: "exec-1"}, nil
}

func (api *fakeAPI) ContainerExecAttach(context.Context, string, container.ExecAttachOptions) (typesHijackedResponse, error) {
	return types.HijackedResponse{Reader: bufio.NewReader(bytes.NewReader(nil))}, nil
}

func (api *fakeAPI) ContainerExecInspect(context.Context, string) (container.ExecInspect, error) {
	return container.ExecInspect{Running: api.execRunning, Pid: api.execPid, ExitCode: 0}, nil
}

func (api *fakeAPI) CopyToContainer(_ context.Context, _, path string, content io.Reader, _ container.CopyToContainerOptions) error {
	api.archivePath = path
	api.archiveBody, _ = io.ReadAll(content)
	return nil
}

func (api *fakeAPI) Close() error { return nil }

func fakeManager(api *fakeAPI, action config.CompletedAction) *ContainerManager {
	return &ContainerManager{
		config: &config.ContainerConfig{Image: "image", NetworkMode: "host", CompletedAction: action},
		api:    api,
		locks:  map[string]*sync.Mutex{},
	}
}

func TestContainerManagerBuildExecProcessWrapsCommandWithTimeout(t *testing.T) {
	api := &fakeAPI{states: []string{"running"}}
	manager := fakeManager(api, config.CompletedStop)

	process, err := manager.BuildExecProcess("container", map[string]string{"A": "B"}, []string{"agent", "-p", "prompt"}, 300, 5)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if process.(*ManagedProcess).execID != "" {
		t.Fatal("exec should not be created before Start")
	}
	if err := process.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	want := []string{"timeout", "-k", "5s", "300s", "agent", "-p", "prompt"}
	if len(api.execCmd) != len(want) {
		t.Fatalf("cmd = %v", api.execCmd)
	}
	for index := range want {
		if api.execCmd[index] != want[index] {
			t.Fatalf("cmd = %v", api.execCmd)
		}
	}
}

func TestCompletedContainerStopActionOnlyStopsRunningContainer(t *testing.T) {
	api := &fakeAPI{states: []string{"running", "exited"}}
	manager := fakeManager(api, config.CompletedStop)

	if !manager.CleanupCompleted("proj/001") {
		t.Fatal("cleanup should succeed")
	}
	if manager.ContainerName("proj/001") != "cairn-dispatch-proj-001" {
		t.Fatalf("name = %q", manager.ContainerName("proj/001"))
	}
	if api.stopCalls != 1 {
		t.Fatalf("stops = %d", api.stopCalls)
	}
}

func TestStoppedCleanupIsNoopWhenContainerAlreadyExited(t *testing.T) {
	api := &fakeAPI{states: []string{"exited"}}
	manager := fakeManager(api, config.CompletedStop)

	if !manager.CleanupStopped("proj_001") {
		t.Fatal("cleanup should succeed")
	}
	if api.stopCalls != 0 {
		t.Fatalf("stops = %d", api.stopCalls)
	}
}

func TestWriteTextFileUsesArchiveAPI(t *testing.T) {
	api := &fakeAPI{states: []string{"running"}}
	manager := fakeManager(api, config.CompletedStop)

	if err := manager.WriteTextFile("container", "/tmp/graph.yaml", "facts: []\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
	if api.archivePath != "/tmp" || len(api.archiveBody) == 0 {
		t.Fatalf("archive path=%q size=%d", api.archivePath, len(api.archiveBody))
	}
}

func TestEnsureRunningPassesConfiguredPlatformAndName(t *testing.T) {
	platform := "linux/arm64"
	api := &fakeAPI{}
	manager := fakeManager(api, config.CompletedStop)
	manager.config.Platform = &platform
	manager.config.CompletedAction = config.CompletedStop

	if _, err := manager.EnsureRunning("proj_001"); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if api.createdName != "cairn-dispatch-proj_001" {
		t.Fatalf("name = %q", api.createdName)
	}
	if api.platform == nil || api.platform.Architecture != "arm64" || api.platform.OS != "linux" {
		t.Fatalf("platform = %+v", api.platform)
	}
}

func TestContainerNamesAreInstanceScoped(t *testing.T) {
	api := &fakeAPI{}
	manager := fakeManager(api, config.CompletedStop)
	manager.instanceID = "node-a"

	if manager.ContainerName("proj_001") != "cairn-dispatch-node-a-proj_001" {
		t.Fatalf("name = %q", manager.ContainerName("proj_001"))
	}
	if manager.ContainerName("proj/1") != "cairn-dispatch-node-a-proj-1" {
		t.Fatalf("name = %q", manager.ContainerName("proj/1"))
	}
	manager.instanceID = ""
	if manager.ContainerName("proj_001") != "cairn-dispatch-proj_001" {
		t.Fatalf("unscoped name = %q", manager.ContainerName("proj_001"))
	}
}
