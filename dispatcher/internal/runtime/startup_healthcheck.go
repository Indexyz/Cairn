package runtime

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Indexyz/Cairn/dispatcher/internal/config"
	"github.com/Indexyz/Cairn/dispatcher/internal/logging"
	"github.com/Indexyz/Cairn/dispatcher/internal/workers/registry"
)

var startupLog = logging.New("runtime.startup")

const detailPreviewLimit = 60

// StartupHealthcheckResult is one worker probe outcome.
type StartupHealthcheckResult struct {
	WorkerName string
	OK         bool
	Status     *int
	DurationMs int
	Detail     string
	Endpoint   string
}

// RunStartupHealthchecks probes every configured worker concurrently.
func RunStartupHealthchecks(dispatch *config.DispatchConfig, showCommands bool) []StartupHealthcheckResult {
	workers := append([]config.WorkerConfig(nil), dispatch.Workers...)
	parallelism := len(workers)
	if dispatch.Runtime.MaxWorkers < parallelism {
		parallelism = dispatch.Runtime.MaxWorkers
	}
	if parallelism > 8 {
		parallelism = 8
	}
	if parallelism < 1 {
		parallelism = 1
	}
	startupLog.Infof("[*] Startup healthcheck: workers=%d parallelism=%d", len(workers), parallelism)

	results := make([]StartupHealthcheckResult, 0, len(workers))
	var mu sync.Mutex
	var group sync.WaitGroup
	slots := make(chan struct{}, parallelism)
	for index := range workers {
		worker := workers[index]
		group.Add(1)
		slots <- struct{}{}
		go func() {
			defer group.Done()
			defer func() { <-slots }()
			result := checkWorker(dispatch, &worker)
			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}()
	}
	group.Wait()

	sort.Slice(results, func(left, right int) bool { return results[left].WorkerName < results[right].WorkerName })
	logReport(results, showCommands)
	return results
}

// FormatFailureSummary renders the all-unhealthy error message.
func FormatFailureSummary(results []StartupHealthcheckResult) string {
	details := make([]string, 0, len(results))
	for _, result := range results {
		if result.OK {
			continue
		}
		status := "-"
		if result.Status != nil {
			status = fmt.Sprint(*result.Status)
		}
		detail := result.Detail
		if detail == "" {
			detail = "-"
		}
		details = append(details, fmt.Sprintf("%s(http=%s, detail=%s)", result.WorkerName, status, detail))
	}
	if len(details) == 0 {
		return "startup healthchecks failed for all workers"
	}
	return "startup healthchecks failed for all workers: " + strings.Join(details, ", ")
}

func checkWorker(dispatch *config.DispatchConfig, worker *config.WorkerConfig) StartupHealthcheckResult {
	driver, err := registry.Get(worker.Type, dispatch.Runtime.Execution)
	if err != nil {
		return StartupHealthcheckResult{WorkerName: worker.Name, Detail: err.Error(), Endpoint: "-"}
	}
	started := time.Now()
	result := driver.CheckHealth(worker, time.Duration(dispatch.Runtime.HealthcheckTimeout)*time.Second)
	durationMs := int(time.Since(started).Milliseconds())
	return StartupHealthcheckResult{
		WorkerName: worker.Name,
		OK:         result.OK,
		Status:     result.Status,
		DurationMs: durationMs,
		Detail:     previewDetail(result.Detail),
		Endpoint:   driver.DescribeHealth(worker),
	}
}

func logReport(results []StartupHealthcheckResult, showCommands bool) {
	if len(results) == 0 {
		startupLog.Warnf("[!] Startup healthcheck: no workers configured")
		return
	}
	workerWidth := len("WORKER")
	for _, result := range results {
		if len(result.WorkerName) > workerWidth {
			workerWidth = len(result.WorkerName)
		}
	}
	lines := []string{"[=] Startup healthcheck results", fmt.Sprintf("%-5s %-*s %-6s %8s  DETAIL", "CHK", workerWidth, "WORKER", "HTTP", "TIME_S")}
	lines = append(lines, fmt.Sprintf("%s %s %s %s  %s", strings.Repeat("-", 5), strings.Repeat("-", workerWidth), strings.Repeat("-", 6), strings.Repeat("-", 8), strings.Repeat("-", 60)))
	healthy := 0
	for _, result := range results {
		if result.OK {
			healthy++
		}
		marker := "[-]"
		if result.OK {
			marker = "[+]"
		}
		status := "-"
		if result.Status != nil {
			status = fmt.Sprint(*result.Status)
		}
		detail := result.Detail
		if detail == "" {
			detail = "-"
		}
		lines = append(lines, fmt.Sprintf("%-5s %-*s %-6s %8.2f  %s", marker, workerWidth, result.WorkerName, status, float64(result.DurationMs)/1000, detail))
	}
	lines = append(lines, fmt.Sprintf("[=] Summary: total=%d healthy=%d unhealthy=%d", len(results), healthy, len(results)-healthy))
	if showCommands {
		lines = append(lines, "", "[=] Startup healthcheck endpoints")
		for _, result := range results {
			lines = append(lines, "- "+result.WorkerName+": "+result.Endpoint)
		}
		lines = append(lines, "")
	}
	startupLog.Infof("\n%s\n", strings.Join(lines, "\n"))
}

func previewDetail(text string) string {
	compact := strings.Join(strings.Fields(text), " ")
	if len(compact) <= detailPreviewLimit {
		return compact
	}
	return compact[:detailPreviewLimit] + "..."
}
