package tasks

import (
	"encoding/json"
	"time"

	"github.com/Indexyz/Cairn/dispatcher/internal/config"
	"github.com/Indexyz/Cairn/dispatcher/internal/contracts"
	"github.com/Indexyz/Cairn/dispatcher/internal/prompting"
	"github.com/Indexyz/Cairn/dispatcher/internal/protocol"
	"github.com/Indexyz/Cairn/dispatcher/internal/runtime"
	"github.com/Indexyz/Cairn/dispatcher/internal/workers"
)

// RunBootstrapTask makes one direct attempt to solve a fresh project.
func RunBootstrapTask(
	dispatch *config.DispatchConfig,
	client Client,
	manager Backend,
	project *protocol.ProjectDetail,
	intent *protocol.Intent,
	worker *config.WorkerConfig,
	cancellation *runtime.TaskCancellation,
) string {
	driver, err := driverFor(worker.Type, dispatch.Runtime.Execution)
	if err != nil {
		log.Errorf("bootstrap driver missing project=%s worker=%s error=%s", project.Project.ID, worker.Name, err)
		return "failed"
	}
	projectID := project.Project.ID
	taskStarted := time.Now()
	healthcheckTimeout := time.Duration(dispatch.Runtime.HealthcheckTimeout) * time.Second
	lease := runtime.NewHeartbeatLease(
		func() protocol.ApiResult { return client.Heartbeat(projectID, intent.ID, worker.Name) },
		"project="+projectID+" intent="+intent.ID, worker.Name, time.Duration(dispatch.Runtime.Interval)*time.Second,
	)
	lease.Start()
	defer lease.Stop()

	containerName, err := manager.EnsureRunning(projectID)
	if err != nil {
		log.Errorf("bootstrap ensure container failed project=%s error=%s", projectID, err)
		BestEffortRelease(client, projectID, intent.ID, worker.Name)
		return "failed"
	}
	writeMCPConfig(manager, projectID, containerName, worker)

	if TaskHealthcheckEnabled(dispatch) {
		log.Infof("checking worker health project=%s intent=%s worker=%s timeout=%ds", projectID, intent.ID, worker.Name, dispatch.Runtime.HealthcheckTimeout)
		health := driver.CheckHealth(worker, healthcheckTimeout)
		if cancellation.IsCancelled() {
			log.Infof("bootstrap cancelled during healthcheck project=%s intent=%s worker=%s reason=%s", projectID, intent.ID, worker.Name, deref(cancellation.Reason()))
			BestEffortRelease(client, projectID, intent.ID, worker.Name)
			return "cancelled"
		}
		if lease.Failure() != nil {
			log.Warnf("heartbeat lost during bootstrap healthcheck project=%s intent=%s worker=%s status=%d", projectID, intent.ID, worker.Name, derefInt(lease.Failure().StatusCode))
			BestEffortRelease(client, projectID, intent.ID, worker.Name)
			return "failed"
		}
		if !health.OK {
			log.Warnf("worker unhealthy project=%s intent=%s worker=%s status=%d detail=%s", projectID, intent.ID, worker.Name, derefInt(health.Status), health.Detail)
			BestEffortRelease(client, projectID, intent.ID, worker.Name)
			return "unhealthy"
		}
	}

	template, err := prompting.Load(dispatch.Runtime.PromptGroup, "bootstrap.md")
	if err != nil {
		log.Errorf("bootstrap prompt missing project=%s error=%s", projectID, err)
		BestEffortRelease(client, projectID, intent.ID, worker.Name)
		return "failed"
	}
	prompt := prompting.Render(template, bootstrapPromptReplacements(project))

	session := driver.PrepareSession()
	execute := driver.BuildExecute(worker, prompt, session)
	executeStarted := time.Now()
	first, err := RunWorkerProcess(manager, containerName, worker, execute.Argv, "bootstrap", dispatch.Tasks.Bootstrap.Timeout, lease, cancellation)
	if err != nil {
		log.Errorf("bootstrap execute failed project=%s error=%s", projectID, err)
		BestEffortRelease(client, projectID, intent.ID, worker.Name)
		return "failed"
	}
	executeMs := elapsedMs(executeStarted)
	session = driver.ExtractSession(execute.Session, first.Stdout, first.Stderr)
	if CancelReason(first, cancellation) != nil {
		log.Infof("bootstrap cancelled project=%s intent=%s worker=%s reason=%s execute_ms=%d", projectID, intent.ID, worker.Name, deref(CancelReason(first, cancellation)), executeMs)
		BestEffortRelease(client, projectID, intent.ID, worker.Name)
		return "cancelled"
	}
	if lease.Failure() != nil {
		log.Warnf("heartbeat lost during bootstrap project=%s intent=%s worker=%s status=%d execute_ms=%d", projectID, intent.ID, worker.Name, derefInt(lease.Failure().StatusCode), executeMs)
		BestEffortRelease(client, projectID, intent.ID, worker.Name)
		return "failed"
	}

	if !DidTimeout(first) && first.ReturnCode == 0 {
		modelOutput := driver.ExtractResponseText(first.Stdout, first.Stderr)
		payload, parseErr := contracts.ParseJSONOutput(modelOutput)
		var kind string
		var data map[string]string
		if parseErr == nil {
			kind, data, parseErr = contracts.ValidateBootstrapExecutePayload(payload)
		}
		if parseErr != nil {
			log.Warnf("bootstrap parse failed project=%s intent=%s worker=%s error=%s execute_ms=%d total_ms=%d stdout_preview=%s stderr_preview=%s",
				projectID, intent.ID, worker.Name, parseErr, executeMs, elapsedMs(taskStarted),
				Preview(first.Stdout, logPreviewLimit), Preview(first.Stderr, logPreviewLimit))
			return runBootstrapConcludeFallback(dispatch, client, manager, worker, driver, project, intent, session, lease, cancellation)
		}
		if kind == "rejected" {
			log.Warnf("bootstrap rejected project=%s intent=%s worker=%s execute_ms=%d total_ms=%d stdout_preview=%s",
				projectID, intent.ID, worker.Name, executeMs, elapsedMs(taskStarted), Preview(first.Stdout, logPreviewLimit))
			BestEffortRelease(client, projectID, intent.ID, worker.Name)
			return "rejected"
		}
		totalMs := elapsedMs(taskStarted)
		return writeBootstrapCompleteResult(client, projectID, intent.ID, worker.Name,
			data["fact_description"], data["complete_description"], "bootstrap", executeMs, &totalMs)
	}

	if DidTimeout(first) {
		log.Warnf("bootstrap timed out project=%s intent=%s worker=%s execute_ms=%d total_ms=%d stdout_preview=%s stderr_preview=%s",
			projectID, intent.ID, worker.Name, executeMs, elapsedMs(taskStarted),
			Preview(first.Stdout, logPreviewLimit), Preview(first.Stderr, logPreviewLimit))
		return runBootstrapConcludeFallback(dispatch, client, manager, worker, driver, project, intent, session, lease, cancellation)
	}

	log.Warnf("bootstrap command failed project=%s intent=%s worker=%s code=%d execute_ms=%d total_ms=%d stdout_preview=%s stderr_preview=%s",
		projectID, intent.ID, worker.Name, first.ReturnCode, executeMs, elapsedMs(taskStarted),
		Preview(first.Stdout, logPreviewLimit), Preview(first.Stderr, logPreviewLimit))
	BestEffortRelease(client, projectID, intent.ID, worker.Name)
	return "failed"
}

