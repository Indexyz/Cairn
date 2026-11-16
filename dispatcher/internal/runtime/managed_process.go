package runtime

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
)

const execKillJoinTimeout = 5 * time.Second

// ManagedProcess is a worker command executed inside a project container.
type ManagedProcess struct {
	api         dockerAPI
	containerID string
	container   string
	argv        []string
	env         map[string]string

	mu         sync.Mutex
	execID     string
	done       chan struct{}
	stdout     []byte
	stderr     []byte
	timedOut   bool
	cancelText *string
	readErr    string
}

// NewManagedProcess prepares a container exec.
func NewManagedProcess(api dockerAPI, containerID, containerName string, argv []string, env map[string]string) *ManagedProcess {
	return &ManagedProcess{api: api, containerID: containerID, container: containerName, argv: argv, env: env}
}

// Start creates the exec and starts streaming its output.
func (process *ManagedProcess) Start() error {
	exec, err := process.api.ContainerExecCreate(context.Background(), process.containerID, container.ExecOptions{
		Cmd:          process.argv,
		Env:          envPairs(process.env),
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return err
	}
	done := make(chan struct{})
	process.mu.Lock()
	process.execID = exec.ID
	process.done = done
	process.mu.Unlock()
	go process.readStream(done)
	return nil
}

// Communicate waits for the exec to finish, killing it after the timeout.
func (process *ManagedProcess) Communicate(timeout time.Duration) ProcessResult {
	process.mu.Lock()
	done := process.done
	process.mu.Unlock()
	if done == nil {
		return ProcessResult{ReturnCode: 1}
	}

	select {
	case <-done:
	case <-time.After(timeout):
		process.mu.Lock()
		process.timedOut = true
		process.mu.Unlock()
		process.Kill()
		select {
		case <-done:
		case <-time.After(execKillJoinTimeout):
		}
	}

	process.mu.Lock()
	defer process.mu.Unlock()
	exitCode := 1
	if process.execID != "" {
		if details, err := process.api.ContainerExecInspect(context.Background(), process.execID); err == nil {
			if !details.Running {
				exitCode = details.ExitCode
			}
		}
	}
	stderr := string(process.stderr)
	if process.readErr != "" && stderr == "" {
		stderr = process.readErr
	}
	return ProcessResult{
		ReturnCode:   exitCode,
		Stdout:       string(process.stdout),
		Stderr:       stderr,
		TimedOut:     process.timedOut,
		Cancelled:    process.cancelText != nil,
		CancelReason: process.cancelText,
	}
}

// Kill sends SIGKILL to the exec pid inside the container.
func (process *ManagedProcess) Kill() {
	process.mu.Lock()
	execID := process.execID
	process.mu.Unlock()
	if execID == "" {
		return
	}
	details, err := process.api.ContainerExecInspect(context.Background(), execID)
	if err != nil || !details.Running || details.Pid == 0 {
		return
	}
	pid := fmt.Sprint(details.Pid)
	for _, argv := range [][]string{
		{"kill", "-KILL", pid},
		{"/bin/sh", "-lc", "kill -KILL " + pid},
		{"sh", "-lc", "kill -KILL " + pid},
	} {
		result, err := process.api.ContainerExecCreate(context.Background(), process.containerID, container.ExecOptions{Cmd: argv})
		if err != nil {
			continue
		}
		if _, err := process.api.ContainerExecAttach(context.Background(), result.ID, container.ExecAttachOptions{}); err == nil {
			return
		}
	}
}

// Cancel records the reason and kills the exec.
func (process *ManagedProcess) Cancel(reason string) {
	process.mu.Lock()
	if process.cancelText == nil {
		process.cancelText = &reason
	}
	process.mu.Unlock()
	process.Kill()
}

func (process *ManagedProcess) readStream(done chan struct{}) {
	process.mu.Lock()
	execID := process.execID
	process.mu.Unlock()
	defer close(done)
	attached, err := process.api.ContainerExecAttach(context.Background(), execID, container.ExecAttachOptions{})
	if err != nil {
		process.mu.Lock()
		process.readErr = err.Error()
		process.mu.Unlock()
		return
	}
	defer func() {
		if attached.Conn != nil {
			attached.Close()
		}
	}()
	_, _ = stdcopy.StdCopy(&lockedBuffer{process: process, stdout: true}, &lockedBuffer{process: process}, attached.Reader)
}

type lockedBuffer struct {
	process *ManagedProcess
	stdout  bool
}

func (buffer *lockedBuffer) Write(chunk []byte) (int, error) {
	buffer.process.mu.Lock()
	if buffer.stdout {
		buffer.process.stdout = append(buffer.process.stdout, chunk...)
	} else {
		buffer.process.stderr = append(buffer.process.stderr, chunk...)
	}
	buffer.process.mu.Unlock()
	return len(chunk), nil
}

func envPairs(env map[string]string) []string {
	pairs := make([]string, 0, len(env))
	for key, value := range env {
		pairs = append(pairs, key+"="+value)
	}
	return pairs
}
