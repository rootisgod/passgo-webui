package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rootisgod/passgo-web/internal/config"
	"github.com/rootisgod/passgo-web/pkg/multipass"
)

// maxRequestBodySize is the global limit for JSON request bodies (1 MB).
const maxRequestBodySize = 1 << 20

func loggingMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start))
	})
}

// bodySizeLimitMiddleware caps request bodies on API endpoints to prevent OOM.
// Excludes file uploads (multipart) which have their own limit via ParseMultipartForm.
func bodySizeLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isAPIPath(r.URL.Path) && r.Body != nil {
			ct := r.Header.Get("Content-Type")
			if !strings.HasPrefix(ct, "multipart/") {
				r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
			}
		}
		next.ServeHTTP(w, r)
	})
}

// securityHeadersMiddleware sets standard security headers on all responses.
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self' wss:; font-src 'self' data:")
		next.ServeHTTP(w, r)
	})
}

func authMiddleware(sessions *sessionStore, cfg *config.Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Skip auth for login endpoint, version, and static frontend assets
		if path == "/api/v1/auth/login" || path == "/api/v1/version" || !isAPIPath(path) {
			next.ServeHTTP(w, r)
			return
		}

		// Check session cookie
		if c, err := r.Cookie("session"); err == nil && sessions.Valid(c.Value) {
			next.ServeHTTP(w, r)
			return
		}

		// Check Authorization header for API clients
		if auth := r.Header.Get("Authorization"); len(auth) > 7 && strings.EqualFold(auth[:7], "Bearer ") {
			bearer := auth[7:]
			// Check session store
			if sessions.Valid(bearer) {
				next.ServeHTTP(w, r)
				return
			}
			// Check persistent API tokens. Constant-time compare is pedantic
			// here (SHA-256 preimage resistance makes timing side-channel
			// infeasible) but it closes the checkbox cleanly.
			hash := sha256Hex(bearer)
			hashBytes := []byte(hash)
			for _, t := range cfg.GetAPITokens() {
				if subtle.ConstantTimeCompare([]byte(t.Hash), hashBytes) == 1 {
					next.ServeHTTP(w, r)
					return
				}
			}
		}

		writeError(w, http.StatusUnauthorized, "authentication required")
	})
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func isAPIPath(path string) bool {
	return strings.HasPrefix(path, "/api/v1/")
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && isAllowedOrigin(origin, r.Host) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// allowedOriginPatterns returns patterns for coder/websocket's AcceptOptions.
// Matches the same logic as isAllowedOrigin: same hostname and localhost.
func allowedOriginPatterns(host string) []string {
	rHost := host
	if i := strings.LastIndex(rHost, ":"); i >= 0 {
		rHost = rHost[:i]
	}
	// Same host on any port, plus localhost for dev
	patterns := []string{rHost + ":*", rHost}
	if rHost != "localhost" && rHost != "127.0.0.1" {
		patterns = append(patterns, "localhost:*", "127.0.0.1:*")
	}
	return patterns
}

// isAllowedOrigin permits same-host origins (any scheme/port) and localhost
// dev servers. This covers the embedded SPA (same origin) and Vite dev mode.
func isAllowedOrigin(origin, host string) bool {
	// Strip scheme from origin to get host[:port]
	o := origin
	if i := strings.Index(o, "://"); i >= 0 {
		o = o[i+3:]
	}
	// Strip port from both to compare hostnames
	oHost := o
	if i := strings.LastIndex(oHost, ":"); i >= 0 {
		oHost = oHost[:i]
	}
	rHost := host
	if i := strings.LastIndex(rHost, ":"); i >= 0 {
		rHost = rHost[:i]
	}

	// Same hostname (e.g. origin "http://192.168.1.5:5173" hitting server on 192.168.1.5:8080)
	if oHost == rHost {
		return true
	}
	// Always allow localhost/127.0.0.1 for dev
	if oHost == "localhost" || oHost == "127.0.0.1" {
		return true
	}
	return false
}

// loginRateLimiter tracks failed login attempts per IP with a sliding window.
type loginRateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	max      int           // max attempts in window
	window   time.Duration // sliding window duration
	stopCh   chan struct{}
}

