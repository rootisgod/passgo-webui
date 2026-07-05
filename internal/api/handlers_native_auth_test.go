package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"
)

func TestCheckNativeAuthStatusMapsInstalledAndAuthenticated(t *testing.T) {
	restore := stubNativeAuth(
		func(binary string) (string, error) {
			switch binary {
			case "codex":
				return "/usr/local/bin/codex", nil
			case "claude":
				return "/usr/local/bin/claude", nil
			default:
				return "", exec.ErrNotFound
			}
		},
		func(ctx context.Context, path string, args ...string) error {
			if path == "/usr/local/bin/codex" {
				return nil
			}
			return errors.New("not logged in")
		},
	)
	defer restore()

	statuses := checkNativeAuthStatus(context.Background())
	if len(statuses) != 2 {
		t.Fatalf("len(statuses) = %d, want 2", len(statuses))
	}
	if statuses[0].ID != "codex" || !statuses[0].Installed || !statuses[0].Authenticated || statuses[0].Status != "authenticated" {
		t.Fatalf("codex status = %+v", statuses[0])
	}
	if statuses[1].ID != "claude" || !statuses[1].Installed || statuses[1].Authenticated || statuses[1].Status != "not_authenticated" {
		t.Fatalf("claude status = %+v", statuses[1])
	}
}

func TestCheckNativeAuthStatusMapsMissingBinary(t *testing.T) {
	restore := stubNativeAuth(
		func(string) (string, error) {
			return "", exec.ErrNotFound
		},
		func(context.Context, string, ...string) error {
			t.Fatal("runner should not be called for missing binaries")
			return nil
		},
	)
	defer restore()

	statuses := checkNativeAuthStatus(context.Background())
	for _, status := range statuses {
		if status.Installed || status.Authenticated || status.Status != "not_installed" {
			t.Fatalf("status = %+v, want not_installed", status)
		}
	}
}

func TestHandleGetNativeAuthStatus(t *testing.T) {
	restore := stubNativeAuth(
		func(binary string) (string, error) {
			return "/usr/local/bin/" + binary, nil
		},
		func(context.Context, string, ...string) error {
			return nil
		},
	)
	defer restore()

	s := &Server{}
	rr := httptest.NewRecorder()
	s.handleGetNativeAuthStatus(rr, httptest.NewRequest(http.MethodGet, "/api/v1/chat/native-auth", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var resp nativeAuthStatusResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Providers) != 2 {
		t.Fatalf("providers = %+v", resp.Providers)
	}
	for _, provider := range resp.Providers {
		if !provider.Installed || !provider.Authenticated || provider.Status != "authenticated" {
			t.Fatalf("provider = %+v", provider)
		}
	}
}

func stubNativeAuth(
	lookPath func(string) (string, error),
	runCommand func(context.Context, string, ...string) error,
) func() {
	oldLookPath := nativeAuthLookPath
	oldRunCommand := nativeAuthRunCommand
	nativeAuthLookPath = lookPath
	nativeAuthRunCommand = runCommand
	return func() {
		nativeAuthLookPath = oldLookPath
		nativeAuthRunCommand = oldRunCommand
	}
}
