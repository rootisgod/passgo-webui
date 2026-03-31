package api

import (
	"net/http"
	"os"

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

type versionResponse struct {
	Version   string `json:"version"`
	BuildTime string `json:"build_time"`
	GitCommit string `json:"git_commit"`
	Hostname  string `json:"hostname"`
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
