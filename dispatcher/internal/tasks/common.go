// Package tasks holds the bootstrap, reason and explore task runners.
package tasks

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Indexyz/Cairn/dispatcher/internal/config"
	"github.com/Indexyz/Cairn/dispatcher/internal/logging"
	"github.com/Indexyz/Cairn/dispatcher/internal/runtime"
)

const (
	processCommunicateGrace = 15
	logPreviewLimit         = 1200
	graphSnapshotRoot       = "/tmp/cairn-prompts"
)

var log = logging.New("cairn.dispatcher.tasks")

// ConcludeWriteResult reports how the conclude write went.
type ConcludeWriteResult struct {
	Status string
	FactID *string
}

// Preview collapses whitespace and truncates long output.
func Preview(text string, limit int) string {
	compact := strings.Join(strings.Fields(text), " ")
	if len(compact) <= limit {
		return compact
	}
	return compact[:limit] + "..."
}

// DidTimeout reports whether the process hit its timeout.
func DidTimeout(result runtime.ProcessResult) bool {
	return !result.Cancelled && (result.TimedOut || result.ReturnCode == 124 || result.ReturnCode == 137)
}

// CancelReason reports why the task was cancelled, if it was.
func CancelReason(result runtime.ProcessResult, cancellation *runtime.TaskCancellation) *string {
	if result.Cancelled {
		if result.CancelReason != nil {
			return result.CancelReason
		}
		return stringPointer("cancelled")
	}
	if cancellation != nil {
		return cancellation.Reason()
	}
	return nil
}

// TaskHealthcheckEnabled reports whether per-task worker probes apply.
func TaskHealthcheckEnabled(dispatch *config.DispatchConfig) bool {
	if dispatch.Runtime.Execution == config.ExecutionLocal {
		return false
	}
	return dispatch.Runtime.WorkerHealthcheck == config.HealthcheckStartupAndTask
}

// WriteGraphSnapshotReference writes the graph YAML into the backend and returns
// the prompt paragraph that points at it.
func WriteGraphSnapshotReference(manager Backend, containerName, graphYAML, phase string) (string, error) {
	path := graphSnapshotRoot + "/" + phase + "-" + strings.ToLower(uuid.NewString()[:12]) + "/graph.yaml"
	if err := manager.WriteTextFile(containerName, path, graphYAML); err != nil {
		return "", err
	}
	return "The graph YAML snapshot is stored in this file inside the current container:\n\n" +
		path +
		"\n\nBefore using the graph, read the entire file and treat its contents as the YAML snapshot " +
		"for this Graph section.", nil
}

// RunWorkerProcess executes one worker command with lease and cancellation wiring.
func RunWorkerProcess(
	manager Backend,
	containerName string,
	worker *config.WorkerConfig,
	argv []string,
	phase string,
	timeoutSeconds int,
	lease *runtime.HeartbeatLease,
	cancellation *runtime.TaskCancellation,
) (runtime.ProcessResult, error) {
	log.Infof("starting container exec container=%s worker=%s phase=%s timeout=%ds", containerName, worker.Name, phase, timeoutSeconds)
	process, err := manager.BuildExecProcess(containerName, worker.Env, argv, timeoutSeconds, 5)
	if err != nil {
		return runtime.ProcessResult{}, err
	}
	if err := process.Start(); err != nil {
		return runtime.ProcessResult{}, err
	}
	if lease != nil {
		lease.AttachProcess(process)
	}
	if cancellation != nil {
		cancellation.AttachProcess(process)
	}
	defer func() {
		if lease != nil {
			lease.AttachProcess(nil)
		}
		if cancellation != nil {
			cancellation.AttachProcess(nil)
		}
	}()
	return process.Communicate(time.Duration(timeoutSeconds+processCommunicateGrace) * time.Second), nil
}

