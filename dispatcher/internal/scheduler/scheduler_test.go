package scheduler

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Indexyz/Cairn/dispatcher/internal/config"
	"github.com/Indexyz/Cairn/dispatcher/internal/protocol"
	"github.com/Indexyz/Cairn/dispatcher/internal/runtime"
)

func testLoop(t *testing.T) *Loop {
	t.Helper()
	dispatch, err := config.Parse([]byte(`
server: http://127.0.0.1:8000
runtime:
  interval: 60
  max_workers: 2
  max_running_projects: 1
  max_project_workers: 2
  healthcheck_timeout: 5
  prompt_group: default
tasks:
  bootstrap: {timeout: 10, conclude_timeout: 5}
  reason: {timeout: 10}
  explore: {timeout: 10, conclude_timeout: 5}
container: {image: i, network_mode: host, completed_action: stop}
workers:
  - {name: only, type: mock, task_types: [bootstrap, reason, explore], max_running: 1, priority: 0}
`))
	if err != nil {
		t.Fatalf("config: %v", err)
	}
	return &Loop{
		Config:               dispatch,
		futures:              map[*RunningTask]bool{},
		cleanupFutures:       map[*CleanupTask]bool{},
		reasonCheckpoints:    map[string]ReasonCheckpoint{},
		runtimeProjectIDs:    map[string]bool{},
		workerUnhealthyUntil: map[string]float64{},
		workerRejectedUntil:  map[string]float64{},
		cleanupPending:       map[string]bool{},
		inactiveCleanupDone:  map[string]string{},
		logState:             map[string]string{},
	}
}

func summary(id, status string) protocol.ProjectSummary {
	return protocol.ProjectSummary{ProjectMeta: protocol.ProjectMeta{ID: id, Title: id, Status: status}}
}

func intent(id, description, creator string, from ...string) protocol.Intent {
	return protocol.Intent{ID: id, Description: description, Creator: creator, From: from, CreatedAt: "2026-01-01T00:00:00Z"}
}

func project(intents ...protocol.Intent) *protocol.ProjectDetail {
	return &protocol.ProjectDetail{
		Project: protocol.ProjectMeta{ID: "proj_001", Status: "active", BootstrapEnabled: true},
		Facts:   []protocol.Fact{{ID: "origin", Description: "start"}, {ID: "goal", Description: "finish"}},
		Intents: intents,
	}
}

func TestChooseWorkerOrdersByPriorityThenRunningCount(t *testing.T) {
	workers := []*config.WorkerConfig{
		{Name: "busy", Priority: 0},
		{Name: "first", Priority: 0},
		{Name: "lower", Priority: 1},
	}

	ordered := chooseWorker(workers, map[string]int{"busy": 2})

	if ordered[0].Name != "first" || ordered[1].Name != "busy" || ordered[2].Name != "lower" {
		names := []string{}
		for _, worker := range ordered {
			names = append(names, worker.Name)
		}
		t.Fatalf("order = %v", names)
	}
}

func TestProjectRequiresBootstrapHonorsFlagAndWorkers(t *testing.T) {
	loop := testLoop(t)
	disabled := project()
	disabled.Project.BootstrapEnabled = false
	if loop.projectRequiresBootstrap(disabled) {
		t.Fatal("disabled bootstrap should not require it")
	}

	loop.Config.Workers[0].TaskTypes = []config.TaskType{config.TaskReason, config.TaskExplore}
	if loop.projectRequiresBootstrap(project()) {
		t.Fatal("no bootstrap worker and no existing intent should skip bootstrap")
	}

	existing := project(intent("i001", bootstrapIntentDescription, bootstrapIntentCreator, "origin"))
	if !loop.projectRequiresBootstrap(existing) {
		t.Fatal("existing bootstrap intent must keep bootstrap before workers change")
	}
}

func TestInitialProjectDetectionAllowsOnlyBootstrapIntents(t *testing.T) {
	loop := testLoop(t)
	if !loop.isInitialProject(project()) {
		t.Fatal("origin+goal project is initial")
	}
	if !loop.isInitialProject(project(intent("i001", bootstrapIntentDescription, bootstrapIntentCreator, "origin"))) {
		t.Fatal("bootstrap intent keeps the project initial")
	}
	explored := project(intent("i001", "work", "reasoner", "origin"))
	explored.Facts = append(explored.Facts, protocol.Fact{ID: "f001"})
	if loop.isInitialProject(explored) {
		t.Fatal("new fact means no longer initial")
	}
}

func TestBootstrapIntentSelectionPrefersUnclaimed(t *testing.T) {
	claimed := intent("i002", bootstrapIntentDescription, bootstrapIntentCreator, "origin")
	worker := "w"
	claimed.Worker = &worker
	unclaimed := intent("i001", bootstrapIntentDescription, bootstrapIntentCreator, "origin")

	selected := getBootstrapIntent(project(claimed, unclaimed))

	if selected == nil || selected.ID != "i001" {
		t.Fatalf("selected = %+v", selected)
	}
}

