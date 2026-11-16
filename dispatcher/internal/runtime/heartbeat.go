package runtime

import (
	"sync"
	"time"

	"github.com/Indexyz/Cairn/dispatcher/internal/logging"
	"github.com/Indexyz/Cairn/dispatcher/internal/protocol"
)

var heartbeatLog = logging.New("cairn.dispatcher.runtime.heartbeat")

const heartbeatFailureGraceMultiplier = 2

// HeartbeatFailure records why a lease was lost.
type HeartbeatFailure struct {
	StatusCode *int
	Text       string
}

// HeartbeatLease renews an intent or reason lease in the background and kills
// the attached process when the lease is lost.
type HeartbeatLease struct {
	heartbeat func() protocol.ApiResult
	scope     string
	worker    string
	interval  time.Duration

	mu          sync.Mutex
	process     ExecProcess
	failure     *HeartbeatFailure
	lastSuccess time.Time
	stop        chan struct{}
	done        chan struct{}
}

// NewHeartbeatLease builds a lease around a heartbeat call.
func NewHeartbeatLease(heartbeat func() protocol.ApiResult, scope, worker string, interval time.Duration) *HeartbeatLease {
	return &HeartbeatLease{
		heartbeat:   heartbeat,
		scope:       scope,
		worker:      worker,
		interval:    interval,
		lastSuccess: time.Now(),
		stop:        make(chan struct{}),
		done:        make(chan struct{}),
	}
}

// Start begins renewing.
func (lease *HeartbeatLease) Start() { go lease.run() }

// Stop ends renewing and waits for the goroutine.
func (lease *HeartbeatLease) Stop() {
	close(lease.stop)
	select {
	case <-lease.done:
	case <-time.After(time.Second):
	}
}

// AttachProcess binds the process killed on lease loss.
func (lease *HeartbeatLease) AttachProcess(process ExecProcess) {
	lease.mu.Lock()
	lease.process = process
	lease.mu.Unlock()
}

// Failure returns the recorded failure, if any.
func (lease *HeartbeatLease) Failure() *HeartbeatFailure {
	lease.mu.Lock()
	defer lease.mu.Unlock()
	return lease.failure
}

func (lease *HeartbeatLease) run() {
	defer close(lease.done)
	ticker := time.NewTicker(lease.interval)
	defer ticker.Stop()
	for {
		select {
		case <-lease.stop:
			return
		case <-ticker.C:
		}
		result := lease.heartbeat()
		if result.Ok() {
			lease.mu.Lock()
			lease.lastSuccess = time.Now()
			lease.mu.Unlock()
			continue
		}
		if result.StatusCode == 403 || result.StatusCode == 409 {
			lease.fail(result.StatusCode, result.Text)
			return
		}
		lease.mu.Lock()
		elapsed := time.Since(lease.lastSuccess)
		lease.mu.Unlock()
		grace := float64(lease.interval)
		if doubled := float64(lease.interval) * heartbeatFailureGraceMultiplier; doubled > grace {
			grace = doubled
		}
		heartbeatLog.Warnf(
			"heartbeat transient failure scope=%s worker=%s status=%d elapsed=%.1fs grace=%.1fs",
			lease.scope, lease.worker, result.StatusCode, elapsed.Seconds(), grace/float64(time.Second),
		)
		if elapsed.Seconds() < grace/float64(time.Second) {
			continue
		}
		status := result.StatusCode
		lease.fail(status, result.Text)
		return
	}
}

func (lease *HeartbeatLease) fail(status int, text string) {
	lease.mu.Lock()
	lease.failure = &HeartbeatFailure{StatusCode: &status, Text: text}
	process := lease.process
	lease.mu.Unlock()
	heartbeatLog.Warnf("heartbeat failed scope=%s worker=%s status=%d", lease.scope, lease.worker, status)
	if process != nil {
		process.Kill()
	}
}
