package tasks

import (
	"github.com/Indexyz/Cairn/dispatcher/internal/protocol"
	"github.com/Indexyz/Cairn/dispatcher/internal/runtime"
	"github.com/Indexyz/Cairn/dispatcher/internal/workers/registry"
)

// Client is the server surface the task runners use.
type Client interface {
	GetProject(projectID string) (*protocol.ProjectDetail, error)
	Heartbeat(projectID, intentID, worker string) protocol.ApiResult
	ClaimReason(projectID, worker, trigger string) protocol.ApiResult
	ReasonHeartbeat(projectID, worker string) protocol.ApiResult
	Release(projectID, intentID, worker string) protocol.ApiResult
	ReleaseReason(projectID, worker string) protocol.ApiResult
	Conclude(projectID, intentID, worker, description string) protocol.ApiResult
	Complete(projectID string, from []string, description, worker string) protocol.ApiResult
	CreateIntent(projectID string, from []string, description, creator string) protocol.ApiResult
}

// Backend is the execution surface the task runners use.
type Backend interface {
	EnsureRunning(projectID string) (string, error)
	BuildExecProcess(containerName string, env map[string]string, command []string, timeoutSeconds int, killAfterSeconds int) (runtime.ExecProcess, error)
	WriteTextFile(containerName, path, content string) error
}

// driverFor resolves a worker driver; tests replace it.
var driverFor = registry.Get
