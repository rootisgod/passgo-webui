package api

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/rootisgod/passgo-web/internal/config"
	"github.com/rootisgod/passgo-web/pkg/multipass"
)

func (s *Server) handleListNetworks(w http.ResponseWriter, r *http.Request) {
	networks, err := s.mp.ListNetworks()
	if err != nil {
		// Networks can fail on some platforms; return empty list rather than error
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	writeJSON(w, http.StatusOK, networks)
}

func (s *Server) handleListCloudInitTemplates(w http.ResponseWriter, r *http.Request) {
	var dirs []string
	if s.cfg.CloudInitDir != "" {
		dirs = append(dirs, s.cfg.CloudInitDir)
	}
	templates, err := s.mp.GetAllCloudInitTemplates(dirs)
	if err != nil {
		templates = nil
	}

	// Add built-in templates from embedded FS
	entries, _ := s.builtinTemplatesFS.ReadDir("cloud-init")
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		templates = append(templates, multipass.TemplateOption{
			Label:   entry.Name(),
			Path:    "builtin:" + entry.Name(),
			BuiltIn: true,
		})
	}

	if templates == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	writeJSON(w, http.StatusOK, templates)
}

func (s *Server) handleFindImages(w http.ResponseWriter, r *http.Request) {
	images, err := s.mp.FindImages()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, images)
}

type versionResponse struct {
	Version   string `json:"version"`
	BuildTime string `json:"build_time"`
	GitCommit string `json:"git_commit"`
	Hostname  string `json:"hostname"`
}

func (s *Server) handleHostResources(w http.ResponseWriter, r *http.Request) {
	res, err := multipass.GetHostResources()
	if err != nil {
		// Log partial failures but still return whatever data we collected
		s.logger.Warn("host resources", "error", err)
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleGetVMDefaults(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.cfg.VMDefaults)
}

func (s *Server) handleUpdateVMDefaults(w http.ResponseWriter, r *http.Request) {
	var req config.VMDefaults
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.CPUs < 1 {
		req.CPUs = 1
	}
	if req.MemoryMB < 512 {
		req.MemoryMB = 512
	}
	if req.DiskGB < 1 {
		req.DiskGB = 1
	}
	s.cfg.VMDefaults = &req
	configPath := config.DefaultConfigPath()
	if err := s.cfg.Save(configPath); err != nil {
		s.logger.Error("failed to save config", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to save configuration")
		return
	}
	writeJSON(w, http.StatusOK, s.cfg.VMDefaults)
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	hostname, _ := os.Hostname()
	writeJSON(w, http.StatusOK, versionResponse{
		Version:   s.version,
		BuildTime: s.buildTime,
		GitCommit: s.gitCommit,
		Hostname:  hostname,
	})
}
