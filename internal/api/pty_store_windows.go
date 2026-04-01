//go:build windows

package api

import (
	"context"
	"fmt"
	"time"

	"github.com/UserExistsError/conpty"
)

type ptyHandle struct {
	cpty *conpty.ConPty
}

// startPtySession creates a new multipass shell ConPTY on Windows.
func startPtySession(vmName string, store *ptyStore) (*ptySession, error) {
	commandLine := fmt.Sprintf("multipass shell %s", vmName)
	cpty, err := conpty.Start(commandLine, conpty.ConPtyDimensions(120, 40))
	if err != nil {
		return nil, err
	}

	sess := &ptySession{
		vmName:     vmName,
		clients:    make(map[*wsClient]struct{}),
		scrollback: newRingBuffer(scrollbackSize),
		lastActive: time.Now(),
		cols:       120,
		rows:       40,
		done:       make(chan struct{}),
		handle:     &ptyHandle{cpty: cpty},
	}

	// ConPTY read pump: reads from ConPTY, broadcasts to all clients
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := cpty.Read(buf)
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
		store.remove(vmName)
		store.logger.Info("PTY session ended", "vm", vmName)
	}()

	return sess, nil
}

// writeToPty writes user input to the ConPTY.
func (ps *ptySession) writeToPty(data []byte) error {
	h := ps.handle.(*ptyHandle)
	_, err := h.cpty.Write(data)
	return err
}

// resize changes the ConPTY window size.
func (ps *ptySession) resize(cols, rows uint16) error {
	ps.mu.Lock()
	ps.cols = cols
	ps.rows = rows
	ps.mu.Unlock()
	h := ps.handle.(*ptyHandle)
	return h.cpty.Resize(int(cols), int(rows))
}

// kill terminates the ConPTY process.
func (ps *ptySession) kill() {
	h := ps.handle.(*ptyHandle)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	h.cpty.Wait(ctx)
	h.cpty.Close()
}
