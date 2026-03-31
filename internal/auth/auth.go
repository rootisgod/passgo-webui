package auth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]time.Time
	ttl      time.Duration
}

func NewSessionStore(ttl time.Duration) *SessionStore {
	return &SessionStore{
		sessions: make(map[string]time.Time),
		ttl:      ttl,
	}
}

func (s *SessionStore) Create() (string, error) {
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

func (s *SessionStore) Valid(token string) bool {
	s.mu.RLock()
	expiry, ok := s.sessions[token]
	s.mu.RUnlock()
	if !ok {
		return false
	}
	if time.Now().After(expiry) {
		s.Delete(token)
		return false
	}
	return true
}

func (s *SessionStore) Delete(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
}

// TokenFromRequest extracts the session token from cookie or Authorization header.
func TokenFromRequest(r *http.Request) string {
	if cookie, err := r.Cookie("session"); err == nil {
		return cookie.Value
	}
	if h := r.Header.Get("Authorization"); len(h) > 7 && h[:7] == "Bearer " {
		return h[7:]
	}
	return ""
}
