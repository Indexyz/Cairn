// Package scheduler runs the dispatch loop.
package scheduler

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/Indexyz/Cairn/dispatcher/internal/config"
	"github.com/Indexyz/Cairn/dispatcher/internal/logging"
	"github.com/Indexyz/Cairn/dispatcher/internal/protocol"
	"github.com/Indexyz/Cairn/dispatcher/internal/runtime"
	"github.com/Indexyz/Cairn/dispatcher/internal/tasks"
	"github.com/Indexyz/Cairn/dispatcher/internal/workers/registry"
)

var log = logging.New("cairn.dispatcher.scheduler.loop")

const (
	instanceIDEnv              = "CAIRN_INSTANCE_ID"
	unhealthyRetryAfterSeconds = 5.0
	rejectedRetryAfterSeconds  = 5.0
	bootstrapIntentDescription = "bootstrap"
	bootstrapIntentCreator     = "dispatcher.bootstrap"
)

// RunnableStatuses may still run tasks; blocked declares no new intents.
var RunnableStatuses = map[string]bool{"active": true, "blocked": true}

// ResolveInstanceID returns this dispatcher's lease identity.
func ResolveInstanceID() string {
	if configured := strings.TrimSpace(os.Getenv(instanceIDEnv)); configured != "" {
		return configured
	}
	return uuid.NewString()[:8]
}

// RunningTask is one in-flight task.
type RunningTask struct {
	ProjectID       string
	TaskType        string
	WorkerName      string
	Cancellation    *runtime.TaskCancellation
	IntentID        *string
	FactCount       *int
	HintCount       *int
	OpenIntentCount *int
	done            chan string
}

// CleanupTask is one in-flight container cleanup.
type CleanupTask struct {
	container    string
	projectID    *string
	targetStatus *string
	run          func() bool
	done         chan bool
}

// ReasonCheckpoint remembers the graph state a reason run started from.
type ReasonCheckpoint struct {
	FactCount       int
	HintCount       int
	OpenIntentCount int
}

// WorkerSelection is the candidate evaluation for one task type.
type WorkerSelection struct {
	Worker           *config.WorkerConfig
	BlockedBusy      []string
	BlockedUnhealthy []string
	BlockedRejected  []string
	BlockedTaskType  []string
}

// Loop is the dispatcher.
type Loop struct {
	configPath string
	Config     *config.DispatchConfig
	InstanceID string
	client     *protocol.Client
	backend    runtime.Backend

	mu                   sync.Mutex
	futures              map[*RunningTask]bool
	cleanupFutures       map[*CleanupTask]bool
	reasonCheckpoints    map[string]ReasonCheckpoint
	runtimeProjectIDs    map[string]bool
	workerUnhealthyUntil map[string]float64
	workerRejectedUntil  map[string]float64
	globalLoad           protocol.LoadSnapshot
	serverSettings       protocol.Settings
	logState             map[string]string
	cleanupPending       map[string]bool
	inactiveCleanupDone  map[string]string
	projectCursor        int
	settingsChecked      bool
	startupChecked       bool
	stop                 chan struct{}
	done                 chan struct{}
	stopOnce             sync.Once
}

// New loads the config and connects the backend.
func New(configPath string) (*Loop, error) {
	dispatch, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}
	loop := &Loop{
		configPath:           configPath,
		Config:               dispatch,
		InstanceID:           ResolveInstanceID(),
		futures:              map[*RunningTask]bool{},
		cleanupFutures:       map[*CleanupTask]bool{},
		reasonCheckpoints:    map[string]ReasonCheckpoint{},
		runtimeProjectIDs:    map[string]bool{},
		workerUnhealthyUntil: map[string]float64{},
		workerRejectedUntil:  map[string]float64{},
		cleanupPending:       map[string]bool{},
		inactiveCleanupDone:  map[string]string{},
		logState:             map[string]string{},
		stop:                 make(chan struct{}),
		done:                 make(chan struct{}),
	}
	log.Infof("dispatcher instance id=%s", loop.InstanceID)
	loop.client = protocol.NewClient(dispatch.Server, 10*time.Second, loop.InstanceID)
	if dispatch.Runtime.Execution == config.ExecutionLocal {
		local := dispatch.Local
		if local == nil {
			local = &config.LocalConfig{CompletedAction: config.LocalKeep}
		}
		loop.backend = runtime.NewLocalBackend(local)
	} else {
		manager, err := runtime.NewContainerManager(dispatch.Container, loop.InstanceID)
		if err != nil {
			return nil, err
		}
		loop.backend = manager
	}
	return loop, nil
}

// Close waits for tasks and releases resources.
func (loop *Loop) Close() {
	loop.mu.Lock()
	running := len(loop.futures)
	projects := map[string]bool{}
	for task := range loop.futures {
		projects[task.ProjectID] = true
	}
	loop.mu.Unlock()
	if running > 0 {
		log.Infof("dispatcher shutting down waiting_for_tasks=%d running_projects=%v", running, sortedKeys(projects))
	}
	for {
		loop.mu.Lock()
		remaining := len(loop.futures) + len(loop.cleanupFutures)
		loop.mu.Unlock()
		if remaining == 0 {
			break
		}
		loop.reapFutures()
		loop.reapCleanupFutures()
		time.Sleep(50 * time.Millisecond)
	}
	loop.backend.Close()
	loop.client.Close()
}

// Run dispatches until stopped or, with once, for a single iteration.
func (loop *Loop) Run(once bool) error {
	defer loop.Close()
	defer close(loop.done)
	if err := loop.syncWorkers(true); err != nil {
		return err
	}
	if err := loop.runStartupHealthchecks(false, false); err != nil {
		return err
	}
	for {
		if loop.stopped() {
			return nil
		}
		if err := loop.syncWorkers(false); err != nil {
			log.Warnf("failed to reload worker models error=%s", err)
		}
		if err := loop.refreshSettings(); err != nil {
			if !loop.settingsChecked {
				return err
			}
			log.Warnf("failed to refresh server settings error=%s", err)
		}
		if !loop.settingsChecked {
			if err := loop.validateServerSettings(); err != nil {
				return err
			}
			loop.settingsChecked = true
		}
		loop.reapFutures()
		loop.reapCleanupFutures()
		loop.refreshGlobalLoad()
		summaries, err := loop.client.ListProjects()
		if err != nil {
			if once {
				return err
			}
			log.Warnf("dispatcher server request failed error=%s retry_in=%ds", err, loop.Config.Runtime.Interval)
			loop.sleep(time.Duration(loop.Config.Runtime.Interval) * time.Second)
			continue
		}
		loop.initializeReasonCheckpoints(summaries)
		loop.refreshRuntimeProjects(summaries)
		loop.cancelInactiveTasks(summaries)
		loop.queueContainerCleanups(summaries)
		loop.dispatchAvailable(summaries)
		if once {
			break
		}
		loop.sleep(time.Duration(loop.Config.Runtime.Interval) * time.Second)
	}
	return nil
}

