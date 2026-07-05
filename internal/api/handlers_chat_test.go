package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rootisgod/passgo-web/internal/aiaccess"
	"github.com/rootisgod/passgo-web/internal/config"
)

func TestGetChatConfigReportsGatewayEnvKeyWithoutRevealingIt(t *testing.T) {
	t.Setenv(aiaccess.VercelGatewayAPIKeyEnv, "env-secret")
	s := &Server{
		cfg: &config.Config{LLM: &config.LLMConfig{
			BaseURL: aiaccess.VercelGatewayBaseURL,
			Model:   aiaccess.VercelGatewayDefaultModel,
		}},
	}

	rr := httptest.NewRecorder()
	s.handleGetChatConfig(rr, httptest.NewRequest(http.MethodGet, "/api/v1/chat/config", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "env-secret") {
		t.Fatal("config response exposed the environment API key")
	}
	var resp chatConfigResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.HasAPIKey {
		t.Fatal("HasAPIKey = false, want true from environment")
	}
	if resp.APIKeySource != aiaccess.KeySourceEnvironment {
		t.Fatalf("APIKeySource = %q, want %q", resp.APIKeySource, aiaccess.KeySourceEnvironment)
	}
	if resp.Provider != "vercel_ai_gateway" {
		t.Fatalf("Provider = %q, want vercel_ai_gateway", resp.Provider)
	}
}

func TestUpdateChatConfigClearsStoredKeyWhenHostChanges(t *testing.T) {
	tmp := t.TempDir()
	s := &Server{
		cfg: &config.Config{LLM: &config.LLMConfig{
			BaseURL: "https://openrouter.ai/api/v1",
			APIKey:  "old-secret",
			Model:   "anthropic/claude-sonnet-4",
		}},
		configPath: filepath.Join(tmp, "config.json"),
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	body := bytes.NewBufferString(`{
		"base_url":"https://ai-gateway.vercel.sh/v1",
		"model":"anthropic/claude-sonnet-5",
		"read_only":true
	}`)

	rr := httptest.NewRecorder()
	s.handleUpdateChatConfig(rr, httptest.NewRequest(http.MethodPut, "/api/v1/chat/config", body))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if s.cfg.LLM.APIKey != "" {
		t.Fatalf("API key was not cleared on host change")
	}
	if strings.Contains(rr.Body.String(), "old-secret") {
		t.Fatal("response exposed old API key")
	}
}

func TestListModelsAllowsLocalProviderWithoutAPIKey(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Fatalf("unexpected Authorization header: %q", r.Header.Get("Authorization"))
		}
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path = %q, want /v1/models", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"llama3.2"}]}`))
	}))
	defer ts.Close()

	s := &Server{cfg: &config.Config{LLM: &config.LLMConfig{
		BaseURL: ts.URL + "/v1",
		Model:   "llama3.2",
	}}}

	rr := httptest.NewRecorder()
	s.handleListModels(rr, httptest.NewRequest(http.MethodGet, "/api/v1/chat/models", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var models []aiaccess.Model
	if err := json.NewDecoder(rr.Body).Decode(&models); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(models) != 1 || models[0].ID != "llama3.2" || models[0].Name != "llama3.2" {
		t.Fatalf("models = %+v", models)
	}
}

func TestUpdateChatConfigNativeProviderDoesNotStoreAPIKey(t *testing.T) {
	tmp := t.TempDir()
	s := &Server{
		cfg:        &config.Config{LLM: config.DefaultLLMConfig()},
		configPath: filepath.Join(tmp, "config.json"),
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	body := bytes.NewBufferString(`{
		"provider":"codex_cli",
		"api_key":"should-not-be-stored",
		"model":"codex-default"
	}`)

	rr := httptest.NewRecorder()
	s.handleUpdateChatConfig(rr, httptest.NewRequest(http.MethodPut, "/api/v1/chat/config", body))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if s.cfg.LLM.Provider != aiaccess.ProviderCodexCLI {
		t.Fatalf("Provider = %q", s.cfg.LLM.Provider)
	}
	if s.cfg.LLM.APIKey != "" || s.cfg.LLM.BaseURL != "" {
		t.Fatalf("native provider stored URL/key: %+v", s.cfg.LLM)
	}
	if strings.Contains(rr.Body.String(), "should-not-be-stored") {
		t.Fatal("response exposed native provider API key")
	}
}

func TestListModelsReturnsNativeProviderModels(t *testing.T) {
	s := &Server{cfg: &config.Config{LLM: &config.LLMConfig{
		Provider: aiaccess.ProviderCodexCLI,
		Model:    "codex-default",
	}}}

	rr := httptest.NewRecorder()
	s.handleListModels(rr, httptest.NewRequest(http.MethodGet, "/api/v1/chat/models", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var models []aiaccess.Model
	if err := json.NewDecoder(rr.Body).Decode(&models); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(models) == 0 || models[0].ID != "codex-default" {
		t.Fatalf("models = %+v", models)
	}
}