func TestBlockedProjectStaysRunnableAndOnlyCountsWhileTasking(t *testing.T) {
	loop := testLoop(t)
	loop.runtimeProjectIDs["blocked"] = true
	cancellation := &runtime.TaskCancellation{}
	loop.futures[&RunningTask{ProjectID: "blocked", TaskType: "explore", WorkerName: "w", Cancellation: cancellation}] = true

	loop.refreshRuntimeProjects([]protocol.ProjectSummary{summary("blocked", "blocked")})
	loop.cancelInactiveTasks([]protocol.ProjectSummary{summary("blocked", "blocked")})

	if !loop.runtimeProjectIDs["blocked"] {
		t.Fatal("blocked project should stay in the runtime")
	}
	if cancellation.Reason() != nil {
		t.Fatal("blocked tasks should not be cancelled")
	}
	if count := loop.runningProjectCount([]protocol.ProjectSummary{summary("blocked", "blocked")}); count != 1 {
		t.Fatalf("running count = %d", count)
	}

	loop.futures = map[*RunningTask]bool{}
	if count := loop.runningProjectCount([]protocol.ProjectSummary{summary("blocked", "blocked")}); count != 0 {
		t.Fatalf("idle blocked count = %d", count)
	}
}

func TestCancelInactiveTasksMarksStoppedAndDeleted(t *testing.T) {
	loop := testLoop(t)
	stopped := &runtime.TaskCancellation{}
	deleted := &runtime.TaskCancellation{}
	loop.futures[&RunningTask{ProjectID: "stopped", TaskType: "explore", WorkerName: "w", Cancellation: stopped}] = true
	loop.futures[&RunningTask{ProjectID: "deleted", TaskType: "reason", WorkerName: "w", Cancellation: deleted}] = true

	loop.cancelInactiveTasks([]protocol.ProjectSummary{summary("stopped", "stopped")})

	if reason := stopped.Reason(); reason == nil || *reason != "stopped" {
		t.Fatalf("stopped reason = %v", reason)
	}
	if reason := deleted.Reason(); reason == nil || *reason != "deleted" {
		t.Fatalf("deleted reason = %v", reason)
	}
}

func TestInitializeReasonCheckpointOnlyForRunnableProjectsWithOpenIntents(t *testing.T) {
	loop := testLoop(t)
	active := summary("active", "active")
	active.UnclaimedIntentCount = 1

	loop.initializeReasonCheckpoints([]protocol.ProjectSummary{
		active,
		summary("idle", "active"),
		summary("stopped", "stopped"),
	})

	if len(loop.reasonCheckpoints) != 1 {
		t.Fatalf("checkpoints = %v", loop.reasonCheckpoints)
	}
	if checkpoint := loop.reasonCheckpoints["active"]; checkpoint.OpenIntentCount != 1 {
		t.Fatalf("checkpoint = %+v", checkpoint)
	}
}

func TestSelectWorkerReportsEveryBlockReason(t *testing.T) {
	loop := testLoop(t)
	busy := &config.WorkerConfig{Name: "busy", TaskTypes: []config.TaskType{config.TaskReason}, MaxRunning: 1}
	unhealthy := &config.WorkerConfig{Name: "unhealthy", TaskTypes: []config.TaskType{config.TaskReason}, MaxRunning: 1}
	rejected := &config.WorkerConfig{Name: "rejected", TaskTypes: []config.TaskType{config.TaskReason}, MaxRunning: 1}
	unsupported := &config.WorkerConfig{Name: "unsupported", TaskTypes: []config.TaskType{config.TaskExplore}, MaxRunning: 1}
	loop.Config.Workers = []config.WorkerConfig{*busy, *unhealthy, *rejected, *unsupported}
	loop.futures[&RunningTask{ProjectID: "proj", TaskType: "reason", WorkerName: "busy", Cancellation: &runtime.TaskCancellation{}}] = true
	now := nowSeconds()
	loop.workerUnhealthyUntil["unhealthy"] = now + 10
	loop.workerRejectedUntil["proj|reason|rejected"] = now + 20

	selection := loop.selectWorker("proj", "reason")

	if selection.Worker != nil {
		t.Fatalf("worker = %v", selection.Worker)
	}
	if len(selection.BlockedBusy) != 1 || selection.BlockedBusy[0] != "busy(1/1)" {
		t.Fatalf("busy = %v", selection.BlockedBusy)
	}
	if len(selection.BlockedUnhealthy) != 1 || !strings.HasPrefix(selection.BlockedUnhealthy[0], "unhealthy(") {
		t.Fatalf("unhealthy = %v", selection.BlockedUnhealthy)
	}
	if len(selection.BlockedRejected) != 1 || !strings.HasPrefix(selection.BlockedRejected[0], "rejected(") {
		t.Fatalf("rejected = %v", selection.BlockedRejected)
	}
	if len(selection.BlockedTaskType) != 1 || selection.BlockedTaskType[0] != "unsupported" {
		t.Fatalf("task type = %v", selection.BlockedTaskType)
	}
}

