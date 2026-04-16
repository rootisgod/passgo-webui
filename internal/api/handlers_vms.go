package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/rootisgod/passgo-web/pkg/multipass"
)

func (s *Server) handleListVMs(w http.ResponseWriter, r *http.Request) {
	vms, err := s.mp.ListVMs()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, vms)
}

func (s *Server) handleGetVM(w http.ResponseWriter, r *http.Request) {
	name, ok := validVMName(w, r, "name")
	if !ok {
		return
	}
	vm, err := s.mp.GetVMInfo(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, vm)
}

type createVMRequest struct {
	Name      string `json:"name"`
	Release   string `json:"release"`
	CPUs      int    `json:"cpus"`
	MemoryMB  int    `json:"memoryMB"`
	DiskGB    int    `json:"diskGB"`
	CloudInit string `json:"cloudInit"`
	Network   string `json:"network"`
	Profile   string `json:"profile"`
	Playbook  string `json:"playbook"`
}

func (s *Server) handleCreateVM(w http.ResponseWriter, r *http.Request) {
	var req createVMRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Resolve profile if specified: defaults → profile → request overrides
	var profileGroup, profilePlaybook string
	if req.Profile != "" {
		s.cfgMu.Lock()
		p, _ := s.cfg.GetProfile(req.Profile)
		s.cfgMu.Unlock()
		if p == nil {
			writeError(w, http.StatusBadRequest, "profile not found: "+req.Profile)
			return
		}
		// Apply profile values where request has zero/empty values
		if req.Release == "" && p.Release != "" {
			req.Release = p.Release
		}
		if req.CPUs == 0 && p.CPUs != 0 {
			req.CPUs = p.CPUs
		}
		if req.MemoryMB == 0 && p.MemoryMB != 0 {
			req.MemoryMB = p.MemoryMB
		}
		if req.DiskGB == 0 && p.DiskGB != 0 {
			req.DiskGB = p.DiskGB
		}
		if req.CloudInit == "" && p.CloudInit != "" {
			req.CloudInit = p.CloudInit
		}
		if req.Network == "" && p.Network != "" {
			req.Network = p.Network
		}
		profileGroup = p.Group
		profilePlaybook = p.Playbook
	}

	// Direct playbook field overrides profile playbook
	if req.Playbook != "" {
		profilePlaybook = req.Playbook
	}

	// Resolve the name now so we can return it immediately
	name := multipass.ResolveLaunchName(req.Name)
	if err := multipass.ValidateVMName(name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Resolve built-in cloud-init templates to a temp file
	cloudInitFile := req.CloudInit
	if strings.HasPrefix(cloudInitFile, "builtin:") {
		templateName := strings.TrimPrefix(cloudInitFile, "builtin:")
		content, err := s.builtinTemplatesFS.ReadFile("cloud-init/" + templateName)
		if err != nil {
			writeError(w, http.StatusBadRequest, "built-in template not found: "+templateName)
			return
		}
		tmpFile := filepath.Join(os.TempDir(), "passgo-cloudinit-"+templateName)
		if err := os.WriteFile(tmpFile, content, 0600); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to write temp cloud-init file")
			return
		}
		cloudInitFile = tmpFile
	}

	// Track and launch asynchronously
	s.launches.start(name)
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				s.launches.fail(name, fmt.Sprintf("panic: %v", rec))
				s.logger.Error("VM launch goroutine panicked", "name", name, "panic", rec)
			}
		}()
		_, err := s.mp.LaunchVM(name, req.Release, req.CPUs, req.MemoryMB, req.DiskGB, cloudInitFile, req.Network)
		if err != nil {
			s.logger.Error("VM launch failed", "name", name, "err", err)
			s.launches.fail(name, err.Error())
			s.eventLog.EmitEvent("vm", "create", "user", name, "failed", err.Error())
		} else {
			s.launches.complete(name)
			s.eventLog.EmitEvent("vm", "create", "user", name, "success", "")

			// Post-launch: assign to group if profile specified one
			if profileGroup != "" {
				s.cfgMu.Lock()
				s.cfg.VMGroups[name] = profileGroup
				s.cfg.Save(s.configPath)
				s.cfgMu.Unlock()
				s.logger.Info("auto-assigned VM to group", "vm", name, "group", profileGroup)
			}

			// Post-launch: enqueue playbook if profile specified one
			if profilePlaybook != "" {
				s.ansibleRunner.enqueue(profilePlaybook, []string{name})
				s.logger.Info("enqueued auto-run playbook", "vm", name, "playbook", profilePlaybook)
			}
		}
		// Clean up temp file if we created one
		if cloudInitFile != req.CloudInit {
			os.Remove(cloudInitFile)
		}
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{"name": name, "message": "VM launch started"})
}

type cloneVMRequest struct {
	Name     string `json:"name"`
	Snapshot string `json:"snapshot"`
}

