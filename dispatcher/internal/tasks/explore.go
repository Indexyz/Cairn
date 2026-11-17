package tasks

import (
	"time"

	"github.com/Indexyz/Cairn/dispatcher/internal/config"
	"github.com/Indexyz/Cairn/dispatcher/internal/contracts"
	"github.com/Indexyz/Cairn/dispatcher/internal/prompting"
	"github.com/Indexyz/Cairn/dispatcher/internal/protocol"
	"github.com/Indexyz/Cairn/dispatcher/internal/runtime"
	"github.com/Indexyz/Cairn/dispatcher/internal/workers"
)

// RunExploreTask executes one claimable intent and writes its fact.
func RunExploreTask(
	dispatch *config.DispatchConfig,
	client Client,
	manager Backend,
	project *protocol.ProjectDetail,
	exportYAML string,
	intent *protocol.Intent,
	worker *config.WorkerConfig,
	cancellation *runtime.TaskCancellation,
) string {
	driver, err := driverFor(worker.Type, dispatch.Runtime.Execution)
	if err != nil {
		log.Errorf("explore driver missing project=%s worker=%s error=%s", project.Project.ID, worker.Name, err)
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
		log.Errorf("explore ensure container failed project=%s error=%s", projectID, err)
		BestEffortRelease(client, projectID, intent.ID, worker.Name)
		return "failed"
	}
	writeMCPConfig(manager, projectID, containerName, worker)

	if TaskHealthcheckEnabled(dispatch) {
		log.Infof("checking worker health project=%s intent=%s worker=%s timeout=%ds", projectID, intent.ID, worker.Name, dispatch.Runtime.HealthcheckTimeout)
		health := driver.CheckHealth(worker, healthcheckTimeout)
		if cancellation.IsCancelled() {
			log.Infof("explore cancelled during healthcheck project=%s intent=%s worker=%s reason=%s", projectID, intent.ID, worker.Name, deref(cancellation.Reason()))
			BestEffortRelease(client, projectID, intent.ID, worker.Name)
			return "cancelled"
		}
		if lease.Failure() != nil {
			log.Warnf("heartbeat lost during explore healthcheck project=%s intent=%s worker=%s status=%d", projectID, intent.ID, worker.Name, derefInt(lease.Failure().StatusCode))
			BestEffortRelease(client, projectID, intent.ID, worker.Name)
			return "failed"
		}
		if !health.OK {
			log.Warnf("worker unhealthy project=%s intent=%s worker=%s status=%d detail=%s", projectID, intent.ID, worker.Name, derefInt(health.Status), health.Detail)
			BestEffortRelease(client, projectID, intent.ID, worker.Name)
			return "unhealthy"
		}
	}

	graphReference, err := WriteGraphSnapshotReference(manager, containerName, trimSpace(exportYAML), "explore_execute")
	if err != nil {
		log.Errorf("explore graph snapshot failed project=%s error=%s", projectID, err)
		BestEffortRelease(client, projectID, intent.ID, worker.Name)
		return "failed"
	}
	template, err := prompting.Load(dispatch.Runtime.PromptGroup, "explore.md")
	if err != nil {
		log.Errorf("explore prompt missing project=%s error=%s", projectID, err)
		BestEffortRelease(client, projectID, intent.ID, worker.Name)
		return "failed"
	}
	prompt := prompting.Render(template, map[string]string{
		"graph_yaml":         graphReference,
		"intent_id":          intent.ID,
		"intent_description": intent.Description,
	})

	session := driver.PrepareSession()
	execute := driver.BuildExecute(worker, prompt, session)
	executeStarted := time.Now()
	first, err := RunWorkerProcess(manager, containerName, worker, execute.Argv, "explore_execute", dispatch.Tasks.Explore.Timeout, lease, cancellation)
	if err != nil {
		log.Errorf("explore execute failed project=%s error=%s", projectID, err)
		BestEffortRelease(client, projectID, intent.ID, worker.Name)
		return "failed"
	}
	executeMs := elapsedMs(executeStarted)
	session = driver.ExtractSession(execute.Session, first.Stdout, first.Stderr)
	if CancelReason(first, cancellation) != nil {
		log.Infof("explore cancelled project=%s intent=%s worker=%s reason=%s execute_ms=%d", projectID, intent.ID, worker.Name, deref(CancelReason(first, cancellation)), executeMs)
		BestEffortRelease(client, projectID, intent.ID, worker.Name)
		return "cancelled"
	}
	if lease.Failure() != nil {
		log.Warnf("heartbeat lost during explore project=%s intent=%s worker=%s status=%d execute_ms=%d", projectID, intent.ID, worker.Name, derefInt(lease.Failure().StatusCode), executeMs)
		BestEffortRelease(client, projectID, intent.ID, worker.Name)
		return "failed"
	}

	if !DidTimeout(first) && first.ReturnCode == 0 {
		modelOutput := driver.ExtractResponseText(first.Stdout, first.Stderr)
		payload, parseErr := contracts.ParseJSONOutput(modelOutput)
		var kind string
		var description *string
		if parseErr == nil {
			kind, description, parseErr = contracts.ValidateExplorePayload(payload)
		}
		if parseErr != nil {
			log.Warnf("explore parse failed project=%s intent=%s worker=%s error=%s execute_ms=%d total_ms=%d stdout_preview=%s stderr_preview=%s",
				projectID, intent.ID, worker.Name, parseErr, executeMs, elapsedMs(taskStarted),
				Preview(first.Stdout, logPreviewLimit), Preview(first.Stderr, logPreviewLimit))
			return runExploreConcludeFallback(dispatch, client, manager, worker, driver, projectID, intent, exportYAML, session, lease, cancellation)
		}
		if kind == "rejected" {
			log.Warnf("explore rejected project=%s intent=%s worker=%s execute_ms=%d total_ms=%d stdout_preview=%s",
				projectID, intent.ID, worker.Name, executeMs, elapsedMs(taskStarted), Preview(first.Stdout, logPreviewLimit))
			BestEffortRelease(client, projectID, intent.ID, worker.Name)
			return "rejected"
		}
		totalMs := elapsedMs(taskStarted)
		return WriteConcludeResult(client, projectID, intent.ID, worker.Name, deref(description), "explore_execute", executeMs, &totalMs)
	}

	if DidTimeout(first) {
		log.Warnf("explore timed out project=%s intent=%s worker=%s execute_ms=%d total_ms=%d stdout_preview=%s stderr_preview=%s",
			projectID, intent.ID, worker.Name, executeMs, elapsedMs(taskStarted),
			Preview(first.Stdout, logPreviewLimit), Preview(first.Stderr, logPreviewLimit))
		return runExploreConcludeFallback(dispatch, client, manager, worker, driver, projectID, intent, exportYAML, session, lease, cancellation)
	}

	log.Warnf("explore command failed project=%s intent=%s worker=%s code=%d execute_ms=%d total_ms=%d stdout_preview=%s stderr_preview=%s",
		projectID, intent.ID, worker.Name, first.ReturnCode, executeMs, elapsedMs(taskStarted),
		Preview(first.Stdout, logPreviewLimit), Preview(first.Stderr, logPreviewLimit))
	BestEffortRelease(client, projectID, intent.ID, worker.Name)
	return "failed"
}