// Stop asks Run to exit after the current iteration.
func (loop *Loop) Stop() {
	if loop.stop == nil {
		return
	}
	loop.stopOnce.Do(func() { close(loop.stop) })
}

// Wait blocks until Run returned.
func (loop *Loop) Wait() {
	if loop.done != nil {
		<-loop.done
	}
}

func (loop *Loop) stopped() bool {
	if loop.stop == nil {
		return false
	}
	select {
	case <-loop.stop:
		return true
	default:
		return false
	}
}

func (loop *Loop) sleep(duration time.Duration) {
	if loop.stop == nil {
		time.Sleep(duration)
		return
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-loop.stop:
	case <-timer.C:
	}
}

// RunStartupHealthchecksOnly probes workers once and exits.
func (loop *Loop) RunStartupHealthchecksOnly() error {
	defer loop.Close()
	if err := loop.syncWorkers(true); err != nil {
		return err
	}
	return loop.runStartupHealthchecks(true, true)
}

func (loop *Loop) refreshGlobalLoad() {
	load, err := loop.client.GetLoad()
	if err != nil {
		log.Warnf("failed to refresh global load error=%s", err)
		return
	}
	loop.mu.Lock()
	loop.globalLoad = *load
	loop.mu.Unlock()
}

func (loop *Loop) globalLoadSnapshot() protocol.LoadSnapshot {
	loop.mu.Lock()
	defer loop.mu.Unlock()
	return loop.globalLoad
}

func (loop *Loop) refreshSettings() error {
	settings, err := loop.client.GetSettings()
	if err != nil {
		return err
	}
	loop.mu.Lock()
	loop.serverSettings = *settings
	loop.mu.Unlock()
	return nil
}

func (loop *Loop) settingsSnapshot() protocol.Settings {
	loop.mu.Lock()
	defer loop.mu.Unlock()
	return loop.serverSettings
}

// maxWorkers prefers the server cap over dispatch.yaml; zero means unset there.
func (loop *Loop) maxWorkers() int {
	if value := loop.settingsSnapshot().MaxWorkers; value > 0 {
		return value
	}
	return loop.Config.Runtime.MaxWorkers
}

func (loop *Loop) maxRunningProjects() int {
	if value := loop.settingsSnapshot().MaxRunningProjects; value > 0 {
		return value
	}
	return loop.Config.Runtime.MaxRunningProjects
}

func (loop *Loop) maxProjectWorkers() int {
	if value := loop.settingsSnapshot().MaxProjectWorkers; value > 0 {
		return value
	}
	return loop.Config.Runtime.MaxProjectWorkers
}

func (loop *Loop) globalRunningTasks() int {
	loop.mu.Lock()
	local := len(loop.futures)
	loop.mu.Unlock()
	global := loop.globalLoadSnapshot().RunningTasks
	if local > global {
		return local
	}
	return global
}

func (loop *Loop) globalProjectTasks(projectID string) int {
	local := loop.projectRunningTaskCount(projectID)
	global := loop.globalLoadSnapshot().Projects[projectID]
	if local > global {
		return local
	}
	return global
}

func (loop *Loop) globalRunningProjects(summaries []protocol.ProjectSummary) int {
	local := loop.runningProjectCount(summaries)
	global := loop.globalLoadSnapshot().RunningProjects
	if local > global {
		return local
	}
	return global
}

func workerSignature(workers []config.WorkerConfig) string {
	builder := strings.Builder{}
	for _, worker := range workers {
		fmt.Fprintf(&builder, "%s|%s|%v|%d|%d|", worker.Name, worker.Type, worker.TaskTypes, worker.MaxRunning, worker.Priority)
		keys := make([]string, 0, len(worker.Env))
		for key := range worker.Env {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(&builder, "%s=%s;", key, worker.Env[key])
		}
		builder.WriteString("\n")
	}
	return builder.String()
}

func (loop *Loop) syncWorkers(seed bool) error {
	previous := workerSignature(loop.Config.Workers)
	models, err := loop.client.ListModels()
	if err != nil {
		return err
	}
	if seed && len(models) == 0 && len(loop.Config.Workers) > 0 {
		seeded, seedErr := loop.client.ReplaceModels(loop.Config.Workers)
		if seedErr != nil {
			return seedErr
		}
		models = seeded
		log.Infof("seeded %d worker model(s) from %s", len(models), loop.configPath)
	}

	if len(models) == 0 {
		loop.Config.Workers = nil
		loop.workerUnhealthyUntil = map[string]float64{}
		loop.workerRejectedUntil = map[string]float64{}
		loop.logChanged("workers/idle", logging.Warning, "no worker models configured, idling until models are added in the UI or in dispatch.yaml")
		return nil
	}
	loop.clearLogState("workers/idle")

	workers := make([]config.WorkerConfig, 0, len(models))
	for _, model := range models {
		worker := model.WorkerConfig
		merged := map[string]string{}
		for key, value := range loop.Config.CommonEnv {
			merged[key] = value
		}
		for key, value := range worker.Env {
			merged[key] = value
		}
		worker.Env = merged
		if loop.Config.Runtime.Execution == config.ExecutionContainer {
			var missing []string
			for _, key := range config.WorkerEnvKeys[worker.Type] {
				if worker.Env[key] == "" {
					missing = append(missing, key)
				}
			}
			if len(missing) > 0 {
				log.Warnf("worker %s missing env keys: %s", worker.Name, strings.Join(missing, ", "))
			}
		}
		workers = append(workers, worker)
	}
	loop.Config.Workers = workers
	if workerSignature(workers) == previous {
		return nil
	}
	names := make([]string, 0, len(workers))
	for _, worker := range workers {
		names = append(names, worker.Name)
	}
	log.Infof("worker models reloaded count=%d names=%v", len(workers), names)
	loop.workerUnhealthyUntil = map[string]float64{}
	loop.workerRejectedUntil = map[string]float64{}
	loop.startupChecked = false
	if err := loop.runStartupHealthchecks(false, false); err != nil {
		log.Errorf("worker model healthcheck failed error=%s", err)
	}
	return nil
}

