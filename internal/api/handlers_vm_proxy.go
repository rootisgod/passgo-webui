package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rootisgod/passgo-web/internal/config"
	"github.com/rootisgod/passgo-web/pkg/multipass"
)

const (
	vmProxyPrefix = "/api/v1/proxy/vms/"
	minProxyPort  = 1
	maxProxyPort  = 65535
)

type proxyRuleCreateRequest struct {
	VM        string `json:"vm"`
	Port      int    `json:"port"`
	Protocol  string `json:"protocol"`
	HostPort  int    `json:"host_port"`
	Label     string `json:"label"`
	Enabled   *bool  `json:"enabled"`
	ExpiresAt string `json:"expires_at"`
}

type proxyRuleUpdateRequest struct {
	Label     *string `json:"label"`
	Enabled   *bool   `json:"enabled"`
	ExpiresAt *string `json:"expires_at"`
}

type proxyRuleResponse struct {
	ID            string `json:"id"`
	VM            string `json:"vm"`
	Port          int    `json:"port"`
	HostPort      int    `json:"host_port,omitempty"`
	Label         string `json:"label,omitempty"`
	Protocol      string `json:"protocol"`
	Enabled       bool   `json:"enabled"`
	ExpiresAt     string `json:"expires_at,omitempty"`
	CreatedAt     string `json:"created_at"`
	ProxyURL      string `json:"proxy_url,omitempty"`
	ListenAddress string `json:"listen_address,omitempty"`
	SSHCommand    string `json:"ssh_command,omitempty"`
	Destination   string `json:"destination"`
	Target        string `json:"target,omitempty"`
	VMState       string `json:"vm_state,omitempty"`
	Status        string `json:"status"`
	StatusDetail  string `json:"status_detail,omitempty"`
}

