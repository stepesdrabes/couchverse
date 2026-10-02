package server

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// clientIP rewrites r.RemoteAddr to the bare client address. Forwarding headers
// are honoured only when the direct peer is a configured trusted proxy, so a
// client cannot spoof X-Forwarded-For to dodge the login and join rate limits.
func clientIP(trusted []netip.Prefix) func(http.Handler) http.Handler {
	isTrusted := func(a netip.Addr) bool {
		for _, p := range trusted {
			if p.Contains(a) {
				return true
			}
		}
		return false
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			peer := parseAddr(r.RemoteAddr)
			client := peer
			if peer.IsValid() && isTrusted(peer) {
				client = forwardedClient(r.Header.Values("X-Forwarded-For"), isTrusted, peer)
			}
			if client.IsValid() {
				r.RemoteAddr = client.String()
			}
			next.ServeHTTP(w, r)
		})
	}
}

// forwardedClient walks X-Forwarded-For right to left: every hop appended by a
// trusted proxy is skipped and the first untrusted address is the client.
func forwardedClient(headers []string, isTrusted func(netip.Addr) bool, peer netip.Addr) netip.Addr {
	var hops []string
	for _, h := range headers {
		hops = append(hops, strings.Split(h, ",")...)
	}
	client := peer
	for i := len(hops) - 1; i >= 0; i-- {
		a := parseAddr(strings.TrimSpace(hops[i]))
		if !a.IsValid() {
			break
		}
		client = a
		if !isTrusted(a) {
			break
		}
	}
	return client
}

func parseAddr(s string) netip.Addr {
	if host, _, err := net.SplitHostPort(s); err == nil {
		s = host
	}
	a, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Addr{}
	}
	return a.Unmap()
}