func runBootstrapConcludeFallback(
	dispatch *config.DispatchConfig,
	client Client,
	manager Backend,
	worker *config.WorkerConfig,
	driver workers.Driver,
	project *protocol.ProjectDetail,
	intent *protocol.Intent,
	session *string,
	lease *runtime.HeartbeatLease,
	cancellation *runtime.TaskCancellation,
) string {
	projectID := project.Project.ID
	if !driver.SupportsConclude() || session == nil || *session == "" {
		log.Infof("bootstrap conclude fallback unavailable project=%s intent=%s worker=%s supports_conclude=%t has_session=%t",
			projectID, intent.ID, worker.Name, driver.SupportsConclude(), session != nil)
		BestEffortRelease(client, projectID, intent.ID, worker.Name)
		return "failed"
	}
	if lease.Failure() != nil {
		log.Warnf("bootstrap conclude fallback skipped because heartbeat already lost project=%s intent=%s worker=%s", projectID, intent.ID, worker.Name)
		BestEffortRelease(client, projectID, intent.ID, worker.Name)
		return "failed"
	}
	if cancellation.IsCancelled() {
		log.Infof("bootstrap conclude fallback skipped because task was cancelled project=%s intent=%s worker=%s reason=%s", projectID, intent.ID, worker.Name, deref(cancellation.Reason()))
		BestEffortRelease(client, projectID, intent.ID, worker.Name)
		return "cancelled"
	}
	if !ProjectAllowsConcludeFallback(client, projectID, worker.Name, intent.ID) {
		BestEffortRelease(client, projectID, intent.ID, worker.Name)
		return "failed"
	}

	containerName, err := manager.EnsureRunning(projectID)
	if err != nil {
		log.Errorf("bootstrap conclude fallback ensure container failed project=%s error=%s", projectID, err)
		BestEffortRelease(client, projectID, intent.ID, worker.Name)
		return "failed"
	}
	template, err := prompting.Load(dispatch.Runtime.PromptGroup, "bootstrap_conclude.md")
	if err != nil {
		log.Errorf("bootstrap conclude prompt missing project=%s error=%s", projectID, err)
		BestEffortRelease(client, projectID, intent.ID, worker.Name)
		return "failed"
	}
	prompt := prompting.Render(template, bootstrapPromptReplacements(project))
	concludeArgv := driver.BuildConclude(worker, prompt, *session)
	log.Infof("starting bootstrap conclude fallback project=%s intent=%s worker=%s", projectID, intent.ID, worker.Name)
	concludeStarted := time.Now()
	result, err := RunWorkerProcess(manager, containerName, worker, concludeArgv, "bootstrap_conclude", dispatch.Tasks.Bootstrap.ConcludeTimeout, lease, cancellation)
	if err != nil {
		log.Errorf("bootstrap conclude fallback failed project=%s error=%s", projectID, err)
		BestEffortRelease(client, projectID, intent.ID, worker.Name)
		return "failed"
	}
	concludeMs := elapsedMs(concludeStarted)
	if CancelReason(result, cancellation) != nil {
		log.Infof("bootstrap conclude cancelled project=%s intent=%s worker=%s reason=%s conclude_ms=%d", projectID, intent.ID, worker.Name, deref(CancelReason(result, cancellation)), concludeMs)
		BestEffortRelease(client, projectID, intent.ID, worker.Name)
		return "cancelled"
	}
	if lease.Failure() != nil {
		BestEffortRelease(client, projectID, intent.ID, worker.Name)
		return "failed"
	}
	if result.TimedOut || result.ReturnCode != 0 {
		log.Warnf("bootstrap conclude failed project=%s intent=%s worker=%s code=%d timed_out=%t conclude_ms=%d stdout_preview=%s stderr_preview=%s",
			projectID, intent.ID, worker.Name, result.ReturnCode, result.TimedOut, concludeMs,
			Preview(result.Stdout, logPreviewLimit), Preview(result.Stderr, logPreviewLimit))
		BestEffortRelease(client, projectID, intent.ID, worker.Name)
		return "failed"
	}
	modelOutput := driver.ExtractResponseText(result.Stdout, result.Stderr)
	payload, parseErr := contracts.ParseJSONOutput(modelOutput)
	if parseErr == nil {
		if data, ok := payload["data"].(map[string]any); ok {
			if complete, ok := data["complete"].(map[string]any); ok {
				encoded, _ := json.Marshal(complete)
				log.Warnf("bootstrap conclude returned unexpected complete payload project=%s intent=%s worker=%s complete_preview=%s",
					projectID, intent.ID, worker.Name, Preview(string(encoded), logPreviewLimit))
			}
		}
	}
	var kind string
	var factDescription *string
	if parseErr == nil {
		kind, factDescription, parseErr = contracts.ValidateBootstrapConcludePayload(payload)
	}
	if parseErr != nil {
		log.Warnf("bootstrap conclude parse failed project=%s intent=%s worker=%s error=%s conclude_ms=%d stdout_preview=%s stderr_preview=%s",
			projectID, intent.ID, worker.Name, parseErr, concludeMs,
			Preview(result.Stdout, logPreviewLimit), Preview(result.Stderr, logPreviewLimit))
		BestEffortRelease(client, projectID, intent.ID, worker.Name)
		return "failed"
	}
	if kind == "rejected" {
		log.Warnf("bootstrap conclude rejected project=%s intent=%s worker=%s conclude_ms=%d stdout_preview=%s",
			projectID, intent.ID, worker.Name, concludeMs, Preview(result.Stdout, logPreviewLimit))
		BestEffortRelease(client, projectID, intent.ID, worker.Name)
		return "rejected"
	}
	return WriteConcludeResult(client, projectID, intent.ID, worker.Name, deref(factDescription), "bootstrap_conclude", concludeMs, nil)
}

