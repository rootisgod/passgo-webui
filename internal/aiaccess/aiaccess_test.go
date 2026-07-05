package aiaccess

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResolveUsesGatewayEnvFallback(t *testing.T) {
	resolved := ResolveWithEnv(Config{
		BaseURL: VercelGatewayBaseURL,
		Model:   VercelGatewayDefaultModel,
	}, func(name string) string {
		if name == VercelGatewayAPIKeyEnv {
			return "env-key"
		}
		return ""
	})

	if resolved.APIKey != "env-key" {
		t.Fatalf("APIKey = %q, want env-key", resolved.APIKey)
	}
	if resolved.KeySource != KeySourceEnvironment {
		t.Fatalf("KeySource = %q, want %q", resolved.KeySource, KeySourceEnvironment)
	}
	if !resolved.IsGateway || resolved.Provider != "vercel_ai_gateway" {
		t.Fatalf("gateway not detected: %+v", resolved)
	}
}

func TestResolveStoredKeyWinsOverGatewayEnv(t *testing.T) {
	resolved := ResolveWithEnv(Config{
		BaseURL: VercelGatewayBaseURL,
		APIKey:  "stored-key",
		Model:   VercelGatewayDefaultModel,
	}, func(string) string {
		return "env-key"
	})

	if resolved.APIKey != "stored-key" {
		t.Fatalf("APIKey = %q, want stored-key", resolved.APIKey)
	}
	if resolved.KeySource != KeySourceStored {
		t.Fatalf("KeySource = %q, want %q", resolved.KeySource, KeySourceStored)
	}
}

func TestIsAllowedBaseURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		ok   bool
	}{
		{"gateway", VercelGatewayBaseURL, true},
		{"https domain", "https://openrouter.ai/api/v1", true},
		{"local ollama", "http://localhost:11434/v1", true},
		{"local loopback", "http://127.0.0.1:11434/v1", true},
		{"local ipv6 loopback", "http://[::1]:11434/v1", true},
		{"http remote", "http://example.com/v1", false},
		{"http query containing localhost", "http://example.com/v1?host=localhost", false},
		{"https private ip", "https://192.168.1.10/v1", false},
		{"https metadata ip", "https://169.254.169.254/v1", false},
		{"userinfo", "https://key@example.com/v1", false},
		{"fragment", "https://example.com/v1#models", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAllowedBaseURL(tt.url); got != tt.ok {
				t.Fatalf("IsAllowedBaseURL(%q) = %v, want %v", tt.url, got, tt.ok)
			}
		})
	}
}

func TestNormalizeGatewayModelsFiltersAndPrioritizesToolUseLanguageModels(t *testing.T) {
	body := []byte(`{
		"object":"list",
		"data":[
			{"id":"image/model","name":"Image","type":"image","tags":["tool-use"]},
			{"id":"text/no-tools","name":"No Tools","type":"language","tags":["vision"]},
			{"id":"openai/gpt-5.5","name":"GPT 5.5","type":"language","tags":["tool-use"]},
			{"id":"anthropic/claude-sonnet-5","name":"Claude Sonnet 5","type":"language","tags":["tool-use"]},
			{"id":"z/model","name":"Zed","type":"language","tags":["tool-use"]}
		]
	}`)

	models, err := NormalizeModels(body, true)
	if err != nil {
		t.Fatalf("NormalizeModels: %v", err)
	}
	if len(models) != 3 {
		t.Fatalf("len(models) = %d, want 3: %+v", len(models), models)
	}
	if models[0].ID != VercelGatewayDefaultModel {
		t.Fatalf("first model = %q, want default %q", models[0].ID, VercelGatewayDefaultModel)
	}
	if models[1].ID != "openai/gpt-5.5" {
		t.Fatalf("second model = %q, want openai/gpt-5.5", models[1].ID)
	}
}

func TestListModelsSendsAuthorizationForNonGatewayProvider(t *testing.T) {
	var auth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"anthropic/claude-sonnet-5","name":"Claude Sonnet 5","type":"language","tags":["tool-use"]}]}`))
	}))
	defer ts.Close()

	models, err := ListModels(context.Background(), ts.Client(), Config{
		BaseURL: ts.URL,
		APIKey:  "stored-key",
		Model:   VercelGatewayDefaultModel,
	})
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if auth != "Bearer stored-key" {
		t.Fatalf("Authorization = %q, want bearer for non-gateway provider", auth)
	}
	if len(models) != 1 || models[0].ID != VercelGatewayDefaultModel {
		t.Fatalf("models = %+v", models)
	}
}

func TestListModelsDoesNotSendAuthorizationForGateway(t *testing.T) {
	var auth string
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		auth = req.Header.Get("Authorization")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(
				`{"data":[{"id":"anthropic/claude-sonnet-5","name":"Claude Sonnet 5","type":"language","tags":["tool-use"]}]}`,
			)),
			Request: req,
		}, nil
	})}

	models, err := ListModels(context.Background(), client, Config{
		BaseURL: VercelGatewayBaseURL,
		APIKey:  "stale-or-invalid-key",
		Model:   VercelGatewayDefaultModel,
	})
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if auth != "" {
		t.Fatalf("Authorization = %q, want empty for gateway model list", auth)
	}
	if len(models) != 1 || models[0].ID != VercelGatewayDefaultModel {
		t.Fatalf("models = %+v", models)
	}
}

func TestProviderErrorExtractsOpenAIStyleMessage(t *testing.T) {
	err := ProviderError("LLM", http.StatusUnauthorized, []byte(`{"error":{"message":"bad key","type":"auth"}}`))
	if err == nil || err.Error() != "LLM returned status 401: bad key" {
		t.Fatalf("error = %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
