package aiaccess

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

const (
	VercelGatewayBaseURL      = "https://ai-gateway.vercel.sh/v1"
	VercelGatewayDefaultModel = "anthropic/claude-sonnet-5"
	VercelGatewayAPIKeyEnv    = "AI_GATEWAY_API_KEY"
)

const (
	ProviderOpenAICompatible = "openai_compatible"
	ProviderVercelGateway    = "vercel_ai_gateway"
	ProviderLocal            = "local"
	ProviderCodexCLI         = "codex_cli"
	ProviderClaudeCodeCLI    = "claude_code_cli"
)

const (
	KeySourceNone        = "none"
	KeySourceStored      = "stored"
	KeySourceEnvironment = "environment"
)

const gatewayModelLimit = 120

var gatewayModelPriority = []string{
	VercelGatewayDefaultModel,
	"anthropic/claude-opus-4.8",
	"openai/gpt-5.5",
	"google/gemini-3.5-flash",
	"google/gemini-3.1-flash-lite",
	"xai/grok-4.3",
	"alibaba/qwen3-coder",
}

type Config struct {
	BaseURL  string
	APIKey   string
	Model    string
	Provider string
}

type ResolvedConfig struct {
	BaseURL     string
	APIKey      string
	Model       string
	KeySource   string
	Provider    string
	Local       bool
	IsGateway   bool
	IsNative    bool
	NeedsAPIKey bool
}

type Model struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func DefaultConfig() Config {
	return Config{
		BaseURL: VercelGatewayBaseURL,
		Model:   VercelGatewayDefaultModel,
	}
}

func Resolve(cfg Config) ResolvedConfig {
	return ResolveWithEnv(cfg, os.Getenv)
}

func ResolveWithEnv(cfg Config, getenv func(string) string) ResolvedConfig {
	baseURL := strings.TrimSpace(cfg.BaseURL)
	apiKey := strings.TrimSpace(cfg.APIKey)
	provider := strings.TrimSpace(cfg.Provider)
	keySource := KeySourceNone
	if apiKey != "" {
		keySource = KeySourceStored
	}

	if provider == ProviderCodexCLI || provider == ProviderClaudeCodeCLI {
		return ResolvedConfig{
			BaseURL:     baseURL,
			APIKey:      "",
			Model:       strings.TrimSpace(cfg.Model),
			KeySource:   KeySourceNone,
			Provider:    provider,
			IsNative:    true,
			NeedsAPIKey: false,
		}
	}

	isGateway := IsVercelGateway(baseURL)
	if apiKey == "" && isGateway {
		if envKey := strings.TrimSpace(getenv(VercelGatewayAPIKeyEnv)); envKey != "" {
			apiKey = envKey
			keySource = KeySourceEnvironment
		}
	}

	local := IsLocalProvider(baseURL)
	if isGateway {
		provider = ProviderVercelGateway
	} else if local {
		provider = ProviderLocal
	} else {
		provider = ProviderOpenAICompatible
	}

	return ResolvedConfig{
		BaseURL:     baseURL,
		APIKey:      apiKey,
		Model:       strings.TrimSpace(cfg.Model),
		KeySource:   keySource,
		Provider:    provider,
		Local:       local,
		IsGateway:   isGateway,
		IsNative:    false,
		NeedsAPIKey: !local,
	}
}

func IsNativeProvider(provider string) bool {
	return provider == ProviderCodexCLI || provider == ProviderClaudeCodeCLI
}

func IsAllowedProvider(provider string) bool {
	switch provider {
	case "", ProviderOpenAICompatible, ProviderVercelGateway, ProviderLocal, ProviderCodexCLI, ProviderClaudeCodeCLI:
		return true
	default:
		return false
	}
}

func (r ResolvedConfig) HasAPIKey() bool {
	return r.APIKey != ""
}

func (r ResolvedConfig) ChatCompletionsURL() string {
	return endpointURL(r.BaseURL, "chat/completions")
}

func (r ResolvedConfig) ModelsURL() string {
	return endpointURL(r.BaseURL, "models")
}

func (r ResolvedConfig) Authorize(req *http.Request) {
	if r.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+r.APIKey)
	}
}

func IsVercelGateway(rawURL string) bool {
	u, ok := parseBaseURL(rawURL)
	if !ok {
		return false
	}
	return strings.EqualFold(u.Scheme, "https") &&
		strings.EqualFold(u.Hostname(), "ai-gateway.vercel.sh") &&
		strings.TrimRight(u.EscapedPath(), "/") == "/v1"
}