func (loop *Loop) runStartupHealthchecks(showCommands, force bool) error {
	if loop.startupChecked {
		return nil
	}
	if len(loop.Config.Workers) == 0 {
		log.Warnf("skip worker healthchecks because no worker models are configured")
		return nil
	}
	if loop.Config.Runtime.Execution == config.ExecutionLocal {
		if err := loop.runLocalBinaryCheck(); err != nil {
			return err
		}
		loop.startupChecked = true
		return nil
	}
	if !force && loop.Config.Runtime.WorkerHealthcheck == config.HealthcheckDisabled {
		log.Infof("skip startup worker healthchecks because runtime.worker_healthcheck=disabled")
		loop.startupChecked = true
		return nil
	}
	results := runtime.RunStartupHealthchecks(loop.Config, showCommands)
	loop.startupChecked = true
	for _, result := range results {
		if result.OK {
			return nil
		}
	}
	return fmt.Errorf("%s", runtime.FormatFailureSummary(results))
}

func (loop *Loop) runLocalBinaryCheck() error {
	binaries := map[string][]string{}
	for _, worker := range loop.Config.Workers {
		driver, err := registry.Get(worker.Type, config.ExecutionLocal)
		if err != nil {
			continue
		}
		binary := driver.LocalBinary()
		if binary == nil {
			continue
		}
		binaries[*binary] = append(binaries[*binary], worker.Name)
	}
	if len(binaries) == 0 {
		return nil
	}
	log.Infof("[*] Local execution: checking %d worker CLI(s) on this host", len(binaries))
	names := make([]string, 0, len(binaries))
	for binary := range binaries {
		names = append(names, binary)
	}
	sort.Strings(names)

	available := []string{}
	missing := []string{}
	for _, binary := range names {
		workers := strings.Join(sortedCopy(binaries[binary]), ", ")
		path, runnable := probeLocalCLI(binary)
		switch {
		case path == "":
			missing = append(missing, binary)
			log.Errorf("[-] %-8s not found on PATH (workers: %s)", binary, workers)
		case runnable:
			available = append(available, binary)
			log.Infof("[+] %-8s %s (workers: %s)", binary, path, workers)
		default:
			available = append(available, binary)
			log.Warnf("[!] %-8s %s found but `%s --help` failed (workers: %s)", binary, path, binary, workers)
		}
	}
	if len(available) == 0 {
		return fmt.Errorf("local execution: none of the configured worker CLIs are installed on PATH (%s). Install them and make sure each runs directly from your shell, then retry.", strings.Join(names, ", "))
	}
	if len(missing) > 0 {
		log.Warnf("[!] Missing CLIs, their workers cannot run: %s. Install them or drop those workers.", strings.Join(sortedCopy(missing), ", "))
	}
	log.Warnf("[!] Local mode uses each CLI's own host config: make sure %s already logged in / configured and usable directly (e.g. `claude -p ...` works) — Cairn injects no API keys.", strings.Join(sortedCopy(available), ", "))
	return nil
}

func probeLocalCLI(binary string) (string, bool) {
	path, err := exec.LookPath(binary)
	if err != nil {
		return "", false
	}
	command := exec.Command(binary, "--help")
	command.Stdout = nil
	command.Stderr = nil
	done := make(chan error, 1)
	if err := command.Start(); err != nil {
		return path, false
	}
	go func() { done <- command.Wait() }()
	select {
	case err := <-done:
		return path, err == nil
	case <-time.After(15 * time.Second):
		_ = command.Process.Kill()
		return path, false
	}
}

func (loop *Loop) dispatchAvailable(summaries []protocol.ProjectSummary) {
	if len(loop.Config.Workers) == 0 {
		loop.logChanged("dispatch/global", logging.Info, "skip dispatch because no worker models are configured")
		return
	}
	if loop.globalRunningTasks() >= loop.maxWorkers() {
		loop.logChanged("dispatch/global", logging.Info, "skip dispatch because max_workers reached running_tasks=%d", loop.globalRunningTasks())
		return
	}
	runnable := make([]protocol.ProjectSummary, 0, len(summaries))
	for _, summary := range summaries {
		if RunnableStatuses[summary.Status] {
			runnable = append(runnable, summary)
		}
	}
	if len(runnable) == 0 {
		loop.logChanged("dispatch/global", logging.Info, "skip dispatch because no runnable projects")
		return
	}

	loop.mu.Lock()
	runningProjects := []protocol.ProjectSummary{}
	idleProjects := []protocol.ProjectSummary{}
	for _, summary := range runnable {
		if loop.runtimeProjectIDs[summary.ID] {
			runningProjects = append(runningProjects, summary)
		} else {
			idleProjects = append(idleProjects, summary)
		}
	}
	runningProjects = loop.orderedProjects(runningProjects)
	idleProjects = loop.orderedProjects(idleProjects)
	loop.mu.Unlock()

	dispatched := true
	for dispatched && loop.globalRunningTasks() < loop.maxWorkers() {
		dispatched = false
		for index := range runningProjects {
			if loop.tryDispatchProject(&runningProjects[index]) {
				dispatched = true
				if loop.globalRunningTasks() >= loop.maxWorkers() {
					return
				}
			}
		}
		if dispatched {
			continue
		}
		if loop.globalRunningProjects(runnable) >= loop.maxRunningProjects() {
			loop.logChanged("dispatch/idle-limit", logging.Info, "skip idle project dispatch because max_running_projects reached running_projects=%d", loop.globalRunningProjects(runnable))
			return
		}
		for index := range idleProjects {
			if loop.globalRunningProjects(runnable) >= loop.maxRunningProjects() {
				loop.logChanged("dispatch/idle-limit", logging.Info, "stop idle project dispatch because max_running_projects reached running_projects=%d", loop.globalRunningProjects(runnable))
				return
			}
			if loop.tryDispatchProject(&idleProjects[index]) {
				dispatched = true
				break
			}
		}
	}
}

