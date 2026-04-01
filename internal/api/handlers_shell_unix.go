//go:build !windows

package api

import (
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/coder/websocket"
	"github.com/creack/pty"
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

	cmd := exec.Command("multipass", "shell", name)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	ptmx, err := pty.Start(cmd)
	if err != nil {
		s.logger.Error("pty start failed", "err", err, "vm", name)
		conn.Close(websocket.StatusInternalError, "failed to start shell")
		return
	}
	defer ptmx.Close()

	ctx := r.Context()

	// PTY → WebSocket
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if err != nil {
				return
			}
			if err := conn.Write(ctx, websocket.MessageBinary, buf[:n]); err != nil {
				return
			}
		}
	}()

	// WebSocket → PTY
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			break
		}

		// Handle terminal resize messages (prefixed with 0x01)
		if len(data) > 0 && data[0] == 1 && len(data) >= 5 {
			cols := uint16(data[1])<<8 | uint16(data[2])
			rows := uint16(data[3])<<8 | uint16(data[4])
			pty.Setsize(ptmx, &pty.Winsize{Cols: cols, Rows: rows})
			continue
		}

		if _, err := ptmx.Write(data); err != nil {
			break
		}
	}

	// Wait a moment then kill the process
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		cmd.Process.Kill()
	}

	conn.Close(websocket.StatusNormalClosure, "shell closed")

	// Suppress unused import errors
	_ = io.Discard
}