func (s *Server) handleCloneVM(w http.ResponseWriter, r *http.Request) {
	source, ok := validVMName(w, r, "name")
	if !ok {
		return
	}

	var req cloneVMRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	destName := req.Name
	if destName == "" {
		destName = s.nextCloneName(source)
	}
	if err := multipass.ValidateVMName(destName); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Snapshot != "" {
		if err := multipass.ValidateVMName(req.Snapshot); err != nil {
			writeError(w, http.StatusBadRequest, "invalid snapshot name: "+err.Error())
			return
		}
	}

	s.launches.start(destName)
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				s.launches.fail(destName, fmt.Sprintf("panic: %v", rec))
				s.logger.Error("VM clone goroutine panicked", "dest", destName, "panic", rec)
			}
		}()
		_, err := s.mp.CloneVM(source, destName)
		if err != nil {
			s.logger.Error("VM clone failed", "source", source, "dest", destName, "err", err)
			s.launches.fail(destName, err.Error())
			s.eventLog.EmitEvent("vm", "clone", "user", destName, "failed", "source="+source+": "+err.Error())
			return
		}
		// If a snapshot was specified, restore the clone to that snapshot's state
		if req.Snapshot != "" {
			if err := s.mp.RestoreSnapshot(destName, req.Snapshot); err != nil {
				s.logger.Error("clone snapshot restore failed", "dest", destName, "snapshot", req.Snapshot, "err", err)
				s.launches.fail(destName, "cloned but failed to restore snapshot: "+err.Error())
				s.eventLog.EmitEvent("vm", "clone", "user", destName, "failed", "snapshot restore failed")
				return
			}
		}
		s.launches.complete(destName)
		s.eventLog.EmitEvent("vm", "clone", "user", destName, "success", "source="+source)
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{"name": destName, "message": "VM clone started"})
}

// nextCloneName finds the next available clone name like "source-clone1", "source-clone2", etc.
func (s *Server) nextCloneName(source string) string {
	vms, _ := s.mp.ListVMs()
	existing := make(map[string]bool)
	for _, vm := range vms {
		existing[vm.Name] = true
	}
	for _, l := range s.launches.list() {
		existing[l.Name] = true
	}
	for i := 1; ; i++ {
		name := fmt.Sprintf("%s-clone%d", source, i)
		if !existing[name] {
			return name
		}
	}
}

func (s *Server) handleListLaunches(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.launches.list())
}

func (s *Server) handleDismissLaunch(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	s.launches.dismiss(name)
	writeMessage(w, "dismissed")
}

