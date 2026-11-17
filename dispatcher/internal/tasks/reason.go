package tasks

import (
	"time"

	"github.com/Indexyz/Cairn/dispatcher/internal/config"
	"github.com/Indexyz/Cairn/dispatcher/internal/contracts"
	"github.com/Indexyz/Cairn/dispatcher/internal/prompting"
	"github.com/Indexyz/Cairn/dispatcher/internal/protocol"
	"github.com/Indexyz/Cairn/dispatcher/internal/runtime"
)

// RunReasonTask asks a worker to judge the graph and write a verdict.
func RunReasonTask(
	dispatch *config.DispatchConfig,
	client Client,
	manager Backend,
	project *protocol.ProjectDetail,
	exportYAML string,
	worker *config.WorkerConfig,
	cancellation *runtime.TaskCancellation,
) string {
	driver, err := driverFor(worker.Type, dispatch.Runtime.Execution)
	if err != nil {
		log.Errorf("reason driver missing project=%s worker=%s error=%s", project.Project.ID, worker.Name, err)
		return "failed"
	}
	projectID := project.Project.ID
	taskStarted := time.Now()
	healthcheckTimeout := time.Duration(dispatch.Runtime.HealthcheckTimeout) * time.Second
	lease := runtime.NewHeartbeatLease(
		func() protocol.ApiResult { return client.ReasonHeartbeat(projectID, worker.Name) },
		"project="+projectID+" reason", worker.Name, time.Duration(dispatch.Runtime.Interval)*time.Second,
	)
	lease.Start()
	defer func() {
		lease.Stop()
		BestEffortReleaseReason(client, projectID, worker.Name)
	}()

	containerName, err := manager.EnsureRunning(projectID)
	if err != nil {
		log.Errorf("reason ensure container failed project=%s error=%s", projectID, err)
		return "failed"
	}
	writeMCPConfig(manager, projectID, containerName, worker)

	if TaskHealthcheckEnabled(dispatch) {
		log.Infof("checking worker health project=%s worker=%s timeout=%ds", projectID, worker.Name, dispatch.Runtime.HealthcheckTimeout)
		health := driver.CheckHealth(worker, healthcheckTimeout)
		if cancellation.IsCancelled() {
			log.Infof("reason cancelled during healthcheck project=%s worker=%s reason=%s", projectID, worker.Name, deref(cancellation.Reason()))
			return "cancelled"
		}
		if lease.Failure() != nil {
			log.Warnf("heartbeat lost during reason healthcheck project=%s worker=%s status=%d", projectID, worker.Name, derefInt(lease.Failure().StatusCode))
			return "failed"
		}
		if !health.OK {
			log.Warnf("worker unhealthy project=%s worker=%s status=%d detail=%s", projectID, worker.Name, derefInt(health.Status), health.Detail)
			return "unhealthy"
		}
	}

	openIntents := make([]map[string]any, 0, len(project.Intents))
	for _, intent := range project.Intents {
		if intent.To != nil {
			continue
		}
		openIntents = append(openIntents, map[string]any{
			"id": intent.ID, "from": intent.From, "description": intent.Description, "worker": intent.Worker,
		})
	}
	allowedFactIDs := make([]string, 0, len(project.Facts))
	for _, fact := range project.Facts {
		if fact.ID != "goal" {
			allowedFactIDs = append(allowedFactIDs, fact.ID)
		}
	}
	log.Debugf("reason context prepared project=%s worker=%s facts=%d", projectID, worker.Name, len(project.Facts))

	graphReference, err := WriteGraphSnapshotReference(manager, containerName, trimSpace(exportYAML), "reason_execute")
	if err != nil {
		log.Errorf("reason graph snapshot failed project=%s error=%s", projectID, err)
		return "failed"
	}
	template, err := prompting.Load(dispatch.Runtime.PromptGroup, "reason.md")
	if err != nil {
		log.Errorf("reason prompt missing project=%s error=%s", projectID, err)
		return "failed"
	}
	prompt := prompting.Render(template, map[string]string{
		"graph_yaml":   graphReference,
		"fact_ids":     prompting.FormatFactIDs(allowedFactIDs),
		"open_intents": prompting.FormatOpenIntents(openIntents),
		"max_intents":  itoa(dispatch.Tasks.Reason.MaxIntents),
	})

	session := driver.PrepareSession()
	execute := driver.BuildExecute(worker, prompt, session)
	executeStarted := time.Now()
	result, err := RunWorkerProcess(manager, containerName, worker, execute.Argv, "reason_execute", dispatch.Tasks.Reason.Timeout, lease, cancellation)
	if err != nil {
		log.Errorf("reason execute failed project=%s error=%s", projectID, err)
		return "failed"
	}
	executeMs := elapsedMs(executeStarted)
	totalMs := elapsedMs(taskStarted)
	_ = driver.ExtractSession(execute.Session, result.Stdout, result.Stderr)

	if CancelReason(result, cancellation) != nil {
		log.Infof("reason cancelled project=%s worker=%s reason=%s execute_ms=%d", projectID, worker.Name, deref(CancelReason(result, cancellation)), executeMs)
		return "cancelled"
	}
	if lease.Failure() != nil {
		log.Warnf("heartbeat lost during reason project=%s worker=%s status=%d execute_ms=%d", projectID, worker.Name, derefInt(lease.Failure().StatusCode), executeMs)
		return "failed"
	}
	if DidTimeout(result) {
		log.Warnf("reason timed out project=%s worker=%s execute_ms=%d total_ms=%d stdout_preview=%s stderr_preview=%s",
			projectID, worker.Name, executeMs, totalMs, Preview(result.Stdout, logPreviewLimit), Preview(result.Stderr, logPreviewLimit))
		return "failed"
	}
	if result.ReturnCode != 0 {
		log.Warnf("reason command failed project=%s worker=%s code=%d execute_ms=%d total_ms=%d stdout_preview=%s stderr_preview=%s",
			projectID, worker.Name, result.ReturnCode, executeMs, totalMs,
			Preview(result.Stdout, logPreviewLimit), Preview(result.Stderr, logPreviewLimit))
		return "failed"
	}

	var kind string
	var data any
	modelOutput := driver.ExtractResponseText(result.Stdout, result.Stderr)
	payload, parseErr := contracts.ParseJSONOutput(modelOutput)
	if parseErr == nil {
		kind, data, parseErr = contracts.ValidateReasonPayload(payload, len(openIntents) == 0, dispatch.Tasks.Reason.MaxIntents)
	}
	if parseErr != nil {
		log.Warnf("reason parse failed project=%s worker=%s error=%s execute_ms=%d total_ms=%d stdout_preview=%s stderr_preview=%s",
			projectID, worker.Name, parseErr, executeMs, totalMs,
			Preview(result.Stdout, logPreviewLimit), Preview(result.Stderr, logPreviewLimit))
		return "failed"
	}

	switch kind {
	case "rejected":
		log.Warnf("reason rejected project=%s worker=%s execute_ms=%d total_ms=%d stdout_preview=%s",
			projectID, worker.Name, executeMs, totalMs, Preview(result.Stdout, logPreviewLimit))
		return "rejected"
	case "complete":
		complete := data.(map[string]any)
		from := stringSlice(complete["from"])
		description := stringValue(complete["description"])
		response := client.Complete(projectID, from, description, worker.Name)
		if response.StatusCode == 403 {
			log.Infof("project became inactive during reason complete project=%s worker=%s", projectID, worker.Name)
			return "success"
		}
		if !response.Ok() {
			log.Warnf("reason complete write failed project=%s worker=%s status=%d body=%s", projectID, worker.Name, response.StatusCode, response.Text)
			return "failed"
		}
		log.Infof("project completed project=%s worker=%s from=%v execute_ms=%d total_ms=%d", projectID, worker.Name, from, executeMs, totalMs)
		return "success"
	case "intents":
		list := data.([]any)
		created := 0
		for _, item := range list {
			intentData := item.(map[string]any)
			from := stringSlice(intentData["from"])
			description := stringValue(intentData["description"])
			response := client.CreateIntent(projectID, from, description, worker.Name)
			if response.StatusCode == 403 {
				log.Infof("project became inactive during reason intent create project=%s worker=%s created=%d", projectID, worker.Name, created)
				return "success"
			}
			if response.StatusCode == 409 {
				log.Infof("reason intent lost race project=%s worker=%s from=%v", projectID, worker.Name, from)
				continue
			}
			if !response.Ok() {
				log.Warnf("reason intent write failed project=%s worker=%s status=%d body=%s", projectID, worker.Name, response.StatusCode, response.Text)
				continue
			}
			created++
			log.Infof("reason created intent project=%s worker=%s from=%v description=%s", projectID, worker.Name, from, description)
		}
		log.Infof("reason finished project=%s worker=%s created_intents=%d/%d execute_ms=%d total_ms=%d", projectID, worker.Name, created, len(list), executeMs, totalMs)
		if created == 0 {
			log.Warnf("reason created no intents project=%s worker=%s attempted=%d execute_ms=%d total_ms=%d", projectID, worker.Name, len(list), executeMs, totalMs)
			return "failed"
		}
		return "success"
	}

	log.Infof("reason finished without graph change project=%s worker=%s execute_ms=%d total_ms=%d", projectID, worker.Name, executeMs, totalMs)
	return "success"
}
