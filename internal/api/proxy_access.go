package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rootisgod/passgo-web/internal/config"
)

const proxyTokenQueryParam = "passgo_proxy_token"

func newProxyAccessToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return "pgpx_" + base64.RawURLEncoding.EncodeToString(raw), nil
}

func proxyTokenPrefix(token string) string {
	if len(token) <= 12 {
		return token
	}
	return token[:12]
}

func proxyAccessTokenAuthorized(cfg *config.Config, r *http.Request, now time.Time) bool {
	if cfg == nil || r == nil {
		return false
	}
	name, port, _, ok := parseVMProxyPath(r.URL.Path)
	if !ok {
		return false
	}
	token := proxyTokenFromRequest(r)
	if token == "" {
		return false
	}
	tokenHash := sha256Hex(token)
	for _, rule := range cfg.GetProxyRules() {
		if proxyRuleProtocol(rule) != "http" || rule.VM != name || rule.Port != port {
			continue
		}
		if !rule.Enabled || proxyRuleExpired(rule, now) || rule.AccessTokenHash == "" {
			return false
		}
		return subtleStringEqual(rule.AccessTokenHash, tokenHash)
	}
	return false
}

func proxyTokenFromRequest(r *http.Request) string {
	if token := r.URL.Query().Get(proxyTokenQueryParam); token != "" {
		return token
	}
	auth := r.Header.Get("Authorization")
	if len(auth) > 7 && strings.EqualFold(auth[:7], "Bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	return ""
}

func stripProxyTokenQuery(rawQuery string) string {
	if rawQuery == "" {
		return ""
	}
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return rawQuery
	}
	values.Del(proxyTokenQueryParam)
	return values.Encode()
}

func addProxyTokenToURL(rawURL, token string) string {
	if rawURL == "" || token == "" {
		return rawURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	values := parsed.Query()
	values.Set(proxyTokenQueryParam, token)
	parsed.RawQuery = values.Encode()
	return parsed.String()
}

func subtleStringEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
