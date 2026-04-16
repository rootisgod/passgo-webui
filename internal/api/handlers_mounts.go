package api

import (
	"encoding/json"
	"net/http"
	"os"
)

func (s *Server) handleListMounts(w http.ResponseWriter, r *http.Request) {
	name, ok := validVMName(w, r, "name")
	if !ok {
		return
	}
	mounts, err := s.mp.ListMounts(name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, mounts)
}

type addMountRequest struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

func (s *Server) handleAddMount(w http.ResponseWriter, r *http.Request) {
	vmName, ok := validVMName(w, r, "name")
	if !ok {
		return
	}
	var req addMountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Source == "" || req.Target == "" {
		writeError(w, http.StatusBadRequest, "source and target are required")
		return
	}
	if err := s.mp.AddMount(vmName, req.Source, req.Target); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeMessage(w, "mount added")
}

type removeMountRequest struct {
	Target string `json:"target"`
}

func (s *Server) handleRemoveMount(w http.ResponseWriter, r *http.Request) {
	vmName, ok := validVMName(w, r, "name")
	if !ok {
		return
	}
	var req removeMountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Target == "" {
		writeError(w, http.StatusBadRequest, "target is required")
		return
	}
	if err := s.mp.RemoveMount(vmName, req.Target); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeMessage(w, "mount removed")
}

type openMountRequest struct {
	Target string `json:"target"`
}

// handleOpenMount opens the host-side source_path of an existing mount in the
// OS's native file manager. The path is looked up from ListMounts and never
// taken from the request body — the body only specifies which mount (by target)
// to open. This keeps the endpoint from becoming an arbitrary-path-opener.
func (s *Server) handleOpenMount(w http.ResponseWriter, r *http.Request) {
	vmName, ok := validVMName(w, r, "name")
	if !ok {
		return
	}
	var req openMountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Target == "" {
		writeError(w, http.StatusBadRequest, "target is required")
		return
	}

	mounts, err := s.mp.ListMounts(vmName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var source string
	for _, m := range mounts {
		if m.TargetPath == req.Target {
			source = m.SourcePath
			break
		}
	}
	if source == "" {
		writeError(w, http.StatusNotFound, "no mount with that target path")
		return
	}
	if _, err := os.Stat(source); err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "source path no longer exists: "+source)
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := openHostPath(source); err != nil {
		if s.eventLog != nil {
			s.eventLog.EmitEvent("vm", "mount_open", "user", vmName, "failed", err.Error())
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if s.eventLog != nil {
		s.eventLog.EmitEvent("vm", "mount_open", "user", vmName, "ok", source)
	}
	writeMessage(w, "opened")
}
