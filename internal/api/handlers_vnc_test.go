package api

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rootisgod/passgo-web/pkg/multipass"
)

func TestVNCAddressForVMRequiresRunningVMWithUsableIPv4(t *testing.T) {
	runner := func(args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "info gui-vm --format json":
			return `{"errors":[],"info":{"gui-vm":{"state":"Running","cpu_count":"2","memory":{"used":0,"total":0},"disks":{},"mounts":{},"ipv4":["127.0.0.1","192.168.64.20"],"snapshot_count":"0"}}}`, nil
		case "info stopped-vm --format json":
			return `{"errors":[],"info":{"stopped-vm":{"state":"Stopped","cpu_count":"2","memory":{"used":0,"total":0},"disks":{},"mounts":{},"ipv4":["192.168.64.21"],"snapshot_count":"0"}}}`, nil
		case "info no-ip-vm --format json":
			return `{"errors":[],"info":{"no-ip-vm":{"state":"Running","cpu_count":"2","memory":{"used":0,"total":0},"disks":{},"mounts":{},"ipv4":["127.0.0.1","169.254.1.2"],"snapshot_count":"0"}}}`, nil
		default:
			return "", fmt.Errorf("unexpected multipass call: %v", args)
		}
	}
	s := &Server{
		mp: multipass.NewClientWithRunner(slog.New(slog.NewTextHandler(io.Discard, nil)), runner),
	}

	rr := httptest.NewRecorder()
	addr, ok := s.vncAddressForVM(rr, "gui-vm", 5901)
	if !ok {
		t.Fatalf("expected running VM to resolve, status=%d body=%s", rr.Code, rr.Body.String())
	}
	if addr != "192.168.64.20:5901" {
		t.Fatalf("addr = %q, want 192.168.64.20:5901", addr)
	}

	rr = httptest.NewRecorder()
	if _, ok := s.vncAddressForVM(rr, "stopped-vm", 5900); ok || rr.Code != http.StatusConflict {
		t.Fatalf("stopped VM: ok=%v status=%d body=%s", ok, rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	if _, ok := s.vncAddressForVM(rr, "no-ip-vm", 5900); ok || rr.Code != http.StatusConflict {
		t.Fatalf("no-ip VM: ok=%v status=%d body=%s", ok, rr.Code, rr.Body.String())
	}
}

func TestValidVNCPort(t *testing.T) {
	for _, tc := range []struct {
		raw    string
		want   int
		wantOK bool
	}{
		{"", 5900, true},
		{"5901", 5901, true},
		{"22", 0, false},
		{"6000", 0, false},
		{"abc", 0, false},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/vms/gui-vm/vnc", nil)
			if tc.raw != "" {
				q := req.URL.Query()
				q.Set("port", tc.raw)
				req.URL.RawQuery = q.Encode()
			}
			rr := httptest.NewRecorder()
			got, ok := validVNCPort(rr, req)
			if ok != tc.wantOK || got != tc.want {
				t.Fatalf("validVNCPort(%q) = (%d, %v), want (%d, %v)", tc.raw, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}
