package api

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	maxEvents      = 10000
	rotateKeep     = 7500
	eventCacheSize = 200
)

// Event represents a single audit log entry.
type Event struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Category  string `json:"category"`           // "vm", "schedule", "ansible", "llm", "config"
	Action    string `json:"action"`
	Actor     string `json:"actor"`              // "user", "scheduler", "llm_agent"
	Resource  string `json:"resource"`
	Result    string `json:"result"`             // "success", "failed", "partial"
	Detail    string `json:"detail,omitempty"`
	Endpoint  string `json:"endpoint,omitempty"` // "POST /api/v1/vms/{name}/start"
}

// EventLog manages an append-only JSONL event log with in-memory cache.
type EventLog struct {
	mu         sync.Mutex
	file       *os.File
	path       string
	count      int
	cache      []Event // circular buffer, newest at end
	dispatcher WebhookDispatcher
}

// NewEventLog opens or creates the events file, loads recent events into cache,
// and rotates if the file exceeds maxEvents lines.
func NewEventLog(path string) (*EventLog, error) {
	el := &EventLog{path: path, cache: make([]Event, 0, eventCacheSize)}

	// Read existing events to build cache and count
	if data, err := os.ReadFile(path); err == nil {
		lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
		if len(lines) == 1 && lines[0] == "" {
			lines = nil
		}
		el.count = len(lines)

		// Rotate if over limit
		if el.count > maxEvents {
			keepLines := lines[el.count-rotateKeep:]
			if err := os.WriteFile(path, []byte(strings.Join(keepLines, "\n")+"\n"), 0600); err != nil {
				return nil, fmt.Errorf("rotate events: %w", err)
			}
			lines = keepLines
			el.count = len(lines)
		}

		// Load last N into cache
		start := 0
		if len(lines) > eventCacheSize {
			start = len(lines) - eventCacheSize
		}
		for _, line := range lines[start:] {
			var e Event
			if json.Unmarshal([]byte(line), &e) == nil {
				el.cache = append(el.cache, e)
			}
		}
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("open events file: %w", err)
	}
	el.file = f

	return el, nil
}

// SetDispatcher sets the webhook dispatcher called after each event emission.
func (el *EventLog) SetDispatcher(d WebhookDispatcher) {
	el.mu.Lock()
	defer el.mu.Unlock()
	el.dispatcher = d
}

// Emit appends an event to the log. It auto-generates ID and Timestamp.
func (el *EventLog) Emit(e Event) {
	now := time.Now().UTC()
	e.Timestamp = now.Format(time.RFC3339)

	b := make([]byte, 4)
	rand.Read(b)
	e.ID = fmt.Sprintf("%d-%s", now.Unix(), hex.EncodeToString(b))

	data, err := json.Marshal(e)
	if err != nil {
		return
	}

	el.mu.Lock()

	el.file.Write(append(data, '\n'))
	el.count++

	// Update cache
	if len(el.cache) >= eventCacheSize {
		el.cache = el.cache[1:]
	}
	el.cache = append(el.cache, e)

	d := el.dispatcher
	el.mu.Unlock()

	// Dispatch webhooks outside the lock
	if d != nil {
		d.DispatchWebhooks(e)
	}
}

// QueryOpts filters for event queries.
type QueryOpts struct {
	Category string
	Actor    string
	Resource string
	Since    time.Time
	Limit    int
	Before   string // cursor: event ID, return events before this one
}

// QueryResult is the API response for event queries.
type QueryResult struct {
	Events     []Event `json:"events"`
	Total      int     `json:"total"`
	HasMore    bool    `json:"has_more"`
	NextBefore string  `json:"next_before,omitempty"`
}

