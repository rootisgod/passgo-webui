package api

import (
	"log/slog"
	"net/http"
	"strings"
	"time"
)

func loggingMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start))
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
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
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
