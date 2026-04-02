package api

import (
	"net/http"

	"github.com/coder/websocket"
)

func (s *Server) handleShell(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	sid := r.PathValue("sessionId")

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: allowedOriginPatterns(r.Host),
	})
	if err != nil {
		s.logger.Error("websocket accept failed", "err", err)
		return
	}
	defer conn.CloseNow()

	// Look up existing PTY session
	sess := s.ptySessions.get(name, sid)
	if sess == nil {
		s.logger.Warn("shell session not found", "vm", name, "session", sid)
		conn.Close(websocket.StatusInternalError, "session not found")
		return
	}

	// Create client with buffered write channel
	client := &wsClient{
		writeCh: make(chan []byte, clientWriteBufSz),
		done:    make(chan struct{}),
	}

	// Attach and replay scrollback so reconnecting user sees recent output
	scrollback := sess.addClient(client)
	defer sess.removeClient(client)

	ctx := r.Context()

	if len(scrollback) > 0 {
		if err := conn.Write(ctx, websocket.MessageBinary, scrollback); err != nil {
			return
		}
	}

	// Write pump: session broadcasts → WebSocket
	go func() {
		for {
			select {
			case data := <-client.writeCh:
				if err := conn.Write(ctx, websocket.MessageBinary, data); err != nil {
					return
				}
			case <-client.done:
				return
			case <-ctx.Done():
				return
			case <-sess.done:
				conn.Close(websocket.StatusGoingAway, "shell process exited")
				return
			}
		}
	}()

	// Read pump: WebSocket → PTY
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			break
		}

		// Handle terminal resize messages (prefixed with 0x01)
		if len(data) > 0 && data[0] == 1 && len(data) >= 5 {
			cols := uint16(data[1])<<8 | uint16(data[2])
			rows := uint16(data[3])<<8 | uint16(data[4])
			sess.resize(cols, rows)
			continue
		}

		if err := sess.writeToPty(data); err != nil {
			break
		}
	}
}

func (s *Server) handleCreateShellSession(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	_, id, err := s.ptySessions.create(name)
	if err != nil {
		s.logger.Error("failed to create shell session", "err", err, "vm", name)
		writeError(w, http.StatusInternalServerError, "failed to start shell: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"sessionId": id})
}

func (s *Server) handleListShellSessions(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	sessions := s.ptySessions.listSessions(name)
	if sessions == nil {
		sessions = []SessionInfo{}
	}
	writeJSON(w, http.StatusOK, sessions)
}

func (s *Server) handleDeleteShellSession(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	sid := r.PathValue("sessionId")
	s.ptySessions.killSession(sessionKey(name, sid))
	writeMessage(w, "session deleted")
}
