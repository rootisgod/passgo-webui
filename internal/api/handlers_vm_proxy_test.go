package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rootisgod/passgo-web/internal/config"
	"github.com/rootisgod/passgo-web/pkg/multipass"
)

func TestProxyRuleHandlersCRUDAndStatus(t *testing.T) {
	const infoApp = `{"errors":[],"info":{"app":{"state":"Running","cpu_count":"2","memory":{"used":0,"total":0},"disks":{},"mounts":{},"ipv4":["192.168.64.20"],"snapshot_count":"0"}}}`
	const infoAll = `{"errors":[],"info":{"app":{"state":"Running","cpu_count":"2","memory":{"used":0,"total":0},"disks":{},"mounts":{},"ipv4":["192.168.64.20"],"snapshot_count":"0"}}}`
	runner := func(args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "info app --format json":
			return infoApp, nil
		case "info --all --format json":
			return infoAll, nil
		default:
			t.Fatalf("unexpected multipass call: %v", args)
			return "", nil
		}
	}
	s := &Server{
		mp:         multipass.NewClientWithRunner(slog.New(slog.NewTextHandler(io.Discard, nil)), runner),
		cfg:        &config.Config{ProxyRules: []config.ProxyRule{}},
		configPath: filepath.Join(t.TempDir(), "config.json"),
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/proxy-rules", bytes.NewBufferString(`{"vm":"app","port":5173,"label":"Vite"}`))
	rr := httptest.NewRecorder()
	s.handleCreateProxyRule(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if len(s.cfg.ProxyRules) != 1 {
		t.Fatalf("proxy rules = %d, want 1", len(s.cfg.ProxyRules))
	}
	ruleID := s.cfg.ProxyRules[0].ID
	if s.cfg.ProxyRules[0].AccessTokenHash == "" {
		t.Fatal("http proxy rule should store an access token hash")
	}

	rr = httptest.NewRecorder()
	s.handleListProxyRules(rr, httptest.NewRequest(http.MethodGet, "/api/v1/proxy-rules", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var listed []proxyRuleResponse
	if err := json.NewDecoder(rr.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listed) != 1 || listed[0].Status != "live" || listed[0].Target != "192.168.64.20:5173" {
		t.Fatalf("listed = %+v, want one live target", listed)
	}
	if listed[0].Protocol != "HTTP/WS" || listed[0].Destination != "app:5173" {
		t.Fatalf("listed protocol/destination = %q/%q, want HTTP/WS app:5173", listed[0].Protocol, listed[0].Destination)
	}

	req = httptest.NewRequest(http.MethodPut, "/api/v1/proxy-rules/"+ruleID, bytes.NewBufferString(`{"enabled":false}`))
	req.SetPathValue("id", ruleID)
	rr = httptest.NewRecorder()
	s.handleUpdateProxyRule(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if s.cfg.ProxyRules[0].Enabled {
		t.Fatal("enabled flag was not updated")
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/proxy-rules/"+ruleID, nil)
	req.SetPathValue("id", ruleID)
	rr = httptest.NewRecorder()
	s.handleDeleteProxyRule(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if len(s.cfg.ProxyRules) != 0 {
		t.Fatalf("proxy rules = %d, want 0", len(s.cfg.ProxyRules))
	}
}

func TestProxyRuleCreateReturnsOneTimeAccessToken(t *testing.T) {
	const infoApp = `{"errors":[],"info":{"app":{"state":"Running","cpu_count":"2","memory":{"used":0,"total":0},"disks":{},"mounts":{},"ipv4":["192.168.64.20"],"snapshot_count":"0"}}}`
	runner := func(args ...string) (string, error) {
		if strings.Join(args, " ") == "info app --format json" {
			return infoApp, nil
		}
		t.Fatalf("unexpected multipass call: %v", args)
		return "", nil
	}
	s := &Server{
		mp:         multipass.NewClientWithRunner(slog.New(slog.NewTextHandler(io.Discard, nil)), runner),
		cfg:        &config.Config{ProxyRules: []config.ProxyRule{}},
		configPath: filepath.Join(t.TempDir(), "config.json"),
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/proxy-rules", bytes.NewBufferString(`{"vm":"app","port":5173,"owner":"agent-1","ttl_minutes":60}`))
	req.Host = "passgo.local:8080"
	rr := httptest.NewRecorder()
	s.handleCreateProxyRule(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var created proxyRuleResponse
	if err := json.NewDecoder(rr.Body).Decode(&created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	if created.AccessToken == "" || !strings.Contains(created.ProxyURL, proxyTokenQueryParam+"=") {
		t.Fatalf("created token/url = %q/%q", created.AccessToken, created.ProxyURL)
	}
	if strings.Contains(s.cfg.ProxyRules[0].AccessTokenHash, created.AccessToken) {
		t.Fatal("stored token hash should not contain plaintext token")
	}
	if s.cfg.ProxyRules[0].Owner != "agent-1" || s.cfg.ProxyRules[0].ExpiresAt == "" {
		t.Fatalf("stored owner/expires = %+v", s.cfg.ProxyRules[0])
	}
}

func TestProxyRuleCreateRejectsDuplicateVMPort(t *testing.T) {
	runner := func(args ...string) (string, error) {
		if strings.Join(args, " ") == "info app --format json" {
			return `{"errors":[],"info":{"app":{"state":"Stopped","cpu_count":"2","memory":{"used":0,"total":0},"disks":{},"mounts":{},"ipv4":[],"snapshot_count":"0"}}}`, nil
		}
		t.Fatalf("unexpected multipass call: %v", args)
		return "", nil
	}
	s := &Server{
		mp: multipass.NewClientWithRunner(slog.New(slog.NewTextHandler(io.Discard, nil)), runner),
		cfg: &config.Config{ProxyRules: []config.ProxyRule{
			{ID: "px_existing", VM: "app", Port: 3000, Enabled: true, CreatedAt: time.Now().UTC().Format(time.RFC3339)},
		}},
		configPath: filepath.Join(t.TempDir(), "config.json"),
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/proxy-rules", bytes.NewBufferString(`{"vm":"app","port":3000}`))
	rr := httptest.NewRecorder()
	s.handleCreateProxyRule(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
}

func TestProxyRuleCreateSSHDefaultsToPort22AndHostPort(t *testing.T) {
	runner := func(args ...string) (string, error) {
		if strings.Join(args, " ") == "info app --format json" {
			return `{"errors":[],"info":{"app":{"state":"Stopped","cpu_count":"2","memory":{"used":0,"total":0},"disks":{},"mounts":{},"ipv4":[],"snapshot_count":"0"}}}`, nil
		}
		t.Fatalf("unexpected multipass call: %v", args)
		return "", nil
	}
	s := &Server{
		mp:         multipass.NewClientWithRunner(slog.New(slog.NewTextHandler(io.Discard, nil)), runner),
		cfg:        &config.Config{ProxyRules: []config.ProxyRule{}},
		configPath: filepath.Join(t.TempDir(), "config.json"),
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/proxy-rules", bytes.NewBufferString(`{"vm":"app","protocol":"ssh","host_port":2222,"bind_address":"127.0.0.1","label":"Agent SSH"}`))
	req.Host = "passgo.local:8080"
	rr := httptest.NewRecorder()
	s.handleCreateProxyRule(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if len(s.cfg.ProxyRules) != 1 {
		t.Fatalf("proxy rules = %d, want 1", len(s.cfg.ProxyRules))
	}
	rule := s.cfg.ProxyRules[0]
	if rule.Protocol != "ssh" || rule.Port != 22 || rule.HostPort != 2222 || rule.BindAddress != "127.0.0.1" || rule.ExpiresAt == "" {
		t.Fatalf("stored rule = %+v, want ssh vm port 22 host port 2222 local bind with expiry", rule)
	}

	var created proxyRuleResponse
	if err := json.NewDecoder(rr.Body).Decode(&created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	if created.Protocol != "SSH" || created.Destination != "app:22" || created.ListenAddress != "127.0.0.1:2222" {
		t.Fatalf("created response = %+v", created)
	}
	if created.SSHCommand != "ssh -p 2222 ubuntu@127.0.0.1" {
		t.Fatalf("ssh command = %q", created.SSHCommand)
	}
	if created.ProxyURL != "" {
		t.Fatalf("ssh rule should not include proxy URL, got %q", created.ProxyURL)
	}
}

func TestProxyRuleCreateSSHAutoAllocatesHostPort(t *testing.T) {
	runner := func(args ...string) (string, error) {
		if strings.Join(args, " ") == "info app --format json" {
			return `{"errors":[],"info":{"app":{"state":"Stopped","cpu_count":"2","memory":{"used":0,"total":0},"disks":{},"mounts":{},"ipv4":[],"snapshot_count":"0"}}}`, nil
		}
		t.Fatalf("unexpected multipass call: %v", args)
		return "", nil
	}
	s := &Server{
		mp:         multipass.NewClientWithRunner(slog.New(slog.NewTextHandler(io.Discard, nil)), runner),
		cfg:        &config.Config{ProxyRules: []config.ProxyRule{}},
		configPath: filepath.Join(t.TempDir(), "config.json"),
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/proxy-rules", bytes.NewBufferString(`{"vm":"app","protocol":"ssh","auto_host_port":true,"bind_address":"127.0.0.1"}`))
	rr := httptest.NewRecorder()
	s.handleCreateProxyRule(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", rr.Code, rr.Body.String())
	}
	rule := s.cfg.ProxyRules[0]
	if rule.HostPort < 2200 || rule.HostPort > 2999 {
		t.Fatalf("auto host port = %d, want 2200-2999", rule.HostPort)
	}
}

func TestCleanupProxyRulesRemovesExpiredRules(t *testing.T) {
	now := time.Now().UTC()
	s := &Server{
		cfg: &config.Config{ProxyRules: []config.ProxyRule{
			{ID: "px_old", VM: "app", Port: 3000, Protocol: "http", Enabled: true, ExpiresAt: now.Add(-time.Minute).Format(time.RFC3339), CreatedAt: now.Format(time.RFC3339)},
			{ID: "px_live", VM: "app", Port: 5173, Protocol: "http", Enabled: true, ExpiresAt: now.Add(time.Hour).Format(time.RFC3339), CreatedAt: now.Format(time.RFC3339)},
		}},
		configPath: filepath.Join(t.TempDir(), "config.json"),
	}
	rr := httptest.NewRecorder()
	s.handleCleanupProxyRules(rr, httptest.NewRequest(http.MethodPost, "/api/v1/proxy-rules/cleanup", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("cleanup status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if len(s.cfg.ProxyRules) != 1 || s.cfg.ProxyRules[0].ID != "px_live" {
		t.Fatalf("proxy rules after cleanup = %+v", s.cfg.ProxyRules)
	}
}

func TestVMHTTPProxyRejectsMissingDisabledAndExpiredRules(t *testing.T) {
	now := time.Now().UTC()
	s := &Server{
		cfg: &config.Config{ProxyRules: []config.ProxyRule{
			{ID: "px_disabled", VM: "disabled", Port: 3000, Enabled: false, CreatedAt: now.Format(time.RFC3339)},
			{ID: "px_expired", VM: "expired", Port: 3000, Enabled: true, ExpiresAt: now.Add(-time.Minute).Format(time.RFC3339), CreatedAt: now.Format(time.RFC3339)},
			{ID: "px_ssh", VM: "ssh-only", Port: 3000, Protocol: "ssh", HostPort: 2300, Enabled: true, CreatedAt: now.Format(time.RFC3339)},
		}},
	}

	tests := []struct {
		path string
		want int
	}{
		{"/api/v1/proxy/vms/missing/3000/", http.StatusForbidden},
		{"/api/v1/proxy/vms/disabled/3000/", http.StatusForbidden},
		{"/api/v1/proxy/vms/expired/3000/", http.StatusGone},
		{"/api/v1/proxy/vms/ssh-only/3000/", http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			rr := httptest.NewRecorder()
			s.handleVMHTTPProxy(rr, httptest.NewRequest(http.MethodGet, tt.path, nil))
			if rr.Code != tt.want {
				t.Fatalf("status = %d, want %d, body = %s", rr.Code, tt.want, rr.Body.String())
			}
		})
	}
}

func TestProxyAddressForVMRequiresRunningVMWithUsableIPv4(t *testing.T) {
	runner := func(args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "info app --format json":
			return `{"errors":[],"info":{"app":{"state":"Running","cpu_count":"2","memory":{"used":0,"total":0},"disks":{},"mounts":{},"ipv4":["127.0.0.1","192.168.64.20"],"snapshot_count":"0"}}}`, nil
		case "info stopped --format json":
			return `{"errors":[],"info":{"stopped":{"state":"Stopped","cpu_count":"2","memory":{"used":0,"total":0},"disks":{},"mounts":{},"ipv4":["192.168.64.21"],"snapshot_count":"0"}}}`, nil
		case "info no-ip --format json":
			return `{"errors":[],"info":{"no-ip":{"state":"Running","cpu_count":"2","memory":{"used":0,"total":0},"disks":{},"mounts":{},"ipv4":["127.0.0.1","169.254.1.2"],"snapshot_count":"0"}}}`, nil
		default:
			return "", errors.New("unexpected multipass call")
		}
	}
	s := &Server{mp: multipass.NewClientWithRunner(slog.New(slog.NewTextHandler(io.Discard, nil)), runner)}

	rr := httptest.NewRecorder()
	addr, ok := s.proxyAddressForVM(rr, "app", 8080)
	if !ok || addr != "192.168.64.20:8080" {
		t.Fatalf("addr = %q ok=%v status=%d body=%s", addr, ok, rr.Code, rr.Body.String())
	}
	rr = httptest.NewRecorder()
	if _, ok := s.proxyAddressForVM(rr, "stopped", 8080); ok || rr.Code != http.StatusConflict {
		t.Fatalf("stopped ok=%v status=%d body=%s", ok, rr.Code, rr.Body.String())
	}
	rr = httptest.NewRecorder()
	if _, ok := s.proxyAddressForVM(rr, "no-ip", 8080); ok || rr.Code != http.StatusConflict {
		t.Fatalf("no-ip ok=%v status=%d body=%s", ok, rr.Code, rr.Body.String())
	}
}

func TestParseVMProxyPath(t *testing.T) {
	name, port, upstreamPath, ok := parseVMProxyPath("/api/v1/proxy/vms/app/5173/src/main.js")
	if !ok || name != "app" || port != 5173 || upstreamPath != "/src/main.js" {
		t.Fatalf("parse = (%q, %d, %q, %v)", name, port, upstreamPath, ok)
	}
	_, _, _, ok = parseVMProxyPath("/api/v1/proxy/vms/app/not-a-port/")
	if ok {
		t.Fatal("expected invalid port to fail")
	}
}

func TestRewriteProxyLocation(t *testing.T) {
	resp := &http.Response{Header: make(http.Header)}
	resp.Header.Set("Location", "/login")
	rewriteProxyLocation(resp, "192.168.64.20:3000", "/api/v1/proxy/vms/app/3000/")
	if got := resp.Header.Get("Location"); got != "/api/v1/proxy/vms/app/3000/login" {
		t.Fatalf("relative location = %q", got)
	}

	resp.Header.Set("Location", "http://192.168.64.20:3000/callback?ok=1")
	rewriteProxyLocation(resp, "192.168.64.20:3000", "/api/v1/proxy/vms/app/3000/")
	if got := resp.Header.Get("Location"); got != "/api/v1/proxy/vms/app/3000/callback?ok=1" {
		t.Fatalf("absolute location = %q", got)
	}
}
