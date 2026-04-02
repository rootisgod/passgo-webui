package api

import (
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
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
		next.ServeHTTP(w, r)
	})
}

func authMiddleware(sessions *sessionStore, next http.Handler) http.Handler {
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
		if auth := r.Header.Get("Authorization"); len(auth) > 7 && auth[:7] == "Bearer " {
			if sessions.Valid(auth[7:]) {
				next.ServeHTTP(w, r)
				return
			}
		}

		writeError(w, http.StatusUnauthorized, "authentication required")
	})
}

func isAPIPath(path string) bool {
	return len(path) >= 8 && path[:8] == "/api/v1/"
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
}

func newLoginRateLimiter(max int, window time.Duration) *loginRateLimiter {
	return &loginRateLimiter{
		attempts: make(map[string][]time.Time),
		max:      max,
		window:   window,
	}
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

// isTLS returns true if the request arrived over HTTPS.
func isTLS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	// Check reverse proxy headers
	if r.Header.Get("X-Forwarded-Proto") == "https" {
		return true
	}
	return false
}