func (s *Server) handleListProxyRules(w http.ResponseWriter, r *http.Request) {
	vmFilter := r.URL.Query().Get("vm")
	if vmFilter != "" {
		if err := multipass.ValidateVMName(vmFilter); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	s.cfgMu.Lock()
	rules := append([]config.ProxyRule(nil), s.cfg.GetProxyRules()...)
	s.cfgMu.Unlock()

	vms, vmErr := s.mp.ListVMs()
	vmByName := make(map[string]multipass.VMInfo, len(vms))
	if vmErr == nil {
		for _, vm := range vms {
			vmByName[vm.Name] = vm
		}
	}

	responses := make([]proxyRuleResponse, 0, len(rules))
	now := time.Now().UTC()
	for _, rule := range rules {
		if vmFilter != "" && rule.VM != vmFilter {
			continue
		}
		responses = append(responses, s.proxyRuleResponse(r, rule, vmByName, vmErr, now))
	}
	writeJSON(w, http.StatusOK, responses)
}

func (s *Server) handleCreateProxyRule(w http.ResponseWriter, r *http.Request) {
	var req proxyRuleCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := multipass.ValidateVMName(req.VM); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	protocol := proxyRuleProtocol(config.ProxyRule{Protocol: req.Protocol})
	if protocol != "http" && protocol != "ssh" {
		writeError(w, http.StatusBadRequest, "protocol must be http or ssh")
		return
	}
	if protocol == "ssh" && req.Port == 0 {
		req.Port = 22
	}
	if !validProxyPort(req.Port) {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("port must be between %d and %d", minProxyPort, maxProxyPort))
		return
	}
	if _, err := s.mp.GetVMInfo(req.VM); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	id, err := newProxyRuleID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate proxy rule ID")
		return
	}
	rule := config.ProxyRule{
		ID:        id,
		VM:        req.VM,
		Port:      req.Port,
		Protocol:  protocol,
		HostPort:  req.HostPort,
		Label:     strings.TrimSpace(req.Label),
		Enabled:   enabled,
		ExpiresAt: strings.TrimSpace(req.ExpiresAt),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := rule.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.cfgMu.Lock()
	if err := s.cfg.AddProxyRule(rule); err != nil {
		s.cfgMu.Unlock()
		if strings.Contains(err.Error(), "already exists") {
			writeError(w, http.StatusConflict, err.Error())
		} else {
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	if err := s.cfg.Save(s.configPath); err != nil {
		s.cfgMu.Unlock()
		writeError(w, http.StatusInternalServerError, "failed to save config")
		return
	}
	s.cfgMu.Unlock()
	s.reconcileTCPForwards()

	s.eventLog.EmitHTTPEvent(r, "config", "create_proxy_rule", rule.VM, "success", proxyRuleEventDetail(rule))
	writeJSON(w, http.StatusCreated, s.proxyRuleResponse(r, rule, nil, nil, time.Now().UTC()))
}

func (s *Server) handleUpdateProxyRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	var req proxyRuleUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	s.cfgMu.Lock()
	existing, _ := s.cfg.GetProxyRule(id)
	if existing == nil {
		s.cfgMu.Unlock()
		writeError(w, http.StatusNotFound, "proxy rule not found")
		return
	}
	rule := *existing
	if req.Label != nil {
		rule.Label = strings.TrimSpace(*req.Label)
	}
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}
	if req.ExpiresAt != nil {
		rule.ExpiresAt = strings.TrimSpace(*req.ExpiresAt)
	}
	if err := s.cfg.UpdateProxyRule(rule); err != nil {
		s.cfgMu.Unlock()
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.cfg.Save(s.configPath); err != nil {
		s.cfgMu.Unlock()
		writeError(w, http.StatusInternalServerError, "failed to save config")
		return
	}
	s.cfgMu.Unlock()
	s.reconcileTCPForwards()

	s.eventLog.EmitHTTPEvent(r, "config", "update_proxy_rule", rule.VM, "success", proxyRuleEventDetail(rule))
	writeJSON(w, http.StatusOK, s.proxyRuleResponse(r, rule, nil, nil, time.Now().UTC()))
}

func (s *Server) handleDeleteProxyRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	s.cfgMu.Lock()
	rule, _ := s.cfg.GetProxyRule(id)
	if rule == nil {
		s.cfgMu.Unlock()
		writeError(w, http.StatusNotFound, "proxy rule not found")
		return
	}
	resource := fmt.Sprintf("%s:%d", rule.VM, rule.Port)
	if err := s.cfg.DeleteProxyRule(id); err != nil {
		s.cfgMu.Unlock()
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err := s.cfg.Save(s.configPath); err != nil {
		s.cfgMu.Unlock()
		writeError(w, http.StatusInternalServerError, "failed to save config")
		return
	}
	s.cfgMu.Unlock()
	s.reconcileTCPForwards()

	s.eventLog.EmitHTTPEvent(r, "config", "delete_proxy_rule", resource, "success", "")
	writeMessage(w, "proxy rule deleted")
}

func (s *Server) handleVMHTTPProxy(w http.ResponseWriter, r *http.Request) {
	name, port, upstreamPath, ok := parseVMProxyPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid proxy path")
		return
	}
	if err := multipass.ValidateVMName(name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !validProxyPort(port) {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("port must be between %d and %d", minProxyPort, maxProxyPort))
		return
	}
	if isWebSocketUpgrade(r) {
		origin := r.Header.Get("Origin")
		if origin != "" && !isAllowedOrigin(origin, r.Host) {
			writeError(w, http.StatusForbidden, "websocket origin not allowed")
			return
		}
	}

	_, access := s.proxyRuleAccess(name, port, time.Now().UTC())
	switch access {
	case "allowed":
	case "expired":
		writeError(w, http.StatusGone, "proxy rule has expired")
		return
	case "disabled":
		writeError(w, http.StatusForbidden, "proxy rule is disabled")
		return
	default:
		writeError(w, http.StatusForbidden, "proxy rule not found for VM and port")
		return
	}

	addr, ok := s.proxyAddressForVM(w, name, port)
	if !ok {
		return
	}

	target := &url.URL{Scheme: "http", Host: addr}
	proxyBase := s.proxyBasePath(name, port)
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	proxy := &httputil.ReverseProxy{
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           dialer.DialContext,
			ForceAttemptHTTP2:     false,
			ResponseHeaderTimeout: 30 * time.Second,
			IdleConnTimeout:       90 * time.Second,
		},
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.SetXForwarded()
			pr.Out.Host = addr
			pr.Out.URL.Path = upstreamPath
			pr.Out.URL.RawPath = ""
			pr.Out.URL.RawQuery = pr.In.URL.RawQuery
			pr.Out.Header.Del("Cookie")
			pr.Out.Header.Del("Authorization")
			pr.Out.Header.Set("X-Forwarded-Prefix", proxyBase)
		},
		ModifyResponse: func(resp *http.Response) error {
			resp.Header.Del("Set-Cookie")
			resp.Header.Set("Content-Security-Policy", "sandbox allow-scripts allow-forms allow-popups allow-downloads; default-src * data: blob: 'unsafe-inline' 'unsafe-eval'; connect-src * ws: wss: data: blob:; img-src * data: blob:; style-src * 'unsafe-inline'; script-src * 'unsafe-inline' 'unsafe-eval' blob:")
			rewriteProxyLocation(resp, addr, proxyBase)
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			s.logger.Warn("vm proxy failed", "vm", name, "port", port, "addr", addr, "err", err)
			writeError(w, http.StatusBadGateway, "proxy target unavailable")
		},
	}
	proxy.ServeHTTP(w, r)
}

