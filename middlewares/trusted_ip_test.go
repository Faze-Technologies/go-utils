package middlewares

import "testing"

func TestTrustedClientFromXFF(t *testing.T) {
	cases := []struct{ xff, want string }{
		{"198.51.100.5, 203.0.113.10", "198.51.100.5"},
		{"8.8.8.8, 198.51.100.5, 203.0.113.10", "198.51.100.5"},
		{"1.1.1.1, 8.8.8.8, 198.51.100.5, 203.0.113.10", "198.51.100.5"},
		{"10.0.0.7", "10.0.0.7"},
		{"", ""},
		{" , ", ""},
	}
	for _, c := range cases {
		if got := trustedClientFromXFF(c.xff); got != c.want {
			t.Errorf("xff %q: got %q, want %q", c.xff, got, c.want)
		}
	}
}
