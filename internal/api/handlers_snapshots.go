package api

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	snapshots, err := s.mp.ListSnapshots(name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, snapshots)
}

type createSnapshotRequest struct {
	Name    string `json:"name"`
	Comment string `json:"comment"`
}

func (s *Server) handleCreateSnapshot(w http.ResponseWriter, r *http.Request) {
	vmName := r.PathValue("name")
	var req createSnapshotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "snapshot name is required")
		return
	}
	if err := s.mp.CreateSnapshot(vmName, req.Name, req.Comment); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeMessage(w, "snapshot created")
}

func (s *Server) handleRestoreSnapshot(w http.ResponseWriter, r *http.Request) {
	vmName := r.PathValue("name")
	snap := r.PathValue("snap")
	if err := s.mp.RestoreSnapshot(vmName, snap); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeMessage(w, "snapshot restored")
}

func (s *Server) handleDeleteSnapshot(w http.ResponseWriter, r *http.Request) {
	vmName := r.PathValue("name")
	snap := r.PathValue("snap")
	if err := s.mp.DeleteSnapshot(vmName, snap); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeMessage(w, "snapshot deleted")
}