func (s *Server) handleStartVM(w http.ResponseWriter, r *http.Request) {
	name, ok := validVMName(w, r, "name")
	if !ok {
		return
	}
	if err := s.mp.StartVM(name); err != nil {
		s.eventLog.EmitHTTPEvent(r, "vm", "start", name, "failed", err.Error())
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.eventLog.EmitHTTPEvent(r, "vm", "start", name, "success", "")
	writeMessage(w, "VM started")
}

func (s *Server) handleStopVM(w http.ResponseWriter, r *http.Request) {
	name, ok := validVMName(w, r, "name")
	if !ok {
		return
	}
	if err := s.mp.StopVM(name); err != nil {
		s.eventLog.EmitHTTPEvent(r, "vm", "stop", name, "failed", err.Error())
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.eventLog.EmitHTTPEvent(r, "vm", "stop", name, "success", "")
	s.ptySessions.killAllSessions(name)
	writeMessage(w, "VM stopped")
}

func (s *Server) handleSuspendVM(w http.ResponseWriter, r *http.Request) {
	name, ok := validVMName(w, r, "name")
	if !ok {
		return
	}
	if err := s.mp.SuspendVM(name); err != nil {
		s.eventLog.EmitHTTPEvent(r, "vm", "suspend", name, "failed", err.Error())
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.eventLog.EmitHTTPEvent(r, "vm", "suspend", name, "success", "")
	writeMessage(w, "VM suspended")
}

type deleteVMRequest struct {
	Purge bool `json:"purge"`
}

func (s *Server) handleDeleteVM(w http.ResponseWriter, r *http.Request) {
	name, ok := validVMName(w, r, "name")
	if !ok {
		return
	}
	var req deleteVMRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}
	if err := s.mp.DeleteVM(name, req.Purge); err != nil {
		s.eventLog.EmitHTTPEvent(r, "vm", "delete", name, "failed", err.Error())
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	detail := ""
	if req.Purge {
		detail = "purge=true"
	}
	s.eventLog.EmitHTTPEvent(r, "vm", "delete", name, "success", detail)
	s.ptySessions.killAllSessions(name)
	writeMessage(w, "VM deleted")
}

func (s *Server) handleRecoverVM(w http.ResponseWriter, r *http.Request) {
	name, ok := validVMName(w, r, "name")
	if !ok {
		return
	}
	if err := s.mp.RecoverVM(name); err != nil {
		s.eventLog.EmitHTTPEvent(r, "vm", "recover", name, "failed", err.Error())
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.eventLog.EmitHTTPEvent(r, "vm", "recover", name, "success", "")
	writeMessage(w, "VM recovered")
}

func (s *Server) handleStartAll(w http.ResponseWriter, r *http.Request) {
	if err := s.mp.StartAll(); err != nil {
		s.eventLog.EmitHTTPEvent(r, "vm", "start_all", "", "failed", err.Error())
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.eventLog.EmitHTTPEvent(r, "vm", "start_all", "", "success", "")
	writeMessage(w, "all stopped VMs started")
}

func (s *Server) handleStopAll(w http.ResponseWriter, r *http.Request) {
	if err := s.mp.StopAll(); err != nil {
		s.eventLog.EmitHTTPEvent(r, "vm", "stop_all", "", "failed", err.Error())
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.eventLog.EmitHTTPEvent(r, "vm", "stop_all", "", "success", "")
	writeMessage(w, "all running VMs stopped")
}

func (s *Server) handlePurge(w http.ResponseWriter, r *http.Request) {
	if err := s.mp.PurgeDeleted(); err != nil {
		s.eventLog.EmitHTTPEvent(r, "vm", "purge", "", "failed", err.Error())
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.eventLog.EmitHTTPEvent(r, "vm", "purge", "", "success", "")
	writeMessage(w, "deleted VMs purged")
}

type execRequest struct {
	Command []string `json:"command"`
}

type execResponse struct {
	Stdout string `json:"stdout"`
}

func (s *Server) handleExecInVM(w http.ResponseWriter, r *http.Request) {
	name, ok := validVMName(w, r, "name")
	if !ok {
		return
	}
	var req execRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Command) == 0 {
		writeError(w, http.StatusBadRequest, "command is required")
		return
	}
	output, err := s.mp.ExecInVM(name, req.Command)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, execResponse{Stdout: output})
}

func (s *Server) handleGetVMConfig(w http.ResponseWriter, r *http.Request) {
	name, ok := validVMName(w, r, "name")
	if !ok {
		return
	}
	cfg, err := s.mp.GetVMConfig(name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

type resizeVMRequest struct {
	CPUs     *int `json:"cpus"`
	MemoryMB *int `json:"memory_mb"`
	DiskGB   *int `json:"disk_gb"`
}

func (s *Server) handleResizeVM(w http.ResponseWriter, r *http.Request) {
	name, ok := validVMName(w, r, "name")
	if !ok {
		return
	}
	var req resizeVMRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.CPUs == nil && req.MemoryMB == nil && req.DiskGB == nil {
		writeError(w, http.StatusBadRequest, "no changes requested")
		return
	}

	vm, err := s.mp.GetVMInfo(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	// CPU and memory changes require the VM to be stopped
	if (req.CPUs != nil || req.MemoryMB != nil) && vm.State != "Stopped" {
		writeError(w, http.StatusConflict, "VM must be stopped to change CPU or memory")
		return
	}

	if req.CPUs != nil && *req.CPUs < multipass.MinCPUCores {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("CPUs must be at least %d", multipass.MinCPUCores))
		return
	}

	if req.MemoryMB != nil {
		if *req.MemoryMB < multipass.MinResizeRAMMB {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("memory must be at least %d MB", multipass.MinResizeRAMMB))
			return
		}
		hostRes, hostErr := multipass.GetHostResources()
		if hostErr == nil && int64(*req.MemoryMB) > hostRes.TotalMemoryMB {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("requested memory (%d MB) exceeds host capacity (%d MB)", *req.MemoryMB, hostRes.TotalMemoryMB))
			return
		}
		if hostErr != nil {
			s.logger.Warn("could not detect host resources for memory validation", "err", hostErr)
		}
	}

	if req.DiskGB != nil {
		if *req.DiskGB < multipass.MinDiskGB {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("disk must be at least %d GB", multipass.MinDiskGB))
			return
		}
		// Use multipass get for the configured disk size (info returns 0 when stopped)
		vmCfg, cfgErr := s.mp.GetVMConfig(name)
		if cfgErr == nil && vmCfg.DiskGB > 0 && int64(*req.DiskGB) < vmCfg.DiskGB {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("disk can only be increased, not decreased (current: %d GB)", vmCfg.DiskGB))
			return
		}
	}

	if req.CPUs != nil {
		if err := s.mp.SetVMCPUs(name, *req.CPUs); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to set CPUs: "+err.Error())
			return
		}
	}
	if req.MemoryMB != nil {
		if err := s.mp.SetVMMemory(name, *req.MemoryMB); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to set memory: "+err.Error())
			return
		}
	}
	if req.DiskGB != nil {
		if err := s.mp.SetVMDisk(name, *req.DiskGB); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to set disk: "+err.Error())
			return
		}
	}

	s.eventLog.EmitHTTPEvent(r, "vm", "resize", name, "success", "")
	writeMessage(w, "VM configuration updated")
}

func (s *Server) handleCloudInitStatus(w http.ResponseWriter, r *http.Request) {
	name, ok := validVMName(w, r, "name")
	if !ok {
		return
	}
	status, err := s.mp.GetCloudInitStatus(name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}
