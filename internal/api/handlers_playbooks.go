package api

import (
	"encoding/json"
	"net/http"

	"github.com/rootisgod/passgo-web/pkg/multipass"
)

type playbookEntry struct {
	Name string `json:"name"`
}

type playbookResponse struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type playbookRequest struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

func (s *Server) handleListPlaybooks(w http.ResponseWriter, r *http.Request) {
	names, err := multipass.ListPlaybooks(s.cfg.PlaybooksDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list playbooks")
		return
	}
	entries := make([]playbookEntry, len(names))
	for i, n := range names {
		entries[i] = playbookEntry{Name: n}
	}
	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) handleGetPlaybook(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	content, err := multipass.ReadPlaybook(s.cfg.PlaybooksDir, name)
	if err != nil {
		writeError(w, http.StatusNotFound, "playbook not found")
		return
	}
	writeJSON(w, http.StatusOK, playbookResponse{Name: name, Content: content})
}

func (s *Server) handleCreatePlaybook(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req playbookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Name == "" || req.Content == "" {
		writeError(w, http.StatusBadRequest, "name and content are required")
		return
	}
	if _, err := multipass.ReadPlaybook(s.cfg.PlaybooksDir, req.Name); err == nil {
		writeError(w, http.StatusConflict, "playbook already exists")
		return
	}
	if err := multipass.WritePlaybook(s.cfg.PlaybooksDir, req.Name, req.Content); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, playbookResponse{Name: req.Name, Content: req.Content})
}

func (s *Server) handleUpdatePlaybook(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}
	if _, err := multipass.ReadPlaybook(s.cfg.PlaybooksDir, name); err != nil {
		writeError(w, http.StatusNotFound, "playbook not found")
		return
	}
	if err := multipass.WritePlaybook(s.cfg.PlaybooksDir, name, req.Content); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, playbookResponse{Name: name, Content: req.Content})
}

func (s *Server) handleDeletePlaybook(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := multipass.DeletePlaybook(s.cfg.PlaybooksDir, name); err != nil {
		writeError(w, http.StatusNotFound, "playbook not found")
		return
	}
	writeMessage(w, "playbook deleted")
}