func (s *Server) proxyRuleResponse(r *http.Request, rule config.ProxyRule, vmByName map[string]multipass.VMInfo, vmErr error, now time.Time) proxyRuleResponse {
	protocol := proxyRuleProtocol(rule)
	resp := proxyRuleResponse{
		ID:          rule.ID,
		VM:          rule.VM,
		Port:        rule.Port,
		HostPort:    rule.HostPort,
		Label:       rule.Label,
		Protocol:    proxyRuleProtocolLabel(protocol),
		Enabled:     rule.Enabled,
		ExpiresAt:   rule.ExpiresAt,
		CreatedAt:   rule.CreatedAt,
		Destination: fmt.Sprintf("%s:%d", rule.VM, rule.Port),
	}
	if protocol == "ssh" {
		resp.ListenAddress = s.sshListenAddressForRule(r, rule)
		resp.SSHCommand = s.sshCommandForRule(r, rule)
	} else {
		resp.ProxyURL = s.proxyURLForRule(r, rule)
	}
	if !rule.Enabled {
		resp.Status = "disabled"
		resp.StatusDetail = "rule is disabled"
		return resp
	}
	if proxyRuleExpired(rule, now) {
		resp.Status = "expired"
		resp.StatusDetail = "rule has expired"
		return resp
	}
	if vmErr != nil {
		resp.Status = "unknown"
		resp.StatusDetail = vmErr.Error()
		return resp
	}
	vm, ok := vmByName[rule.VM]
	if vmByName == nil {
		vm, ok = s.getVMInfoNoResponse(rule.VM)
	}
	if !ok {
		resp.Status = "vm_missing"
		resp.StatusDetail = "vm not found"
		return resp
	}
	resp.VMState = vm.State
	if ip, ok := firstUsableIPv4(vm); ok {
		resp.Target = net.JoinHostPort(ip.String(), strconv.Itoa(rule.Port))
	}
	if vm.State != "Running" {
		resp.Status = "vm_stopped"
		resp.StatusDetail = "vm is not running"
		return resp
	}
	if resp.Target == "" {
		resp.Status = "no_ip"
		resp.StatusDetail = "vm has no usable IPv4 address"
		return resp
	}
	if protocol == "ssh" {
		if listening, detail := s.tcpForwards.status(rule.ID); !listening {
			resp.Status = "listen_error"
			resp.StatusDetail = detail
			return resp
		}
	}
	resp.Status = "live"
	return resp
}

func (s *Server) getVMInfoNoResponse(name string) (multipass.VMInfo, bool) {
	vm, err := s.mp.GetVMInfo(name)
	return vm, err == nil
}

func (s *Server) proxyRuleAccess(name string, port int, now time.Time) (config.ProxyRule, string) {
	s.cfgMu.Lock()
	defer s.cfgMu.Unlock()
	for _, rule := range s.cfg.GetProxyRules() {
		if proxyRuleProtocol(rule) != "http" || rule.VM != name || rule.Port != port {
			continue
		}
		if !rule.Enabled {
			return rule, "disabled"
		}
		if proxyRuleExpired(rule, now) {
			return rule, "expired"
		}
		return rule, "allowed"
	}
	return config.ProxyRule{}, "not_found"
}

func (s *Server) proxyAddressForVM(w http.ResponseWriter, name string, port int) (string, bool) {
	vm, err := s.mp.GetVMInfo(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return "", false
	}
	if vm.State != "Running" {
		writeError(w, http.StatusConflict, "vm must be running to use proxy")
		return "", false
	}
	ip, ok := firstUsableIPv4(vm)
	if !ok {
		writeError(w, http.StatusConflict, "vm has no usable IPv4 address for proxy")
		return "", false
	}
	return net.JoinHostPort(ip.String(), strconv.Itoa(port)), true
}

