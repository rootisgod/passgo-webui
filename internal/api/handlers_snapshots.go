package api

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	name, ok := validVMName(w, r, "name")
	if !ok {
		return
	}
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
	vmName, ok := validVMName(w, r, "name")
	if !ok {
		return
	}
	var req createSnapshotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "snapshot name is required")
		return
	}
	if err := s.mp.CreateSnapshot(vmName, req.Name, req.Comment); err != nil {
		s.eventLog.EmitHTTPEvent(r, "vm", "create_snapshot", vmName, "failed", err.Error())
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.eventLog.EmitHTTPEvent(r, "vm", "create_snapshot", vmName, "success", "snapshot="+req.Name)
	writeMessage(w, "snapshot created")
}

func (s *Server) handleRestoreSnapshot(w http.ResponseWriter, r *http.Request) {
	vmName, ok := validVMName(w, r, "name")
	if !ok {
		return
	}
	snap, ok := validSnapshotName(w, r, "snap")
	if !ok {
		return
	}
	if err := s.mp.RestoreSnapshot(vmName, snap); err != nil {
		s.eventLog.EmitHTTPEvent(r, "vm", "restore_snapshot", vmName, "failed", err.Error())
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.eventLog.EmitHTTPEvent(r, "vm", "restore_snapshot", vmName, "success", "snapshot="+snap)
	writeMessage(w, "snapshot restored")
}

func (s *Server) handleDeleteSnapshot(w http.ResponseWriter, r *http.Request) {
	vmName, ok := validVMName(w, r, "name")
	if !ok {
		return
	}
	snap, ok := validSnapshotName(w, r, "snap")
	if !ok {
		return
	}
	if err := s.mp.DeleteSnapshot(vmName, snap); err != nil {
		s.eventLog.EmitHTTPEvent(r, "vm", "delete_snapshot", vmName, "failed", err.Error())
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.eventLog.EmitHTTPEvent(r, "vm", "delete_snapshot", vmName, "success", "snapshot="+snap)
	writeMessage(w, "snapshot deleted")
}
