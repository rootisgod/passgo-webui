package api

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ansibleRun tracks a running or completed ansible-playbook execution.
type ansibleRun struct {
	mu        sync.Mutex
	Playbook  string    `json:"playbook"`
	VMs       []string  `json:"vms"`
	Status    string    `json:"status"` // "running", "success", "failed"
	ExitCode  int       `json:"exit_code"`
	StartedAt time.Time `json:"started_at"`
	output    []string
	cancel    context.CancelFunc
	// subscribers get notified of new output lines
	subs []chan string
}

func (r *ansibleRun) addLine(line string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.output = append(r.output, line)
	for _, ch := range r.subs {
		select {
		case ch <- line:
		default:
		}
	}
}

func (r *ansibleRun) finish(exitCode int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ExitCode = exitCode
	if exitCode == 0 {
		r.Status = "success"
	} else {
		r.Status = "failed"
	}
	// Close all subscriber channels
	for _, ch := range r.subs {
		close(ch)
	}
	r.subs = nil
}

// subscribe returns a channel that receives new output lines, plus the current buffered output.
func (r *ansibleRun) subscribe() ([]string, chan string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	snapshot := make([]string, len(r.output))
	copy(snapshot, r.output)
	if r.Status != "running" {
		return snapshot, nil
	}
	ch := make(chan string, 256)
	r.subs = append(r.subs, ch)
	return snapshot, ch
}

func (r *ansibleRun) unsubscribe(ch chan string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, s := range r.subs {
		if s == ch {
			r.subs = append(r.subs[:i], r.subs[i+1:]...)
			break
		}
	}
}

// queueEntry represents a pending ansible run waiting to execute.
type queueEntry struct {
	Playbook string   `json:"playbook"`
	VMs      []string `json:"vms"`
}

// ansibleRunner manages the current ansible run and a FIFO queue for auto-runs.
type ansibleRunner struct {
	mu      sync.Mutex
	current *ansibleRun
	queue   []queueEntry
	// startFunc is called to build and start queued runs. Set by the server.
	startFunc func(playbook string, vms []string)
}

func (ar *ansibleRunner) getCurrent() *ansibleRun {
	ar.mu.Lock()
	defer ar.mu.Unlock()
	return ar.current
}

func (ar *ansibleRunner) start(playbook string, vms []string, cmd *exec.Cmd, inventoryPath string) *ansibleRun {
	ctx, cancel := context.WithCancel(context.Background())

	run := &ansibleRun{
		Playbook:  playbook,
		VMs:       vms,
		Status:    "running",
		StartedAt: time.Now(),
		cancel:    cancel,
	}

	ar.mu.Lock()
	ar.current = run
	ar.mu.Unlock()

	// Show the command being run so users can reproduce manually
	run.addLine("$ ANSIBLE_HOST_KEY_CHECKING=False " + strings.Join(cmd.Args, " "))
	run.addLine("")

	// Start the process in its own process group so we can kill all children
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		run.addLine("Failed to start ansible-playbook: " + err.Error())
		run.finish(1)
		return run
	}

	// Monitor for cancel — kill entire process group
	go func() {
		<-ctx.Done()
		if cmd.Process != nil {
			syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	}()

	lines := make(chan string, 64)
	streamReader := func(r io.Reader) {
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}

	go streamReader(stdout)
	go func() {
		streamReader(stderr)
		cmd.Wait()
		close(lines)
	}()

	go func() {
		for line := range lines {
			run.addLine(line)
		}
		exitCode := 0
		if cmd.ProcessState != nil && !cmd.ProcessState.Success() {
			exitCode = cmd.ProcessState.ExitCode()
		}
		run.finish(exitCode)
		cancel() // clean up context

		// Clean up inventory file
		if inventoryPath != "" {
			os.Remove(inventoryPath)
		}

		// Process next queued run
		ar.dequeueNext()
	}()

	return run
}

func (ar *ansibleRunner) cancel() bool {
	ar.mu.Lock()
	run := ar.current
	ar.mu.Unlock()
	if run == nil {
		return false
	}
	run.mu.Lock()
	status := run.Status
	run.mu.Unlock()
	if status != "running" {
		return false
	}
	run.cancel()
	return true
}

func (ar *ansibleRunner) clear() {
	ar.mu.Lock()
	defer ar.mu.Unlock()
	if ar.current != nil && ar.current.Status != "running" {
		ar.current = nil
	}
}

// enqueue adds a playbook run to the queue. If no run is active, starts immediately.
func (ar *ansibleRunner) enqueue(playbook string, vms []string) {
	ar.mu.Lock()
	isIdle := ar.current == nil || ar.current.Status != "running"
	if !isIdle {
		ar.queue = append(ar.queue, queueEntry{Playbook: playbook, VMs: vms})
		ar.mu.Unlock()
		return
	}
	ar.mu.Unlock()
	// Start immediately
	if ar.startFunc != nil {
		ar.startFunc(playbook, vms)
	}
}

// dequeueNext starts the next queued run if any. Called after a run finishes.
func (ar *ansibleRunner) dequeueNext() {
	ar.mu.Lock()
	if len(ar.queue) == 0 {
		ar.mu.Unlock()
		return
	}
	entry := ar.queue[0]
	ar.queue = ar.queue[1:]
	// Clear current so startFunc can set it
	ar.current = nil
	ar.mu.Unlock()

	if ar.startFunc != nil {
		ar.startFunc(entry.Playbook, entry.VMs)
	}
}

// getQueue returns a copy of the current queue.
func (ar *ansibleRunner) getQueue() []queueEntry {
	ar.mu.Lock()
	defer ar.mu.Unlock()
	q := make([]queueEntry, len(ar.queue))
	copy(q, ar.queue)
	return q
}

// clearQueue removes all pending queue entries.
func (ar *ansibleRunner) clearQueue() {
	ar.mu.Lock()
	defer ar.mu.Unlock()
	ar.queue = nil
}
