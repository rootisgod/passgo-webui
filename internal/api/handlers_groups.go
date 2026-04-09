package api

import (
	"encoding/json"
	"net/http"
	"slices"

	"github.com/rootisgod/passgo-web/internal/config"
)

type groupsResponse struct {
	Groups   []string          `json:"groups"`
	VMGroups map[string]string `json:"vmGroups"`
}

func (s *Server) handleListGroups(w http.ResponseWriter, r *http.Request) {
	s.groupMu.Lock()
	defer s.groupMu.Unlock()

	// Auto-clean stale VM assignments
	vms, err := s.mp.ListVMs()
	if err == nil {
		vmNames := make(map[string]bool, len(vms))
		for _, vm := range vms {
			vmNames[vm.Name] = true
		}
		changed := false
		for name := range s.cfg.VMGroups {
			if !vmNames[name] {
				delete(s.cfg.VMGroups, name)
				changed = true
			}
		}
		if changed {
			s.cfg.Save(config.DefaultConfigPath())
		}
	}

	writeJSON(w, http.StatusOK, groupsResponse{
		Groups:   s.cfg.Groups,
		VMGroups: s.cfg.VMGroups,
	})
}

func (s *Server) handleCreateGroup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	s.groupMu.Lock()
	defer s.groupMu.Unlock()

	if slices.Contains(s.cfg.Groups, req.Name) {
		writeError(w, http.StatusConflict, "group already exists")
		return
	}

	s.cfg.Groups = append(s.cfg.Groups, req.Name)
	if err := s.cfg.Save(config.DefaultConfigPath()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save config")
		return
	}
	s.eventLog.EmitHTTPEvent(r, "config", "create_group", req.Name, "success", "")
	writeMessage(w, "group created")
}

func (s *Server) handleRenameGroup(w http.ResponseWriter, r *http.Request) {
	oldName := r.PathValue("name")
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	s.groupMu.Lock()
	defer s.groupMu.Unlock()

	idx := slices.Index(s.cfg.Groups, oldName)
	if idx < 0 {
		writeError(w, http.StatusNotFound, "group not found")
		return
	}

	if oldName != req.Name && slices.Contains(s.cfg.Groups, req.Name) {
		writeError(w, http.StatusConflict, "group name already in use")
		return
	}

	s.cfg.Groups[idx] = req.Name
	for vm, g := range s.cfg.VMGroups {
		if g == oldName {
			s.cfg.VMGroups[vm] = req.Name
		}
	}
	if err := s.cfg.Save(config.DefaultConfigPath()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save config")
		return
	}
	s.eventLog.EmitHTTPEvent(r, "config", "rename_group", req.Name, "success", "from="+oldName)
	writeMessage(w, "group renamed")
}

func (s *Server) handleDeleteGroup(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	s.groupMu.Lock()
	defer s.groupMu.Unlock()

	idx := slices.Index(s.cfg.Groups, name)
	if idx < 0 {
		writeError(w, http.StatusNotFound, "group not found")
		return
	}

	s.cfg.Groups = slices.Delete(s.cfg.Groups, idx, idx+1)
	for vm, g := range s.cfg.VMGroups {
		if g == name {
			delete(s.cfg.VMGroups, vm)
		}
	}
	if err := s.cfg.Save(config.DefaultConfigPath()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save config")
		return
	}
	s.eventLog.EmitHTTPEvent(r, "config", "delete_group", name, "success", "")
	writeMessage(w, "group deleted")
}

func (s *Server) handleAssignVmGroup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		VM    string `json:"vm"`
		Group string `json:"group"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.VM == "" {
		writeError(w, http.StatusBadRequest, "vm is required")
		return
	}

	s.groupMu.Lock()
	defer s.groupMu.Unlock()

	if req.Group != "" && !slices.Contains(s.cfg.Groups, req.Group) {
		writeError(w, http.StatusNotFound, "group not found")
		return
	}

	if req.Group == "" {
		delete(s.cfg.VMGroups, req.VM)
	} else {
		s.cfg.VMGroups[req.VM] = req.Group
	}
	if err := s.cfg.Save(config.DefaultConfigPath()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save config")
		return
	}
	s.eventLog.EmitHTTPEvent(r, "config", "assign_group", req.VM, "success", "group="+req.Group)
	writeMessage(w, "vm group updated")
}

func (s *Server) handleReorderGroups(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Groups []string `json:"groups"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	s.groupMu.Lock()
	defer s.groupMu.Unlock()

	s.cfg.Groups = req.Groups
	if err := s.cfg.Save(config.DefaultConfigPath()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save config")
		return
	}
	writeMessage(w, "groups reordered")
}
