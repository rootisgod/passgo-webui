package api

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"sync"
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

// ansibleRunner manages the current ansible run. At most one run at a time.
type ansibleRunner struct {
	mu      sync.Mutex
	current *ansibleRun
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

	// Start the process and stream output in the background
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		run.addLine("Failed to start ansible-playbook: " + err.Error())
		run.finish(1)
		return run
	}

	// Monitor for cancel
	go func() {
		<-ctx.Done()
		if cmd.Process != nil {
			cmd.Process.Kill()
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
