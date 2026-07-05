package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rootisgod/passgo-web/internal/aiaccess"
	"github.com/rootisgod/passgo-web/internal/config"
)

type chatRequestBody struct {
	Message        string        `json:"message"`
	History        []chatMessage `json:"history"`
	ConfirmedTools []string      `json:"confirmed_tools,omitempty"` // tool call IDs the user has approved
}

type chatConfigResponse struct {
	BaseURL      string `json:"base_url"`
	Model        string `json:"model"`
	HasAPIKey    bool   `json:"has_api_key"`
	APIKeySource string `json:"api_key_source"`
	Provider     string `json:"provider"`
	ReadOnly     bool   `json:"read_only"`
}

type updateChatConfigRequest struct {
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key"`
	Model    string `json:"model"`
	Provider string `json:"provider"`
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
	cfg := s.llmConfigSnapshot()
	resolved := resolveLLMConfig(cfg)
	if resolved.IsNative {
		if err := nativeProviderReady(r.Context(), resolved.Provider); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	} else {
		if cfg.BaseURL == "" || cfg.Model == "" {
			writeError(w, http.StatusBadRequest, "LLM base URL and model must be configured")
			return
		}
		if !isAllowedLLMBaseURL(cfg.BaseURL) {
			writeError(w, http.StatusBadRequest, "invalid LLM base URL configured")
			return
		}
		if resolved.NeedsAPIKey && !resolved.HasAPIKey() {
			msg := "API key required for remote LLM providers. Configure it in Chat Settings."
			if resolved.IsGateway {
				msg = "Vercel AI Gateway API key required. Add one in Chat Settings or set AI_GATEWAY_API_KEY on the server."
			}
			writeError(w, http.StatusBadRequest, msg)
			return
		}
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
	go s.runAgentLoop(ctx, &cfg, messages, confirmed, eventCh)

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
	cfg := s.llmConfigSnapshot()
	writeJSON(w, http.StatusOK, chatConfigResponseFor(cfg))
}

// handleUpdateChatConfig updates the LLM configuration and persists it.
func (s *Server) handleUpdateChatConfig(w http.ResponseWriter, r *http.Request) {
	var req updateChatConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.BaseURL = strings.TrimSpace(req.BaseURL)
	req.APIKey = strings.TrimSpace(req.APIKey)
	req.Model = strings.TrimSpace(req.Model)
	req.Provider = strings.TrimSpace(req.Provider)

	if !aiaccess.IsAllowedProvider(req.Provider) {
		writeError(w, http.StatusBadRequest, "invalid provider")
		return
	}
	if !aiaccess.IsNativeProvider(req.Provider) && req.BaseURL != "" {
		if !isAllowedLLMBaseURL(req.BaseURL) {
			writeError(w, http.StatusBadRequest, "invalid base URL: must be HTTPS or a local address (localhost/127.0.0.1)")
			return
		}
	}

	s.cfgMu.Lock()
	if s.cfg.LLM == nil {
		s.cfg.LLM = config.DefaultLLMConfig()
	}
	if req.Provider != "" {
		s.cfg.LLM.Provider = req.Provider
	}
	if aiaccess.IsNativeProvider(s.cfg.LLM.Provider) {
		s.cfg.LLM.BaseURL = ""
		s.cfg.LLM.APIKey = ""
	} else if req.BaseURL != "" {
		// Clear API key when switching to a different host to prevent credential leakage
		if s.cfg.LLM.BaseURL != "" && !sameURLHost(s.cfg.LLM.BaseURL, req.BaseURL) {
			s.cfg.LLM.APIKey = ""
		}
		s.cfg.LLM.BaseURL = req.BaseURL
	}
	if req.APIKey != "" && !aiaccess.IsNativeProvider(s.cfg.LLM.Provider) {
		s.cfg.LLM.APIKey = req.APIKey
	}
	if req.Model != "" {
		s.cfg.LLM.Model = req.Model
	}
	if req.ReadOnly != nil {
		s.cfg.LLM.ReadOnly = *req.ReadOnly
	}

	if err := s.cfg.Save(s.configPath); err != nil {
		s.cfgMu.Unlock()
		s.logger.Error("failed to save config", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to save configuration")
		return
	}

	cfg := *s.cfg.LLM
	s.cfgMu.Unlock()

	writeJSON(w, http.StatusOK, chatConfigResponseFor(cfg))
}

// handleListModels proxies to the provider's /models endpoint and returns
// a normalized list of {id, name} objects.
func (s *Server) handleListModels(w http.ResponseWriter, r *http.Request) {
	cfg := s.llmConfigSnapshot()
	resolved := resolveLLMConfig(cfg)
	if resolved.IsNative {
		writeJSON(w, http.StatusOK, nativeProviderModels(resolved.Provider))
		return
	}
	if cfg.BaseURL == "" {
		writeError(w, http.StatusBadRequest, "LLM provider not configured")
		return
	}
	if !isAllowedLLMBaseURL(cfg.BaseURL) {
		writeError(w, http.StatusBadRequest, "invalid LLM base URL configured")
		return
	}

	models, err := aiaccess.ListModels(r.Context(), nil, aiaccess.Config{
		BaseURL: cfg.BaseURL,
		APIKey:  cfg.APIKey,
		Model:   cfg.Model,
	})
	if err != nil {
		status := http.StatusBadGateway
		if strings.Contains(err.Error(), "API key required") {
			status = http.StatusBadRequest
		}
		writeError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, models)
}

func (s *Server) llmConfigSnapshot() config.LLMConfig {
	s.cfgMu.Lock()
	defer s.cfgMu.Unlock()
	if s.cfg == nil || s.cfg.LLM == nil {
		return *config.DefaultLLMConfig()
	}
	return *s.cfg.LLM
}

func chatConfigResponseFor(cfg config.LLMConfig) chatConfigResponse {
	resolved := resolveLLMConfig(cfg)
	return chatConfigResponse{
		BaseURL:      cfg.BaseURL,
		Model:        cfg.Model,
		HasAPIKey:    resolved.HasAPIKey(),
		APIKeySource: resolved.KeySource,
		Provider:     resolved.Provider,
		ReadOnly:     cfg.ReadOnly,
	}
}

func resolveLLMConfig(cfg config.LLMConfig) aiaccess.ResolvedConfig {
	return aiaccess.Resolve(aiaccess.Config{
		BaseURL:  cfg.BaseURL,
		APIKey:   cfg.APIKey,
		Model:    cfg.Model,
		Provider: cfg.Provider,
	})
}

// isLocalProvider checks if the base URL points to a local service (e.g. Ollama).
func isLocalProvider(baseURL string) bool {
	return aiaccess.IsLocalProvider(baseURL)
}

// isAllowedLLMBaseURL validates that the base URL is safe to use as an LLM endpoint.
// Allows HTTPS URLs and local HTTP URLs (localhost/127.0.0.1). Blocks private
// IP ranges, link-local addresses, and cloud metadata endpoints to prevent SSRF.
func isAllowedLLMBaseURL(rawURL string) bool {
	return aiaccess.IsAllowedBaseURL(rawURL)
}

// sameURLHost checks if two URLs have the same host (ignoring port and path).
func sameURLHost(a, b string) bool {
	return aiaccess.SameURLHost(a, b)
}