func runExploreConcludeFallback(
	dispatch *config.DispatchConfig,
	client Client,
	manager Backend,
	worker *config.WorkerConfig,
	driver workers.Driver,
	projectID string,
	intent *protocol.Intent,
	exportYAML string,
	session *string,
	lease *runtime.HeartbeatLease,
	cancellation *runtime.TaskCancellation,
) string {
	if !driver.SupportsConclude() || session == nil || *session == "" {
		log.Infof("conclude fallback unavailable project=%s intent=%s worker=%s supports_conclude=%t has_session=%t",
			projectID, intent.ID, worker.Name, driver.SupportsConclude(), session != nil)
		BestEffortRelease(client, projectID, intent.ID, worker.Name)
		return "failed"
	}
	if lease.Failure() != nil {
		log.Warnf("conclude fallback skipped because heartbeat already lost project=%s intent=%s worker=%s", projectID, intent.ID, worker.Name)
		BestEffortRelease(client, projectID, intent.ID, worker.Name)
		return "failed"
	}
	if cancellation.IsCancelled() {
		log.Infof("conclude fallback skipped because task was cancelled project=%s intent=%s worker=%s reason=%s", projectID, intent.ID, worker.Name, deref(cancellation.Reason()))
		BestEffortRelease(client, projectID, intent.ID, worker.Name)
		return "cancelled"
	}
	if !ProjectAllowsConcludeFallback(client, projectID, worker.Name, intent.ID) {
		BestEffortRelease(client, projectID, intent.ID, worker.Name)
		return "failed"
	}

	containerName, err := manager.EnsureRunning(projectID)
	if err != nil {
		log.Errorf("conclude fallback ensure container failed project=%s error=%s", projectID, err)
		BestEffortRelease(client, projectID, intent.ID, worker.Name)
		return "failed"
	}
	graphReference, err := WriteGraphSnapshotReference(manager, containerName, trimSpace(exportYAML), "explore_conclude")
	if err != nil {
		log.Errorf("conclude fallback snapshot failed project=%s error=%s", projectID, err)
		BestEffortRelease(client, projectID, intent.ID, worker.Name)
		return "failed"
	}
	template, err := prompting.Load(dispatch.Runtime.PromptGroup, "explore_conclude.md")
	if err != nil {
		log.Errorf("conclude prompt missing project=%s error=%s", projectID, err)
		BestEffortRelease(client, projectID, intent.ID, worker.Name)
		return "failed"
	}
	prompt := prompting.Render(template, map[string]string{
		"graph_yaml":         graphReference,
		"intent_id":          intent.ID,
		"intent_description": intent.Description,
	})
	concludeArgv := driver.BuildConclude(worker, prompt, *session)
	log.Infof("starting conclude fallback project=%s intent=%s worker=%s", projectID, intent.ID, worker.Name)
	concludeStarted := time.Now()
	result, err := RunWorkerProcess(manager, containerName, worker, concludeArgv, "explore_conclude", dispatch.Tasks.Explore.ConcludeTimeout, lease, cancellation)
	if err != nil {
		log.Errorf("conclude fallback failed project=%s error=%s", projectID, err)
		BestEffortRelease(client, projectID, intent.ID, worker.Name)
		return "failed"
	}
	concludeMs := elapsedMs(concludeStarted)
	if CancelReason(result, cancellation) != nil {
		log.Infof("conclude cancelled project=%s intent=%s worker=%s reason=%s conclude_ms=%d", projectID, intent.ID, worker.Name, deref(CancelReason(result, cancellation)), concludeMs)
		BestEffortRelease(client, projectID, intent.ID, worker.Name)
		return "cancelled"
	}
	if lease.Failure() != nil {
		BestEffortRelease(client, projectID, intent.ID, worker.Name)
		return "failed"
	}
	if result.TimedOut || result.ReturnCode != 0 {
		log.Warnf("conclude failed project=%s intent=%s worker=%s code=%d timed_out=%t conclude_ms=%d stdout_preview=%s stderr_preview=%s",
			projectID, intent.ID, worker.Name, result.ReturnCode, result.TimedOut, concludeMs,
			Preview(result.Stdout, logPreviewLimit), Preview(result.Stderr, logPreviewLimit))
		BestEffortRelease(client, projectID, intent.ID, worker.Name)
		return "failed"
	}
	modelOutput := driver.ExtractResponseText(result.Stdout, result.Stderr)
	payload, parseErr := contracts.ParseJSONOutput(modelOutput)
	var kind string
	var description *string
	if parseErr == nil {
		kind, description, parseErr = contracts.ValidateExplorePayload(payload)
	}
	if parseErr != nil {
		log.Warnf("conclude parse failed project=%s intent=%s worker=%s error=%s conclude_ms=%d stdout_preview=%s stderr_preview=%s",
			projectID, intent.ID, worker.Name, parseErr, concludeMs,
			Preview(result.Stdout, logPreviewLimit), Preview(result.Stderr, logPreviewLimit))
		BestEffortRelease(client, projectID, intent.ID, worker.Name)
		return "failed"
	}
	if kind == "rejected" {
		log.Warnf("conclude rejected project=%s intent=%s worker=%s conclude_ms=%d stdout_preview=%s",
			projectID, intent.ID, worker.Name, concludeMs, Preview(result.Stdout, logPreviewLimit))
		BestEffortRelease(client, projectID, intent.ID, worker.Name)
		return "rejected"
	}
	return WriteConcludeResult(client, projectID, intent.ID, worker.Name, deref(description), "explore_conclude", concludeMs, nil)
}
