package api

import (
	"encoding/json"
	"net/http"

	"github.com/rootisgod/passgo-web/internal/config"
)

func (s *Server) handleListProfiles(w http.ResponseWriter, r *http.Request) {
	s.groupMu.Lock()
	profiles := s.cfg.GetProfiles()
	s.groupMu.Unlock()
	writeJSON(w, http.StatusOK, profiles)
}

func (s *Server) handleCreateProfile(w http.ResponseWriter, r *http.Request) {
	var p config.Profile
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	s.groupMu.Lock()
	err := s.cfg.AddProfile(p)
	if err != nil {
		s.groupMu.Unlock()
		if err.Error() == "profile with id \""+p.ID+"\" already exists" {
			writeError(w, http.StatusConflict, err.Error())
		} else {
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	if saveErr := s.cfg.Save(config.DefaultConfigPath()); saveErr != nil {
		s.groupMu.Unlock()
		s.logger.Error("failed to save config", "err", saveErr)
		writeError(w, http.StatusInternalServerError, "failed to save configuration")
		return
	}
	s.groupMu.Unlock()

	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var p config.Profile
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	p.ID = id

	s.groupMu.Lock()
	err := s.cfg.UpdateProfile(p)
	if err != nil {
		s.groupMu.Unlock()
		if err.Error() == "profile \""+id+"\" not found" {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	if saveErr := s.cfg.Save(config.DefaultConfigPath()); saveErr != nil {
		s.groupMu.Unlock()
		s.logger.Error("failed to save config", "err", saveErr)
		writeError(w, http.StatusInternalServerError, "failed to save configuration")
		return
	}
	s.groupMu.Unlock()

	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleDeleteProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	s.groupMu.Lock()
	err := s.cfg.DeleteProfile(id)
	if err != nil {
		s.groupMu.Unlock()
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if saveErr := s.cfg.Save(config.DefaultConfigPath()); saveErr != nil {
		s.groupMu.Unlock()
		s.logger.Error("failed to save config", "err", saveErr)
		writeError(w, http.StatusInternalServerError, "failed to save configuration")
		return
	}
	s.groupMu.Unlock()

	writeMessage(w, "profile deleted")
}
