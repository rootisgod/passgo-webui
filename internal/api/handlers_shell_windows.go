//go:build windows

package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/UserExistsError/conpty"
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

	// Start multipass shell in a Windows ConPTY (pseudo-terminal)
	commandLine := fmt.Sprintf("multipass shell %s", name)
	cpty, err := conpty.Start(commandLine, conpty.ConPtyDimensions(120, 40))
	if err != nil {
		s.logger.Error("conpty start failed", "err", err, "vm", name)
		conn.Close(websocket.StatusInternalError, "failed to start shell")
		return
	}
	defer cpty.Close()

	ctx := r.Context()

	// ConPTY → WebSocket
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := cpty.Read(buf)
			if n > 0 {
				if werr := conn.Write(ctx, websocket.MessageBinary, buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// WebSocket → ConPTY
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			break
		}

		// Handle terminal resize messages (prefixed with 0x01)
		if len(data) > 0 && data[0] == 1 && len(data) >= 5 {
			cols := int(data[1])<<8 | int(data[2])
			rows := int(data[3])<<8 | int(data[4])
			if resizeErr := cpty.Resize(cols, rows); resizeErr != nil {
				s.logger.Warn("conpty resize failed", "err", resizeErr)
			}
			continue
		}

		if _, err := cpty.Write(data); err != nil {
			break
		}
	}

	// Wait for process to exit
	waitCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cpty.Wait(waitCtx)

	conn.Close(websocket.StatusNormalClosure, "shell closed")
}