func (loop *Loop) orderedProjects(summaries []protocol.ProjectSummary) []protocol.ProjectSummary {
	if len(summaries) == 0 {
		return nil
	}
	ids := make([]string, 0, len(summaries))
	for _, summary := range summaries {
		ids = append(ids, summary.ID)
	}
	sort.Strings(ids)
	offset := loop.projectCursor % len(ids)
	orderedIDs := append(append([]string{}, ids[offset:]...), ids[:offset]...)
	loop.projectCursor++
	byID := map[string]protocol.ProjectSummary{}
	for _, summary := range summaries {
		byID[summary.ID] = summary
	}
	ordered := make([]protocol.ProjectSummary, 0, len(orderedIDs))
	for _, id := range orderedIDs {
		ordered = append(ordered, byID[id])
	}
	return ordered
}

func (loop *Loop) tryDispatchProject(summary *protocol.ProjectSummary) bool {
	skipScope := "project:" + summary.ID + ":skip"
	containerName := loop.backend.ContainerName(summary.ID)
	loop.mu.Lock()
	pending := loop.cleanupPending[containerName]
	loop.mu.Unlock()
	if pending {
		loop.logChanged(skipScope+":cleanup_pending", logging.Debug, "skip project=%s because container cleanup is still pending container=%s", summary.ID, containerName)
		return false
	}
	if loop.projectRunningTaskCount(summary.ID) >= loop.maxProjectWorkers() {
		loop.logChanged(skipScope+":max_project_workers", logging.Info, "skip project=%s because max_project_workers reached running_tasks=%v", summary.ID, loop.projectRunningTaskSummary(summary.ID))
		return false
	}

	project, err := loop.client.GetProject(summary.ID)
	if err != nil {
		log.Warnf("skip project=%s because fetch failed error=%s", summary.ID, err)
		return false
	}
	if !RunnableStatuses[project.Project.Status] {
		loop.logChanged(skipScope+":status", logging.Info, "skip project=%s because status=%s", summary.ID, project.Project.Status)
		return false
	}
	if loop.isInitialProject(project) {
		if project.Project.Reason != nil {
			return false
		}
		if loop.projectRequiresBootstrap(project) {
			return loop.dispatchInitialProject(project)
		}
		exportYAML, err := loop.client.ExportProject(summary.ID)
		if err != nil {
			log.Warnf("export failed project=%s error=%s", summary.ID, err)
			return false
		}
		return loop.dispatchReason(project, exportYAML, "initial")
	}
	if project.Project.Reason == nil {
		if trigger := loop.reasonTrigger(project); trigger != nil {
			exportYAML, err := loop.client.ExportProject(summary.ID)
			if err != nil {
				log.Warnf("export failed project=%s error=%s", summary.ID, err)
				return false
			}
			return loop.dispatchReason(project, exportYAML, *trigger)
		}
	}
	runningIntentIDs := loop.projectRunningExploreIntents(summary.ID)
	unclaimed := []protocol.Intent{}
	for _, intent := range project.Intents {
		if intent.To == nil && intent.Worker == nil && !runningIntentIDs[intent.ID] && !isBootstrapIntent(&intent) {
			unclaimed = append(unclaimed, intent)
		}
	}
	if len(runningIntentIDs) > 0 && len(unclaimed) == 0 {
		loop.logChanged(skipScope+":explore_running", logging.Debug, "skip explore project=%s because all unclaimed intents are already running locally intents=%v", summary.ID, sortedKeys(runningIntentIDs))
	}
	if len(unclaimed) > 0 {
		newest := unclaimed[0]
		for _, intent := range unclaimed[1:] {
			if newestIntentBefore(&newest, &intent) {
				newest = intent
			}
		}
		exportYAML, err := loop.client.ExportProject(summary.ID)
		if err != nil {
			log.Warnf("export failed project=%s error=%s", summary.ID, err)
			return false
		}
		return loop.dispatchExplore(project, exportYAML, &newest)
	}
	if project.Project.Reason != nil {
		loop.logChanged(skipScope+":reason_claimed", logging.Debug, "skip reason project=%s because reason is already claimed by %s", summary.ID, project.Project.Reason.Worker)
		return false
	}
	loop.logChanged(skipScope+":graph_unchanged", logging.Debug, "skip reason project=%s because reason state unchanged facts=%d hints=%d open_intents=%d intents=%d",
		summary.ID, len(project.Facts), len(project.Hints), loop.projectOpenIntentCount(project), len(project.Intents))
	return false
}

func newestIntentBefore(current, candidate *protocol.Intent) bool {
	return candidate.CreatedAt > current.CreatedAt
}

func (loop *Loop) dispatchInitialProject(project *protocol.ProjectDetail) bool {
	intent := getBootstrapIntent(project)
	if intent == nil {
		intent = loop.createBootstrapIntent(project.Project.ID)
		if intent == nil {
			return false
		}
	}
	projectID := project.Project.ID
	if loop.projectHasRunningBootstrap(projectID) {
		loop.logChanged("project:"+projectID+":skip:bootstrap_running", logging.Debug, "skip bootstrap project=%s because bootstrap task is already running locally", projectID)
		return false
	}
	if intent.Worker != nil {
		loop.logChanged("project:"+projectID+":skip:bootstrap_claimed", logging.Debug, "skip bootstrap project=%s because bootstrap intent=%s is already claimed by %s", projectID, intent.ID, *intent.Worker)
		return false
	}
	return loop.dispatchBootstrap(project, intent)
}

