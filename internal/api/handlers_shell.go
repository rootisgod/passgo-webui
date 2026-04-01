package api

import (
	"net/http"

	"github.com/coder/websocket"
)

func (s *Server) handleShell(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // Allow any origin for local tool
	})
	if err != nil {
		s.logger.Error("websocket accept failed", "err", err)
		return
	}
	defer conn.CloseNow()

	// Get or create persistent PTY session for this VM
	sess, err := s.ptySessions.getOrCreate(name)
	if err != nil {
		s.logger.Error("pty session failed", "err", err, "vm", name)
		conn.Close(websocket.StatusInternalError, "failed to start shell")
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