// ProjectAllowsConcludeFallback reports whether the project can still take writes.
func ProjectAllowsConcludeFallback(client Client, projectID, workerName string, intentID string) bool {
	project, err := client.GetProject(projectID)
	if err != nil {
		return false
	}
	if project.Project.Status == "active" {
		return true
	}
	log.Infof(
		"skip conclude fallback because project is no longer active project=%s intent=%s worker=%s status=%s",
		projectID, intentID, workerName, project.Project.Status,
	)
	return false
}

// BestEffortReleaseReason drops a reason lease, tolerating inactive projects.
func BestEffortReleaseReason(client Client, projectID, workerName string) {
	response := client.ReleaseReason(projectID, workerName)
	switch {
	case !response.Ok() && response.StatusCode != 403 && response.StatusCode != 409:
		log.Warnf("reason release failed project=%s worker=%s status=%d", projectID, workerName, response.StatusCode)
	case response.Ok():
		log.Infof("released reason project=%s worker=%s", projectID, workerName)
	default:
		log.Infof("reason release skipped project=%s worker=%s status=%d", projectID, workerName, response.StatusCode)
	}
}

// BestEffortRelease drops an intent claim, tolerating inactive projects.
func BestEffortRelease(client Client, projectID, intentID, workerName string) {
	response := client.Release(projectID, intentID, workerName)
	switch {
	case !response.Ok() && response.StatusCode != 403 && response.StatusCode != 409:
		log.Warnf("release failed project=%s intent=%s worker=%s status=%d", projectID, intentID, workerName, response.StatusCode)
	case response.Ok():
		log.Infof("released intent project=%s intent=%s worker=%s", projectID, intentID, workerName)
	default:
		log.Infof("release skipped project=%s intent=%s worker=%s status=%d", projectID, intentID, workerName, response.StatusCode)
	}
}

// WriteConcludeResult writes a conclusion and logs the outcome.
func WriteConcludeResult(client Client, projectID, intentID, workerName, description, source string, phaseMs int, totalMs *int) string {
	return WriteConcludeResultWithFactID(client, projectID, intentID, workerName, description, source, phaseMs, totalMs).Status
}

// WriteConcludeResultWithFactID writes a conclusion and returns the produced fact id.
func WriteConcludeResultWithFactID(client Client, projectID, intentID, workerName, description, source string, phaseMs int, totalMs *int) ConcludeWriteResult {
	response := client.Conclude(projectID, intentID, workerName, description)
	if response.Ok() {
		var factID *string
		data := response.Data
		if raw, ok := data.(json.RawMessage); ok {
			var decoded map[string]any
			if err := json.Unmarshal(raw, &decoded); err == nil {
				data = decoded
			}
		}
		if object, ok := data.(map[string]any); ok {
			if fact, ok := object["fact"].(map[string]any); ok {
				if id, ok := fact["id"].(string); ok && id != "" {
					factID = &id
				}
			}
		}
		if totalMs == nil {
			log.Infof("intent concluded project=%s intent=%s worker=%s source=%s phase_ms=%d", projectID, intentID, workerName, source, phaseMs)
		} else {
			log.Infof("intent concluded project=%s intent=%s worker=%s source=%s phase_ms=%d total_ms=%d", projectID, intentID, workerName, source, phaseMs, *totalMs)
		}
		return ConcludeWriteResult{Status: "success", FactID: factID}
	}
	if response.StatusCode == 403 {
		log.Infof("project became inactive during conclude project=%s intent=%s worker=%s", projectID, intentID, workerName)
	} else {
		log.Warnf("conclude write failed project=%s intent=%s worker=%s status=%d body=%s", projectID, intentID, workerName, response.StatusCode, response.Text)
	}
	BestEffortRelease(client, projectID, intentID, workerName)
	return ConcludeWriteResult{Status: "failed"}
}

func stringPointer(value string) *string { return &value }

func elapsedMs(started time.Time) int {
	return int(time.Since(started).Milliseconds())
}
