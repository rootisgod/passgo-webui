package api

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handleListMounts(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
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
	vmName := r.PathValue("name")
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
	vmName := r.PathValue("name")
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
