package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

const (
	scrollbackSize   = 64 * 1024       // 64KB ring buffer for terminal replay
	ptySessionTTL    = 30 * time.Minute // idle session lifetime
	reaperInterval   = 1 * time.Minute  // how often the reaper checks
	clientWriteBufSz = 256              // buffered channel size per client
)

// ringBuffer is a circular byte buffer for scrollback replay.
type ringBuffer struct {
	buf  []byte
	size int
	pos  int
	full bool
}

func newRingBuffer(size int) *ringBuffer {
	return &ringBuffer{buf: make([]byte, size), size: size}
}

func (rb *ringBuffer) Write(p []byte) {
	for _, b := range p {
		rb.buf[rb.pos] = b
		rb.pos++
		if rb.pos >= rb.size {
			rb.pos = 0
			rb.full = true
		}
	}
}

func (rb *ringBuffer) Bytes() []byte {
	if !rb.full {
		return append([]byte(nil), rb.buf[:rb.pos]...)
	}
	out := make([]byte, rb.size)
	n := copy(out, rb.buf[rb.pos:])
	copy(out[n:], rb.buf[:rb.pos])
	return out
}

// wsClient represents a single WebSocket viewer attached to a ptySession.
type wsClient struct {
	writeCh chan []byte
	done    chan struct{}
}

// ptySession holds one persistent shell session for a VM.
type ptySession struct {
	vmName     string
	sessionID  string
	mu         sync.Mutex
	clients    map[*wsClient]struct{}
	scrollback *ringBuffer
	lastActive time.Time
	cols       uint16
	rows       uint16
	done       chan struct{} // closed when PTY process exits
	handle     any          // platform-specific: *ptyHandle (unix) or *conptyHandle (windows)
}

// addClient registers a WebSocket viewer and returns the scrollback snapshot.
func (ps *ptySession) addClient(c *wsClient) []byte {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.clients[c] = struct{}{}
	ps.lastActive = time.Now()
	return ps.scrollback.Bytes()
}

// removeClient unregisters a WebSocket viewer.
func (ps *ptySession) removeClient(c *wsClient) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	delete(ps.clients, c)
	select {
	case <-c.done:
	default:
		close(c.done)
	}
	ps.lastActive = time.Now()
}

// broadcast sends PTY output to all connected clients and the scrollback buffer.
func (ps *ptySession) broadcast(data []byte) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.scrollback.Write(data)
	for c := range ps.clients {
		select {
		case c.writeCh <- append([]byte(nil), data...):
		default:
			// Client is slow — drop rather than block the PTY reader.
		}
	}
}

// clientCount returns number of attached WebSocket clients.
func (ps *ptySession) clientCount() int {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return len(ps.clients)
}

// isAlive returns true if the PTY process is still running.
func (ps *ptySession) isAlive() bool {
	select {
	case <-ps.done:
		return false
	default:
		return true
	}
}

// sessionKey returns the composite map key for a VM session.
func sessionKey(vmName, sessionID string) string {
	return vmName + ":" + sessionID
}

// SessionInfo is the JSON-serializable info for a shell session.
type SessionInfo struct {
	SessionID string `json:"sessionId"`
	Alive     bool   `json:"alive"`
}

// ptyStore manages all active PTY sessions, keyed by vmName:sessionID.
type ptyStore struct {
	mu       sync.Mutex
	sessions map[string]*ptySession
	logger   *slog.Logger
	stopCh   chan struct{}
}

func newPtyStore(logger *slog.Logger) *ptyStore {
	ps := &ptyStore{
		sessions: make(map[string]*ptySession),
		logger:   logger,
		stopCh:   make(chan struct{}),
	}
	go ps.reaper()
	return ps
}

// create spawns a new PTY session for a VM and returns it with its session ID.
func (store *ptyStore) create(vmName string) (*ptySession, string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, "", fmt.Errorf("generate session ID: %w", err)
	}
	id := hex.EncodeToString(b)

	sess, err := startPtySession(vmName, id, store)
	if err != nil {
		return nil, "", err
	}

	key := sessionKey(vmName, id)
	store.mu.Lock()
	store.sessions[key] = sess
	store.mu.Unlock()

	store.logger.Info("created PTY session", "vm", vmName, "session", id)
	return sess, id, nil
}

// get returns an existing live session or nil.
func (store *ptyStore) get(vmName, sessionID string) *ptySession {
	key := sessionKey(vmName, sessionID)
	store.mu.Lock()
	defer store.mu.Unlock()
	sess, ok := store.sessions[key]
	if ok && sess.isAlive() {
		return sess
	}
	return nil
}

// listSessions returns info about all sessions for a VM.
func (store *ptyStore) listSessions(vmName string) []SessionInfo {
	prefix := vmName + ":"
	store.mu.Lock()
	defer store.mu.Unlock()

	var result []SessionInfo
	for key, sess := range store.sessions {
		if strings.HasPrefix(key, prefix) {
			result = append(result, SessionInfo{
				SessionID: sess.sessionID,
				Alive:     sess.isAlive(),
			})
		}
	}
	return result
}

// remove cleans up a session from the store by composite key.
func (store *ptyStore) remove(key string) {
	store.mu.Lock()
	defer store.mu.Unlock()
	delete(store.sessions, key)
}

// killSession terminates a specific PTY session by composite key.
func (store *ptyStore) killSession(key string) {
	store.mu.Lock()
	sess, ok := store.sessions[key]
	if ok {
		delete(store.sessions, key)
	}
	store.mu.Unlock()
	if ok {
		sess.kill()
		store.logger.Info("killed PTY session", "key", key)
	}
}

// killAllSessions terminates all PTY sessions for a VM.
func (store *ptyStore) killAllSessions(vmName string) {
	prefix := vmName + ":"
	store.mu.Lock()
	var toKill []*ptySession
	for key, sess := range store.sessions {
		if strings.HasPrefix(key, prefix) {
			toKill = append(toKill, sess)
			delete(store.sessions, key)
		}
	}
	store.mu.Unlock()
	for _, sess := range toKill {
		sess.kill()
	}
	if len(toKill) > 0 {
		store.logger.Info("killed all PTY sessions", "vm", vmName, "count", len(toKill))
	}
}

// reaper periodically kills sessions that have been idle past TTL.
func (store *ptyStore) reaper() {
	ticker := time.NewTicker(reaperInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			var toKill []*ptySession
			store.mu.Lock()
			for key, sess := range store.sessions {
				sess.mu.Lock()
				idle := len(sess.clients) == 0 && time.Since(sess.lastActive) > ptySessionTTL
				dead := !sess.isAlive()
				sess.mu.Unlock()
				if idle || dead {
					if idle {
						store.logger.Info("reaping idle PTY session", "key", key,
							"idle", time.Since(sess.lastActive).Round(time.Second))
						toKill = append(toKill, sess)
					}
					delete(store.sessions, key)
				}
			}
			store.mu.Unlock()
			for _, sess := range toKill {
				sess.kill()
			}
		case <-store.stopCh:
			return
		}
	}
}

// shutdown kills all sessions.
func (store *ptyStore) shutdown() {
	close(store.stopCh)
	store.mu.Lock()
	var toKill []*ptySession
	for key, sess := range store.sessions {
		toKill = append(toKill, sess)
		delete(store.sessions, key)
	}
	store.mu.Unlock()
	for _, sess := range toKill {
		sess.kill()
	}
}
