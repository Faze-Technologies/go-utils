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

// ResolveClientIP returns the client IP from X-Forwarded-For, falling back to the socket peer.
// GCP's external Application Load Balancer APPENDS "<client IP>, <load balancer IP>" to whatever the
// client sent, so the leftmost entry is client-controlled. The trusted client IP is the second-to-last
// entry; a single entry (local dev, in-cluster) is used as is.
// If you ever move off GCP (e.g. add Cloudflare), revisit this.
func ResolveClientIP(h http.Header, remoteAddr string) string {
	var xff string

	for _, key := range []string{
		"X-Forwarded-For",
		"X-Forwarded-for",
		"X-forwarded-For",
		"X-forwarded-for",
		"x-Forwarded-For",
		"x-Forwarded-for",
		"x-forwarded-For",
		"x-forwarded-for",

		"X_Forwarded_For",
		"X_Forwarded_for",
		"X_forwarded_For",
		"X_forwarded_for",
		"x_Forwarded_For",
		"x_Forwarded_for",
		"x_forwarded_For",
		"x_forwarded_for",
	} {
		if xff = h.Get(key); xff != "" {
			break
		}
	}
	if xff != "" {
		var parts []string
		for _, p := range strings.Split(xff, ",") {
			if p = normalizeIP(p); p != "" {
				parts = append(parts, p)
			}
		}
		trusted := ""
		switch {
		case len(parts) >= 2:
			trusted = parts[len(parts)-2]
		case len(parts) == 1:
			trusted = parts[0]
		}
		if ip := net.ParseIP(trusted); ip != nil {
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