func parseVMProxyPath(path string) (string, int, string, bool) {
	if !strings.HasPrefix(path, vmProxyPrefix) {
		return "", 0, "", false
	}
	rest := strings.TrimPrefix(path, vmProxyPrefix)
	parts := strings.SplitN(rest, "/", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", 0, "", false
	}
	port, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", 0, "", false
	}
	upstreamPath := "/"
	if len(parts) == 3 {
		upstreamPath += parts[2]
	}
	return parts[0], port, upstreamPath, true
}

func validProxyPort(port int) bool {
	return port >= minProxyPort && port <= maxProxyPort
}

func newProxyRuleID() (string, error) {
	idBytes := make([]byte, 8)
	if _, err := rand.Read(idBytes); err != nil {
		return "", err
	}
	return "px_" + hex.EncodeToString(idBytes), nil
}

func proxyRuleExpired(rule config.ProxyRule, now time.Time) bool {
	if rule.ExpiresAt == "" {
		return false
	}
	expiresAt, err := time.Parse(time.RFC3339, rule.ExpiresAt)
	return err != nil || !now.Before(expiresAt)
}

func (s *Server) proxyURLForRule(r *http.Request, rule config.ProxyRule) string {
	scheme := "http"
	if isTLS(r, s.cfg.TrustProxy) {
		scheme = "https"
	}
	host := s.externalHostForRequest(r)
	return fmt.Sprintf("%s://%s%s", scheme, host, s.proxyBasePath(rule.VM, rule.Port))
}

func (s *Server) sshListenAddressForRule(r *http.Request, rule config.ProxyRule) string {
	host := externalHostname(s.externalHostForRequest(r))
	return net.JoinHostPort(host, strconv.Itoa(rule.HostPort))
}

func (s *Server) sshCommandForRule(r *http.Request, rule config.ProxyRule) string {
	host := sshCommandHost(externalHostname(s.externalHostForRequest(r)))
	return fmt.Sprintf("ssh -p %d ubuntu@%s", rule.HostPort, host)
}

func (s *Server) externalHostForRequest(r *http.Request) string {
	host := r.Host
	if s.cfg != nil && s.cfg.TrustProxy {
		if forwardedHost := firstForwardedValue(r.Header.Get("X-Forwarded-Host")); forwardedHost != "" {
			host = forwardedHost
		}
	}
	if host == "" {
		return "localhost"
	}
	return host
}

func externalHostname(host string) string {
	if host == "" {
		return "localhost"
	}
	if hostname, _, err := net.SplitHostPort(host); err == nil {
		return strings.Trim(hostname, "[]")
	}
	if strings.Count(host, ":") == 1 {
		return strings.Split(host, ":")[0]
	}
	return strings.Trim(host, "[]")
}

func sshCommandHost(host string) string {
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		return "[" + host + "]"
	}
	return host
}

func proxyRuleProtocolLabel(protocol string) string {
	if protocol == "ssh" {
		return "SSH"
	}
	return "HTTP/WS"
}

func proxyRuleEventDetail(rule config.ProxyRule) string {
	if proxyRuleProtocol(rule) == "ssh" {
		return fmt.Sprintf("protocol=ssh port=%d host_port=%d", rule.Port, rule.HostPort)
	}
	return fmt.Sprintf("protocol=http port=%d", rule.Port)
}

func (s *Server) proxyBasePath(name string, port int) string {
	return fmt.Sprintf("%s%s/%d/", vmProxyPrefix, url.PathEscape(name), port)
}

func firstForwardedValue(value string) string {
	if value == "" {
		return ""
	}
	parts := strings.Split(value, ",")
	return strings.TrimSpace(parts[0])
}

func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket") && strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")
}

func rewriteProxyLocation(resp *http.Response, targetHost, proxyBase string) {
	location := resp.Header.Get("Location")
	if location == "" {
		return
	}
	if strings.HasPrefix(location, "/") {
		resp.Header.Set("Location", strings.TrimRight(proxyBase, "/")+location)
		return
	}
	locURL, err := url.Parse(location)
	if err != nil || locURL.Host != targetHost {
		return
	}
	locURL.Scheme = ""
	locURL.Host = ""
	if locURL.Path == "" {
		locURL.Path = "/"
	}
	resp.Header.Set("Location", strings.TrimRight(proxyBase, "/")+locURL.RequestURI())
}