func TestReasonTriggerReportsGraphChangesOnly(t *testing.T) {
	loop := testLoop(t)
	loop.reasonCheckpoints["proj_001"] = ReasonCheckpoint{FactCount: 2, HintCount: 1, OpenIntentCount: 1}

	unchanged := project(intent("i001", "work", "reasoner", "origin"))
	if trigger := loop.reasonTrigger(unchanged); trigger != nil {
		t.Fatalf("unchanged trigger = %v", *trigger)
	}

	changed := project(intent("i001", "work", "reasoner", "origin"))
	changed.Facts = append(changed.Facts, protocol.Fact{ID: "f001"})
	changed.Hints = []protocol.Hint{{ID: "h001"}, {ID: "h002"}}
	changed.Intents[0].To = stringPtr("f001")
	trigger := loop.reasonTrigger(changed)
	if trigger == nil || *trigger != "facts:2->3,hints:1->2,open_intents:1->0" {
		t.Fatalf("trigger = %v", trigger)
	}
}

func TestReapFuturesRecordsOutcomes(t *testing.T) {
	loop := testLoop(t)
	facts, hints, open := 3, 1, 1
	unhealthyTask := &RunningTask{ProjectID: "p1", TaskType: "explore", WorkerName: "w1", Cancellation: &runtime.TaskCancellation{}, done: make(chan string, 1)}
	rejectedTask := &RunningTask{ProjectID: "p2", TaskType: "reason", WorkerName: "w2", Cancellation: &runtime.TaskCancellation{}, done: make(chan string, 1)}
	successTask := &RunningTask{ProjectID: "p3", TaskType: "reason", WorkerName: "w3", Cancellation: &runtime.TaskCancellation{},
		FactCount: &facts, HintCount: &hints, OpenIntentCount: &open, done: make(chan string, 1)}
	loop.futures[unhealthyTask] = true
	loop.futures[rejectedTask] = true
	loop.futures[successTask] = true
	unhealthyTask.done <- "unhealthy"
	rejectedTask.done <- "rejected"
	successTask.done <- "success"

	loop.reapFutures()

	if len(loop.futures) != 0 {
		t.Fatalf("futures = %d", len(loop.futures))
	}
	if loop.workerUnhealthyUntil["w1"] == 0 {
		t.Fatal("unhealthy worker not marked")
	}
	if loop.workerRejectedUntil["p2|reason|w2"] == 0 {
		t.Fatal("rejected worker not marked")
	}
	checkpoint, ok := loop.reasonCheckpoints["p3"]
	if !ok || checkpoint.FactCount != 3 || checkpoint.OpenIntentCount != 1 {
		t.Fatalf("checkpoint = %+v", checkpoint)
	}
}

func TestLogChangedDeduplicatesUntilCleared(t *testing.T) {
	loop := testLoop(t)
	loop.logChanged("scope", 1, "message %s", "a")
	loop.logChanged("scope", 1, "message %s", "a")
	loop.logChanged("scope", 1, "message %s", "b")

	if loop.logState["scope"] == "" {
		t.Fatal("state missing")
	}
	loop.clearLogState("scope")
	if _, ok := loop.logState["scope"]; ok {
		t.Fatal("state not cleared")
	}
}

var _ = time.Now

func localLoop(t *testing.T, workerType config.WorkerType) *Loop {
	t.Helper()
	loop := testLoop(t)
	loop.Config.Runtime.Execution = config.ExecutionLocal
	loop.Config.Workers = []config.WorkerConfig{{
		Name: "local", Type: workerType,
		TaskTypes: []config.TaskType{config.TaskReason}, MaxRunning: 1,
	}}
	return loop
}

func fakeCLI(t *testing.T, name string) {
	t.Helper()
	binDir := t.TempDir()
	path := binDir + "/" + name
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake cli: %v", err)
	}
	t.Setenv("PATH", binDir)
}

func TestLocalCLICheckPassesWhenCLIIsPresent(t *testing.T) {
	loop := localLoop(t, config.WorkerMock)
	fakeCLI(t, "python3")

	if err := loop.runLocalBinaryCheck(); err != nil {
		t.Fatalf("check: %v", err)
	}
}

func TestLocalCLICheckFailsWhenNoneIsInstalled(t *testing.T) {
	loop := localLoop(t, config.WorkerClaudeCode)
	loop.Config.Workers = append(loop.Config.Workers, config.WorkerConfig{
		Name: "second", Type: config.WorkerPi,
		TaskTypes: []config.TaskType{config.TaskReason}, MaxRunning: 1,
	})
	t.Setenv("PATH", t.TempDir())

	err := loop.runLocalBinaryCheck()
	if err == nil || !strings.Contains(err.Error(), "none of the configured worker CLIs") {
		t.Fatalf("error = %v", err)
	}
}
