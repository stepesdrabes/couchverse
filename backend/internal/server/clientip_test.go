package server

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestClientIP(t *testing.T) {
	proxies := []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}
	cases := []struct {
		name   string
		remote string
		xff    []string
		want   string
	}{
		{"direct client keeps its address without the port", "203.0.113.7:51234", nil, "203.0.113.7"},
		{"untrusted peer cannot spoof forwarding headers", "203.0.113.7:51234", []string{"1.2.3.4"}, "203.0.113.7"},
		{"trusted proxy forwards the client", "10.0.0.2:443", []string{"198.51.100.9"}, "198.51.100.9"},
		{"trusted proxy chain is skipped right to left", "10.0.0.2:443", []string{"6.6.6.6, 198.51.100.9, 10.0.0.3"}, "198.51.100.9"},
		{"split headers are joined in order", "10.0.0.2:443", []string{"6.6.6.6", "198.51.100.9"}, "198.51.100.9"},
		{"garbage hop stops the walk at the last good address", "10.0.0.2:443", []string{"198.51.100.9, nonsense"}, "10.0.0.2"},
		{"trusted proxy without a header is the client", "10.0.0.2:443", nil, "10.0.0.2"},
		{"ipv4-mapped ipv6 peer is unmapped", "[::ffff:10.0.0.2]:443", []string{"198.51.100.9"}, "198.51.100.9"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			h := clientIP(proxies)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { got = r.RemoteAddr }))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tc.remote
			for _, v := range tc.xff {
				req.Header.Add("X-Forwarded-For", v)
			}
			h.ServeHTTP(httptest.NewRecorder(), req)
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
