package api

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rootisgod/passgo-web/internal/config"
)

type tcpForwardManager struct {
	server   *Server
	mu       sync.Mutex
	forwards map[string]*tcpForward
	errors   map[string]string
}

type tcpForward struct {
	manager  *tcpForwardManager
	rule     config.ProxyRule
	listener net.Listener
}

func newTCPForwardManager(s *Server) *tcpForwardManager {
	return &tcpForwardManager{
		server:   s,
		forwards: make(map[string]*tcpForward),
		errors:   make(map[string]string),
	}
}

func (s *Server) reconcileTCPForwards() {
	if s != nil && s.tcpForwards != nil {
		s.tcpForwards.reconcile()
	}
}

func (s *Server) shutdownTCPForwards() {
	if s != nil && s.tcpForwards != nil {
		s.tcpForwards.shutdown()
	}
}

func (m *tcpForwardManager) reconcile() {
	if m == nil || m.server == nil || m.server.cfg == nil {
		return
	}

	now := time.Now().UTC()
	m.server.cfgMu.Lock()
	rules := append([]config.ProxyRule(nil), m.server.cfg.GetProxyRules()...)
	m.server.cfgMu.Unlock()

	wanted := make(map[string]config.ProxyRule)
	ruleErrors := make(map[string]string)
	for _, rule := range rules {
		if proxyRuleProtocol(rule) != "ssh" || !rule.Enabled || proxyRuleExpired(rule, now) {
			continue
		}
		normalized := rule
		if err := normalized.Validate(); err != nil {
			if normalized.ID != "" {
				ruleErrors[normalized.ID] = err.Error()
			}
			continue
		}
		wanted[normalized.ID] = normalized
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for id, err := range ruleErrors {
		m.errors[id] = err
	}

	for id, forward := range m.forwards {
		rule, ok := wanted[id]
		if !ok || !sameTCPForwardRule(forward.rule, rule) {
			forward.close()
			delete(m.forwards, id)
			if _, hasRuleError := ruleErrors[id]; !ok && !hasRuleError {
				delete(m.errors, id)
			}
			continue
		}
		delete(wanted, id)
	}

	for id, rule := range wanted {
		listenAddr := net.JoinHostPort("", strconv.Itoa(rule.HostPort))
		listener, err := net.Listen("tcp", listenAddr)
		if err != nil {
			m.errors[id] = err.Error()
			m.logWarn("ssh proxy listen failed", "rule", id, "host_port", rule.HostPort, "err", err)
			continue
		}

		forward := &tcpForward{
			manager:  m,
			rule:     rule,
			listener: listener,
		}
		m.forwards[id] = forward
		delete(m.errors, id)
		m.logInfo("ssh proxy listening", "rule", id, "vm", rule.VM, "target_port", rule.Port, "host_port", rule.HostPort)
		go forward.serve()
	}
}

func (m *tcpForwardManager) shutdown() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, forward := range m.forwards {
		forward.close()
		delete(m.forwards, id)
	}
	clear(m.errors)
}

func (m *tcpForwardManager) status(id string) (bool, string) {
	if m == nil {
		return false, "ssh forward manager is not running"
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.forwards[id]; ok {
		return true, ""
	}
	if err := m.errors[id]; err != "" {
		return false, err
	}
	return false, "ssh listener is not running"
}

func (m *tcpForwardManager) removeForward(id string, forward *tcpForward, err error) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if current := m.forwards[id]; current == forward {
		delete(m.forwards, id)
		if err != nil && !errors.Is(err, net.ErrClosed) {
			m.errors[id] = err.Error()
		}
	}
}

func (m *tcpForwardManager) logInfo(msg string, args ...any) {
	if m != nil && m.server != nil && m.server.logger != nil {
		m.server.logger.Info(msg, args...)
	}
}

func (m *tcpForwardManager) logWarn(msg string, args ...any) {
	if m != nil && m.server != nil && m.server.logger != nil {
		m.server.logger.Warn(msg, args...)
	}
}

func (f *tcpForward) serve() {
	for {
		conn, err := f.listener.Accept()
		if err != nil {
			f.manager.removeForward(f.rule.ID, f, err)
			if !errors.Is(err, net.ErrClosed) {
				f.manager.logWarn("ssh proxy accept failed", "rule", f.rule.ID, "host_port", f.rule.HostPort, "err", err)
			}
			return
		}
		go f.handle(conn)
	}
}

func (f *tcpForward) handle(client net.Conn) {
	defer client.Close()

	target, err := f.manager.server.tcpTargetAddress(f.rule)
	if err != nil {
		f.manager.logWarn("ssh proxy target unavailable", "rule", f.rule.ID, "vm", f.rule.VM, "err", err)
		return
	}

	upstream, err := net.DialTimeout("tcp", target, 10*time.Second)
	if err != nil {
		f.manager.logWarn("ssh proxy dial failed", "rule", f.rule.ID, "target", target, "err", err)
		return
	}
	defer upstream.Close()

	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(upstream, client)
		_ = upstream.Close()
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(client, upstream)
		_ = client.Close()
		done <- struct{}{}
	}()
	<-done
}

func (f *tcpForward) close() {
	if f != nil && f.listener != nil {
		_ = f.listener.Close()
	}
}

func (s *Server) tcpTargetAddress(rule config.ProxyRule) (string, error) {
	if s == nil || s.mp == nil {
		return "", fmt.Errorf("multipass client is unavailable")
	}
	if !validProxyPort(rule.Port) {
		return "", fmt.Errorf("port must be between %d and %d", minProxyPort, maxProxyPort)
	}
	vm, err := s.mp.GetVMInfo(rule.VM)
	if err != nil {
		return "", err
	}
	if vm.State != "Running" {
		return "", fmt.Errorf("vm is not running")
	}
	ip, ok := firstUsableIPv4(vm)
	if !ok {
		return "", fmt.Errorf("vm has no usable IPv4 address")
	}
	return net.JoinHostPort(ip.String(), strconv.Itoa(rule.Port)), nil
}

func sameTCPForwardRule(a, b config.ProxyRule) bool {
	return a.ID == b.ID &&
		a.VM == b.VM &&
		a.Port == b.Port &&
		a.HostPort == b.HostPort &&
		a.Enabled == b.Enabled &&
		a.ExpiresAt == b.ExpiresAt &&
		proxyRuleProtocol(a) == proxyRuleProtocol(b)
}

func proxyRuleProtocol(rule config.ProxyRule) string {
	switch strings.ToLower(strings.TrimSpace(rule.Protocol)) {
	case "", "http", "http/ws", "web", "websocket":
		return "http"
	case "ssh", "tcp":
		return "ssh"
	default:
		return strings.ToLower(strings.TrimSpace(rule.Protocol))
	}
}
