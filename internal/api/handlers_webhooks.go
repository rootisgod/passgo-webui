package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/rootisgod/passgo-web/internal/config"
)

type webhookResponse struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	URL        string   `json:"url"`
	Enabled    bool     `json:"enabled"`
	Categories []string `json:"categories"`
	Results    []string `json:"results"`
	Secret     bool     `json:"secret"`
	CreatedAt  string   `json:"created_at"`
}

func toWebhookResponse(wh config.Webhook) webhookResponse {
	return webhookResponse{
		ID:         wh.ID,
		Name:       wh.Name,
		URL:        wh.URL,
		Enabled:    wh.Enabled,
		Categories: wh.Categories,
		Results:    wh.Results,
		Secret:     wh.Secret != "",
		CreatedAt:  wh.CreatedAt,
	}
}

func (srv *Server) handleListWebhooks(w http.ResponseWriter, r *http.Request) {
	srv.cfgMu.Lock()
	webhooks := srv.cfg.GetWebhooks()
	srv.cfgMu.Unlock()

	resp := make([]webhookResponse, len(webhooks))
	for i, wh := range webhooks {
		resp[i] = toWebhookResponse(wh)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (srv *Server) handleCreateWebhook(w http.ResponseWriter, r *http.Request) {
	var req config.Webhook
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Generate ID
	idBytes := make([]byte, 8)
	if _, err := rand.Read(idBytes); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate ID")
		return
	}
	req.ID = "wh_" + hex.EncodeToString(idBytes)
	req.CreatedAt = time.Now().UTC().Format(time.RFC3339)

	srv.cfgMu.Lock()
	if err := srv.cfg.AddWebhook(req); err != nil {
		srv.cfgMu.Unlock()
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := srv.cfg.Save(srv.configPath); err != nil {
		srv.cfgMu.Unlock()
		writeError(w, http.StatusInternalServerError, "failed to save config")
		return
	}
	srv.cfgMu.Unlock()

	srv.eventLog.EmitHTTPEvent(r, "config", "create_webhook", req.Name, "success", "")
	writeJSON(w, http.StatusCreated, toWebhookResponse(req))
}

func (srv *Server) handleUpdateWebhook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req config.Webhook
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.ID = id

	srv.cfgMu.Lock()
	existing, _ := srv.cfg.GetWebhook(id)
	if existing == nil {
		srv.cfgMu.Unlock()
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}

	// Preserve created_at and secret if not provided
	if req.CreatedAt == "" {
		req.CreatedAt = existing.CreatedAt
	}
	if req.Secret == "" {
		req.Secret = existing.Secret
	}

	if err := srv.cfg.UpdateWebhook(req); err != nil {
		srv.cfgMu.Unlock()
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := srv.cfg.Save(srv.configPath); err != nil {
		srv.cfgMu.Unlock()
		writeError(w, http.StatusInternalServerError, "failed to save config")
		return
	}
	srv.cfgMu.Unlock()

	srv.eventLog.EmitHTTPEvent(r, "config", "update_webhook", req.Name, "success", "")
	writeJSON(w, http.StatusOK, toWebhookResponse(req))
}

func (srv *Server) handleDeleteWebhook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	srv.cfgMu.Lock()
	if err := srv.cfg.DeleteWebhook(id); err != nil {
		srv.cfgMu.Unlock()
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err := srv.cfg.Save(srv.configPath); err != nil {
		srv.cfgMu.Unlock()
		writeError(w, http.StatusInternalServerError, "failed to save config")
		return
	}
	srv.cfgMu.Unlock()

	srv.eventLog.EmitHTTPEvent(r, "config", "delete_webhook", id, "success", "")
	writeMessage(w, "webhook deleted")
}

func (srv *Server) handleTestWebhook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	srv.cfgMu.Lock()
	wh, _ := srv.cfg.GetWebhook(id)
	if wh == nil {
		srv.cfgMu.Unlock()
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}
	whCopy := *wh
	srv.cfgMu.Unlock()

	testEvent := Event{
		ID:        "test",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Category:  "test",
		Action:    "test",
		Actor:     "user",
		Resource:  "test",
		Result:    "success",
		Detail:    "This is a test notification from PassGo Web",
	}

	// Fire directly (bypass loop prevention since this is an explicit test)
	go srv.fireWebhook(whCopy, testEvent)

	writeMessage(w, "test notification sent")
}