func (loop *Loop) dispatchReason(project *protocol.ProjectDetail, exportYAML, trigger string) bool {
	projectID := project.Project.ID
	selection := loop.selectWorker(projectID, "reason")
	worker := selection.Worker
	if worker == nil {
		loop.logChanged("project:"+projectID+":worker:reason", logging.Info,
			"no worker available for reason project=%s blocked_busy=%v blocked_unhealthy=%v blocked_rejected=%v",
			projectID, selection.BlockedBusy, selection.BlockedUnhealthy, selection.BlockedRejected)
		return false
	}
	loop.clearLogState("project:" + projectID + ":worker:reason")
	claim := loop.client.ClaimReason(projectID, worker.Name, trigger)
	if claim.StatusCode == 403 || claim.StatusCode == 409 {
		level := logging.Info
		if claim.StatusCode == 409 {
			level = logging.Warning
		}
		loop.logAt(level, "reason claim failed project=%s worker=%s status=%d", projectID, worker.Name, claim.StatusCode)
		return false
	}
	if !claim.Ok() {
		log.Warnf("reason claim failed project=%s worker=%s status=%d", projectID, worker.Name, claim.StatusCode)
		return false
	}
	facts := len(project.Facts)
	hints := len(project.Hints)
	open := loop.projectOpenIntentCount(project)
	cancellation := &runtime.TaskCancellation{}
	task := &RunningTask{
		ProjectID: projectID, TaskType: "reason", WorkerName: worker.Name,
		Cancellation: cancellation, FactCount: &facts, HintCount: &hints, OpenIntentCount: &open,
		done: make(chan string, 1),
	}
	loop.submit(task, func() string {
		return runReason(loop, project, exportYAML, worker, cancellation)
	})
	loop.clearProjectLogState(projectID)
	log.Infof("dispatched reason project=%s worker=%s trigger=%s", projectID, worker.Name, trigger)
	return true
}

func (loop *Loop) dispatchBootstrap(project *protocol.ProjectDetail, intent *protocol.Intent) bool {
	projectID := project.Project.ID
	selection := loop.selectWorker(projectID, "bootstrap")
	worker := selection.Worker
	if worker == nil {
		loop.logChanged("project:"+projectID+":worker:bootstrap", logging.Info,
			"no worker available for bootstrap project=%s intent=%s blocked_busy=%v blocked_unhealthy=%v blocked_rejected=%v",
			projectID, intent.ID, selection.BlockedBusy, selection.BlockedUnhealthy, selection.BlockedRejected)
		return false
	}
	loop.clearLogState("project:" + projectID + ":worker:bootstrap")
	claim := loop.client.Heartbeat(projectID, intent.ID, worker.Name)
	if claim.StatusCode == 403 || claim.StatusCode == 409 {
		level := logging.Info
		if claim.StatusCode == 409 {
			level = logging.Warning
		}
		loop.logAt(level, "bootstrap claim failed project=%s intent=%s worker=%s status=%d", projectID, intent.ID, worker.Name, claim.StatusCode)
		return false
	}
	if !claim.Ok() {
		log.Warnf("bootstrap claim failed project=%s intent=%s worker=%s status=%d", projectID, intent.ID, worker.Name, claim.StatusCode)
		return false
	}
	cancellation := &runtime.TaskCancellation{}
	intentID := intent.ID
	task := &RunningTask{
		ProjectID: projectID, TaskType: "bootstrap", WorkerName: worker.Name,
		Cancellation: cancellation, IntentID: &intentID, done: make(chan string, 1),
	}
	loop.submit(task, func() string {
		return runBootstrap(loop, project, intent, worker, cancellation)
	})
	loop.clearProjectLogState(projectID)
	log.Infof("dispatched bootstrap project=%s intent=%s worker=%s", projectID, intent.ID, worker.Name)
	return true
}

func (loop *Loop) dispatchExplore(project *protocol.ProjectDetail, exportYAML string, intent *protocol.Intent) bool {
	projectID := project.Project.ID
	selection := loop.selectWorker(projectID, "explore")
	worker := selection.Worker
	if worker == nil {
		loop.logChanged("project:"+projectID+":worker:explore", logging.Info,
			"no worker available for explore project=%s intent=%s blocked_busy=%v blocked_unhealthy=%v blocked_rejected=%v",
			projectID, intent.ID, selection.BlockedBusy, selection.BlockedUnhealthy, selection.BlockedRejected)
		return false
	}
	loop.clearLogState("project:" + projectID + ":worker:explore")
	claim := loop.client.Heartbeat(projectID, intent.ID, worker.Name)
	if claim.StatusCode == 403 || claim.StatusCode == 409 {
		level := logging.Info
		if claim.StatusCode == 409 {
			level = logging.Warning
		}
		loop.logAt(level, "explore claim failed project=%s intent=%s worker=%s status=%d", projectID, intent.ID, worker.Name, claim.StatusCode)
		return false
	}
	if !claim.Ok() {
		log.Warnf("explore claim failed project=%s intent=%s worker=%s status=%d", projectID, intent.ID, worker.Name, claim.StatusCode)
		return false
	}
	cancellation := &runtime.TaskCancellation{}
	intentID := intent.ID
	task := &RunningTask{
		ProjectID: projectID, TaskType: "explore", WorkerName: worker.Name,
		Cancellation: cancellation, IntentID: &intentID, done: make(chan string, 1),
	}
	loop.submit(task, func() string {
		return runExplore(loop, project, exportYAML, intent, worker, cancellation)
	})
	loop.clearProjectLogState(projectID)
	log.Infof("dispatched explore project=%s intent=%s worker=%s", projectID, intent.ID, worker.Name)
	return true
}

func (loop *Loop) selectWorker(projectID, taskType string) WorkerSelection {
	now := float64(time.Now().UnixNano()) / 1e9
	runningCounts := loop.workerCounts()
	globalLoad := loop.globalLoadSnapshot()
	selection := WorkerSelection{}
	candidates := []*config.WorkerConfig{}
	for index := range loop.Config.Workers {
		worker := &loop.Config.Workers[index]
		eligible := false
		for _, typeName := range worker.TaskTypes {
			if string(typeName) == taskType {
				eligible = true
				break
			}
		}
		if !eligible {
			selection.BlockedTaskType = append(selection.BlockedTaskType, worker.Name)
			continue
		}
		running := runningCounts[worker.Name]
		if global := globalLoad.Workers[worker.Name]; global > running {
			running = global
		}
		if running >= worker.MaxRunning {
			selection.BlockedBusy = append(selection.BlockedBusy, fmt.Sprintf("%s(%d/%d)", worker.Name, running, worker.MaxRunning))
			continue
		}
		if until := loop.workerUnhealthyUntil[worker.Name]; until > now {
			selection.BlockedUnhealthy = append(selection.BlockedUnhealthy, fmt.Sprintf("%s(%.1fs)", worker.Name, until-now))
			continue
		}
		if until := loop.workerRejectedUntil[projectID+"|"+taskType+"|"+worker.Name]; until > now {
			selection.BlockedRejected = append(selection.BlockedRejected, fmt.Sprintf("%s(%.1fs)", worker.Name, until-now))
			continue
		}
		candidates = append(candidates, worker)
	}
	if len(candidates) == 0 {
		log.Debugf("worker selection project=%s task=%s no candidates blocked_busy=%v blocked_unhealthy=%v blocked_rejected=%v blocked_task_type=%v",
			projectID, taskType, selection.BlockedBusy, selection.BlockedUnhealthy, selection.BlockedRejected, selection.BlockedTaskType)
		return selection
	}
	ordered := chooseWorker(candidates, runningCounts)
	selection.Worker = ordered[0]
	return selection
}

