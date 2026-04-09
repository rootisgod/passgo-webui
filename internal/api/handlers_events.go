package api

import (
	"net/http"
	"strconv"
	"time"
)

func (s *Server) handleListEvents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	opts := QueryOpts{
		Category: q.Get("category"),
		Actor:    q.Get("actor"),
		Resource: q.Get("resource"),
		Before:   q.Get("before"),
	}

	if s := q.Get("since"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			opts.Since = t
		}
	}

	if l := q.Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			opts.Limit = n
		}
	}

	result := s.eventLog.Query(opts)
	writeJSON(w, http.StatusOK, result)
}
