package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rootisgod/passgo-web/pkg/multipass"
)

func newVNCServer(t *testing.T, responses map[string]string) *Server {
	t.Helper()
	runner := func(args ...string) (string, error) {
		key := strings.Join(args, " ")
		if response, ok := responses[key]; ok {
			return response, nil
		}
		t.Fatalf("unexpected multipass call: %s", key)
		return "", nil
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return &Server{mp: multipass.NewClientWithRunner(logger, runner), logger: logger}
}

func TestGetVNCConsoleReturnsTunnelDetails(t *testing.T) {
	srv := newVNCServer(t, map[string]string{
		"info desktop --format json":             `{"info":{"desktop":{"state":"Running"}}}`,
		"exec desktop -- cat " + vncPasswordPath: "random-password\n",
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vms/desktop/vnc", nil)
	req.SetPathValue("name", "desktop")
	rr := httptest.NewRecorder()

	srv.handleGetVNCConsole(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var response vncConsoleResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if !response.Available || response.Password != "random-password" {
		t.Fatalf("unexpected response: %+v", response)
	}
	if response.URL != "/api/v1/vms/desktop/vnc/websocket" {
		t.Fatalf("url = %q", response.URL)
	}
}

func TestGetVNCConsoleRequiresRunningVM(t *testing.T) {
	srv := newVNCServer(t, map[string]string{
		"info desktop --format json": `{"info":{"desktop":{"state":"Stopped"}}}`,
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vms/desktop/vnc", nil)
	req.SetPathValue("name", "desktop")
	rr := httptest.NewRecorder()

	srv.handleGetVNCConsole(rr, req)

	var response vncConsoleResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Available || !strings.Contains(response.Message, "running") {
		t.Fatalf("unexpected response: %+v", response)
	}
}
