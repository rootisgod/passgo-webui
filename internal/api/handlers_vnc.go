package api

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"time"

	"github.com/coder/websocket"
	"github.com/rootisgod/passgo-web/pkg/multipass"
)

const (
	defaultVNCPort = 5900
	minVNCPort     = 5900
	maxVNCPort     = 5999
)

func (s *Server) handleVMVNC(w http.ResponseWriter, r *http.Request) {
	name, ok := validVMName(w, r, "name")
	if !ok {
		return
	}

	port, ok := validVNCPort(w, r)
	if !ok {
		return
	}

	addr, ok := s.vncAddressForVM(w, name, port)
	if !ok {
		return
	}

	clientWS, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: allowedOriginPatterns(r.Host),
		Subprotocols:   []string{"binary"},
	})
	if err != nil {
		s.logger.Error("vnc websocket accept failed", "vm", name, "err", err)
		return
	}
	defer clientWS.CloseNow()

	ctx := r.Context()
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	tcpConn, err := (&net.Dialer{}).DialContext(dialCtx, "tcp", addr)
	if err != nil {
		s.logger.Warn("vnc tcp dial failed", "vm", name, "addr", addr, "err", err)
		clientWS.Close(websocket.StatusInternalError, "failed to connect to VNC server")
		return
	}
	defer tcpConn.Close()

	done := make(chan struct{}, 2)

	go func() {
		defer func() { done <- struct{}{} }()
		buf := make([]byte, 32*1024)
		for {
			n, err := tcpConn.Read(buf)
			if n > 0 {
				if writeErr := clientWS.Write(ctx, websocket.MessageBinary, buf[:n]); writeErr != nil {
					return
				}
			}
			if err != nil {
				if err != io.EOF {
					s.logger.Debug("vnc tcp read ended", "vm", name, "err", err)
				}
				return
			}
		}
	}()

	go func() {
		defer func() { done <- struct{}{} }()
		for {
			messageType, data, err := clientWS.Read(ctx)
			if err != nil {
				return
			}
			if messageType != websocket.MessageBinary && messageType != websocket.MessageText {
				continue
			}
			if _, err := tcpConn.Write(data); err != nil {
				return
			}
		}
	}()

	<-done
	_ = tcpConn.Close()
	_ = clientWS.Close(websocket.StatusNormalClosure, "vnc session closed")
}

func validVNCPort(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw := r.URL.Query().Get("port")
	if raw == "" {
		return defaultVNCPort, true
	}
	port, err := strconv.Atoi(raw)
	if err != nil || port < minVNCPort || port > maxVNCPort {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("vnc port must be between %d and %d", minVNCPort, maxVNCPort))
		return 0, false
	}
	return port, true
}

func (s *Server) vncAddressForVM(w http.ResponseWriter, name string, port int) (string, bool) {
	vm, err := s.mp.GetVMInfo(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return "", false
	}
	if vm.State != "Running" {
		writeError(w, http.StatusConflict, "vm must be running to access graphics")
		return "", false
	}

	ip, ok := firstUsableIPv4(vm)
	if !ok {
		writeError(w, http.StatusConflict, "vm has no usable IPv4 address for VNC")
		return "", false
	}
	return net.JoinHostPort(ip.String(), strconv.Itoa(port)), true
}

func firstUsableIPv4(vm multipass.VMInfo) (netip.Addr, bool) {
	for _, candidate := range vm.IPv4 {
		ip, err := netip.ParseAddr(candidate)
		if err != nil || !ip.Is4() {
			continue
		}
		if ip.IsUnspecified() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsMulticast() {
			continue
		}
		return ip, true
	}
	return netip.Addr{}, false
}
