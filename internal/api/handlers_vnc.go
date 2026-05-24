package api

import (
	"context"
	"net"
	"net/http"
	"net/http/httputil"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

const defaultNoVNCPort = "6080"

type vncConsoleResponse struct {
	Available bool   `json:"available"`
	URL       string `json:"url,omitempty"`
	Host      string `json:"host,omitempty"`
	Port      string `json:"port,omitempty"`
	Message   string `json:"message,omitempty"`
}

func (s *Server) handleGetVNCConsole(w http.ResponseWriter, r *http.Request) {
	name, ok := validVMName(w, r, "name")
	if !ok {
		return
	}

	host, message, ok := s.vncHostForVM(name)
	if !ok {
		writeJSON(w, http.StatusOK, vncConsoleResponse{Available: false, Port: defaultNoVNCPort, Message: message})
		return
	}

	available := canConnect(r.Context(), net.JoinHostPort(host, defaultNoVNCPort), 700*time.Millisecond)
	resp := vncConsoleResponse{
		Available: available,
		URL:       "/api/v1/vms/" + url.PathEscape(name) + "/vnc/proxy/vnc.html?autoconnect=true&resize=remote",
		Host:      host,
		Port:      defaultNoVNCPort,
	}
	if !available {
		resp.Message = "VM has an IP, but no noVNC service responded on port " + defaultNoVNCPort + ". Install/start noVNC inside the Ubuntu VM, or use a cloud-init profile that exposes it."
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleVNCProxy(w http.ResponseWriter, r *http.Request) {
	name, ok := validVMName(w, r, "name")
	if !ok {
		return
	}

	host, message, ok := s.vncHostForVM(name)
	if !ok {
		writeError(w, http.StatusBadGateway, message)
		return
	}

	target := &url.URL{Scheme: "http", Host: net.JoinHostPort(host, defaultNoVNCPort)}
	prefix := "/api/v1/vms/" + url.PathEscape(name) + "/vnc/proxy"
	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host
		req.URL.Path = strings.TrimPrefix(req.URL.Path, prefix)
		if req.URL.Path == "" {
			req.URL.Path = "/"
		}
	}
	proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
		writeError(rw, http.StatusBadGateway, "failed to reach noVNC on "+target.Host+": "+err.Error())
	}
	proxy.ServeHTTP(w, r)
}

func (s *Server) vncHostForVM(name string) (string, string, bool) {
	vm, err := s.mp.GetVMInfo(name)
	if err != nil {
		return "", err.Error(), false
	}
	if vm.State != "Running" {
		return "", "VM must be running before opening a graphical console", false
	}
	host, ok := firstSafeIPv4(vm.IPv4)
	if !ok {
		return "", "VM does not have a usable IPv4 address yet", false
	}
	return host, "", true
}

func firstSafeIPv4(values []string) (string, bool) {
	for _, raw := range values {
		candidate := strings.TrimSpace(raw)
		if candidate == "" || candidate == "--" || candidate == "N/A" {
			continue
		}
		if strings.Contains(candidate, "/") {
			if prefix, err := netip.ParsePrefix(candidate); err == nil {
				candidate = prefix.Addr().String()
			}
		}
		addr, err := netip.ParseAddr(candidate)
		if err != nil || !addr.Is4() {
			continue
		}
		if addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() {
			return addr.String(), true
		}
	}
	return "", false
}

func canConnect(ctx context.Context, address string, timeout time.Duration) bool {
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
