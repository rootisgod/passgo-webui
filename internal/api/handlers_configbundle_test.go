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

func TestExportConfigOmitsLLMAPIKey(t *testing.T) {
	tmp := t.TempDir()
	s := &Server{
		cfg: &config.Config{
			CloudInitDir: filepath.Join(tmp, "cloud-init"),
			PlaybooksDir: filepath.Join(tmp, "playbooks"),
			Groups:       []string{},
			VMGroups:     map[string]string{},
			VMTemplates:  map[string]bool{},
			LLM: &config.LLMConfig{
				BaseURL:  aiaccess.VercelGatewayBaseURL,
				APIKey:   "secret-key",
				Model:    aiaccess.VercelGatewayDefaultModel,
				Provider: aiaccess.ProviderVercelGateway,
			},
		},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	rr := httptest.NewRecorder()
	s.handleExportConfig(rr, httptest.NewRequest(http.MethodGet, "/api/v1/config/export", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "secret-key") || strings.Contains(rr.Body.String(), "api_key") {
		t.Fatal("export included the LLM API key")
	}
	var bundle configBundle
	if err := json.NewDecoder(rr.Body).Decode(&bundle); err != nil {
		t.Fatalf("decode bundle: %v", err)
	}
	if bundle.Config == nil || bundle.Config.LLM == nil {
		t.Fatal("export missing LLM config")
	}
	if bundle.Config.LLM.BaseURL != aiaccess.VercelGatewayBaseURL {
		t.Fatalf("base_url = %q", bundle.Config.LLM.BaseURL)
	}
	if bundle.Config.LLM.Provider != aiaccess.ProviderVercelGateway {
		t.Fatalf("provider = %q", bundle.Config.LLM.Provider)
	}
}

func TestImportConfigPreservesExistingLLMAPIKey(t *testing.T) {
	tmp := t.TempDir()
	s := &Server{
		cfg: &config.Config{
			CloudInitDir: filepath.Join(tmp, "cloud-init"),
			PlaybooksDir: filepath.Join(tmp, "playbooks"),
			Groups:       []string{},
			VMGroups:     map[string]string{},
			VMTemplates:  map[string]bool{},
			LLM: &config.LLMConfig{
				BaseURL: "https://openrouter.ai/api/v1",
				APIKey:  "keep-secret",
				Model:   "anthropic/claude-sonnet-4",
			},
		},
		configPath: filepath.Join(tmp, "config.json"),
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	bundle := configBundle{
		Version: 1,
		Config: &configExport{
			LLM: &llmConfigExport{
				BaseURL:  "",
				Model:    "codex-default",
				ReadOnly: true,
				Provider: aiaccess.ProviderCodexCLI,
			},
		},
		CloudInitTemplates: map[string]string{},
		Playbooks:          map[string]string{},
	}
	data, err := json.Marshal(bundle)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	s.handleImportConfig(rr, httptest.NewRequest(http.MethodPost, "/api/v1/config/import", bytes.NewReader(data)))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if s.cfg.LLM.APIKey != "keep-secret" {
		t.Fatalf("APIKey = %q, want preserved secret", s.cfg.LLM.APIKey)
	}
	if s.cfg.LLM.Provider != aiaccess.ProviderCodexCLI || s.cfg.LLM.Model != "codex-default" || !s.cfg.LLM.ReadOnly {
		t.Fatalf("LLM config not imported correctly: %+v", s.cfg.LLM)
	}
}
