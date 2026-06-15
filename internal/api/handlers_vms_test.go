package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rootisgod/passgo-web/internal/config"
	"github.com/rootisgod/passgo-web/pkg/multipass"
)

func TestVMTemplateHandlers(t *testing.T) {
	const listJSON = `{
		"list": [
			{"name": "base", "state": "Stopped", "ipv4": [], "release": "Ubuntu 24.04 LTS"},
			{"name": "workload", "state": "Running", "ipv4": [], "release": "Ubuntu 24.04 LTS"}
		]
	}`

	runner := func(args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "info --all --format json":
			return "", errors.New("force list fallback")
		case "list --format json":
			return listJSON, nil
		default:
			t.Fatalf("unexpected multipass call: %v", args)
			return "", nil
		}
	}

	tmp := t.TempDir()
	s := &Server{
		mp:         multipass.NewClientWithRunner(slog.New(slog.NewTextHandler(io.Discard, nil)), runner),
		cfg:        &config.Config{VMTemplates: map[string]bool{}},
		configPath: filepath.Join(tmp, "config.json"),
	}

	req := httptest.NewRequest(http.MethodPut, "/api/v1/vms/base/template", bytes.NewBufferString(`{"template":true}`))
	req.SetPathValue("name", "base")
	rr := httptest.NewRecorder()
	s.handleSetVMTemplate(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("set template status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if !s.cfg.VMTemplates["base"] {
		t.Fatal("template flag was not persisted in config")
	}

	rr = httptest.NewRecorder()
	s.handleListVMTemplates(rr, httptest.NewRequest(http.MethodGet, "/api/v1/vm-templates", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("list template status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var listed vmTemplatesResponse
	if err := json.NewDecoder(rr.Body).Decode(&listed); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !listed.Templates["base"] {
		t.Fatalf("listed templates = %+v, want base=true", listed.Templates)
	}

	req = httptest.NewRequest(http.MethodPut, "/api/v1/vms/workload/template", bytes.NewBufferString(`{"template":true}`))
	req.SetPathValue("name", "workload")
	rr = httptest.NewRecorder()
	s.handleSetVMTemplate(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("running vm template status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if s.cfg.VMTemplates["workload"] {
		t.Fatal("running VM should not be marked as a template")
	}

	req = httptest.NewRequest(http.MethodPut, "/api/v1/vms/base/template", bytes.NewBufferString(`{"template":false}`))
	req.SetPathValue("name", "base")
	rr = httptest.NewRecorder()
	s.handleSetVMTemplate(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("clear template status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if s.cfg.VMTemplates["base"] {
		t.Fatal("template flag was not cleared")
	}
}

func TestTemplateVMProtectedFromStartAndDelete(t *testing.T) {
	runner := func(args ...string) (string, error) {
		t.Fatalf("unexpected multipass call for protected template: %v", args)
		return "", nil
	}
	s := &Server{
		mp:  multipass.NewClientWithRunner(slog.New(slog.NewTextHandler(io.Discard, nil)), runner),
		cfg: &config.Config{VMTemplates: map[string]bool{"base": true}},
	}

	tests := []struct {
		name    string
		method  string
		path    string
		body    string
		handler func(http.ResponseWriter, *http.Request)
		want    string
	}{
		{
			name:    "start",
			method:  http.MethodPost,
			path:    "/api/v1/vms/base/start",
			handler: s.handleStartVM,
			want:    "remove template tag before starting this VM",
		},
		{
			name:    "delete",
			method:  http.MethodDelete,
			path:    "/api/v1/vms/base",
			body:    `{"purge":true}`,
			handler: s.handleDeleteVM,
			want:    "remove template tag before deleting this VM",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.SetPathValue("name", "base")
			rr := httptest.NewRecorder()

			tt.handler(rr, req)

			if rr.Code != http.StatusConflict {
				t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
			}
			if !strings.Contains(rr.Body.String(), tt.want) {
				t.Fatalf("body = %s, want %q", rr.Body.String(), tt.want)
			}
		})
	}
}

func TestStartAllSkipsTemplateVMs(t *testing.T) {
	const listJSON = `{
		"list": [
			{"name": "base", "state": "Stopped", "ipv4": [], "release": "Ubuntu 24.04 LTS"},
			{"name": "workload", "state": "Stopped", "ipv4": [], "release": "Ubuntu 24.04 LTS"},
			{"name": "running", "state": "Running", "ipv4": [], "release": "Ubuntu 24.04 LTS"}
		]
	}`

	var starts []string
	runner := func(args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "info --all --format json":
			return "", errors.New("force list fallback")
		case "list --format json":
			return listJSON, nil
		case "start workload":
			starts = append(starts, "workload")
			return "", nil
		default:
			t.Fatalf("unexpected multipass call: %v", args)
			return "", nil
		}
	}

	s := &Server{
		mp:  multipass.NewClientWithRunner(slog.New(slog.NewTextHandler(io.Discard, nil)), runner),
		cfg: &config.Config{VMTemplates: map[string]bool{"base": true}},
	}

	rr := httptest.NewRecorder()
	s.handleStartAll(rr, httptest.NewRequest(http.MethodPost, "/api/v1/vms/start-all", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if len(starts) != 1 || starts[0] != "workload" {
		t.Fatalf("started VMs = %v, want [workload]", starts)
	}
}
