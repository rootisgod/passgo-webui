package api

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/rootisgod/passgo-web/internal/config"
	"github.com/rootisgod/passgo-web/pkg/multipass"
)

func TestHandleInstallSSHKeyValidatesAndInstallsKey(t *testing.T) {
	publicKey := testAuthorizedKey(t)
	var sawExec bool
	runner := func(args ...string) (string, error) {
		if len(args) < 7 || args[0] != "exec" || args[1] != "app" || args[2] != "--" || args[3] != "sh" || args[4] != "-c" {
			t.Fatalf("unexpected multipass args: %v", args)
		}
		if args[len(args)-1] != publicKey {
			t.Fatalf("public key arg = %q, want %q", args[len(args)-1], publicKey)
		}
		sawExec = true
		return "", nil
	}
	s := &Server{
		mp:  multipass.NewClientWithRunner(slog.New(slog.NewTextHandler(io.Discard, nil)), runner),
		cfg: &config.Config{VMDefaults: &config.VMDefaults{SSHPublicKey: publicKey}},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/vms/app/ssh-key", bytes.NewBufferString(`{}`))
	req.SetPathValue("name", "app")
	rr := httptest.NewRecorder()
	s.handleInstallSSHKey(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	if !sawExec {
		t.Fatal("expected multipass exec call")
	}
}

func TestHandleInstallSSHKeyRejectsBadKey(t *testing.T) {
	s := &Server{cfg: &config.Config{}}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/vms/app/ssh-key", strings.NewReader(`{"public_key":"not-a-key"}`))
	req.SetPathValue("name", "app")
	rr := httptest.NewRecorder()
	s.handleInstallSSHKey(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
}

func testAuthorizedKey(t *testing.T) string {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("ssh public key: %v", err)
	}
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPub)))
}