// Query returns events matching the given filters, newest first.
func (el *EventLog) Query(opts QueryOpts) QueryResult {
	if opts.Limit <= 0 {
		opts.Limit = 50
	}
	if opts.Limit > 200 {
		opts.Limit = 200
	}

	hasFilters := opts.Category != "" || opts.Actor != "" || opts.Resource != "" || !opts.Since.IsZero() || opts.Before != ""

	el.mu.Lock()

	// Fast path: no filters, serve from cache
	if !hasFilters {
		result := el.fromCache(opts.Limit)
		result.Total = el.count
		el.mu.Unlock()
		return result
	}

	// Slow path: scan file
	count := el.count
	el.mu.Unlock()

	return el.scanFile(opts, count)
}

func (el *EventLog) fromCache(limit int) QueryResult {
	n := len(el.cache)
	if limit > n {
		limit = n
	}
	events := make([]Event, limit)
	for i := 0; i < limit; i++ {
		events[i] = el.cache[n-1-i]
	}
	return QueryResult{
		Events:     events,
		HasMore:    n > limit,
		NextBefore: nextBefore(events),
	}
}

func (el *EventLog) scanFile(opts QueryOpts, total int) QueryResult {
	el.mu.Lock()
	path := el.path
	el.mu.Unlock()

	f, err := os.Open(path)
	if err != nil {
		return QueryResult{Events: []Event{}}
	}
	defer f.Close()

	// Read all matching events (file is small, capped at 10k lines)
	var all []Event
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 512*1024)
	beforeFound := opts.Before == ""
	for scanner.Scan() {
		var e Event
		if json.Unmarshal(scanner.Bytes(), &e) != nil {
			continue
		}
		if !beforeFound {
			if e.ID == opts.Before {
				beforeFound = true
			}
			continue
		}
		// Skip the cursor event itself
		if e.ID == opts.Before {
			continue
		}
		if !el.matchesFilters(e, opts) {
			continue
		}
		all = append(all, e)
	}

	// Reverse to newest-first
	for i, j := 0, len(all)-1; i < j; i, j = i+1, j-1 {
		all[i], all[j] = all[j], all[i]
	}

	hasMore := len(all) > opts.Limit
	if hasMore {
		all = all[:opts.Limit]
	}

	return QueryResult{
		Events:     all,
		Total:      total,
		HasMore:    hasMore,
		NextBefore: nextBefore(all),
	}
}

func (el *EventLog) matchesFilters(e Event, opts QueryOpts) bool {
	if opts.Category != "" && e.Category != opts.Category {
		return false
	}
	if opts.Actor != "" && e.Actor != opts.Actor {
		return false
	}
	if opts.Resource != "" && !strings.Contains(strings.ToLower(e.Resource), strings.ToLower(opts.Resource)) {
		return false
	}
	if !opts.Since.IsZero() {
		t, err := time.Parse(time.RFC3339, e.Timestamp)
		if err != nil || t.Before(opts.Since) {
			return false
		}
	}
	return true
}

func nextBefore(events []Event) string {
	if len(events) > 0 {
		return events[len(events)-1].ID
	}
	return ""
}

// EmitEvent is a nil-safe helper for emitting events (non-HTTP contexts like scheduler, goroutines).
func (el *EventLog) EmitEvent(category, action, actor, resource, result, detail string) {
	if el == nil {
		return
	}
	el.Emit(Event{
		Category: category,
		Action:   action,
		Actor:    actor,
		Resource: resource,
		Result:   result,
		Detail:   detail,
	})
}

// EmitHTTPEvent is a nil-safe helper that captures the HTTP method + path from the request.
func (el *EventLog) EmitHTTPEvent(r *http.Request, category, action, resource, result, detail string) {
	if el == nil {
		return
	}
	el.Emit(Event{
		Category: category,
		Action:   action,
		Actor:    "user",
		Resource: resource,
		Result:   result,
		Detail:   detail,
		Endpoint: r.Method + " " + r.URL.Path,
	})
}

// Close closes the underlying file.
func (el *EventLog) Close() {
	el.mu.Lock()
	defer el.mu.Unlock()
	if el.file != nil {
		el.file.Close()
	}
}
