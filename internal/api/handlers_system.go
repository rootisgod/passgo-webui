package api

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/rootisgod/passgo-web/internal/auth"
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

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string `json:"token"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Username != s.cfg.Username || !s.cfg.CheckPassword(req.Password) {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := s.sessions.Create()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(24 * time.Hour / time.Second),
	})

	writeJSON(w, http.StatusOK, loginResponse{Token: token})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := auth.TokenFromRequest(r)
	if token != "" {
		s.sessions.Delete(token)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	writeMessage(w, "logged out")
}