func (loop *Loop) workerCounts() map[string]int {
	counts := map[string]int{}
	loop.mu.Lock()
	for task := range loop.futures {
		counts[task.WorkerName]++
	}
	loop.mu.Unlock()
	return counts
}

func (loop *Loop) projectRunningTaskCount(projectID string) int {
	count := 0
	loop.mu.Lock()
	for task := range loop.futures {
		if task.ProjectID == projectID {
			count++
		}
	}
	loop.mu.Unlock()
	return count
}

func (loop *Loop) projectRunningTaskSummary(projectID string) []string {
	summary := []string{}
	loop.mu.Lock()
	for task := range loop.futures {
		if task.ProjectID != projectID {
			continue
		}
		if task.IntentID == nil {
			summary = append(summary, task.TaskType+":"+task.WorkerName)
		} else {
			summary = append(summary, task.TaskType+":"+task.WorkerName+":"+*task.IntentID)
		}
	}
	loop.mu.Unlock()
	sort.Strings(summary)
	return summary
}

func (loop *Loop) projectHasRunningBootstrap(projectID string) bool {
	loop.mu.Lock()
	defer loop.mu.Unlock()
	for task := range loop.futures {
		if task.ProjectID == projectID && task.TaskType == "bootstrap" {
			return true
		}
	}
	return false
}

func (loop *Loop) projectRunningExploreIntents(projectID string) map[string]bool {
	out := map[string]bool{}
	loop.mu.Lock()
	for task := range loop.futures {
		if task.ProjectID == projectID && task.TaskType == "explore" && task.IntentID != nil {
			out[*task.IntentID] = true
		}
	}
	loop.mu.Unlock()
	return out
}

func (loop *Loop) runningProjectCount(summaries []protocol.ProjectSummary) int {
	runningIDs := map[string]bool{}
	loop.mu.Lock()
	for task := range loop.futures {
		runningIDs[task.ProjectID] = true
	}
	count := 0
	for _, summary := range summaries {
		if summary.Status == "active" && loop.runtimeProjectIDs[summary.ID] {
			count++
		} else if summary.Status == "blocked" && runningIDs[summary.ID] {
			count++
		}
	}
	loop.mu.Unlock()
	return count
}

func (loop *Loop) projectOpenIntentCount(project *protocol.ProjectDetail) int {
	count := 0
	for _, intent := range project.Intents {
		if intent.To == nil {
			count++
		}
	}
	return count
}

func isBootstrapIntent(intent *protocol.Intent) bool {
	return intent.Description == bootstrapIntentDescription &&
		intent.Creator == bootstrapIntentCreator &&
		len(intent.From) == 1 && intent.From[0] == "origin" &&
		intent.To == nil
}

func getBootstrapIntent(project *protocol.ProjectDetail) *protocol.Intent {
	intents := []*protocol.Intent{}
	for index := range project.Intents {
		intent := &project.Intents[index]
		if isBootstrapIntent(intent) {
			intents = append(intents, intent)
		}
	}
	if len(intents) == 0 {
		return nil
	}
	if len(intents) > 1 {
		ids := make([]string, 0, len(intents))
		for _, intent := range intents {
			ids = append(ids, intent.ID)
		}
		log.Warnf("project has multiple bootstrap intents project=%s intents=%v", project.Project.ID, ids)
	}
	sort.Slice(intents, func(left, right int) bool {
		leftClaimed := intents[left].Worker != nil
		rightClaimed := intents[right].Worker != nil
		if leftClaimed != rightClaimed {
			return !leftClaimed
		}
		if intents[left].CreatedAt != intents[right].CreatedAt {
			return intents[left].CreatedAt < intents[right].CreatedAt
		}
		return intents[left].ID < intents[right].ID
	})
	return intents[0]
}

func (loop *Loop) isInitialProject(project *protocol.ProjectDetail) bool {
	factIDs := map[string]bool{}
	for _, fact := range project.Facts {
		factIDs[fact.ID] = true
	}
	if len(project.Facts) != 2 || !factIDs["origin"] || !factIDs["goal"] {
		return false
	}
	if len(project.Intents) == 0 {
		return true
	}
	for index := range project.Intents {
		if !isBootstrapIntent(&project.Intents[index]) {
			return false
		}
	}
	return true
}

func (loop *Loop) projectRequiresBootstrap(project *protocol.ProjectDetail) bool {
	if !project.Project.BootstrapEnabled {
		return false
	}
	if getBootstrapIntent(project) != nil {
		return true
	}
	for _, worker := range loop.Config.Workers {
		for _, taskType := range worker.TaskTypes {
			if taskType == config.TaskBootstrap {
				return true
			}
		}
	}
	return false
}

func (loop *Loop) createBootstrapIntent(projectID string) *protocol.Intent {
	response := loop.client.CreateIntent(projectID, []string{"origin"}, bootstrapIntentDescription, bootstrapIntentCreator)
	if response.StatusCode == 403 {
		log.Infof("project became inactive before bootstrap intent create project=%s", projectID)
		return nil
	}
	if !response.Ok() {
		log.Warnf("bootstrap intent write failed project=%s status=%d body=%s", projectID, response.StatusCode, response.Text)
		return nil
	}
	raw, ok := response.Data.(json.RawMessage)
	if !ok {
		log.Warnf("bootstrap intent create returned empty body project=%s", projectID)
		return nil
	}
	var intent protocol.Intent
	if err := json.Unmarshal(raw, &intent); err != nil {
		log.Warnf("bootstrap intent create returned invalid body project=%s error=%s", projectID, err)
		return nil
	}
	log.Infof("created bootstrap intent project=%s intent=%s", projectID, intent.ID)
	return &intent
}