func newLoginRateLimiter(max int, window time.Duration) *loginRateLimiter {
	rl := &loginRateLimiter{
		attempts: make(map[string][]time.Time),
		max:      max,
		window:   window,
		stopCh:   make(chan struct{}),
	}
	go rl.reaper()
	return rl
}

func (rl *loginRateLimiter) reaper() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.sweep()
		case <-rl.stopCh:
			return
		}
	}
}

// sweep deletes entries whose most-recent attempt is older than the window.
func (rl *loginRateLimiter) sweep() {
	cutoff := time.Now().Add(-rl.window)
	rl.mu.Lock()
	defer rl.mu.Unlock()
	for ip, times := range rl.attempts {
		if len(times) == 0 || times[len(times)-1].Before(cutoff) {
			delete(rl.attempts, ip)
		}
	}
}

func (rl *loginRateLimiter) Shutdown() {
	close(rl.stopCh)
}

// allow checks if the IP is within the rate limit. Returns false if blocked.
func (rl *loginRateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Trim expired attempts
	recent := rl.attempts[ip][:0]
	for _, t := range rl.attempts[ip] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	rl.attempts[ip] = recent

	return len(recent) < rl.max
}

// record adds a failed attempt for the given IP.
func (rl *loginRateLimiter) record(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.attempts[ip] = append(rl.attempts[ip], time.Now())
}

// apiRateLimiter applies a per-IP sliding window rate limit to specific API paths.
type apiRateLimiter struct {
	mu         sync.Mutex
	requests   map[string][]time.Time
	max        int
	window     time.Duration
	trustProxy bool
	stopCh     chan struct{}
}

func newAPIRateLimiter(max int, window time.Duration, trustProxy bool) *apiRateLimiter {
	rl := &apiRateLimiter{
		requests:   make(map[string][]time.Time),
		max:        max,
		window:     window,
		trustProxy: trustProxy,
		stopCh:     make(chan struct{}),
	}
	go rl.reaper()
	return rl
}

func (rl *apiRateLimiter) reaper() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.sweep()
		case <-rl.stopCh:
			return
		}
	}
}

func (rl *apiRateLimiter) sweep() {
	cutoff := time.Now().Add(-rl.window)
	rl.mu.Lock()
	defer rl.mu.Unlock()
	for ip, times := range rl.requests {
		if len(times) == 0 || times[len(times)-1].Before(cutoff) {
			delete(rl.requests, ip)
		}
	}
}

func (rl *apiRateLimiter) Shutdown() {
	close(rl.stopCh)
}

func (rl *apiRateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	recent := rl.requests[ip][:0]
	for _, t := range rl.requests[ip] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	rl.requests[ip] = recent

	if len(recent) >= rl.max {
		return false
	}
	rl.requests[ip] = append(recent, now)
	return true
}

func (rl *apiRateLimiter) remoteIP(r *http.Request) string {
	return clientIPFromRequest(r, rl.trustProxy)
}

// clientIPFromRequest extracts the client IP, respecting trustProxy setting.
func clientIPFromRequest(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if ip, _, _ := strings.Cut(xff, ","); ip != "" {
				return strings.TrimSpace(ip)
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// isTLS returns true if the request arrived over HTTPS. Only trusts
// X-Forwarded-Proto when trustProxy is true.
func isTLS(r *http.Request, trustProxy bool) bool {
	if r.TLS != nil {
		return true
	}
	if trustProxy && r.Header.Get("X-Forwarded-Proto") == "https" {
		return true
	}
	return false
}

// validVMName reads a VM name path parameter, writes a 400 response
// and returns "", false on failure. Rejects the flag-injection class
// (e.g. "--all") before the name ever reaches exec.Command argv.
func validVMName(w http.ResponseWriter, r *http.Request, key string) (string, bool) {
	name := r.PathValue(key)
	if err := multipass.ValidateVMName(name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return "", false
	}
	return name, true
}

// validSnapshotName reads a snapshot path parameter with the same rules as VM names.
func validSnapshotName(w http.ResponseWriter, r *http.Request, key string) (string, bool) {
	name := r.PathValue(key)
	if err := multipass.ValidateVMName(name); err != nil {
		writeError(w, http.StatusBadRequest, "invalid snapshot name: "+err.Error())
		return "", false
	}
	return name, true
}
