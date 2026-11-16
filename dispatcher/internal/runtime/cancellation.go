// Package runtime provides the execution substrate: host subprocesses and
// per-project containers behind one backend interface.
package runtime

import (
	"sync"
)

// TaskCancellation carries a cancellation reason and the live process.
type TaskCancellation struct {
	mu      sync.Mutex
	process ExecProcess
	reason  *string
}

// AttachProcess binds the process that may need killing.
func (cancellation *TaskCancellation) AttachProcess(process ExecProcess) {
	cancellation.mu.Lock()
	cancellation.process = process
	reason := cancellation.reason
	cancellation.mu.Unlock()
	if process != nil && reason != nil {
		process.Cancel(*reason)
	}
}

// Cancel records the first reason and kills any attached process.
func (cancellation *TaskCancellation) Cancel(reason string) bool {
	cancellation.mu.Lock()
	already := cancellation.reason != nil
	if !already {
		cancellation.reason = &reason
	}
	process := cancellation.process
	cancellation.mu.Unlock()
	if process != nil {
		process.Cancel(reason)
	}
	return !already
}

// IsCancelled reports whether a reason was recorded.
func (cancellation *TaskCancellation) IsCancelled() bool {
	cancellation.mu.Lock()
	defer cancellation.mu.Unlock()
	return cancellation.reason != nil
}

// Reason returns the recorded cancellation reason.
func (cancellation *TaskCancellation) Reason() *string {
	cancellation.mu.Lock()
	defer cancellation.mu.Unlock()
	return cancellation.reason
}
