//go:build !windows

package api

import (
	"os"
	"os/exec"
	"time"

	"github.com/creack/pty"
)

type ptyHandle struct {
	ptmx *os.File
	cmd  *exec.Cmd
}

// startPtySession creates a new multipass shell PTY on Unix/macOS.
func startPtySession(vmName, sessionID string, store *ptyStore) (*ptySession, error) {
	cmd := exec.Command("multipass", "shell", vmName)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}

	sess := &ptySession{
		vmName:     vmName,
		sessionID:  sessionID,
		clients:    make(map[*wsClient]struct{}),
		scrollback: newRingBuffer(scrollbackSize),
		lastActive: time.Now(),
		cols:       120,
		rows:       40,
		done:       make(chan struct{}),
		handle:     &ptyHandle{ptmx: ptmx, cmd: cmd},
	}

	key := sessionKey(vmName, sessionID)

	// PTY read pump: reads from PTY, broadcasts to all clients
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])
				sess.broadcast(data)
			}
			if err != nil {
				break
			}
		}
		close(sess.done)
		store.remove(key)
		store.logger.Info("PTY session ended", "vm", vmName, "session", sessionID)
	}()

	// Collect exit status in background
	go cmd.Wait()

	return sess, nil
}

// writeToPty writes user input to the PTY.
func (ps *ptySession) writeToPty(data []byte) error {
	h := ps.handle.(*ptyHandle)
	_, err := h.ptmx.Write(data)
	return err
}

// resize changes the PTY window size.
func (ps *ptySession) resize(cols, rows uint16) error {
	ps.mu.Lock()
	ps.cols = cols
	ps.rows = rows
	ps.mu.Unlock()
	h := ps.handle.(*ptyHandle)
	return pty.Setsize(h.ptmx, &pty.Winsize{Cols: cols, Rows: rows})
}

// kill terminates the PTY process.
func (ps *ptySession) kill() {
	h := ps.handle.(*ptyHandle)
	h.ptmx.Close()
	if h.cmd.Process != nil {
		h.cmd.Process.Kill()
	}
}
