package runtime

import (
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Indexyz/Cairn/dispatcher/internal/logging"
)

var log = logging.New("cairn.dispatcher.runtime.process")

const (
	streamJoinTimeout    = 5 * time.Second
	forceKillReapTimeout = 2 * time.Second
	readChunkSize        = 65536
)

// LocalProcess runs a worker command as a host subprocess in its own process
// group, with a SIGTERM -> grace -> SIGKILL shutdown.
type LocalProcess struct {
	command   []string
	env       map[string]string
	cwd       string
	timeout   *int
	termGrace time.Duration

	mu         sync.Mutex
	process    *exec.Cmd
	done       chan struct{}
	stdout     []byte
	stderr     []byte
	drains     sync.WaitGroup
	timedOut   bool
	cancelText *string
}

// NewLocalProcess prepares a host subprocess.
func NewLocalProcess(command []string, cwd string, env map[string]string, timeoutSeconds *int, termGraceSeconds int) *LocalProcess {
	grace := time.Duration(termGraceSeconds) * time.Second
	if grace < time.Second {
		grace = time.Second
	}
	return &LocalProcess{command: command, cwd: cwd, env: env, timeout: timeoutSeconds, termGrace: grace}
}

// Start launches the process and its output drainers.
func (process *LocalProcess) Start() error {
	command := exec.Command(process.command[0], process.command[1:]...)
	command.Dir = process.cwd
	command.Env = mergedEnv(process.env)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := command.StderrPipe()
	if err != nil {
		return err
	}
	done := make(chan struct{})
	process.mu.Lock()
	process.process = command
	process.done = done
	process.mu.Unlock()
	if err := command.Start(); err != nil {
		return err
	}
	go func() {
		_ = command.Wait()
		close(done)
	}()
	process.drains.Add(2)
	go process.drain(stdout, &process.stdout)
	go process.drain(stderr, &process.stderr)
	return nil
}

// Communicate waits for completion, honouring the configured timeout.
func (process *LocalProcess) Communicate(timeout time.Duration) ProcessResult {
	process.mu.Lock()
	command := process.process
	done := process.done
	configured := process.timeout
	process.mu.Unlock()
	if command == nil || done == nil {
		return ProcessResult{ReturnCode: 1}
	}
	waitFor := timeout
	if configured != nil {
		waitFor = time.Duration(*configured) * time.Second
	}

	select {
	case <-done:
	case <-time.After(waitFor):
		process.mu.Lock()
		process.timedOut = true
		process.mu.Unlock()
		process.terminate()
		select {
		case <-done:
		case <-time.After(forceKillReapTimeout):
		}
	}

	if !waitGroupTimeout(&process.drains, streamJoinTimeout) {
		// Best effort: readers usually finish on EOF right after the process exits.
	}

	process.mu.Lock()
	defer process.mu.Unlock()
	returnCode := 1
	if command.ProcessState != nil {
		returnCode = command.ProcessState.ExitCode()
		if returnCode < 0 {
			returnCode = 137
		}
	}
	return ProcessResult{
		ReturnCode:   returnCode,
		Stdout:       string(process.stdout),
		Stderr:       string(process.stderr),
		TimedOut:     process.timedOut,
		Cancelled:    process.cancelText != nil,
		CancelReason: process.cancelText,
	}
}

// Kill terminates the process group.
func (process *LocalProcess) Kill() { process.terminate() }

// Cancel records the reason and terminates the process group.
func (process *LocalProcess) Cancel(reason string) {
	process.mu.Lock()
	if process.cancelText == nil {
		process.cancelText = &reason
	}
	process.mu.Unlock()
	process.terminate()
}

func (process *LocalProcess) terminate() {
	process.mu.Lock()
	command, done := process.process, process.done
	process.mu.Unlock()
	if command == nil || command.Process == nil || done == nil {
		return
	}
	select {
	case <-done:
		return
	default:
	}
	signalGroup(command.Process.Pid, syscall.SIGTERM)
	select {
	case <-done:
		return
	case <-time.After(process.termGrace):
		signalGroup(command.Process.Pid, syscall.SIGKILL)
	}
}

func signalGroup(pid int, signal syscall.Signal) {
	_ = syscall.Kill(-pid, signal)
}

func (process *LocalProcess) drain(pipe io.ReadCloser, sink *[]byte) {
	defer process.drains.Done()
	defer pipe.Close()
	buffer := make([]byte, readChunkSize)
	for {
		count, err := pipe.Read(buffer)
		if count > 0 {
			process.mu.Lock()
			*sink = append(*sink, buffer[:count]...)
			process.mu.Unlock()
		}
		if err != nil {
			return
		}
	}
}

func waitGroupTimeout(group *sync.WaitGroup, timeout time.Duration) bool {
	done := make(chan struct{})
	go func() {
		group.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}

func mergedEnv(overrides map[string]string) []string {
	merged := map[string]string{}
	for _, entry := range os.Environ() {
		if index := strings.IndexByte(entry, '='); index > 0 {
			merged[entry[:index]] = entry[index+1:]
		}
	}
	for key, value := range overrides {
		merged[key] = value
	}
	environ := make([]string, 0, len(merged))
	for key, value := range merged {
		environ = append(environ, key+"="+value)
	}
	return environ
}

var _ = streamJoinTimeout
