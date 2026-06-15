package api

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rootisgod/passgo-web/internal/config"
)

func TestProxyAccessTokenAuthorizedScopesToRule(t *testing.T) {
	token := "pgpx_test_token"
	now := time.Now().UTC()
	cfg := &config.Config{ProxyRules: []config.ProxyRule{
		{ID: "px_http", VM: "app", Port: 3000, Protocol: "http", Enabled: true, AccessTokenHash: sha256Hex(token), CreatedAt: now.Format(time.RFC3339)},
		{ID: "px_other", VM: "other", Port: 3000, Protocol: "http", Enabled: true, AccessTokenHash: sha256Hex(token), CreatedAt: now.Format(time.RFC3339)},
	}}

	req := httptest.NewRequest("GET", "/api/v1/proxy/vms/app/3000/?"+proxyTokenQueryParam+"="+token, nil)
	if !proxyAccessTokenAuthorized(cfg, req, now) {
		t.Fatal("expected token to authorize matching VM/port")
	}
	req = httptest.NewRequest("GET", "/api/v1/proxy/vms/app/3001/?"+proxyTokenQueryParam+"="+token, nil)
	if proxyAccessTokenAuthorized(cfg, req, now) {
		t.Fatal("token should not authorize a different port")
	}
	req = httptest.NewRequest("GET", "/api/v1/proxy/vms/app/3000/?"+proxyTokenQueryParam+"=wrong", nil)
	if proxyAccessTokenAuthorized(cfg, req, now) {
		t.Fatal("wrong token should not authorize")
	}
}

func TestStripProxyTokenQuery(t *testing.T) {
	got := stripProxyTokenQuery("a=1&" + proxyTokenQueryParam + "=secret&b=2")
	if strings.Contains(got, proxyTokenQueryParam) {
		t.Fatalf("token param was not stripped: %q", got)
	}
	if !strings.Contains(got, "a=1") || !strings.Contains(got, "b=2") {
		t.Fatalf("non-token params were not preserved: %q", got)
	}
}
