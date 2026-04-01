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
	name := r.PathValue("name")
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
}

func (s *Server) handleCreateVM(w http.ResponseWriter, r *http.Request) {
	var req createVMRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Resolve the name now so we can return it immediately
	name := multipass.ResolveLaunchName(req.Name)

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
		_, err := s.mp.LaunchVM(name, req.Release, req.CPUs, req.MemoryMB, req.DiskGB, cloudInitFile, req.Network)
		if err != nil {
			s.logger.Error("VM launch failed", "name", name, "err", err)
			s.launches.fail(name, err.Error())
		} else {
			s.launches.complete(name)
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
	source := r.PathValue("name")

	var req cloneVMRequest
	json.NewDecoder(r.Body).Decode(&req)

	destName := req.Name
	if destName == "" {
		destName = s.nextCloneName(source)
	}

	s.launches.start(destName)
	go func() {
		_, err := s.mp.CloneVM(source, destName)
		if err != nil {
			s.logger.Error("VM clone failed", "source", source, "dest", destName, "err", err)
			s.launches.fail(destName, err.Error())
			return
		}
		// If a snapshot was specified, restore the clone to that snapshot's state
		if req.Snapshot != "" {
			if err := s.mp.RestoreSnapshot(destName, req.Snapshot); err != nil {
				s.logger.Error("clone snapshot restore failed", "dest", destName, "snapshot", req.Snapshot, "err", err)
				s.launches.fail(destName, "cloned but failed to restore snapshot: "+err.Error())
				return
			}
		}
		s.launches.complete(destName)
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
	name := r.PathValue("name")
	if err := s.mp.StartVM(name); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeMessage(w, "VM started")
}

func (s *Server) handleStopVM(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.mp.StopVM(name); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeMessage(w, "VM stopped")
}

func (s *Server) handleSuspendVM(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.mp.SuspendVM(name); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeMessage(w, "VM suspended")
}

type deleteVMRequest struct {
	Purge bool `json:"purge"`
}

func (s *Server) handleDeleteVM(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req deleteVMRequest
	json.NewDecoder(r.Body).Decode(&req) // body is optional
	if err := s.mp.DeleteVM(name, req.Purge); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeMessage(w, "VM deleted")
}

func (s *Server) handleRecoverVM(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.mp.RecoverVM(name); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeMessage(w, "VM recovered")
}

func (s *Server) handleStartAll(w http.ResponseWriter, r *http.Request) {
	if err := s.mp.StartAll(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeMessage(w, "all stopped VMs started")
}

func (s *Server) handleStopAll(w http.ResponseWriter, r *http.Request) {
	if err := s.mp.StopAll(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeMessage(w, "all running VMs stopped")
}

func (s *Server) handlePurge(w http.ResponseWriter, r *http.Request) {
	if err := s.mp.PurgeDeleted(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeMessage(w, "deleted VMs purged")
}

type execRequest struct {
	Command []string `json:"command"`
}

type execResponse struct {
	Stdout string `json:"stdout"`
}

func (s *Server) handleExecInVM(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
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

func (s *Server) handleCloudInitStatus(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	status, err := s.mp.GetCloudInitStatus(name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}
