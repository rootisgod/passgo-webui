package api

import (
	"io"
	"log/slog"
	"path/filepath"
	"testing"
)

func TestEventLogPaginationReturnsOlderEventsWithoutDuplicates(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	log, err := NewEventLog(filepath.Join(t.TempDir(), "events.jsonl"), logger)
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()

	for _, resource := range []string{"one", "two", "three", "four", "five"} {
		log.EmitEvent("vm", "test", "user", resource, "success", "")
	}

	first := log.Query(QueryOpts{Limit: 2})
	if len(first.Events) != 2 || first.Events[0].Resource != "five" || first.Events[1].Resource != "four" {
		t.Fatalf("unexpected first page: %+v", first.Events)
	}
	second := log.Query(QueryOpts{Limit: 2, Before: first.NextBefore})
	if len(second.Events) != 2 || second.Events[0].Resource != "three" || second.Events[1].Resource != "two" {
		t.Fatalf("unexpected second page: %+v", second.Events)
	}
	if second.Events[0].ID == first.Events[0].ID || second.Events[0].ID == first.Events[1].ID {
		t.Fatal("second page duplicated an event from the first page")
	}
}
