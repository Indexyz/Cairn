package runtime

import (
	"strings"
	"testing"

	"github.com/Indexyz/Cairn/dispatcher/internal/config"
)

func healthConfig(workers ...config.WorkerConfig) *config.DispatchConfig {
	return &config.DispatchConfig{
		Runtime: config.RuntimeConfig{
			MaxWorkers:         2,
			HealthcheckTimeout: 1,
			Execution:          config.ExecutionContainer,
		},
		Workers: workers,
	}
}

func mockWorker(name, behavior string) config.WorkerConfig {
	return config.WorkerConfig{
		Name: name, Type: config.WorkerMock,
		TaskTypes: []config.TaskType{config.TaskReason}, MaxRunning: 1,
		Env: map[string]string{"MOCK_HEALTHCHECK": behavior},
	}
}

func TestRunStartupHealthchecksReportsEachWorker(t *testing.T) {
	dispatch := healthConfig(
		mockWorker("ok-worker", `{"delay":[0,0],"outcomes":{"ok":1.0,"fail":0.0}}`),
		mockWorker("fail-worker", `{"delay":[0,0],"outcomes":{"ok":0.0,"fail":1.0}}`),
	)

	results := RunStartupHealthchecks(dispatch, false)

	if len(results) != 2 {
		t.Fatalf("results = %d", len(results))
	}
	if results[0].WorkerName != "fail-worker" || results[0].OK {
		t.Fatalf("first = %+v", results[0])
	}
	if results[1].WorkerName != "ok-worker" || !results[1].OK {
		t.Fatalf("second = %+v", results[1])
	}
}

func TestFormatFailureSummaryIncludesWorkerDetails(t *testing.T) {
	status := 401
	results := []StartupHealthcheckResult{
		{WorkerName: "worker-a", OK: false, Status: &status, DurationMs: 12, Detail: "unauthorized", Endpoint: "POST http://api/v1/messages"},
		{WorkerName: "worker-b", OK: true, Status: intPtr(200), DurationMs: 8, Endpoint: "POST http://api/v1/messages"},
	}

	summary := FormatFailureSummary(results)

	want := "startup healthchecks failed for all workers: worker-a(http=401, detail=unauthorized)"
	if summary != want {
		t.Fatalf("summary = %q", summary)
	}
	if strings.Count(summary, "worker-b") != 0 {
		t.Fatalf("healthy worker leaked into summary: %q", summary)
	}
}

func intPtr(value int) *int { return &value }
