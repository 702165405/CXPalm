package server

import (
	"net"
	"net/http"
	"net/netip"
)

// A dedicated proxy header is trusted only with explicit local configuration,
// and only over the loopback tunnel. The public proxy must overwrite this header.
func (s *Server) clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return ""
	}
	ip, err := netip.ParseAddr(host)
	if err != nil || ip.Zone() != "" {
		return ""
	}
	if s.Config.TrustedProxyIP && ip.IsLoopback() {
		values := r.Header.Values("X-CXPalm-Client-IP")
		if len(values) != 1 {
			return ""
		}
		ip, err = netip.ParseAddr(values[0])
		if err != nil || ip.Zone() != "" {
			return ""
		}
	}
	return ip.Unmap().String()
}
