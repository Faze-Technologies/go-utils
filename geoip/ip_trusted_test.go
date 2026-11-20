package geoip

import (
	"net/http"
	"testing"
)

func TestResolveClientIPUsesLoadBalancerEntry(t *testing.T) {
	cases := []struct {
		name, xff, remote, want string
	}{
		{"normal user: client, lb", "198.51.100.5, 203.0.113.10", "10.0.0.1:1234", "198.51.100.5"},
		{"spoofed: fake, client, lb", "8.8.8.8, 198.51.100.5, 203.0.113.10", "10.0.0.1:1234", "198.51.100.5"},
		{"spoofed twice", "1.1.1.1, 8.8.8.8, 198.51.100.5, 203.0.113.10", "10.0.0.1:1234", "198.51.100.5"},
		{"single entry (local or in-cluster)", "10.0.0.7", "10.0.0.1:1234", "10.0.0.7"},
		{"no header uses the socket peer", "", "10.1.2.3:5555", "10.1.2.3"},
		{"garbage falls back to the socket peer", "not-an-ip, also-bad", "10.1.2.3:5555", "10.1.2.3"},
	}
	for _, c := range cases {
		h := http.Header{}
		if c.xff != "" {
			h.Set("X-Forwarded-For", c.xff)
		}
		if got := ResolveClientIP(h, c.remote); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