func IsLocalProvider(rawURL string) bool {
	u, ok := parseBaseURL(rawURL)
	if !ok {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// IsAllowedBaseURL allows remote HTTPS providers and local HTTP providers.
// Literal internal, link-local, multicast, and unspecified IPs are blocked for
// remote HTTPS to keep configurable provider URLs from becoming an SSRF path.
func IsAllowedBaseURL(rawURL string) bool {
	u, ok := parseBaseURL(rawURL)
	if !ok {
		return false
	}
	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "http":
		return IsLocalProvider(rawURL)
	case "https":
		if IsLocalProvider(rawURL) {
			return true
		}
		ip := net.ParseIP(u.Hostname())
		return ip == nil || isPublicIP(ip)
	default:
		return false
	}
}

func SameURLHost(a, b string) bool {
	ua, okA := parseBaseURL(a)
	ub, okB := parseBaseURL(b)
	if !okA || !okB {
		return false
	}
	return strings.EqualFold(ua.Hostname(), ub.Hostname())
}

func ListModels(ctx context.Context, client *http.Client, cfg Config) ([]Model, error) {
	resolved := Resolve(cfg)
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	if resolved.NeedsAPIKey && !resolved.HasAPIKey() && !resolved.IsGateway {
		return nil, fmt.Errorf("API key required to fetch models from remote provider")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resolved.ModelsURL(), nil)
	if err != nil {
		return nil, fmt.Errorf("create models request: %w", err)
	}
	// AI Gateway /models currently works without auth. Do not attach a possibly
	// stale stored key, because an invalid bearer can change behavior.
	if !resolved.IsGateway {
		resolved.Authorize(req)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to reach provider: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read provider response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, ProviderError("provider", resp.StatusCode, body)
	}
	return NormalizeModels(body, resolved.IsGateway)
}

func NormalizeModels(body []byte, gateway bool) ([]Model, error) {
	var modelsResp struct {
		Data []struct {
			ID     string   `json:"id"`
			Name   string   `json:"name"`
			Type   string   `json:"type"`
			Tags   []string `json:"tags"`
			Object string   `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return nil, fmt.Errorf("parse models response: %w", err)
	}

	models := make([]Model, 0, len(modelsResp.Data))
	seen := make(map[string]bool, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		id := strings.TrimSpace(m.ID)
		if id == "" || seen[id] {
			continue
		}
		if gateway && !isUsefulGatewayChatModel(m.Type, m.Tags) {
			continue
		}
		name := strings.TrimSpace(m.Name)
		if name == "" {
			name = id
		}
		models = append(models, Model{ID: id, Name: name})
		seen[id] = true
	}

	sortModels(models, gateway)
	if gateway && len(models) > gatewayModelLimit {
		models = models[:gatewayModelLimit]
	}
	return models, nil
}

func ProviderError(prefix string, statusCode int, body []byte) error {
	msg := extractProviderError(body)
	if msg == "" {
		msg = http.StatusText(statusCode)
	}
	return fmt.Errorf("%s returned status %d: %s", prefix, statusCode, truncate(msg, 300))
}

func endpointURL(baseURL, path string) string {
	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path, "/")
}

func parseBaseURL(rawURL string) (*url.URL, bool) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, false
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, false
	}
	if strings.Contains(u.Host, "@") {
		return nil, false
	}
	return u, true
}

func isPublicIP(ip net.IP) bool {
	return ip.IsGlobalUnicast() &&
		!ip.IsPrivate() &&
		!ip.IsLoopback() &&
		!ip.IsLinkLocalUnicast() &&
		!ip.IsLinkLocalMulticast() &&
		!ip.IsUnspecified() &&
		!ip.IsMulticast()
}

func isUsefulGatewayChatModel(modelType string, tags []string) bool {
	if modelType != "" && modelType != "language" {
		return false
	}
	if len(tags) == 0 {
		return true
	}
	for _, tag := range tags {
		if tag == "tool-use" {
			return true
		}
	}
	return false
}

func sortModels(models []Model, gateway bool) {
	priority := map[string]int{}
	if gateway {
		for i, id := range gatewayModelPriority {
			priority[id] = i + 1
		}
	}
	sort.Slice(models, func(i, j int) bool {
		pi, iPriority := priority[models[i].ID]
		pj, jPriority := priority[models[j].ID]
		if iPriority || jPriority {
			if !iPriority {
				return false
			}
			if !jPriority {
				return true
			}
			return pi < pj
		}
		return strings.ToLower(models[i].Name) < strings.ToLower(models[j].Name)
	})
}

func extractProviderError(body []byte) string {
	var openAIError struct {
		Error any `json:"error"`
	}
	if err := json.Unmarshal(body, &openAIError); err == nil {
		switch e := openAIError.Error.(type) {
		case string:
			return e
		case map[string]any:
			if msg, ok := e["message"].(string); ok {
				return msg
			}
			if code, ok := e["code"].(string); ok {
				return code
			}
		}
	}
	return strings.TrimSpace(string(body))
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