func (loop *Loop) reasonTrigger(project *protocol.ProjectDetail) *string {
	openIntentCount := loop.projectOpenIntentCount(project)
	checkpoint, ok := loop.reasonCheckpoints[project.Project.ID]
	if !ok {
		return stringPtr("initial")
	}
	changes := []string{}
	if len(project.Facts) > checkpoint.FactCount {
		changes = append(changes, fmt.Sprintf("facts:%d->%d", checkpoint.FactCount, len(project.Facts)))
	}
	if len(project.Hints) > checkpoint.HintCount {
		changes = append(changes, fmt.Sprintf("hints:%d->%d", checkpoint.HintCount, len(project.Hints)))
	}
	if checkpoint.OpenIntentCount > 0 && openIntentCount == 0 {
		changes = append(changes, fmt.Sprintf("open_intents:%d->0", checkpoint.OpenIntentCount))
	}
	if len(changes) == 0 {
		return nil
	}
	trigger := strings.Join(changes, ",")
	return &trigger
}

func (loop *Loop) submit(task *RunningTask, run func() string) {
	loop.mu.Lock()
	loop.futures[task] = true
	loop.runtimeProjectIDs[task.ProjectID] = true
	loop.mu.Unlock()
	go func() {
		outcome := "failed"
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Errorf("task crashed project=%s task=%s worker=%s panic=%v", task.ProjectID, task.TaskType, task.WorkerName, recovered)
				outcome = "failed"
			}
			task.done <- outcome
		}()
		outcome = run()
	}()
}

func (loop *Loop) reapFutures() {
	loop.mu.Lock()
	tasks := make([]*RunningTask, 0, len(loop.futures))
	for task := range loop.futures {
		tasks = append(tasks, task)
	}
	loop.mu.Unlock()

	for _, task := range tasks {
		var outcome string
		select {
		case outcome = <-task.done:
		default:
			continue
		}
		loop.mu.Lock()
		delete(loop.futures, task)
		loop.mu.Unlock()
		if outcome == "cancelled" {
			log.Infof("task cancelled project=%s task=%s worker=%s", task.ProjectID, task.TaskType, task.WorkerName)
		} else if outcome != "success" {
			log.Warnf("task finished project=%s task=%s worker=%s outcome=%s", task.ProjectID, task.TaskType, task.WorkerName, outcome)
		}
		loop.clearProjectLogState(task.ProjectID)
		if outcome == "unhealthy" {
			loop.mu.Lock()
			loop.workerUnhealthyUntil[task.WorkerName] = nowSeconds() + unhealthyRetryAfterSeconds
			loop.mu.Unlock()
			log.Infof("worker marked unhealthy worker=%s retry_after=%.0fs", task.WorkerName, unhealthyRetryAfterSeconds)
		} else {
			loop.mu.Lock()
			delete(loop.workerUnhealthyUntil, task.WorkerName)
			loop.mu.Unlock()
		}
		rejectionKey := task.ProjectID + "|" + task.TaskType + "|" + task.WorkerName
		if outcome == "rejected" {
			loop.mu.Lock()
			loop.workerRejectedUntil[rejectionKey] = nowSeconds() + rejectedRetryAfterSeconds
			loop.mu.Unlock()
			log.Infof("worker marked rejected project=%s task=%s worker=%s retry_after=%.0fs", task.ProjectID, task.TaskType, task.WorkerName, rejectedRetryAfterSeconds)
		} else {
			loop.mu.Lock()
			delete(loop.workerRejectedUntil, rejectionKey)
			loop.mu.Unlock()
		}
		if outcome == "success" && task.TaskType == "reason" {
			loop.mu.Lock()
			loop.reasonCheckpoints[task.ProjectID] = ReasonCheckpoint{
				FactCount: *task.FactCount, HintCount: *task.HintCount, OpenIntentCount: *task.OpenIntentCount,
			}
			loop.mu.Unlock()
		}
	}
}

func (loop *Loop) queueContainerCleanups(summaries []protocol.ProjectSummary) {
	for _, summary := range summaries {
		switch summary.Status {
		case "completed":
			loop.queueCleanup(summary.ID, &summary.Status, loop.backend.NeedsCompletedCleanup(summary.ID), func() bool {
				return loop.backend.CleanupCompleted(summary.ID)
			})
		case "stopped":
			loop.queueCleanup(summary.ID, &summary.Status, loop.backend.NeedsStoppedCleanup(summary.ID), func() bool {
				return loop.backend.CleanupStopped(summary.ID)
			})
		}
	}
}

func (loop *Loop) queueCleanup(projectID string, targetStatus *string, needed bool, run func() bool) {
	container := loop.backend.ContainerName(projectID)
	loop.mu.Lock()
	if loop.inactiveCleanupDone[projectID] == *targetStatus || loop.cleanupPending[container] {
		loop.mu.Unlock()
		return
	}
	if !needed {
		loop.inactiveCleanupDone[projectID] = *targetStatus
		loop.mu.Unlock()
		return
	}
	task := &CleanupTask{container: container, projectID: &projectID, targetStatus: targetStatus, run: run, done: make(chan bool, 1)}
	loop.cleanupFutures[task] = true
	loop.cleanupPending[container] = true
	loop.mu.Unlock()
	go func() {
		success := false
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Errorf("container cleanup crashed container=%s panic=%v", container, recovered)
				success = false
			}
			task.done <- success
		}()
		success = run()
	}()
}

func (loop *Loop) reapCleanupFutures() {
	loop.mu.Lock()
	tasks := make([]*CleanupTask, 0, len(loop.cleanupFutures))
	for task := range loop.cleanupFutures {
		tasks = append(tasks, task)
	}
	loop.mu.Unlock()
	for _, task := range tasks {
		var success bool
		select {
		case success = <-task.done:
		default:
			continue
		}
		loop.mu.Lock()
		delete(loop.cleanupFutures, task)
		delete(loop.cleanupPending, task.container)
		if success && task.projectID != nil && task.targetStatus != nil {
			loop.inactiveCleanupDone[*task.projectID] = *task.targetStatus
		} else if task.projectID != nil {
			delete(loop.inactiveCleanupDone, *task.projectID)
		}
		loop.mu.Unlock()
	}
}

