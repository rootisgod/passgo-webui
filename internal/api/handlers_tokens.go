package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rootisgod/passgo-web/internal/config"
)

type tokenResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Prefix    string `json:"prefix"`
	CreatedAt string `json:"created_at"`
}

type tokenCreateResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Prefix    string `json:"prefix"`
	Token     string `json:"token"`
	CreatedAt string `json:"created_at"`
}

func (srv *Server) handleListTokens(w http.ResponseWriter, r *http.Request) {
	srv.groupMu.Lock()
	tokens := srv.cfg.GetAPITokens()
	srv.groupMu.Unlock()

	resp := make([]tokenResponse, len(tokens))
	for i, t := range tokens {
		resp[i] = tokenResponse{
			ID:        t.ID,
			Name:      t.Name,
			Prefix:    t.Prefix,
			CreatedAt: t.CreatedAt,
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (srv *Server) handleCreateToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if len(req.Name) > 64 {
		writeError(w, http.StatusBadRequest, "name must be 64 characters or less")
		return
	}

	// Check for duplicate names
	srv.groupMu.Lock()
	for _, t := range srv.cfg.GetAPITokens() {
		if t.Name == req.Name {
			srv.groupMu.Unlock()
			writeError(w, http.StatusConflict, fmt.Sprintf("token with name %q already exists", req.Name))
			return
		}
	}

	// Generate token ID
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		srv.groupMu.Unlock()
		writeError(w, http.StatusInternalServerError, "failed to generate token ID")
		return
	}
	id := hex.EncodeToString(idBytes)

	// Generate raw token: pgo_ + 32 random hex bytes
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		srv.groupMu.Unlock()
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}
	rawToken := "pgo_" + hex.EncodeToString(tokenBytes)

	// Hash the token for storage
	hash := sha256.Sum256([]byte(rawToken))
	hashHex := hex.EncodeToString(hash[:])

	// First 12 chars as prefix (pgo_ + 8 hex)
	prefix := rawToken[:12]

	now := time.Now().UTC().Format(time.RFC3339)

	token := config.APIToken{
		ID:        id,
		Name:      req.Name,
		Prefix:    prefix,
		Hash:      hashHex,
		CreatedAt: now,
	}

	srv.cfg.AddAPIToken(token)
	if err := srv.cfg.Save(config.DefaultConfigPath()); err != nil {
		srv.groupMu.Unlock()
		writeError(w, http.StatusInternalServerError, "failed to save config")
		return
	}
	srv.groupMu.Unlock()

	srv.eventLog.EmitHTTPEvent(r, "config", "create_token", req.Name, "success", "")
	writeJSON(w, http.StatusCreated, tokenCreateResponse{
		ID:        id,
		Name:      req.Name,
		Prefix:    prefix,
		Token:     rawToken,
		CreatedAt: now,
	})
}

func (srv *Server) handleDeleteToken(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	srv.groupMu.Lock()
	if err := srv.cfg.DeleteAPIToken(id); err != nil {
		srv.groupMu.Unlock()
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err := srv.cfg.Save(config.DefaultConfigPath()); err != nil {
		srv.groupMu.Unlock()
		writeError(w, http.StatusInternalServerError, "failed to save config")
		return
	}
	srv.groupMu.Unlock()

	srv.eventLog.EmitHTTPEvent(r, "config", "delete_token", id, "success", "")
	writeMessage(w, "token deleted")
}
