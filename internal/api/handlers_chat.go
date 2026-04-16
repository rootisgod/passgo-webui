package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/rootisgod/passgo-web/internal/config"
)

type chatRequestBody struct {
	Message        string        `json:"message"`
	History        []chatMessage `json:"history"`
	ConfirmedTools []string      `json:"confirmed_tools,omitempty"` // tool call IDs the user has approved
}

type chatConfigResponse struct {
	BaseURL   string `json:"base_url"`
	Model     string `json:"model"`
	HasAPIKey bool   `json:"has_api_key"`
	ReadOnly  bool   `json:"read_only"`
}

type updateChatConfigRequest struct {
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key"`
	Model    string `json:"model"`
	ReadOnly *bool  `json:"read_only,omitempty"` // pointer so we can distinguish unset from false
}

// handleChat accepts a user message and streams the LLM response via SSE.
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	var req chatRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		writeError(w, http.StatusBadRequest, "message is required")
		return
	}

	// Validate LLM config
	cfg := s.cfg.LLM
	if cfg == nil {
		writeError(w, http.StatusBadRequest, "LLM not configured. Open Chat Settings to configure.")
		return
	}
	if cfg.BaseURL == "" || cfg.Model == "" {
		writeError(w, http.StatusBadRequest, "LLM base URL and model must be configured")
		return
	}
	// Require API key for non-local providers
	if cfg.APIKey == "" && !isLocalProvider(cfg.BaseURL) {
		writeError(w, http.StatusBadRequest, "API key required for remote LLM providers. Configure it in Chat Settings.")
		return
	}

	// Set up SSE
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Build conversation from history + new message
	messages := make([]chatMessage, len(req.History))
	copy(messages, req.History)
	messages = append(messages, chatMessage{Role: "user", Content: req.Message})

	// Build confirmed tools set
	confirmed := make(map[string]bool, len(req.ConfirmedTools))
	for _, id := range req.ConfirmedTools {
		confirmed[id] = true
	}

	// Run agent loop with timeout to prevent runaway cost
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	eventCh := make(chan sseEvent, 64)
	go s.runAgentLoop(ctx, messages, confirmed, eventCh)

	for event := range eventCh {
		data, err := json.Marshal(event)
		if err != nil {
			continue
		}
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}
}

// handleGetChatConfig returns the current LLM configuration (without the API key).
func (s *Server) handleGetChatConfig(w http.ResponseWriter, r *http.Request) {
	cfg := s.cfg.LLM
	if cfg == nil {
		cfg = &config.LLMConfig{
			BaseURL: "https://openrouter.ai/api/v1",
			Model:   "anthropic/claude-sonnet-4",
		}
	}
	writeJSON(w, http.StatusOK, chatConfigResponse{
		BaseURL:   cfg.BaseURL,
		Model:     cfg.Model,
		HasAPIKey: cfg.APIKey != "",
		ReadOnly:  cfg.ReadOnly,
	})
}

// handleUpdateChatConfig updates the LLM configuration and persists it.
func (s *Server) handleUpdateChatConfig(w http.ResponseWriter, r *http.Request) {
	var req updateChatConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if s.cfg.LLM == nil {
		s.cfg.LLM = &config.LLMConfig{}
	}
	if req.BaseURL != "" {
		if !isAllowedLLMBaseURL(req.BaseURL) {
			writeError(w, http.StatusBadRequest, "invalid base URL: must be HTTPS or a local address (localhost/127.0.0.1)")
			return
		}
		// Clear API key when switching to a different host to prevent credential leakage
		if s.cfg.LLM.BaseURL != "" && !sameURLHost(s.cfg.LLM.BaseURL, req.BaseURL) {
			s.cfg.LLM.APIKey = ""
		}
		s.cfg.LLM.BaseURL = req.BaseURL
	}
	if req.APIKey != "" {
		s.cfg.LLM.APIKey = req.APIKey
	}
	if req.Model != "" {
		s.cfg.LLM.Model = req.Model
	}
	if req.ReadOnly != nil {
		s.cfg.LLM.ReadOnly = *req.ReadOnly
	}

	if err := s.cfg.Save(s.configPath); err != nil {
		s.logger.Error("failed to save config", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to save configuration")
		return
	}

	writeJSON(w, http.StatusOK, chatConfigResponse{
		BaseURL:   s.cfg.LLM.BaseURL,
		Model:     s.cfg.LLM.Model,
		HasAPIKey: s.cfg.LLM.APIKey != "",
		ReadOnly:  s.cfg.LLM.ReadOnly,
	})
}

// handleListModels proxies to the provider's /models endpoint and returns
// a normalized list of {id, name} objects.
func (s *Server) handleListModels(w http.ResponseWriter, r *http.Request) {
	cfg := s.cfg.LLM
	if cfg == nil || cfg.BaseURL == "" {
		writeError(w, http.StatusBadRequest, "LLM provider not configured")
		return
	}
	if cfg.APIKey == "" && !isLocalProvider(cfg.BaseURL) {
		writeError(w, http.StatusBadRequest, "API key required to fetch models from remote provider")
		return
	}

	url := strings.TrimRight(cfg.BaseURL, "/") + "/models"
	req, err := http.NewRequestWithContext(r.Context(), "GET", url, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create request")
		return
	}
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("failed to reach provider: %s", err.Error()))
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to read provider response")
		return
	}

	if resp.StatusCode != http.StatusOK {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("provider returned status %d: %s", resp.StatusCode, truncate(string(body), 200)))
		return
	}

	// Parse the OpenAI-compatible response
	var modelsResp struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		writeError(w, http.StatusBadGateway, "failed to parse models response")
		return
	}

	// Normalize to {id, name} — use id as name fallback
	type modelEntry struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	models := make([]modelEntry, 0, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		name := m.Name
		if name == "" {
			name = m.ID
		}
		models = append(models, modelEntry{ID: m.ID, Name: name})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].Name < models[j].Name
	})

	writeJSON(w, http.StatusOK, models)
}

// isLocalProvider checks if the base URL points to a local service (e.g. Ollama).
func isLocalProvider(baseURL string) bool {
	lower := strings.ToLower(baseURL)
	return strings.Contains(lower, "localhost") || strings.Contains(lower, "127.0.0.1")
}

// isAllowedLLMBaseURL validates that the base URL is safe to use as an LLM endpoint.
// Allows HTTPS URLs and local HTTP URLs (localhost/127.0.0.1). Blocks private
// IP ranges, link-local addresses, and cloud metadata endpoints to prevent SSRF.
func isAllowedLLMBaseURL(rawURL string) bool {
	lower := strings.ToLower(rawURL)

	// Must be http or https
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return false
	}

	// HTTPS is always allowed (remote providers)
	if strings.HasPrefix(lower, "https://") {
		return true
	}

	// HTTP is only allowed for localhost/127.0.0.1 (e.g. Ollama)
	return isLocalProvider(rawURL)
}

// sameURLHost checks if two URLs have the same host (ignoring port and path).
func sameURLHost(a, b string) bool {
	hostA := extractHost(a)
	hostB := extractHost(b)
	return strings.EqualFold(hostA, hostB)
}

func extractHost(rawURL string) string {
	u := rawURL
	if i := strings.Index(u, "://"); i >= 0 {
		u = u[i+3:]
	}
	// Strip path
	if i := strings.Index(u, "/"); i >= 0 {
		u = u[:i]
	}
	// Strip port
	if i := strings.LastIndex(u, ":"); i >= 0 {
		u = u[:i]
	}
	return u
}
