package api

import (
	"encoding/json"
	"net/http"

	"github.com/rootisgod/passgo-web/pkg/multipass"
)

type cloudInitTemplateRequest struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type cloudInitTemplateResponse struct {
	Name    string `json:"name"`
	Content string `json:"content"`
	BuiltIn bool   `json:"builtIn,omitempty"`
}

func (s *Server) handleGetCloudInitTemplate(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	// Check built-in templates first
	if data, err := s.builtinTemplatesFS.ReadFile("cloud-init/" + name); err == nil {
		writeJSON(w, http.StatusOK, cloudInitTemplateResponse{Name: name, Content: string(data), BuiltIn: true})
		return
	}

	// Fall back to user templates
	if s.cfg.CloudInitDir == "" {
		writeError(w, http.StatusNotFound, "template not found")
		return
	}
	content, err := multipass.ReadCloudInitTemplate(s.cfg.CloudInitDir, name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cloudInitTemplateResponse{Name: name, Content: content})
}

func (s *Server) handleCreateCloudInitTemplate(w http.ResponseWriter, r *http.Request) {
	if s.cfg.CloudInitDir == "" {
		writeError(w, http.StatusBadRequest, "cloud-init directory not configured")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB limit
	var req cloudInitTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.Content == "" {
		writeError(w, http.StatusBadRequest, "name and content are required")
		return
	}
	// Check if file already exists
	if _, err := multipass.ReadCloudInitTemplate(s.cfg.CloudInitDir, req.Name); err == nil {
		writeError(w, http.StatusConflict, "template already exists")
		return
	}
	if err := multipass.WriteCloudInitTemplate(s.cfg.CloudInitDir, req.Name, req.Content); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, cloudInitTemplateResponse{Name: req.Name, Content: req.Content})
}

func (s *Server) handleUpdateCloudInitTemplate(w http.ResponseWriter, r *http.Request) {
	if s.cfg.CloudInitDir == "" {
		writeError(w, http.StatusBadRequest, "cloud-init directory not configured")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	name := r.PathValue("name")
	var req cloudInitTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}
	// Verify file exists
	if _, err := multipass.ReadCloudInitTemplate(s.cfg.CloudInitDir, name); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err := multipass.WriteCloudInitTemplate(s.cfg.CloudInitDir, name, req.Content); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cloudInitTemplateResponse{Name: name, Content: req.Content})
}

func (s *Server) handleDeleteCloudInitTemplate(w http.ResponseWriter, r *http.Request) {
	if s.cfg.CloudInitDir == "" {
		writeError(w, http.StatusBadRequest, "cloud-init directory not configured")
		return
	}
	name := r.PathValue("name")
	if err := multipass.DeleteCloudInitTemplate(s.cfg.CloudInitDir, name); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeMessage(w, "template deleted")
}
