package api

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coder/websocket"
)

const vncPasswordPath = "/home/ubuntu/.config/tigervnc/passgo-password"

type vncConsoleResponse struct {
	Available bool   `json:"available"`
	URL       string `json:"url,omitempty"`
	Password  string `json:"password,omitempty"`
	Message   string `json:"message,omitempty"`
}

func (s *Server) handleGetVNCConsole(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	name, ok := validVMName(w, r, "name")
	if !ok {
		return
	}

	vm, err := s.mp.GetVMInfo(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if vm.State != "Running" {
		writeJSON(w, http.StatusOK, vncConsoleResponse{
			Message: "VM must be running before opening a graphical console",
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	password, err := s.mp.ExecInVMWithContext(ctx, name, []string{"cat", vncPasswordPath})
	if err != nil || strings.TrimSpace(password) == "" {
		writeJSON(w, http.StatusOK, vncConsoleResponse{
			Message: "PassGo VNC is not configured in this VM. Reprovision it with the current Ubuntu desktop template.",
		})
		return
	}

	writeJSON(w, http.StatusOK, vncConsoleResponse{
		Available: true,
		URL:       "/api/v1/vms/" + url.PathEscape(name) + "/vnc/websocket",
		Password:  strings.TrimSpace(password),
	})
}

func (s *Server) handleVNCWebSocket(w http.ResponseWriter, r *http.Request) {
	name, ok := validVMName(w, r, "name")
	if !ok {
		return
	}

	vm, err := s.mp.GetVMInfo(name)
	if err != nil || vm.State != "Running" {
		writeError(w, http.StatusConflict, "VM must be running before opening a graphical console")
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: allowedOriginPatterns(r.Host),
	})
	if err != nil {
		s.logger.Error("VNC websocket accept failed", "err", err, "vm", name)
		return
	}

	netConn := websocket.NetConn(r.Context(), conn, websocket.MessageBinary)
	defer netConn.Close()

	if err := s.mp.ProxyVNC(r.Context(), name, netConn); err != nil && r.Context().Err() == nil {
		s.logger.Warn("VNC tunnel ended", "err", err, "vm", name)
	}
}
