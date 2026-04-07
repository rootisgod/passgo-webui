package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rootisgod/passgo-web/pkg/multipass"
)

type ansibleRunRequest struct {
	Playbook string   `json:"playbook"`
	VMs      []string `json:"vms"`
	Groups   []string `json:"groups,omitempty"`
}

type ansibleStatusResponse struct {
	Installed    bool   `json:"installed"`
	Version      string `json:"version,omitempty"`
	Error        string `json:"error,omitempty"`
	PlaybooksDir string `json:"playbooks_dir"`
}

type ansibleOutputEvent struct {
	Type     string `json:"type"`
	Content  string `json:"content,omitempty"`
	ExitCode int    `json:"exit_code,omitempty"`
}

func (s *Server) handleAnsibleStatus(w http.ResponseWriter, r *http.Request) {
	path, err := exec.LookPath("ansible-playbook")
	if err != nil {
		writeJSON(w, http.StatusOK, ansibleStatusResponse{
			Installed:    false,
			Error:        "ansible-playbook not found in PATH",
			PlaybooksDir: s.cfg.PlaybooksDir,
		})
		return
	}

	out, err := exec.Command(path, "--version").Output()
	version := ""
	if err == nil {
		lines := strings.SplitN(string(out), "\n", 2)
		if len(lines) > 0 {
			version = strings.TrimSpace(lines[0])
		}
	}

	writeJSON(w, http.StatusOK, ansibleStatusResponse{
		Installed:    true,
		Version:      version,
		PlaybooksDir: s.cfg.PlaybooksDir,
	})
}

func (s *Server) handleRunPlaybook(w http.ResponseWriter, r *http.Request) {
	// Single concurrent run
	if !s.ansibleRunMu.TryLock() {
		writeError(w, http.StatusConflict, "a playbook is already running")
		return
	}
	defer s.ansibleRunMu.Unlock()

	var req ansibleRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Playbook == "" {
		writeError(w, http.StatusBadRequest, "playbook is required")
		return
	}
	if len(req.VMs) == 0 && len(req.Groups) == 0 {
		writeError(w, http.StatusBadRequest, "at least one VM or group must be selected")
		return
	}

	// Verify ansible-playbook exists
	ansiblePath, err := exec.LookPath("ansible-playbook")
	if err != nil {
		writeError(w, http.StatusBadRequest, "ansible-playbook not found in PATH")
		return
	}

	// Verify playbook exists
	_, err = multipass.ReadPlaybook(s.cfg.PlaybooksDir, req.Playbook)
	if err != nil {
		writeError(w, http.StatusNotFound, "playbook not found")
		return
	}
	playbookPath := filepath.Join(s.cfg.PlaybooksDir, req.Playbook)

	// Resolve target VMs: explicit VMs + VMs from selected groups
	targetVMs := make([]string, len(req.VMs))
	copy(targetVMs, req.VMs)

	if len(req.Groups) > 0 {
		s.groupMu.Lock()
		for vm, group := range s.cfg.VMGroups {
			for _, g := range req.Groups {
				if group == g {
					// Avoid duplicates
					found := false
					for _, t := range targetVMs {
						if t == vm {
							found = true
							break
						}
					}
					if !found {
						targetVMs = append(targetVMs, vm)
					}
				}
			}
		}
		s.groupMu.Unlock()
	}

	// Generate inventory
	user := "ubuntu"
	sshKey := ""
	if s.cfg.VMDefaults != nil {
		sshKey = s.cfg.VMDefaults.SSHPrivateKey
	}
	inventory, err := s.generateInventoryYAML(targetVMs, user, sshKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate inventory")
		return
	}

	// Write temp inventory
	tmpFile, err := os.CreateTemp("", "passgo-ansible-inventory-*.yml")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create temp inventory")
		return
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(inventory); err != nil {
		tmpFile.Close()
		writeError(w, http.StatusInternalServerError, "failed to write inventory")
		return
	}
	tmpFile.Close()

	// Set up SSE
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Build command
	cmd := exec.CommandContext(r.Context(), ansiblePath, "-i", tmpFile.Name(), playbookPath)
	cmd.Env = append(os.Environ(),
		"ANSIBLE_FORCE_COLOR=true",
		"ANSIBLE_HOST_KEY_CHECKING=False",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		sendSSEEvent(w, flusher, ansibleOutputEvent{Type: "error", Content: "failed to create stdout pipe"})
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		sendSSEEvent(w, flusher, ansibleOutputEvent{Type: "error", Content: "failed to create stderr pipe"})
		return
	}

	if err := cmd.Start(); err != nil {
		sendSSEEvent(w, flusher, ansibleOutputEvent{Type: "error", Content: fmt.Sprintf("failed to start ansible-playbook: %v", err)})
		return
	}

	// Stream output from both stdout and stderr
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
		// stderr finishes last (or at same time), signal done reading
		// Wait for command to finish before closing channel
		cmd.Wait()
		close(lines)
	}()

	for line := range lines {
		sendSSEEvent(w, flusher, ansibleOutputEvent{Type: "output", Content: line})
	}

	exitCode := 0
	if cmd.ProcessState != nil && !cmd.ProcessState.Success() {
		exitCode = cmd.ProcessState.ExitCode()
	}

	sendSSEEvent(w, flusher, ansibleOutputEvent{Type: "done", ExitCode: exitCode})
}

func sendSSEEvent(w http.ResponseWriter, flusher http.Flusher, event ansibleOutputEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}
