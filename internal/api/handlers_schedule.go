package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rootisgod/passgo-web/internal/config"
)

func (s *Server) handleRunScheduleNow(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.scheduler.runNow(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeMessage(w, "schedule executed")
}

func (s *Server) handleScheduleHistory(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.scheduler.getHistory())
}

func (s *Server) handleListSchedules(w http.ResponseWriter, r *http.Request) {
	s.groupMu.Lock()
	schedules := s.cfg.GetSchedules()
	s.groupMu.Unlock()
	writeJSON(w, http.StatusOK, schedules)
}

func (s *Server) handleCreateSchedule(w http.ResponseWriter, r *http.Request) {
	var sched config.Schedule
	if err := json.NewDecoder(r.Body).Decode(&sched); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	s.groupMu.Lock()
	err := s.cfg.AddSchedule(sched)
	if err != nil {
		s.groupMu.Unlock()
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "already exists") {
			status = http.StatusConflict
		}
		writeError(w, status, err.Error())
		return
	}
	saveErr := s.cfg.Save(config.DefaultConfigPath())
	s.groupMu.Unlock()

	if saveErr != nil {
		writeError(w, http.StatusInternalServerError, "save config: "+saveErr.Error())
		return
	}
	writeJSON(w, http.StatusCreated, sched)
}

func (s *Server) handleUpdateSchedule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var sched config.Schedule
	if err := json.NewDecoder(r.Body).Decode(&sched); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	sched.ID = id

	s.groupMu.Lock()
	err := s.cfg.UpdateSchedule(sched)
	if err != nil {
		s.groupMu.Unlock()
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	saveErr := s.cfg.Save(config.DefaultConfigPath())
	s.groupMu.Unlock()

	if saveErr != nil {
		writeError(w, http.StatusInternalServerError, "save config: "+saveErr.Error())
		return
	}
	writeJSON(w, http.StatusOK, sched)
}

func (s *Server) handleDeleteSchedule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	s.groupMu.Lock()
	err := s.cfg.DeleteSchedule(id)
	if err != nil {
		s.groupMu.Unlock()
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	saveErr := s.cfg.Save(config.DefaultConfigPath())
	s.groupMu.Unlock()

	if saveErr != nil {
		writeError(w, http.StatusInternalServerError, "save config: "+saveErr.Error())
		return
	}
	writeMessage(w, "schedule deleted")
}
