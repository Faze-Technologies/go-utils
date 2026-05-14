package geoip

import (
	"net"
	"net/http"
	"strings"
)

func normalizeIP(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "::ffff:")
	return s
}

// ResolveClientIP returns the leftmost IP from X-Forwarded-For, falling back
// to the socket peer. GCP's L7 load balancer strips client-supplied XFF and
// writes its own, so the leftmost entry is the real client. If you ever move
// off GCP (e.g. add Cloudflare), revisit this.
func ResolveClientIP(h http.Header, remoteAddr string) string {
	if xff := h.Get("X-Forwarded-For"); xff != "" {
		first := normalizeIP(strings.Split(xff, ",")[0])
		if ip := net.ParseIP(first); ip != nil {
			return ip.String()
		}
	}
	host := remoteAddr
	if h, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = h
	}
	if ip := net.ParseIP(normalizeIP(host)); ip != nil {
		return ip.String()
	}
	return ""
}
