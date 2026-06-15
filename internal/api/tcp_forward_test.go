package api

import (
	"io"
	"log/slog"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/rootisgod/passgo-web/internal/config"
)

func TestTCPForwardManagerListensForEnabledSSHRule(t *testing.T) {
	hostPort := freeTCPPort(t)
	s := &Server{
		cfg: &config.Config{ProxyRules: []config.ProxyRule{
			{ID: "px_ssh", VM: "app", Port: 22, Protocol: "ssh", HostPort: hostPort, Enabled: true, CreatedAt: "2026-06-15T12:00:00Z"},
		}},
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	s.tcpForwards = newTCPForwardManager(s)
	s.reconcileTCPForwards()
	defer s.shutdownTCPForwards()

	if listening, detail := s.tcpForwards.status("px_ssh"); !listening {
		t.Fatalf("listener status = false, detail = %q", detail)
	}

	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(hostPort)), time.Second)
	if err != nil {
		t.Fatalf("dial managed ssh listener: %v", err)
	}
	conn.Close()
}

func TestTCPForwardManagerRejectsInvalidSSHRuleBeforeListen(t *testing.T) {
	s := &Server{
		cfg: &config.Config{ProxyRules: []config.ProxyRule{
			{ID: "px_bad", VM: "app", Port: 22, Protocol: "ssh", Enabled: true, CreatedAt: "2026-06-15T12:00:00Z"},
		}},
	}
	s.tcpForwards = newTCPForwardManager(s)
	s.reconcileTCPForwards()
	defer s.shutdownTCPForwards()

	listening, detail := s.tcpForwards.status("px_bad")
	if listening {
		t.Fatal("invalid ssh rule should not be listening")
	}
	if detail == "" {
		t.Fatal("invalid ssh rule should expose a status detail")
	}
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve tcp port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}