func bootstrapPromptReplacements(project *protocol.ProjectDetail) map[string]string {
	facts := map[string]string{}
	for _, fact := range project.Facts {
		facts[fact.ID] = fact.Description
	}
	hints := make([]map[string]any, 0, len(project.Hints))
	for _, hint := range project.Hints {
		hints = append(hints, map[string]any{
			"id": hint.ID, "content": hint.Content, "creator": hint.Creator, "created_at": hint.CreatedAt,
		})
	}
	return map[string]string{
		"origin": facts["origin"],
		"goal":   facts["goal"],
		"hints":  prompting.FormatHints(hints),
	}
}

func writeBootstrapCompleteResult(
	client Client,
	projectID, intentID, workerName, factDescription, completeDescription, source string,
	phaseMs int,
	totalMs *int,
) string {
	conclude := WriteConcludeResultWithFactID(client, projectID, intentID, workerName, factDescription, source, phaseMs, totalMs)
	if conclude.Status != "success" {
		return "failed"
	}
	if conclude.FactID == nil {
		log.Warnf("bootstrap complete deferred because conclude response omitted fact id project=%s intent=%s worker=%s source=%s",
			projectID, intentID, workerName, source)
		return "success"
	}
	response := client.Complete(projectID, []string{*conclude.FactID}, completeDescription, workerName)
	if response.StatusCode == 403 || response.StatusCode == 409 {
		log.Infof("bootstrap complete deferred project=%s intent=%s worker=%s source=%s status=%d fact_id=%s",
			projectID, intentID, workerName, source, response.StatusCode, *conclude.FactID)
		return "success"
	}
	if !response.Ok() {
		log.Warnf("bootstrap complete write failed project=%s intent=%s worker=%s source=%s fact_id=%s status=%d body=%s",
			projectID, intentID, workerName, source, *conclude.FactID, response.StatusCode, response.Text)
		return "success"
	}
	if totalMs == nil {
		log.Infof("bootstrap completed project=%s intent=%s worker=%s source=%s from=%s phase_ms=%d",
			projectID, intentID, workerName, source, *conclude.FactID, phaseMs)
	} else {
		log.Infof("bootstrap completed project=%s intent=%s worker=%s source=%s from=%s phase_ms=%d total_ms=%d",
			projectID, intentID, workerName, source, *conclude.FactID, phaseMs, *totalMs)
	}
	return "success"
}