func (loop *Loop) refreshRuntimeProjects(summaries []protocol.ProjectSummary) {
	runnableIDs := map[string]bool{}
	runnableStatus := map[string]bool{}
	for _, summary := range summaries {
		if RunnableStatuses[summary.Status] {
			runnableIDs[summary.ID] = true
			runnableStatus[summary.ID] = true
		}
	}
	statusByID := map[string]string{}
	for _, summary := range summaries {
		statusByID[summary.ID] = summary.Status
	}
	loop.mu.Lock()
	for id := range loop.runtimeProjectIDs {
		if !runnableIDs[id] {
			delete(loop.runtimeProjectIDs, id)
		}
	}
	for id, status := range loop.inactiveCleanupDone {
		if runnableStatus[id] || statusByID[id] != status {
			delete(loop.inactiveCleanupDone, id)
		}
	}
	loop.mu.Unlock()
}

func (loop *Loop) cancelInactiveTasks(summaries []protocol.ProjectSummary) {
	statusByProject := map[string]string{}
	for _, summary := range summaries {
		statusByProject[summary.ID] = summary.Status
	}
	loop.mu.Lock()
	tasks := make([]*RunningTask, 0, len(loop.futures))
	for task := range loop.futures {
		tasks = append(tasks, task)
	}
	loop.mu.Unlock()
	for _, task := range tasks {
		status, ok := statusByProject[task.ProjectID]
		if !ok {
			status = "deleted"
		}
		if RunnableStatuses[status] {
			continue
		}
		if task.Cancellation.Cancel(status) {
			log.Infof("cancelling running task for inactive project project=%s task=%s worker=%s status=%s", task.ProjectID, task.TaskType, task.WorkerName, status)
		}
	}
}

func (loop *Loop) initializeReasonCheckpoints(summaries []protocol.ProjectSummary) {
	loop.mu.Lock()
	defer loop.mu.Unlock()
	for _, summary := range summaries {
		if !RunnableStatuses[summary.Status] {
			continue
		}
		if _, exists := loop.reasonCheckpoints[summary.ID]; exists {
			continue
		}
		openIntentCount := summary.WorkingIntentCount + summary.UnclaimedIntentCount
		if openIntentCount == 0 {
			continue
		}
		loop.reasonCheckpoints[summary.ID] = ReasonCheckpoint{
			FactCount: summary.FactCount, HintCount: summary.HintCount, OpenIntentCount: openIntentCount,
		}
	}
}

func (loop *Loop) bestEffortRelease(projectID, intentID, workerName string) {
	response := loop.client.Release(projectID, intentID, workerName)
	if !response.Ok() && response.StatusCode != 403 && response.StatusCode != 409 {
		log.Warnf("release failed project=%s intent=%s worker=%s status=%d", projectID, intentID, workerName, response.StatusCode)
	}
}

func (loop *Loop) bestEffortReleaseReason(projectID, workerName string) {
	response := loop.client.ReleaseReason(projectID, workerName)
	if !response.Ok() && response.StatusCode != 403 && response.StatusCode != 409 {
		log.Warnf("reason release failed project=%s worker=%s status=%d", projectID, workerName, response.StatusCode)
	}
}

func (loop *Loop) logChanged(scope string, level logging.Level, message string, args ...any) {
	signature := fmt.Sprintf("%d|%s|%v", level, message, args)
	loop.mu.Lock()
	if loop.logState[scope] == signature {
		loop.mu.Unlock()
		return
	}
	loop.logState[scope] = signature
	loop.mu.Unlock()
	loop.logAt(level, message, args...)
}

func (loop *Loop) logAt(level logging.Level, message string, args ...any) {
	switch level {
	case logging.Debug:
		log.Debugf(message, args...)
	case logging.Warning:
		log.Warnf(message, args...)
	case logging.Error:
		log.Errorf(message, args...)
	default:
		log.Infof(message, args...)
	}
}

func (loop *Loop) clearLogState(scope string) {
	loop.mu.Lock()
	delete(loop.logState, scope)
	loop.mu.Unlock()
}

func (loop *Loop) clearProjectLogState(projectID string) {
	prefix := "project:" + projectID + ":"
	loop.mu.Lock()
	for scope := range loop.logState {
		if strings.HasPrefix(scope, prefix) {
			delete(loop.logState, scope)
		}
	}
	loop.mu.Unlock()
}

func (loop *Loop) validateServerSettings() error {
	settings := loop.settingsSnapshot()
	interval := loop.Config.Runtime.Interval
	checks := []struct {
		name  string
		value int
	}{{"intent_timeout", settings.IntentTimeout}, {"reason_timeout", settings.ReasonTimeout}}
	for _, check := range checks {
		if check.value <= interval {
			return fmt.Errorf("server %s=%ds must be greater than dispatcher interval=%ds", check.name, check.value, interval)
		}
		if check.value < interval*2 {
			log.Warnf("server %s is tight %s=%ds interval=%ds; heartbeat slack is only %ds", check.name, check.name, check.value, interval, check.value-interval)
			continue
		}
		log.Infof("server setting validated %s=%ds interval=%ds", check.name, check.value, interval)
	}
	if settings.MaxWorkers > 0 && settings.MaxProjectWorkers > settings.MaxWorkers {
		return fmt.Errorf("server max_project_workers=%d must not exceed server max_workers=%d", settings.MaxProjectWorkers, settings.MaxWorkers)
	}
	return nil
}

func nowSeconds() float64 { return float64(time.Now().UnixNano()) / 1e9 }

func stringPtr(value string) *string { return &value }

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedCopy(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func runReason(loop *Loop, project *protocol.ProjectDetail, exportYAML string, worker *config.WorkerConfig, cancellation *runtime.TaskCancellation) string {
	return tasks.RunReasonTask(loop.Config, loop.client, loop.backend, project, exportYAML, worker, cancellation)
}

func runBootstrap(loop *Loop, project *protocol.ProjectDetail, intent *protocol.Intent, worker *config.WorkerConfig, cancellation *runtime.TaskCancellation) string {
	return tasks.RunBootstrapTask(loop.Config, loop.client, loop.backend, project, intent, worker, cancellation)
}

func runExplore(loop *Loop, project *protocol.ProjectDetail, exportYAML string, intent *protocol.Intent, worker *config.WorkerConfig, cancellation *runtime.TaskCancellation) string {
	return tasks.RunExploreTask(loop.Config, loop.client, loop.backend, project, exportYAML, intent, worker, cancellation)
}
