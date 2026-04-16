package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type sessionStore struct {
	mu       sync.RWMutex
	sessions map[string]time.Time // token → expiry
	ttl      time.Duration
	stopCh   chan struct{}
}

func newSessionStore(ttl time.Duration) *sessionStore {
	s := &sessionStore{
		sessions: make(map[string]time.Time),
		ttl:      ttl,
		stopCh:   make(chan struct{}),
	}
	// Periodic reaper: without this, tokens that are never looked up again
	// (e.g. user closed the tab) stay in the map for the process lifetime.
	go s.reaper()
	return s
}

func (s *sessionStore) reaper() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			s.mu.Lock()
			for token, expiry := range s.sessions {
				if now.After(expiry) {
					delete(s.sessions, token)
				}
			}
			s.mu.Unlock()
		case <-s.stopCh:
			return
		}
	}
}

func (s *sessionStore) Shutdown() {
	close(s.stopCh)
}

func (s *sessionStore) Create() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	s.mu.Lock()
	s.sessions[token] = time.Now().Add(s.ttl)
	s.mu.Unlock()
	return token, nil
}

func (s *sessionStore) Valid(token string) bool {
	s.mu.RLock()
	expiry, ok := s.sessions[token]
	s.mu.RUnlock()
	if !ok {
		return false
	}
	if time.Now().After(expiry) {
		s.mu.Lock()
		delete(s.sessions, token)
		s.mu.Unlock()
		return false
	}
	return true
}

func (s *sessionStore) Delete(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
}

func (srv *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	// Rate limit by IP
	ip := clientIPFromRequest(r, srv.cfg.TrustProxy)
	if !srv.loginLimiter.allow(ip) {
		writeError(w, http.StatusTooManyRequests, "too many login attempts, try again later")
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if !srv.checkCredentials(req.Username, req.Password) {
		srv.loginLimiter.record(ip)
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := srv.sessions.Create()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   isTLS(r, srv.cfg.TrustProxy),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})
	writeMessage(w, "ok")
}

func (srv *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("session"); err == nil {
		srv.sessions.Delete(c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   isTLS(r, srv.cfg.TrustProxy),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	writeMessage(w, "logged out")
}

// checkCredentials validates username/password against the bcrypt-hashed password.
// Plaintext passwords are auto-migrated to bcrypt on startup via MigratePassword.
func (srv *Server) checkCredentials(username, password string) bool {
	if username != srv.cfg.Username {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(srv.cfg.Password), []byte(password)) == nil
}

